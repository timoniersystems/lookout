# Kubernetes Quick Reference

Fast reference for common operations on the Kind cluster.

## Access

```bash
# SSH to EC2
ssh ubuntu@10.0.3.142

# Set kubeconfig context
export KUBECONFIG=~/.kube/config
kubectl config use-context kind-lookout
```

## ArgoCD Access

**Admin Credentials:**
- Username: `admin`
- Initial Password: `8-YH8PRzc2BD6GtX` ⚠️ **CHANGE IMMEDIATELY**

**Access UI:**
```bash
# From your local machine
ssh -L 8080:localhost:8080 ubuntu@10.0.3.142 \
  'kubectl port-forward svc/argocd-server -n argocd 8080:443'

# Open browser to: https://localhost:8080
```

**Change Password:**
```bash
# Port-forward first, then:
argocd login localhost:8080
argocd account update-password
```

## Cluster Status

```bash
# One-liner health check
kubectl get nodes,gateways -A,pods -n envoy-gateway-system,pods -n argocd

# Detailed status
kubectl cluster-info
kubectl get gateway -A
kubectl get httproute -A
```

## Deploy Application

### 1. Create HTTPRoute for Production

```bash
cat <<EOF | kubectl apply -f -
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: lookout-app
  namespace: production
spec:
  parentRefs:
  - name: lookout-production
  rules:
  - matches:
    - path:
        type: PathPrefix
        value: /
    backendRefs:
    - name: lookout-service
      port: 3000
EOF
```

### 2. Create Deployment

```bash
cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: lookout-app
  namespace: production
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
---
apiVersion: v1
kind: Service
metadata:
  name: lookout-service
  namespace: production
spec:
  selector:
    app: lookout
  ports:
  - port: 3000
    targetPort: 3000
EOF
```

## Port Mappings

| Environment | HTTP | HTTPS | Access |
|-------------|------|-------|--------|
| Production  | 80   | 443   | http://10.0.3.142 |
| Staging     | 10080| 10443 | http://10.0.3.142:10080 |
| Docker-Compose | 7080 | 7443 | https://10.0.3.142:7443 |

## Common Tasks

### View Logs
```bash
# Application logs
kubectl logs -n production -l app=lookout --tail=100 -f

# Envoy Gateway logs
kubectl logs -n envoy-gateway-system -l gateway.envoyproxy.io/owning-gateway-name=lookout-production --tail=100 -f

# ArgoCD logs
kubectl logs -n argocd -l app.kubernetes.io/name=argocd-server --tail=100 -f
```

### Debug Pod
```bash
# Get pod name
kubectl get pods -n production

# Exec into pod
kubectl exec -it -n production lookout-app-xxx-yyy -- sh

# Describe pod
kubectl describe pod -n production lookout-app-xxx-yyy
```

### Scale Deployment
```bash
# Scale up
kubectl scale deployment lookout-app -n production --replicas=3

# Scale down
kubectl scale deployment lookout-app -n production --replicas=1
```

### Update Image
```bash
# Update to new image
kubectl set image deployment/lookout-app -n production \
  lookout=ghcr.io/timoniersystems/lookout:v1.2.3

# Check rollout status
kubectl rollout status deployment/lookout-app -n production

# Rollback if needed
kubectl rollout undo deployment/lookout-app -n production
```

### Secrets Management
```bash
# Create TLS secret
kubectl create secret tls lookout-tls \
  --cert=path/to/cert.pem \
  --key=path/to/key.pem \
  -n production

# Create generic secret
kubectl create secret generic app-secrets \
  --from-literal=NVD_API_KEY=your-key-here \
  -n production

# View secrets (base64 encoded)
kubectl get secret lookout-tls -n production -o yaml
```

## Troubleshooting

### Gateway Not Programmed
```bash
# Check gateway status
kubectl describe gateway lookout-production -n production

# Check TLS secret exists
kubectl get secret lookout-tls -n production

# Check Envoy proxy pods
kubectl get pods -n envoy-gateway-system
```

### Application Not Accessible
```bash
# Check HTTPRoute
kubectl describe httproute lookout-app -n production

# Check service endpoints
kubectl get endpoints lookout-service -n production

# Check if pods are ready
kubectl get pods -n production -l app=lookout
```

### ArgoCD App Not Syncing
```bash
# Check application status
kubectl get application lookout-production -n argocd

# View sync status
kubectl describe application lookout-production -n argocd

# Manually sync
kubectl patch application lookout-production -n argocd \
  --type merge -p '{"operation": {"sync": {}}}'
```

## Useful Aliases

Add to your `~/.bashrc` or `~/.zshrc`:

```bash
alias k='kubectl'
alias kgp='kubectl get pods'
alias kgs='kubectl get svc'
alias kgn='kubectl get nodes'
alias kga='kubectl get all -A'
alias kl='kubectl logs'
alias kx='kubectl exec -it'
alias kd='kubectl describe'
alias kns='kubectl config set-context --current --namespace'
```

## Emergency Commands

### Restart All Pods in Namespace
```bash
kubectl rollout restart deployment -n production
```

### Delete Failed Pods
```bash
kubectl delete pods --field-selector status.phase=Failed -n production
```

### Force Delete Stuck Pod
```bash
kubectl delete pod <pod-name> -n production --force --grace-period=0
```

### Restart Kind Cluster
```bash
kind delete cluster --name lookout
kind create cluster --config /tmp/kind-cluster-config-simple.yaml

# Then re-install components (see KUBERNETES_SETUP.md)
```

## Health Checks

### Quick Health Check Script
```bash
#!/bin/bash
echo "=== Cluster ==="
kubectl get nodes
echo ""
echo "=== Gateways ==="
kubectl get gateway -A
echo ""
echo "=== Deployments ==="
kubectl get deployments -A
echo ""
echo "=== Services ==="
kubectl get svc -A | grep -E "production|staging"
```

Save as `~/check-k8s.sh` and run: `bash ~/check-k8s.sh`

## Next Steps

1. **Create Git repository structure** for K8s manifests
2. **Set up ArgoCD Applications** for automated deployments
3. **Configure TLS certificates** (cert-manager or manual)
4. **Deploy Dgraph** as StatefulSet
5. **Set up monitoring** (Prometheus + Grafana)
6. **Configure backups** for Dgraph data

See [KUBERNETES_SETUP.md](KUBERNETES_SETUP.md) for detailed instructions.

---

**Quick Links:**
- [Full Setup Guide](KUBERNETES_SETUP.md)
- [Gateway API Docs](https://gateway-api.sigs.k8s.io/)
- [ArgoCD Docs](https://argo-cd.readthedocs.io/)
