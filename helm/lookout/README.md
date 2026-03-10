# Lookout Helm Chart

Helm chart for deploying Lookout CVE vulnerability analysis tool with Dgraph on Kubernetes using Gateway API.

## Prerequisites

- Kubernetes 1.26+
- Helm 3.8+
- Gateway API v1.2.1+ CRDs installed
- Envoy Gateway (or another Gateway API controller) installed
- PersistentVolume provisioner support (for Dgraph persistence)

## Installing Gateway API and Envoy Gateway

If not already installed:

```bash
# Install Gateway API CRDs
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.2.1/standard-install.yaml

# Install Envoy Gateway
kubectl apply -f https://github.com/envoyproxy/gateway/releases/download/v1.3.0/install.yaml

# Create GatewayClass
kubectl apply -f - <<EOF
apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata:
  name: eg
spec:
  controllerName: gateway.envoyproxy.io/gatewayclass-controller
EOF
```

## Installing the Chart

### Production Deployment

```bash
# Production is managed by ArgoCD with semver image updater
# Deploy the ArgoCD application:
kubectl apply -f k8s/argocd/production-application.yaml

# Prerequisites in production namespace:
kubectl create namespace production
kubectl create secret tls lookout-tls --cert=path/to/cert.pem --key=path/to/key.pem -n production
kubectl create secret docker-registry ghcr-secret --docker-server=ghcr.io ... -n production
kubectl create secret generic aws-credentials --from-literal=access-key-id=... --from-literal=secret-access-key=... -n production
```

### Staging Deployment

```bash
# Create namespace
kubectl create namespace staging

# Create TLS secret
kubectl create secret tls lookout-tls \
  --cert=path/to/staging-cert.pem \
  --key=path/to/staging-key.pem \
  -n staging

# Install chart
helm install lookout ./helm/lookout \
  --namespace staging \
  --values helm/lookout/values.staging.yaml
```

## Configuration

### Key Values

| Parameter | Description | Default |
|-----------|-------------|---------|
| `global.environment` | Environment name | `production` |
| `global.gatewayAPI.gatewayClassName` | Gateway API class | `eg` |
| `gateway.enabled` | Enable Gateway resource | `true` |
| `httproute.hostnames` | Domain names | `["lookout.example.com"]` |
| `lookout-app.replicaCount` | Number of app replicas | `2` |
| `lookout-app.image.repository` | App image | `ghcr.io/timoniersystems/lookout` |
| `lookout-app.image.tag` | App image tag | `latest` |
| `lookout-app.secrets.NVD_API_KEY` | NVD API key | `""` |
| `dgraph.zero.replicaCount` | Dgraph zero replicas | `1` |
| `dgraph.alpha.replicaCount` | Dgraph alpha replicas | `1` |
| `dgraph.alpha.persistence.size` | Dgraph data volume size | `50Gi` |
| `gateway.allowRoutesFromAllNamespaces` | Allow cross-namespace HTTPRoutes | `false` |
| `httproute.gatewayNamespace` | Gateway namespace for cross-namespace ref | `""` |
| `basicAuth.enabled` | Enable HTTP basic auth | `false` |
| `basicAuth.externalSecret.enabled` | Use ESO for basic auth credentials | `false` |
| `lookout-app.trivyCache.persistence.enabled` | Enable persistent Trivy DB cache | `false` |
| `lookout-app.trivyCache.persistence.size` | Trivy cache PVC size | `2Gi` |

### Full Values Override

Create a custom `values.yaml`:

```yaml
httproute:
  hostnames:
    - "lookout.yourdomain.com"

lookout-app:
  replicaCount: 3
  image:
    tag: "v1.2.3"
  resources:
    limits:
      cpu: 1000m
      memory: 1Gi
  secrets:
    NVD_API_KEY: "your-key-here"

dgraph:
  alpha:
    replicaCount: 3
    persistence:
      size: 100Gi
      storageClass: "fast-ssd"
```

Then install:

```bash
helm install lookout ./helm/lookout -f values.yaml -n production
```

### Staging vs Production

| Feature | Staging | Production |
|---------|---------|------------|
| Image tag | `main` (latest commit) | Semver tags (`v1.0.0`) |
| ArgoCD strategy | Digest (auto-deploy on push) | Semver (deploy on tag) |
| Gateway | Owns the shared `lookout` gateway | No gateway (cross-namespace HTTPRoute) |
| Hostnames | `lookout-stg.timonier.io` | `lookout-prod.timonier.io`, `lookout.timonier.io` |
| Basic auth | Enabled (username: staging) | Enabled (username: production) |
| Replicas | 1 (Kind cluster) | 1 (Kind cluster) |
| Image pull | Always | IfNotPresent |
| Trivy cache | PVC-backed (2Gi) | PVC-backed (2Gi) |

## Upgrading the Chart

```bash
helm upgrade lookout ./helm/lookout \
  --namespace production \
  --values helm/lookout/values.production.yaml
```

## Uninstalling the Chart

```bash
helm uninstall lookout --namespace production
```

**Note:** This does NOT delete PersistentVolumeClaims. Delete them manually if needed:

```bash
kubectl delete pvc -n production -l app.kubernetes.io/name=dgraph
```

## Accessing the Application

### Via Gateway (Production)

If DNS is configured:
```bash
https://lookout.yourdomain.com
```

### Port-Forward (Development/Testing)

