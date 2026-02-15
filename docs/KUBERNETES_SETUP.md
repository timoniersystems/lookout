# Kubernetes Setup with Kind and ArgoCD

> **⚠️ NOTE:** This guide contains ArgoCD setup instructions. ArgoCD is not currently deployed in the cluster due to resource constraints. For current deployment instructions, see [KIND_DEPLOYMENT.md](KIND_DEPLOYMENT.md).

Complete guide for the Kind cluster running on EC2 with Envoy Gateway and ArgoCD for GitOps.

## Environment Overview

**EC2 Instance:** `<EC2_INSTANCE_IP>`
- OS: Ubuntu 24.04 (Linux 6.14.0-1018-aws)
- RAM: 7.6GB
- Disk: 234GB available
- Docker: 29.2.1

## Architecture

```
┌──────────────────────────────────────────────────────────┐
│  EC2 Instance (<EC2_INSTANCE_IP>)                               │
│                                                          │
│  ┌────────────────────────────────────────────────────┐ │
│  │ Docker Compose (existing - runs in parallel)       │ │
│  │  - Production Lookout: https://<EC2_INSTANCE_IP>:7443     │ │
│  │  - Dgraph: 8080, 9080                              │ │
│  └────────────────────────────────────────────────────┘ │
│                                                          │
│  ┌────────────────────────────────────────────────────┐ │
│  │ Kind Cluster: lookout                              │ │
│  │                                                      │ │
│  │  Namespaces:                                       │ │
│  │  ├─ production   (port 80/443)                     │ │
│  │  ├─ staging      (port 10080/10443)                │ │
│  │  └─ argocd       (GitOps controller)               │ │
│  │                                                      │ │
│  │  Gateway API (Kubernetes standard):                │ │
│  │  └─ Envoy Gateway (replaces deprecated nginx)      │ │
│  └────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────┘
```

## Port Mappings

| Service | Environment | Kind Port | Host Port | URL |
|---------|------------|-----------|-----------|-----|
| HTTP | Production | 80 | 80 | http://<EC2_INSTANCE_IP> |
| HTTPS | Production | 443 | 443 | https://<EC2_INSTANCE_IP> |
| HTTP | Staging | 8080 | 10080 | http://<EC2_INSTANCE_IP>:10080 |
| HTTPS | Staging | 8443 | 10443 | https://<EC2_INSTANCE_IP>:10443 |
| ArgoCD | - | - | - | Port-forward required |

**Existing Docker Compose ports:**
- HTTP: 7080 → HTTPS: 7443 (production via docker-compose)
- Dgraph: 8080 (HTTP API), 9080 (gRPC)
- Ratel: 8000

## Installed Components

### 1. Kind v0.27.0
Kubernetes in Docker - lightweight K8s cluster for local/testing environments.

```bash
# Check cluster
ssh ubuntu@<EC2_INSTANCE_IP> 'kind get clusters'
# Expected: lookout
```

### 2. kubectl v1.35.1
Kubernetes CLI tool.

```bash
# Check version
ssh ubuntu@<EC2_INSTANCE_IP> 'kubectl version --client'
```

### 3. Gateway API v1.2.1
Official Kubernetes ingress replacement (future-proof, ingress-nginx is retiring March 2026).

```bash
# Check Gateway API CRDs
ssh ubuntu@<EC2_INSTANCE_IP> 'kubectl get crd | grep gateway'
```

### 4. Envoy Gateway v1.3.0
Modern ingress controller implementing Gateway API standard.

**GatewayClass:** `eg` (Envoy Gateway)

```bash
# Check Envoy Gateway
ssh ubuntu@<EC2_INSTANCE_IP> 'kubectl get gatewayclass'
ssh ubuntu@<EC2_INSTANCE_IP> 'kubectl get pods -n envoy-gateway-system'
```

### 5. ArgoCD (Latest Stable)
GitOps continuous delivery tool.

**Admin Credentials:**
- Username: `admin`
- Password: `8-YH8PRzc2BD6GtX`

```bash
# Access ArgoCD UI (port-forward from local machine)
ssh -L 8080:localhost:8080 ubuntu@<EC2_INSTANCE_IP> \
  'kubectl port-forward svc/argocd-server -n argocd 8080:443'
# Then open: https://localhost:8080
```

## Quick Start

### Access the Cluster

