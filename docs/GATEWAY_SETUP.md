# Gateway API Setup with Envoy Gateway

This guide explains how to set up Kubernetes Gateway API with Envoy Gateway for the Lookout application, including TLS certificate configuration.

## Architecture

```
┌─────────────────────────────────────────────────┐
│  External Traffic (HTTP/HTTPS)                  │
└────────────────┬────────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────────┐
│  LoadBalancer / NodePort                        │
│  - HTTP: Port 8080 → NodePort 32497            │
│  - HTTPS: Port 8443 → NodePort 30387           │
└────────────────┬────────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────────┐
│  Envoy Gateway Proxy                            │
│  - TLS termination                              │
│  - Request routing                              │
│  - Load balancing                               │
└────────────────┬────────────────────────────────┘
                 │
                 ▼ HTTPRoute
┌─────────────────────────────────────────────────┐
│  Lookout Application Service                    │
│  - ClusterIP: 3000                              │
└─────────────────────────────────────────────────┘
```

## Prerequisites

- Kubernetes cluster (kind, EKS, GKE, etc.)
- kubectl configured
- Envoy Gateway installed
- openssl (for generating certificates)

## Components

### 1. GatewayClass

Defines the Envoy Gateway controller that will manage Gateway resources.

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata:
  name: eg
spec:
  controllerName: gateway.envoyproxy.io/gatewayclass-controller
```

### 2. Gateway

Configures HTTP and HTTPS listeners with TLS termination.

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: lookout-staging
  namespace: staging
spec:
  gatewayClassName: eg
  listeners:
  - name: http
    protocol: HTTP
    port: 8080
    allowedRoutes:
      namespaces:
        from: Same
  - name: https
    protocol: HTTPS
    port: 8443
    tls:
      mode: Terminate
      certificateRefs:
      - kind: Secret
        name: lookout-tls
    allowedRoutes:
      namespaces:
        from: Same
```

### 3. HTTPRoute

Routes traffic from the Gateway to backend services.

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: lookout
  namespace: staging
spec:
  parentRefs:
  - name: lookout-staging
  hostnames:
  - "lookout-stg.timonier.io"
  rules:
  - matches:
    - path:
        type: PathPrefix
        value: /
    backendRefs:
    - name: lookout-staging-lookout-app
      port: 3000
```

## Quick Setup

### Automated Setup (Recommended)

Run the setup script:

```bash
# Use defaults (staging namespace, lookout-stg.timonier.io)
./scripts/setup-gateway.sh

# Or customize
NAMESPACE=production DOMAIN=lookout.timonier.io ./scripts/setup-gateway.sh
```

This script will:
1. Create the GatewayClass
2. Generate a self-signed TLS certificate
3. Create the TLS secret in the specified namespace

### Manual Setup

#### Step 1: Create GatewayClass

```bash
kubectl apply -f - <<EOF
apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata:
  name: eg
spec:
  controllerName: gateway.envoyproxy.io/gatewayclass-controller
EOF
```

#### Step 2: Generate TLS Certificate

```bash
# Generate self-signed certificate
openssl req -x509 -newkey rsa:4096 \
  -keyout tls.key \
  -out tls.crt \
  -days 365 \
  -nodes \
  -subj "/CN=lookout-stg.timonier.io/O=Timonier Systems" \
  -addext "subjectAltName=DNS:lookout-stg.timonier.io,DNS:*.lookout-stg.timonier.io"

# Create Kubernetes secret
kubectl create secret tls lookout-tls \
  --cert=tls.crt \
  --key=tls.key \
  --namespace=staging