```bash
# Lookout app
kubectl port-forward -n production svc/lookout-app 3000:3000

# Dgraph Ratel UI
kubectl port-forward -n production svc/dgraph-ratel 8000:8000

# Dgraph Alpha HTTP API
kubectl port-forward -n production svc/dgraph-alpha 8080:8080
```

## Monitoring

### Check Deployment Status

```bash
# All resources
kubectl get all -n production

# Gateway status
kubectl get gateway -n production

# HTTPRoute status
kubectl get httproute -n production

# Pod status
kubectl get pods -n production

# PVC status
kubectl get pvc -n production
```

### View Logs

```bash
# Lookout app logs
kubectl logs -n production -l app.kubernetes.io/name=lookout-app -f

# Dgraph alpha logs
kubectl logs -n production -l app.kubernetes.io/component=alpha -f

# Dgraph zero logs
kubectl logs -n production -l app.kubernetes.io/component=zero -f
```

## Persistence

Dgraph uses StatefulSets with PersistentVolumeClaims:

- **Zero:** 10Gi (metadata and cluster coordination)
- **Alpha:** 50Gi (graph data)

### Backup Dgraph Data

```bash
# Export data
kubectl exec -n production dgraph-alpha-0 -- \
  curl -X POST localhost:8080/admin/export

# Copy backup
kubectl cp production/dgraph-alpha-0:/dgraph/export ./dgraph-backup
```

### Restore Dgraph Data

```bash
# Copy backup to pod
kubectl cp ./dgraph-backup production/dgraph-alpha-0:/dgraph/import

# Import data
kubectl exec -n production dgraph-alpha-0 -- \
  dgraph live --files /dgraph/import --alpha localhost:9080
```

## Troubleshooting

### Gateway Not Ready

```bash
# Check Gateway status
kubectl describe gateway lookout -n production

# Check Envoy Gateway controller
kubectl get pods -n envoy-gateway-system

# Check TLS secret exists
kubectl get secret lookout-tls -n production
```

### Pod CrashLoopBackOff

```bash
# Check pod logs
kubectl logs -n production <pod-name>

# Describe pod for events
kubectl describe pod -n production <pod-name>

# Check resource constraints
kubectl top pods -n production
```

### Dgraph Alpha Can't Connect to Zero

```bash
# Check if zero is ready
kubectl get pods -n production -l app.kubernetes.io/component=zero

# Test zero connectivity from alpha pod
kubectl exec -n production dgraph-alpha-0 -- \
  nc -zv dgraph-zero-0.dgraph-zero 5080

# Check zero logs
kubectl logs -n production dgraph-zero-0
```

### Application Can't Connect to Dgraph

```bash
# Check if alpha is ready
kubectl get pods -n production -l app.kubernetes.io/component=alpha

# Test alpha connectivity from app pod
kubectl exec -n production <lookout-app-pod> -- \
  nc -zv dgraph-alpha 9080

# Check environment variables
kubectl exec -n production <lookout-app-pod> -- env | grep DGRAPH
```

## Production Recommendations

### High Availability

For production, increase replicas:

```yaml
lookout-app:
  replicaCount: 3

dgraph:
  zero:
    replicaCount: 3
  alpha:
    replicaCount: 3
```

### Resource Limits

Set appropriate resource limits based on load:

```yaml
lookout-app:
  resources:
    limits:
      cpu: 2000m
      memory: 4Gi
    requests:
      cpu: 1000m
      memory: 2Gi

dgraph:
  alpha:
    resources:
      limits:
        cpu: 4000m
        memory: 8Gi
      requests:
        cpu: 2000m
        memory: 4Gi
```

### Storage Class

Use fast storage for Dgraph:

```yaml
dgraph:
  alpha:
    persistence:
      storageClass: "ssd"
      size: 100Gi
```

### Secrets Management

Use Sealed Secrets or External Secrets Operator instead of plain secrets:

```bash
# Install sealed-secrets
kubectl apply -f https://github.com/bitnami-labs/sealed-secrets/releases/download/v0.24.0/controller.yaml

# Create sealed secret
kubectl create secret generic lookout-secrets \
  --from-literal=NVD_API_KEY="your-key" \
  --dry-run=client -o yaml | \
  kubeseal -o yaml > sealed-secret.yaml

kubectl apply -f sealed-secret.yaml -n production
```

### Monitoring and Alerting

Set up Prometheus monitoring:

```yaml
# ServiceMonitor for Prometheus Operator
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: lookout
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: lookout-app
  endpoints:
  - port: http
    path: /metrics
```

## ArgoCD Integration

Both staging and production environments are managed by ArgoCD Application manifests:

```yaml
# Staging Application (k8s/argocd/staging-application.yaml)
# - Watches main branch
# - Uses digest-based image updater
# - Deploys to staging namespace

# Production Application (k8s/argocd/production-application.yaml)
# - Watches main branch for Helm chart changes
# - Uses semver-based image updater (v*.*.*)
# - Deploys to production namespace
# - No Gateway resource (uses cross-namespace ref to staging gateway)
```

### Staging ArgoCD Application

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: lookout-staging
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/timoniersystems/lookout.git
    targetRevision: main
    path: helm/lookout
    helm:
      valueFiles:
        - values.staging.yaml
  destination:
    server: https://kubernetes.default.svc
    namespace: staging
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
      - CreateNamespace=false
```

### Production ArgoCD Application

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
    path: helm/lookout
    helm:
      valueFiles:
        - values.production.yaml
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

## License

See repository LICENSE file.

## Support

- GitHub Issues: https://github.com/timoniersystems/lookout/issues
- Documentation: https://github.com/timoniersystems/lookout/docs