```bash
# SSH to EC2
ssh ubuntu@<EC2_INSTANCE_IP>

# Check cluster status
kubectl cluster-info
kubectl get nodes

# Check all namespaces
kubectl get ns

# Check gateways
kubectl get gateway -A
```

### Deploy to Production Namespace

Example HTTPRoute for production:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: lookout-app
  namespace: production
spec:
  parentRefs:
  - name: lookout-production
  hostnames:
  - "lookout.example.com"
  rules:
  - matches:
    - path:
        type: PathPrefix
        value: /
    backendRefs:
    - name: lookout-service
      port: 3000
```

### Deploy to Staging Namespace

Example HTTPRoute for staging:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: lookout-app
  namespace: staging
spec:
  parentRefs:
  - name: lookout-staging
  hostnames:
  - "staging.lookout.example.com"
  rules:
  - matches:
    - path:
        type: PathPrefix
        value: /
    backendRefs:
    - name: lookout-service
      port: 3000
```

## ArgoCD Setup for GitOps

### 1. Access ArgoCD UI

From your local machine:
```bash
ssh -L 8080:localhost:8080 ubuntu@<EC2_INSTANCE_IP> \
  'kubectl port-forward svc/argocd-server -n argocd 8080:443'
```

Open: https://localhost:8080
- Username: `admin`
- Password: `8-YH8PRzc2BD6GtX`

**IMPORTANT:** Change the password after first login!

### 2. Create ArgoCD Application

Create an Application to deploy Lookout from your Git repository:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: lookout-production
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/timoniersystems/lookout.git
    targetRevision: main
    path: k8s/production  # You'll create this directory
  destination:
    server: https://kubernetes.default.svc
    namespace: production
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
    - CreateNamespace=false
```

### 3. Recommended Repository Structure

```
lookout/
├── k8s/
│   ├── base/
│   │   ├── deployment.yaml      # Lookout app deployment
│   │   ├── service.yaml         # Kubernetes service
│   │   └── kustomization.yaml
│   ├── production/
│   │   ├── gateway.yaml         # Production Gateway (already created)
│   │   ├── httproute.yaml       # HTTPRoute for routing
│   │   ├── secrets.yaml         # TLS certificates (encrypted with sealed-secrets)
│   │   └── kustomization.yaml
│   └── staging/
│       ├── gateway.yaml         # Staging Gateway (already created)
│       ├── httproute.yaml
│       ├── secrets.yaml
│       └── kustomization.yaml
```

## Next Steps

### 1. Create Kubernetes Manifests

Convert your Docker Compose setup to Kubernetes manifests:

```bash
# Create k8s directory structure
mkdir -p k8s/{base,production,staging}

# Create base deployment for Lookout app
cat > k8s/base/deployment.yaml << 'EOF'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: lookout-app
  labels:
    app: lookout
spec:
  replicas: 2
  selector:
    matchLabels:
      app: lookout
  template:
    metadata:
      labels:
        app: lookout
    spec:
      containers:
      - name: lookout
        image: ghcr.io/timoniersystems/lookout:latest
        ports:
        - containerPort: 3000
        env:
        - name: DGRAPH_HOST
          value: dgraph-alpha
        - name: DGRAPH_PORT
          value: "9080"
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
EOF
```

### 2. Set Up TLS Certificates

Generate self-signed certificates for testing or use cert-manager for Let's Encrypt:

```bash
# Generate self-signed cert for staging
ssh ubuntu@<EC2_INSTANCE_IP> 'openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout /tmp/staging.key -out /tmp/staging.crt \
  -subj "/CN=staging.lookout.local"

# Create Kubernetes TLS secret
kubectl create secret tls lookout-tls \
  --cert=/tmp/staging.crt --key=/tmp/staging.key \
  -n staging'
```

### 3. Install Sealed Secrets (for GitOps)

Store encrypted secrets in Git safely:

```bash
ssh ubuntu@<EC2_INSTANCE_IP> 'kubectl apply -f \
  https://github.com/bitnami-labs/sealed-secrets/releases/download/v0.24.0/controller.yaml'

# Install kubeseal CLI locally
brew install kubeseal  # macOS
# or download from https://github.com/bitnami-labs/sealed-secrets/releases
```

### 4. Install Cert-Manager (Optional)

Automatic Let's Encrypt certificates:

```bash
ssh ubuntu@<EC2_INSTANCE_IP> 'kubectl apply -f \
  https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml'