# Cleanup
rm tls.key tls.crt
```

#### Step 3: Deploy Gateway and HTTPRoute

These are deployed automatically via Helm when using ArgoCD:

```bash
# If deploying manually
kubectl apply -f helm/lookout/templates/gateway.yaml
kubectl apply -f helm/lookout/templates/httproute.yaml
```

## Verification

### Check GatewayClass

```bash
kubectl get gatewayclass
```

Expected output:
```
NAME   CONTROLLER                                      ACCEPTED   AGE
eg     gateway.envoyproxy.io/gatewayclass-controller   True       1m
```

### Check Gateway

```bash
kubectl get gateway -n staging
kubectl describe gateway lookout-staging -n staging
```

Expected status:
- **Accepted**: True
- **Programmed**: True (or False if no LoadBalancer IP assigned in kind)
- **HTTP Listener**: Programmed
- **HTTPS Listener**: Programmed, ResolvedRefs

### Check Envoy Service

```bash
kubectl get svc -n envoy-gateway-system
```

You should see a LoadBalancer service for your Gateway:
```
NAME                                     TYPE           CLUSTER-IP      EXTERNAL-IP   PORT(S)
envoy-staging-lookout-staging-e530b775   LoadBalancer   10.96.207.155   <pending>     8080:32497/TCP,8443:30387/TCP
```

### Check HTTPRoute

```bash
kubectl get httproute -n staging
kubectl describe httproute lookout -n staging
```

### Test Connectivity

#### In kind cluster (NodePort):

```bash
# HTTP
curl -H "Host: lookout-stg.timonier.io" http://localhost:32497/health

# HTTPS (self-signed cert)
curl -k -H "Host: lookout-stg.timonier.io" https://localhost:30387/health
```

#### On EC2 with kind:

```bash
# HTTP
curl -H "Host: lookout-stg.timonier.io" http://10.0.3.142:32497/health

# HTTPS
curl -k -H "Host: lookout-stg.timonier.io" https://10.0.3.142:30387/health
```

#### With actual DNS (EKS + ALB):

```bash
# HTTP
curl http://lookout-stg.timonier.io/health

# HTTPS
curl https://lookout-stg.timonier.io/health
```

## TLS Certificate Management

### Self-Signed Certificates (Development)

For local development and testing, use self-signed certificates:

```bash
./scripts/setup-gateway.sh
```

Browsers will show security warnings - this is expected.

### Production Certificates

For production, use one of these options:

#### Option 1: cert-manager with Let's Encrypt

Install cert-manager:
```bash
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml
```

Create ClusterIssuer:
```yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: admin@timonier.io
    privateKeySecretRef:
      name: letsencrypt-prod
    solvers:
    - http01:
        ingress:
          class: nginx
```

Create Certificate:
```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: lookout-tls
  namespace: staging
spec:
  secretName: lookout-tls
  issuerRef:
    name: letsencrypt-prod
    kind: ClusterIssuer
  dnsNames:
  - lookout-stg.timonier.io
```

#### Option 2: AWS Certificate Manager (ACM) with ALB

When using AWS Load Balancer Controller with ALB, you can use ACM certificates:

1. Create certificate in ACM for your domain
2. Annotate the Gateway service:
   ```yaml
   metadata:
     annotations:
       service.beta.kubernetes.io/aws-load-balancer-ssl-cert: arn:aws:acm:region:account:certificate/xxx
       service.beta.kubernetes.io/aws-load-balancer-backend-protocol: http
   ```

## Deployment Environments

### kind Cluster (Local/EC2)

- Service type: LoadBalancer (shows as Pending)
- Access via NodePort
- Self-signed certificates recommended
- Useful for development and testing

**Access:**
- HTTP: `http://<EC2-IP>:<NodePort>`
- HTTPS: `https://<EC2-IP>:<NodePort>`

### AWS EKS with ALB

- Install AWS Load Balancer Controller
- Service type: LoadBalancer (creates ALB)
- Use ACM certificates or cert-manager
- Production-ready

**Access:**
- HTTP: `http://<ALB-DNS-Name>`
- HTTPS: `https://<ALB-DNS-Name>`

### Google GKE with GCLB

- Service type: LoadBalancer (creates GCLB)
- Use Google-managed certificates or cert-manager
- Production-ready

## Troubleshooting

### Gateway Not Accepting Connections