```

### 5. Set Up Dgraph in Kubernetes

Convert Dgraph to StatefulSet for production-ready deployment:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: dgraph-zero
  namespace: production
spec:
  ports:
  - port: 5080
    name: grpc
  - port: 6080
    name: http
  clusterIP: None
  selector:
    app: dgraph-zero
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: dgraph-zero
  namespace: production
spec:
  serviceName: dgraph-zero
  replicas: 1
  selector:
    matchLabels:
      app: dgraph-zero
  template:
    metadata:
      labels:
        app: dgraph-zero
    spec:
      containers:
      - name: zero
        image: dgraph/dgraph:v25.0.0
        ports:
        - containerPort: 5080
        - containerPort: 6080
        command:
        - dgraph
        - zero
        - --my=dgraph-zero-0.dgraph-zero:5080
        volumeMounts:
        - name: data
          mountPath: /dgraph
  volumeClaimTemplates:
  - metadata:
      name: data
    spec:
      accessModes: [ "ReadWriteOnce" ]
      resources:
        requests:
          storage: 10Gi
```

## Troubleshooting

### Check Cluster Status

```bash
ssh ubuntu@<EC2_INSTANCE_IP> '
  echo "=== Cluster Info ===" && kubectl cluster-info
  echo && echo "=== Nodes ===" && kubectl get nodes
  echo && echo "=== Namespaces ===" && kubectl get ns
  echo && echo "=== Gateways ===" && kubectl get gateway -A
  echo && echo "=== Envoy Pods ===" && kubectl get pods -n envoy-gateway-system
  echo && echo "=== ArgoCD Status ===" && kubectl get pods -n argocd
'
```

### View Gateway Status

```bash
ssh ubuntu@<EC2_INSTANCE_IP> 'kubectl get gateway lookout-production -n production -o yaml'
```

### Check Envoy Proxy Logs

```bash
# Production
ssh ubuntu@<EC2_INSTANCE_IP> 'kubectl logs -n envoy-gateway-system -l gateway.envoyproxy.io/owning-gateway-name=lookout-production'

# Staging
ssh ubuntu@<EC2_INSTANCE_IP> 'kubectl logs -n envoy-gateway-system -l gateway.envoyproxy.io/owning-gateway-name=lookout-staging'
```

### Restart Kind Cluster

```bash
ssh ubuntu@<EC2_INSTANCE_IP> '
  kind delete cluster --name lookout
  kind create cluster --config /tmp/kind-cluster-config-simple.yaml
'
```

## Common Commands

### Kubectl Context

```bash
# Current context
kubectl config current-context

# Switch context (if multiple clusters)
kubectl config use-context kind-lookout
```

### Port Forwarding

```bash
# Forward ArgoCD UI
kubectl port-forward svc/argocd-server -n argocd 8080:443

# Forward Dgraph Ratel (if deployed in cluster)
kubectl port-forward svc/dgraph-ratel -n production 8000:8000
```

### Apply Manifests

```bash
# Apply a single file
kubectl apply -f k8s/production/httproute.yaml

# Apply directory
kubectl apply -f k8s/production/

# Apply with Kustomize
kubectl apply -k k8s/production/
```

### View Logs

```bash
# All pods in namespace
kubectl logs -n production -l app=lookout --tail=100 -f

# Specific pod
kubectl logs -n production lookout-app-xxx-yyy -f
```

## References

- [Kind Documentation](https://kind.sigs.k8s.io/)
- [Gateway API Docs](https://gateway-api.sigs.k8s.io/)
- [Envoy Gateway](https://gateway.envoyproxy.io/)
- [ArgoCD](https://argo-cd.readthedocs.io/)
- [Ingress NGINX Retirement Notice](https://kubernetes.io/blog/2025/11/11/ingress-nginx-retirement/)

## Security Notes

1. **Change ArgoCD admin password immediately**
2. **Use Sealed Secrets or External Secrets Operator for sensitive data**
3. **Enable RBAC for ArgoCD projects**
4. **Use cert-manager for production TLS certificates**
5. **Set resource limits on all deployments**
6. **Enable network policies for namespace isolation**

---

**Setup Date:** 2026-02-15
**Cluster:** lookout (Kind v0.27.0, K8s v1.32.2)
**Location:** EC2 instance <EC2_INSTANCE_IP>