1. **Check GatewayClass exists:**
   ```bash
   kubectl get gatewayclass eg
   ```

2. **Check Gateway status:**
   ```bash
   kubectl describe gateway lookout-staging -n staging
   ```

3. **Check Envoy pods:**
   ```bash
   kubectl get pods -n envoy-gateway-system
   ```

### HTTPS Listener Issues

1. **Check TLS secret exists:**
   ```bash
   kubectl get secret lookout-tls -n staging
   ```

2. **Verify secret has correct keys:**
   ```bash
   kubectl get secret lookout-tls -n staging -o jsonpath='{.data}' | jq 'keys'
   # Should show: ["tls.crt", "tls.key"]
   ```

3. **Check listener status:**
   ```bash
   kubectl get gateway lookout-staging -n staging -o jsonpath='{.status.listeners[?(@.name=="https")].conditions}' | jq .
   ```

### HTTPRoute Not Working

1. **Check if HTTPRoute is attached to Gateway:**
   ```bash
   kubectl get httproute lookout -n staging -o yaml | grep -A 5 parentRefs
   ```

2. **Verify hostname configuration:**
   ```bash
   kubectl get httproute lookout -n staging -o jsonpath='{.spec.hostnames}'
   ```

3. **Check backend service exists:**
   ```bash
   kubectl get svc lookout-staging-lookout-app -n staging
   ```

### No LoadBalancer IP (kind)

This is expected in kind clusters. Use NodePort instead:

```bash
kubectl get svc -n envoy-gateway-system | grep envoy-staging
```

### Connection Timeouts

1. **Verify backend pods are running:**
   ```bash
   kubectl get pods -n staging -l app.kubernetes.io/name=lookout-app
   ```

2. **Check Envoy logs:**
   ```bash
   kubectl logs -n envoy-gateway-system -l gateway.envoyproxy.io/owning-gateway-name=lookout-staging
   ```

3. **Test backend directly:**
   ```bash
   kubectl port-forward -n staging svc/lookout-staging-lookout-app 3000:3000
   curl http://localhost:3000/health
   ```

## Integration with ArgoCD

The Gateway and HTTPRoute resources are managed by ArgoCD when deployed via Helm:

1. Gateway is defined in `helm/lookout/templates/gateway.yaml`
2. HTTPRoute is defined in `helm/lookout/templates/httproute.yaml`
3. TLS secret must be created manually (or via cert-manager)
4. GatewayClass must exist before deploying

**Setup order:**
1. Run `./scripts/setup-gateway.sh` (creates GatewayClass and TLS secret)
2. Deploy via ArgoCD (creates Gateway and HTTPRoute)

## Configuration

### Helm Values

Customize Gateway configuration in `values.staging.yaml`:

```yaml
gateway:
  enabled: true
  name: lookout-staging
  listeners:
    http:
      port: 8080
      protocol: HTTP
    https:
      port: 8443
      protocol: HTTPS
      tls:
        enabled: true
        certificateRef:
          name: lookout-tls
          kind: Secret

httproute:
  enabled: true
  hostnames:
    - "lookout-stg.timonier.io"
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /
      backendRefs:
        - name: lookout-staging-lookout-app
          port: 3000
```

## Security Considerations

1. **TLS Configuration:**
   - Use TLS 1.2+ only
   - Configure strong cipher suites
   - Enable HSTS in production

2. **Certificate Management:**
   - Rotate certificates regularly
   - Use cert-manager for automatic renewal
   - Never commit private keys to git

3. **Access Control:**
   - Use authentication/authorization middleware
   - Configure RBAC for Gateway resources
   - Limit allowed routes and namespaces

## References

- [Gateway API Documentation](https://gateway-api.sigs.k8s.io/)
- [Envoy Gateway Documentation](https://gateway.envoyproxy.io/)
- [cert-manager Documentation](https://cert-manager.io/)
- [AWS Load Balancer Controller](https://kubernetes-sigs.github.io/aws-load-balancer-controller/)
