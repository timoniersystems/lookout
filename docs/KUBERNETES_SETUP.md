# Kubernetes Setup Guide

Complete guide for deploying Lookout on Kubernetes with kind cluster, Gateway API, ArgoCD (with auto-deploy), and AWS ALB.

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Chapter 1: Kind Cluster Setup](#chapter-1-kind-cluster-setup)
- [Chapter 2: Gateway API Setup](#chapter-2-gateway-api-setup)
- [Chapter 3: ArgoCD GitOps Setup](#chapter-3-argocd-gitops-setup)
  - [3.10 ArgoCD Image Updater for Auto-Deploy](#310-argocd-image-updater-for-auto-deploy)
- [Chapter 4: AWS ALB Integration](#chapter-4-aws-alb-integration)
- [Chapter 5: Secrets Management with External Secrets Operator](#chapter-5-secrets-management-with-external-secrets-operator)
- [Operations Guide](#operations-guide)
- [Troubleshooting](#troubleshooting)

---

## Overview

This guide covers the complete Kubernetes deployment stack for Lookout:

1. **Kind Cluster** - Local Kubernetes cluster running on EC2
2. **Gateway API** - Modern ingress using Envoy Gateway with fixed NodePorts
3. **ArgoCD** - GitOps continuous deployment from GitHub Container Registry
4. **AWS ALB** - Application Load Balancer for production access

### Environment

**EC2 Instance:** `10.0.3.142`
- OS: Ubuntu 24.04 (Linux 6.14.0-1018-aws)
- RAM: 7.6GB
- Disk: 234GB available
- Docker: 29.2.1
- Kind: v0.27.0
- kubectl: v1.35.1

---

## Architecture

### Full Stack Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                           Internet                               │
└───────────────────────────────┬─────────────────────────────────┘
                                │
                ┌───────────────▼────────────────┐
                │  Route53: lookout-stg.timonier.io  │
                └───────────────┬────────────────┘
                                │
                ┌───────────────▼────────────────┐
                │   Application Load Balancer    │
                │  - HTTPS Listener (443)        │
                │  - HTTP Redirect (80→443)      │
                │  - SSL/TLS Termination         │
                └───────────────┬────────────────┘
                                │
                ┌───────────────▼────────────────┐
                │  Target Groups                 │
                │  - HTTP TG → Port 32080        │
                │  - HTTPS TG → Port 32443       │
                │  Health Check: /health         │
                └───────────────┬────────────────┘
                                │
                ┌───────────────▼────────────────┐
                │  EC2 Instance: 10.0.3.142      │
                │  ┌──────────────────────────┐  │
                │  │  Kind Cluster            │  │
                │  │  ┌────────────────────┐  │  │
                │  │  │ Envoy Gateway      │  │  │
                │  │  │ HTTP: :32080       │  │  │
                │  │  │ HTTPS: :32443      │  │  │
                │  │  │ (Fixed NodePorts)  │  │  │
                │  │  └─────────┬──────────┘  │  │
                │  │            │              │  │
                │  │  ┌─────────▼──────────┐  │  │
                │  │  │  staging namespace │  │  │
                │  │  │  - HTTPRoute       │  │  │
                │  │  │  - Lookout App     │  │  │
                │  │  │  - Dgraph          │  │  │
                │  │  └────────────────────┘  │  │
                │  │  ┌────────────────────┐  │  │
                │  │  │  argocd namespace  │  │  │
                │  │  │  - ArgoCD Server   │  │  │
                │  │  │  - Image Updater   │  │  │
                │  │  │  - Applications    │  │  │
                │  │  └────────────────────┘  │  │
                │  └──────────────────────────┘  │
                └─────────────────────────────────┘
                        │
                        ▼
                ┌───────────────────────┐
                │  GitHub Container     │
                │  Registry (ghcr.io)   │
                │  - lookout:main       │
                │  - lookout:sha        │
                └───────────────────────┘
```

### Traffic Flow

```
Internet → Route53 → ALB (TLS termination) → EC2 Instance →
  → Kind NodePorts (32080/32443) → Envoy Gateway → HTTPRoute →
  → Lookout App Service → Lookout Pods
```

### GitOps Deployment Flow

```
Developer Push → GitHub Actions → Build & Test →
  → Push to ghcr.io → ArgoCD Detects → Sync to Staging →
  → Health Checks → Deployment Complete
```

---

## Chapter 1: Kind Cluster Setup

### 1.1 Prerequisites

- Ubuntu 24.04 or later
- Docker installed and running
- SSH access to EC2 instance

### 1.2 Install Kind

```bash
# Download and install kind
curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.27.0/kind-linux-amd64
chmod +x ./kind
sudo mv ./kind /usr/local/bin/kind

# Verify installation
kind version
```

### 1.3 Install kubectl

```bash
# Download kubectl
curl -LO "https://dl.k8s.io/release/v1.35.1/bin/linux/amd64/kubectl"
chmod +x kubectl
sudo mv kubectl /usr/local/bin/

# Verify installation
kubectl version --client
```

### 1.4 Create Kind Cluster

```bash
# Create cluster configuration
cat > /tmp/kind-cluster-config.yaml <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: lookout
nodes:
- role: control-plane
  extraPortMappings:
  # Fixed HTTP NodePort for ALB
  - containerPort: 32080
    hostPort: 32080
    protocol: TCP
  # Fixed HTTPS NodePort for ALB
  - containerPort: 32443
    hostPort: 32443
    protocol: TCP
EOF

# Create cluster
kind create cluster --config /tmp/kind-cluster-config.yaml

# Verify cluster
kubectl cluster-info
kubectl get nodes
```

### 1.5 Install Gateway API CRDs

```bash
# Install Gateway API v1.2.1
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.2.1/standard-install.yaml

# Verify CRDs
kubectl get crd | grep gateway
```

### 1.6 Install Envoy Gateway

```bash
# Install Envoy Gateway v1.3.0
helm install eg oci://docker.io/envoyproxy/gateway-helm \
  --version v1.3.0 \
  --namespace envoy-gateway-system \
  --create-namespace

# Wait for Envoy Gateway to be ready
kubectl wait --timeout=5m \
  -n envoy-gateway-system \
  deployment/envoy-gateway \
  --for=condition=Available

# Verify installation
kubectl get pods -n envoy-gateway-system
```

### 1.7 Create Namespaces

```bash
# Create staging namespace
kubectl create namespace staging

# Create production namespace (optional)
kubectl create namespace production

# Verify namespaces
kubectl get namespaces
```

---

## Chapter 2: Gateway API Setup

### 2.1 Overview

The Gateway API provides a modern, Kubernetes-native ingress solution using Envoy Gateway. For ALB integration, we configure **fixed NodePorts** to ensure stable target group configuration.

### 2.2 Fixed NodePorts Configuration

**Why Fixed NodePorts?**
- Prevents NodePorts from changing when Gateway is recreated
- No need to reconfigure ALB target groups after Gateway updates
- Predictable port mapping for security groups

**Configured Ports:**
- HTTP: `32080` → Gateway port 8080
- HTTPS: `32443` → Gateway port 8443

### 2.3 Setup Gateway with Fixed NodePorts

Run the automated setup script:

```bash
cd ~/lookout
./scripts/setup-fixed-nodeports.sh
```

This creates an `EnvoyProxy` resource with fixed NodePort configuration:

```yaml
apiVersion: gateway.envoyproxy.io/v1alpha1
kind: EnvoyProxy
metadata:
  name: custom-proxy-config
  namespace: staging
spec:
  provider:
    type: Kubernetes
    kubernetes:
      envoyService:
        type: LoadBalancer
        annotations:
          service.beta.kubernetes.io/aws-load-balancer-type: "nlb"
        patch:
          type: StrategicMerge
          value:
            spec:
              ports:
              - name: http
                port: 8080
                targetPort: 8080
                protocol: TCP
                nodePort: 32080  # Fixed HTTP NodePort
              - name: https
                port: 8443
                targetPort: 8443
                protocol: TCP
                nodePort: 32443  # Fixed HTTPS NodePort
```

### 2.4 Create GatewayClass

```bash
cd ~/lookout
./scripts/setup-gateway.sh
```

This script:
1. Creates the Envoy Gateway GatewayClass (`eg`)
2. Generates a self-signed TLS certificate
3. Creates the `lookout-tls` secret in staging namespace

Alternatively, create manually:

```bash
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

### 2.5 Generate TLS Certificates

```bash
# Generate self-signed certificate (valid 365 days)
openssl req -x509 -newkey rsa:4096 \
  -keyout /tmp/tls.key \
  -out /tmp/tls.crt \
  -days 365 \
  -nodes \
  -subj "/CN=lookout-stg.timonier.io/O=Timonier Systems" \
  -addext "subjectAltName=DNS:lookout-stg.timonier.io,DNS:*.lookout-stg.timonier.io"

# Create Kubernetes TLS secret
kubectl create secret tls lookout-tls \
  --cert=/tmp/tls.crt \
  --key=/tmp/tls.key \
  --namespace=staging

# Cleanup temporary files
rm /tmp/tls.key /tmp/tls.crt

# Verify secret
kubectl get secret lookout-tls -n staging
```

### 2.6 Verify Gateway Configuration

```bash
# Check GatewayClass
kubectl get gatewayclass

# Check Gateway
kubectl get gateway -n staging

# Check Envoy Gateway service with fixed NodePorts
kubectl get svc -n envoy-gateway-system -l gateway.envoyproxy.io/owning-gateway-name=lookout-staging

# Expected output:
# NAME                                     TYPE           PORT(S)
# envoy-staging-lookout-staging-*          LoadBalancer   8080:32080/TCP,8443:32443/TCP
```

### 2.7 Test Gateway Connectivity

```bash
# Test HTTP (from EC2)
curl -H "Host: lookout-stg.timonier.io" http://localhost:32080/health

# Test HTTPS (self-signed cert)
curl -k -H "Host: lookout-stg.timonier.io" https://localhost:32443/health
```

### 2.8 Production TLS Certificates (Optional)

For production, use cert-manager with Let's Encrypt:

```bash
# Install cert-manager
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml

# Create ClusterIssuer
kubectl apply -f - <<EOF
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
EOF

# Create Certificate
kubectl apply -f - <<EOF
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
EOF
```

---

## Chapter 3: ArgoCD GitOps Setup

### 3.1 Overview

ArgoCD provides GitOps-based continuous deployment, automatically syncing your Kubernetes cluster with your Git repository and pulling new images from GitHub Container Registry.

**Deployment Flow:**
1. Developer pushes to `main` branch
2. GitHub Actions builds Docker image
3. Pushes image to `ghcr.io/timoniersystems/lookout:main`
4. ArgoCD detects changes (Git + Image)
5. Automatically deploys to staging namespace

### 3.2 Install ArgoCD

```bash
cd ~/lookout
./scripts/setup-argocd.sh
```

This script:
- Installs ArgoCD in `argocd` namespace
- Waits for all pods to be ready
- Displays the initial admin password

**Save the admin password displayed by the script!**

Manual installation:

```bash
# Create namespace
kubectl create namespace argocd

# Install ArgoCD
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml

# Wait for ArgoCD to be ready
kubectl wait --for=condition=available --timeout=300s \
  deployment/argocd-server -n argocd

# Get initial admin password
kubectl -n argocd get secret argocd-initial-admin-secret \
  -o jsonpath="{.data.password}" | base64 -d
```

### 3.3 Access ArgoCD UI

From your local machine, create an SSH tunnel:

```bash
ssh -L 8080:localhost:8080 ubuntu@10.0.3.142 \
  'kubectl port-forward svc/argocd-server -n argocd 8080:443'
```

Open https://localhost:8080
- Username: `admin`
- Password: (from setup script output)

**IMPORTANT:** Change the password after first login!

### 3.4 Configure GitHub Container Registry Access

Create a secret to pull images from ghcr.io:

```bash
# Set your GitHub credentials
export GITHUB_USERNAME=<your-github-username>
export GITHUB_TOKEN=<your-github-token>

# Run the script
cd ~/lookout
./scripts/create-ghcr-secret.sh staging
```

**Creating a GitHub Token:**
1. Go to https://github.com/settings/tokens
2. Click "Generate new token (classic)"
3. Select scopes: `read:packages` (minimum) or `repo` (for private repos)
4. Copy the token

Manual secret creation:

```bash
kubectl create secret docker-registry ghcr-secret \
  --docker-server=ghcr.io \
  --docker-username=$GITHUB_USERNAME \
  --docker-password=$GITHUB_TOKEN \
  --docker-email=$GITHUB_USERNAME@users.noreply.github.com \
  --namespace=staging
```

### 3.5 Configure GitHub Repository Access (Private Repos)

For private GitHub repositories, create repository credentials:

```bash
# Set credentials
export GITHUB_USERNAME=<your-github-username>
export GITHUB_TOKEN=<your-github-token>  # Token with 'repo' scope

# Run the script
cd ~/lookout
./scripts/setup-argocd-github-repo.sh
```

This creates a secret with repository credentials that ArgoCD uses to access your Helm charts.

### 3.6 Deploy ArgoCD Application

Deploy the staging application:

```bash
kubectl apply -f k8s/argocd/staging-application.yaml
```

This creates an ArgoCD Application that:
- Watches the `main` branch
- Deploys Helm chart from `helm/lookout/`
- Uses `values.staging.yaml`
- Pulls images from `ghcr.io/timoniersystems/lookout:main`
- Deploys to `staging` namespace
- Auto-syncs on changes

Verify the application:

```bash
# Check application status
kubectl get application -n argocd

# Get application details
kubectl describe application lookout-staging -n argocd

# Check deployed resources
kubectl get all -n staging
```

### 3.7 Install ArgoCD Image Updater (Optional)

For automatic image updates when new images are pushed to ghcr.io:

```bash
cd ~/lookout
./scripts/setup-argocd-image-updater.sh
```

This enables ArgoCD to automatically detect new images and update deployments.

### 3.8 Manual Sync

Trigger a manual sync if needed:

```bash
# Using kubectl
kubectl patch application lookout-staging -n argocd \
  --type merge -p '{"operation":{"initiatedBy":{"username":"admin"},"sync":{}}}'
```

### 3.9 Verify Deployment

```bash
# Check pods
kubectl get pods -n staging

# Check services
kubectl get svc -n staging

# Check Gateway and HTTPRoute
kubectl get gateway,httproute -n staging

# View application logs
kubectl logs -n staging -l app.kubernetes.io/name=lookout-app -f

# Test health endpoint
curl -H "Host: lookout-stg.timonier.io" http://localhost:32080/health
```

### 3.10 ArgoCD Image Updater for Auto-Deploy

#### Overview

ArgoCD Image Updater enables automatic deployment when new Docker images are pushed to GHCR, even if they use the same tag (e.g., `main`). This is achieved by monitoring container registries for new image digests.

**How It Works:**
1. **GitHub Actions** builds and pushes a new Docker image to GHCR with tag `main`
2. **ArgoCD Image Updater** (runs every 2 minutes) checks GHCR for new image digests
3. When a new digest is detected, Image Updater updates the Application spec with the new digest
4. **ArgoCD** (with auto-sync enabled) automatically deploys the new image to Kubernetes

#### Components

**ArgoCD Image Updater:**
- Namespace: `argocd`
- Deployment: `argocd-image-updater`
- Check Interval: 2 minutes
- Strategy: `digest` - watches for changes in image SHA256 digest

#### Configuration

The Image Updater is configured via Application annotations:

```yaml
# Application annotations
metadata:
  annotations:
    argocd-image-updater.argoproj.io/image-list: lookout=ghcr.io/timoniersystems/lookout:main
    argocd-image-updater.argoproj.io/lookout.helm.image-name: lookout-app.image.repository
    argocd-image-updater.argoproj.io/lookout.helm.image-tag: lookout-app.image.tag
    argocd-image-updater.argoproj.io/lookout.force-update: "true"
    argocd-image-updater.argoproj.io/lookout.update-strategy: digest
    argocd-image-updater.argoproj.io/lookout.pull-secret: pullsecret:argocd/ghcr-secret
    argocd-image-updater.argoproj.io/write-back-method: argocd

# Auto-sync policy
spec:
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
      allowEmpty: false
```

#### Verification

**Check Image Updater Status:**
```bash
# Check if Image Updater is running
kubectl get pods -n argocd -l app=argocd-image-updater

# View Image Updater logs
kubectl logs -n argocd -l app=argocd-image-updater --tail=100

# Look for lines like:
# "Setting new image to ghcr.io/timoniersystems/lookout:main@sha256:..."
# "Successfully updated the live application spec"
```

**Check Current Image Digest:**
```bash
# Check what image is deployed
kubectl get deployment -n staging lookout-staging-lookout-app \
  -o jsonpath='{.spec.template.spec.containers[0].image}'

# Should show something like:
# ghcr.io/timoniersystems/lookout:main@sha256:32ae185ebc207a2fe9881570a51ad1d9247a49ec69ed6daa1fd4036153123b59
```

**Check Application Status:**
```bash
# Check if auto-sync is working
kubectl get application lookout-staging -n argocd \
  -o jsonpath='{.status.sync.status}' && echo

# Should show: Synced

# Check application health
kubectl get application lookout-staging -n argocd \
  -o jsonpath='{.status.health.status}' && echo

# Should show: Healthy (or Progressing during deployment)
```

#### Testing Auto-Deploy

**Trigger a New Build:**
1. Push code changes to the `main` branch
2. GitHub Actions will build and push a new Docker image
3. Wait up to 2 minutes for Image Updater to detect the change
4. ArgoCD will automatically deploy the new image

**Force Image Updater Check:**
```bash
# Restart Image Updater to trigger immediate check
kubectl rollout restart deployment argocd-image-updater -n argocd

# Watch the logs
kubectl logs -n argocd -l app=argocd-image-updater -f
```

**Monitor Deployment:**
```bash
# Watch application status
kubectl get application lookout-staging -n argocd -w

# Watch pods for rolling update
kubectl get pods -n staging -l app.kubernetes.io/name=lookout-app -w

# Check pod events
kubectl describe pod -n staging -l app.kubernetes.io/name=lookout-app
```

#### Deployment Timeline

The complete deployment timeline after pushing code:
1. **0-5 min**: GitHub Actions builds and pushes Docker image
2. **0-2 min**: Image Updater detects new digest (next check cycle)
3. **0-1 min**: ArgoCD syncs the change
4. **1-2 min**: Kubernetes rolling update completes

**Total time**: Approximately 5-10 minutes from code push to deployment

#### Troubleshooting Image Updater

**Image Updater Not Detecting Changes:**
```bash
# Check Image Updater logs for errors
kubectl logs -n argocd -l app=argocd-image-updater --tail=200

# Verify GHCR credentials are correct
kubectl get secret ghcr-secret -n argocd -o yaml

# Verify Image Updater can access GHCR
kubectl exec -n argocd deployment/argocd-image-updater -- \
  curl -H "Authorization: Bearer $(kubectl get secret ghcr-secret -n argocd -o jsonpath='{.data.\.dockerconfigjson}' | base64 -d | jq -r '.auths."ghcr.io".password')" \
  https://ghcr.io/v2/timoniersystems/lookout/tags/list
```

**Application Not Auto-Syncing:**
```bash
# Verify auto-sync is enabled
kubectl get application lookout-staging -n argocd \
  -o jsonpath='{.spec.syncPolicy.automated}' | jq .

# Should show:
# {
#   "allowEmpty": false,
#   "prune": true,
#   "selfHeal": true
# }

# Manually trigger sync if needed
kubectl patch application lookout-staging -n argocd \
  --type merge -p '{"operation":{"initiatedBy":{"username":"admin"},"sync":{}}}'
```

**Force Application Refresh:**
```bash
# View what would be synced
kubectl get application lookout-staging -n argocd \
  -o jsonpath='{.status.sync.revision}'

# Force refresh
kubectl patch application lookout-staging -n argocd \
  --type merge -p '{"metadata":{"annotations":{"argocd.argoproj.io/refresh":"hard"}}}'
```

#### Required Secrets

The following secrets are required for Image Updater:
- `ghcr-secret` in namespace `argocd` - GHCR credentials for Image Updater
- `ghcr-secret` in namespace `staging` - GHCR credentials for pulling images
- `argocd-image-updater-secret` in namespace `argocd` - ArgoCD API token

These secrets are already configured and should not need manual updates.

#### Security

- Image Updater uses digest verification for image integrity
- GHCR credentials are stored in Kubernetes secrets
- Auto-sync includes `prune: true` to remove orphaned resources
- `selfHeal: true` ensures desired state is maintained

---

## Chapter 4: AWS ALB Integration

### 4.1 Overview

The Application Load Balancer provides production-grade access for `lookout-stg.timonier.io` with:
- TLS termination with ACM certificates
- HTTP to HTTPS redirection (301 redirect)
- Health checks with `/health` endpoint
- Multi-AZ availability
- Route53 DNS integration
- Integration with fixed NodePorts from Gateway

**Traffic Flow:**
```
Internet → Route53 (lookout-stg.timonier.io) → ALB (TLS termination) →
  → Target Groups (32080/32443) → EC2 Instance → Kind NodePorts →
  → Envoy Gateway → HTTPRoute → Lookout App
```

### 4.2 Prerequisites

Before configuring ALB:

1. **Gateway with Fixed NodePorts configured** (completed in Chapter 2)
   - HTTP NodePort: `32080`
   - HTTPS NodePort: `32443`
2. AWS account with appropriate IAM permissions
3. VPC with EC2 instance at `10.0.3.142`
4. Domain `timonier.io` managed in Route53
5. SSL certificate for `*.timonier.io` or `lookout-stg.timonier.io` in AWS Certificate Manager

**Verify Fixed NodePorts:**
```bash
kubectl get svc -n envoy-gateway-system -l gateway.envoyproxy.io/owning-gateway-name=lookout-staging

# Expected: 8080:32080/TCP,8443:32443/TCP
```

### 4.3 Kind-Specific Setup (Required for Kind Clusters)

**IMPORTANT:** If you're using a Kind cluster (like in this setup), Kind NodePorts are **not exposed to the EC2 host** by default. They only exist within the Kind Docker container network. You must complete these additional steps before setting up the ALB:

#### Step 1: Setup NodePort Forwarding

Run the automated script to forward NodePorts from the Kind container to the EC2 host:

```bash
# SSH to EC2
ssh ubuntu@10.0.3.142

# Run the NodePort forwarding script
./scripts/setup-kind-nodeport-forwarding.sh
```

This script:
- Installs `socat` if not already installed
- Creates port forwarding from EC2 host ports 32080 and 32443 to Kind container
- Verifies the forwarding is working

**Manual setup (if script fails):**

```bash
# Get Kind container IP
KIND_IP=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' lookout-control-plane)

# Install socat
sudo apt-get update && sudo apt-get install -y socat

# Forward port 32080 (HTTP)
sudo nohup socat TCP4-LISTEN:32080,fork,reuseaddr TCP4:${KIND_IP}:32080 > /tmp/socat-32080.log 2>&1 &

# Forward port 32443 (HTTPS)
sudo nohup socat TCP4-LISTEN:32443,fork,reuseaddr TCP4:${KIND_IP}:32443 > /tmp/socat-32443.log 2>&1 &

# Verify forwarding is active
ps aux | grep "[s]ocat.*3204"
```

#### Step 2: Setup Health Check HTTPRoute

ALB health checks don't send the `Host: lookout-stg.timonier.io` header. Create an HTTPRoute that accepts `/health` requests without hostname restrictions:

```bash
# Run the health check HTTPRoute setup script
./scripts/setup-health-httproute.sh
```

This creates an HTTPRoute that:
- Accepts requests to `/health` from any hostname
- Forwards to the Lookout app service
- Allows ALB health checks to succeed

**Manual setup (if script fails):**

```bash
kubectl apply -f - <<EOF
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: lookout-health
  namespace: staging
spec:
  parentRefs:
  - name: lookout-staging
  rules:
  - matches:
    - path:
        type: Exact
        value: /health
    backendRefs:
    - name: lookout-staging-lookout-app
      port: 3000
EOF
```

**Test the setup:**

```bash
# Test HTTP port without Host header
curl http://localhost:32080/health

# Test HTTPS port without Host header
curl -k https://localhost:32443/health

# Both should return: {"service":"lookout-ui","status":"healthy",...}
```

### 4.4 Step 1: Create Target Groups

You'll need **two target groups** - one for HTTP and one for HTTPS.

#### HTTP Target Group

1. Navigate to **EC2 Console** → **Target Groups** → **Create target group**

2. **Basic Configuration:**
   - Target type: `Instances`
   - Target group name: `lookout-http-tg`
   - Protocol: `HTTP`
   - Port: `32080` (fixed HTTP NodePort)
   - VPC: Select the VPC where your EC2 instance is running

3. **Health Check Settings:**
   - Health check protocol: `HTTP`
   - Health check path: `/health`
   - Advanced health check settings:
     - Healthy threshold: `2`
     - Unhealthy threshold: `2`
     - Timeout: `5 seconds`
     - Interval: `10 seconds`
     - Success codes: `200`

4. **Register Targets:**
   - Select your EC2 instance
   - Port: `32080`
   - Click "Include as pending below"
   - Click "Create target group"

#### HTTPS Target Group

Repeat the above steps with:
- Target group name: `lookout-https-tg`
- **Protocol: `HTTPS`** (ALB re-encrypts traffic and forwards HTTPS to target)
- Port: `32443` (fixed HTTPS NodePort - Envoy Gateway terminates TLS)
- **Health check protocol: `HTTPS`** (Port 32443 uses TLS)
- Health check path: `/health`
- Register same EC2 instance on port `32443`

**Important:**
- Target group protocol is `HTTPS` (ALB terminates TLS from client, then re-encrypts for backend)
- Health check protocol is `HTTPS` (because Envoy Gateway's port 32443 uses TLS)
- This provides end-to-end encryption from client to Envoy Gateway

### 4.4 Step 2: Configure Security Groups

#### EC2 Instance Security Group

Add inbound rules to allow traffic from ALB:

```
Inbound Rules:
- Type: Custom TCP
  Port: 32080
  Source: <ALB Security Group>
  Description: Allow HTTP traffic from ALB

- Type: Custom TCP
  Port: 32443
  Source: <ALB Security Group>
  Description: Allow HTTPS traffic from ALB
```

#### Create ALB Security Group

1. Navigate to **EC2** → **Security Groups** → **Create security group**

2. **Basic details:**
   - Security group name: `lookout-alb-sg`
   - Description: Security group for Lookout ALB
   - VPC: Same VPC as EC2 instance

3. **Inbound rules:**
   ```
   - Type: HTTP
     Port: 80
     Source: 0.0.0.0/0
     Description: Allow HTTP from anywhere

   - Type: HTTPS
     Port: 443
     Source: 0.0.0.0/0
     Description: Allow HTTPS from anywhere
   ```

4. **Outbound rules:**
   ```
   - Type: All traffic
     Destination: 0.0.0.0/0
   ```

### 4.5 Step 3: Request SSL Certificate

#### Using AWS Certificate Manager (ACM)

1. Navigate to **Certificate Manager** → **Request certificate**
2. Choose "Request a public certificate"
3. Domain names: `lookout-stg.timonier.io` (or `*.timonier.io` for wildcard)
4. Validation method: DNS validation (recommended)
5. Follow ACM instructions to add DNS validation records to Route53
6. Wait for certificate to be issued (usually a few minutes)

#### Import Existing Certificate (Alternative)

If you already have an SSL certificate:
1. Navigate to **Certificate Manager** → **Import certificate**
2. Provide certificate body, private key, and certificate chain

### 4.6 Step 4: Create Application Load Balancer

1. **Navigate to EC2 Console** → **Load Balancers** → **Create Load Balancer**

2. **Choose Load Balancer Type:** Application Load Balancer

3. **Basic Configuration:**
   - Load balancer name: `lookout-alb`
   - Scheme: `Internet-facing`
   - IP address type: `IPv4`

4. **Network Mapping:**
   - VPC: Select the same VPC as your EC2 instance
   - Availability Zones: Select at least 2 AZs for high availability
   - Select public subnets in each AZ

5. **Security Groups:**
   - Select the `lookout-alb-sg` security group
   - Remove the default security group

6. **Listeners and Routing:**

   **HTTP Listener (Port 80):**
   - Protocol: HTTP
   - Port: 80
   - Default action: Redirect to HTTPS
     - Protocol: HTTPS
     - Port: 443
     - Status code: 301 (Permanent redirect)

   **HTTPS Listener (Port 443):**
   - Protocol: HTTPS
   - Port: 443
   - Default SSL/TLS certificate: Select the certificate from Step 3
   - Default action: Forward to `lookout-https-tg`

7. **Review and Create**

### 4.7 Step 5: Configure DNS in Route53

1. Navigate to **Route53** → **Hosted zones** → Select `timonier.io`

2. **Create Record:**
   - Record name: `lookout-stg`
   - Record type: `A - Routes traffic to an IPv4 address`
   - Alias: Yes
   - Route traffic to:
     - Alias to Application and Classic Load Balancer
     - Region: Select your region
     - Load balancer: Select `lookout-alb`
   - Routing policy: Simple routing
   - Click "Create records"

### 4.7 AWS CLI Alternative

For automation, use the provided script or run these AWS CLI commands manually.

#### Quick Setup with Script

Use the automated script ([`scripts/setup-alb.sh`](../scripts/setup-alb.sh)):

```bash
# Set required environment variables
export AWS_REGION=us-east-1
export VPC_ID=vpc-xxxxx
export EC2_INSTANCE_ID=i-xxxxx
export SUBNET_1=subnet-xxxxx
export SUBNET_2=subnet-xxxxx
export SUBNET_3=subnet-xxxxx
export CERTIFICATE_ARN=arn:aws:acm:us-east-1:xxxxx:certificate/xxxxx
export HOSTED_ZONE_ID=Z0xxxxx

# Optional: Enable multi-environment support (staging + production)
export ENABLE_PROD=true

# Run the script
./scripts/setup-alb.sh
```

The script will:
- ✅ Verify fixed NodePorts are configured
- ✅ Create ALB and target security groups
- ✅ Create HTTP and HTTPS target groups (ports 32080, 32443) with Protocol=HTTPS
- ✅ Automatically fix misconfigured target groups (wrong protocol)
- ✅ Register EC2 instance with target groups
- ✅ Create Application Load Balancer
- ✅ Configure HTTP→HTTPS redirect listener (301)
- ✅ Configure HTTPS listener with ACM certificate and end-to-end encryption
- ✅ Configure host-based routing (if ENABLE_PROD=true):
  - `lookout-stg.timonier.io` → staging target group
  - `lookout-prod.timonier.io`, `lookout.timonier.io` → production target group
- ✅ Verify target health

**Multi-Environment Support:**
- With `ENABLE_PROD=true`: Creates separate target groups for staging and production with host-based routing
- Without `ENABLE_PROD`: Single staging environment only

#### Manual Setup (Individual Commands)

Alternatively, run these commands individually:

##### Set Variables

```bash
# Configuration
export AWS_REGION=us-east-1
export VPC_ID=vpc-xxxxx  # Your VPC ID
export EC2_INSTANCE_ID=i-xxxxx  # Your EC2 instance ID
export SUBNET_1=subnet-xxxxx  # Public subnet in AZ 1
export SUBNET_2=subnet-xxxxx  # Public subnet in AZ 2
export CERTIFICATE_ARN=arn:aws:acm:us-east-1:xxxxx:certificate/xxxxx  # Your ACM certificate ARN
export HOSTED_ZONE_ID=Z0xxxxx  # Your Route53 hosted zone ID
```

#### Create Security Groups

```bash
# Create ALB security group
ALB_SG_ID=$(aws ec2 create-security-group \
  --group-name lookout-alb-sg \
  --description "Security group for Lookout ALB" \
  --vpc-id $VPC_ID \
  --region $AWS_REGION \
  --output text --query 'GroupId')

echo "ALB Security Group ID: $ALB_SG_ID"

# Add inbound rules to ALB security group
aws ec2 authorize-security-group-ingress \
  --group-id $ALB_SG_ID \
  --ip-permissions \
    IpProtocol=tcp,FromPort=80,ToPort=80,IpRanges='[{CidrIp=0.0.0.0/0,Description="Allow HTTP from anywhere"}]' \
    IpProtocol=tcp,FromPort=443,ToPort=443,IpRanges='[{CidrIp=0.0.0.0/0,Description="Allow HTTPS from anywhere"}]' \
  --region $AWS_REGION

# Get EC2 instance security group ID
EC2_SG_ID=$(aws ec2 describe-instances \
  --instance-ids $EC2_INSTANCE_ID \
  --region $AWS_REGION \
  --query 'Reservations[0].Instances[0].SecurityGroups[0].GroupId' \
  --output text)

echo "EC2 Security Group ID: $EC2_SG_ID"

# Add inbound rules to EC2 security group (allow traffic from ALB)
aws ec2 authorize-security-group-ingress \
  --group-id $EC2_SG_ID \
  --ip-permissions \
    IpProtocol=tcp,FromPort=32080,ToPort=32080,UserIdGroupPairs="[{GroupId=$ALB_SG_ID,Description='Allow HTTP from ALB'}]" \
    IpProtocol=tcp,FromPort=32443,ToPort=32443,UserIdGroupPairs="[{GroupId=$ALB_SG_ID,Description='Allow HTTPS from ALB'}]" \
  --region $AWS_REGION
```

#### Create Target Groups

```bash
# Create HTTP target group
HTTP_TG_ARN=$(aws elbv2 create-target-group \
  --name lookout-http-tg \
  --protocol HTTP \
  --port 32080 \
  --vpc-id $VPC_ID \
  --health-check-protocol HTTP \
  --health-check-path /health \
  --health-check-interval-seconds 10 \
  --health-check-timeout-seconds 5 \
  --healthy-threshold-count 2 \
  --unhealthy-threshold-count 2 \
  --matcher HttpCode=200 \
  --region $AWS_REGION \
  --query 'TargetGroups[0].TargetGroupArn' \
  --output text)

echo "HTTP Target Group ARN: $HTTP_TG_ARN"

# Create HTTPS target group (Protocol=HTTPS for end-to-end encryption)
HTTPS_TG_ARN=$(aws elbv2 create-target-group \
  --name lookout-https-tg \
  --protocol HTTPS \
  --port 32443 \
  --vpc-id $VPC_ID \
  --health-check-protocol HTTPS \
  --health-check-path /health \
  --health-check-interval-seconds 10 \
  --health-check-timeout-seconds 5 \
  --healthy-threshold-count 2 \
  --unhealthy-threshold-count 2 \
  --matcher HttpCode=200 \
  --region $AWS_REGION \
  --query 'TargetGroups[0].TargetGroupArn' \
  --output text)

echo "HTTPS Target Group ARN: $HTTPS_TG_ARN"

# Register EC2 instance with both target groups
aws elbv2 register-targets \
  --target-group-arn $HTTP_TG_ARN \
  --targets Id=$EC2_INSTANCE_ID,Port=32080 \
  --region $AWS_REGION

aws elbv2 register-targets \
  --target-group-arn $HTTPS_TG_ARN \
  --targets Id=$EC2_INSTANCE_ID,Port=32443 \
  --region $AWS_REGION
```

#### Create Application Load Balancer

```bash
# Create ALB
ALB_ARN=$(aws elbv2 create-load-balancer \
  --name lookout-alb \
  --subnets $SUBNET_1 $SUBNET_2 \
  --security-groups $ALB_SG_ID \
  --scheme internet-facing \
  --type application \
  --ip-address-type ipv4 \
  --region $AWS_REGION \
  --query 'LoadBalancers[0].LoadBalancerArn' \
  --output text)

echo "ALB ARN: $ALB_ARN"

# Get ALB DNS name
ALB_DNS=$(aws elbv2 describe-load-balancers \
  --load-balancer-arns $ALB_ARN \
  --region $AWS_REGION \
  --query 'LoadBalancers[0].DNSName' \
  --output text)

echo "ALB DNS: $ALB_DNS"

# Get ALB Hosted Zone ID (for Route53 alias)
ALB_ZONE_ID=$(aws elbv2 describe-load-balancers \
  --load-balancer-arns $ALB_ARN \
  --region $AWS_REGION \
  --query 'LoadBalancers[0].CanonicalHostedZoneId' \
  --output text)

echo "ALB Hosted Zone ID: $ALB_ZONE_ID"
```

#### Create Listeners

```bash
# Create HTTP listener with redirect to HTTPS
HTTP_LISTENER_ARN=$(aws elbv2 create-listener \
  --load-balancer-arn $ALB_ARN \
  --protocol HTTP \
  --port 80 \
  --default-actions Type=redirect,RedirectConfig='{Protocol=HTTPS,Port=443,StatusCode=HTTP_301}' \
  --region $AWS_REGION \
  --query 'Listeners[0].ListenerArn' \
  --output text)

echo "HTTP Listener ARN: $HTTP_LISTENER_ARN"

# Create HTTPS listener
HTTPS_LISTENER_ARN=$(aws elbv2 create-listener \
  --load-balancer-arn $ALB_ARN \
  --protocol HTTPS \
  --port 443 \
  --certificates CertificateArn=$CERTIFICATE_ARN \
  --default-actions Type=forward,TargetGroupArn=$HTTPS_TG_ARN \
  --region $AWS_REGION \
  --query 'Listeners[0].ListenerArn' \
  --output text)

echo "HTTPS Listener ARN: $HTTPS_LISTENER_ARN"
```

#### Configure Route53

```bash
# Create Route53 A record (alias to ALB)
cat > /tmp/route53-change.json <<EOF
{
  "Changes": [
    {
      "Action": "UPSERT",
      "ResourceRecordSet": {
        "Name": "lookout-stg.timonier.io",
        "Type": "A",
        "AliasTarget": {
          "HostedZoneId": "$ALB_ZONE_ID",
          "DNSName": "$ALB_DNS",
          "EvaluateTargetHealth": true
        }
      }
    }
  ]
}
EOF

aws route53 change-resource-record-sets \
  --hosted-zone-id $HOSTED_ZONE_ID \
  --change-batch file:///tmp/route53-change.json \
  --region $AWS_REGION

rm /tmp/route53-change.json

echo "Route53 record created for lookout-stg.timonier.io"
```

#### Request ACM Certificate (Optional)

If you don't have a certificate yet:

```bash
# Request ACM certificate
CERT_ARN=$(aws acm request-certificate \
  --domain-name lookout-stg.timonier.io \
  --validation-method DNS \
  --region $AWS_REGION \
  --query 'CertificateArn' \
  --output text)

echo "Certificate ARN: $CERT_ARN"

# Get DNS validation records
aws acm describe-certificate \
  --certificate-arn $CERT_ARN \
  --region $AWS_REGION \
  --query 'Certificate.DomainValidationOptions[0].ResourceRecord'

# Add the CNAME record to Route53 for validation
# (Copy the Name and Value from the output above)

# Check certificate status
aws acm describe-certificate \
  --certificate-arn $CERT_ARN \
  --region $AWS_REGION \
  --query 'Certificate.Status' \
  --output text
```

#### Verify Target Health

```bash
# Check HTTP target group health
aws elbv2 describe-target-health \
  --target-group-arn $HTTP_TG_ARN \
  --region $AWS_REGION

# Check HTTPS target group health
aws elbv2 describe-target-health \
  --target-group-arn $HTTPS_TG_ARN \
  --region $AWS_REGION

# Both should show State: healthy
```

#### Complete Script

All commands above are available in the automated script: [`scripts/setup-alb.sh`](../scripts/setup-alb.sh)

The script includes:
- Error handling and validation
- Idempotent operations (safe to re-run)
- Color-coded output
- Progress indicators
- Health check verification
- Helpful summary with next steps

See the [Quick Setup with Script](#quick-setup-with-script) section above for usage instructions.

### 4.8 Step 6: Verify the Setup

```bash
# Check Target Health (AWS Console)
# EC2 → Target Groups → lookout-http-tg → Targets tab
# EC2 → Target Groups → lookout-https-tg → Targets tab
# Both should show "healthy"

# Test DNS Resolution
nslookup lookout-stg.timonier.io
# Should return ALB's IP addresses

# Test HTTPS Access
curl -I https://lookout-stg.timonier.io/health
# Should return 200 OK

# Test HTTP Redirect
curl -I http://lookout-stg.timonier.io
# Should return 301 redirect to HTTPS
```

### 4.9 Troubleshooting ALB

#### Target is Unhealthy

1. Check EC2 security group allows traffic from ALB on ports 32080 and 32443
2. Verify the Gateway service is running with fixed NodePorts:
   ```bash
   kubectl get svc -n envoy-gateway-system
   ```
3. Check health check path is accessible:
   ```bash
   curl -H "Host: lookout-stg.timonier.io" http://localhost:32080/health
   curl -k -H "Host: lookout-stg.timonier.io" https://localhost:32443/health
   ```
4. Check Kind cluster status:
   ```bash
   kubectl get pods -n staging
   ```

#### SSL Certificate Issues

1. Ensure certificate covers `lookout-stg.timonier.io`
2. Check certificate status in ACM (must be "Issued")
3. Verify DNS validation records exist in Route53

#### 502 Bad Gateway

1. Check target health in target group
2. Verify Envoy Gateway is running:
   ```bash
   kubectl get pods -n envoy-gateway-system
   ```
3. Check HTTPRoute configuration:
   ```bash
   kubectl get httproute -n staging -o yaml
   ```

#### DNS Not Resolving

1. Wait 5-10 minutes for DNS propagation
2. Check Route53 record is correctly configured
3. Verify ALB is in "active" state
4. Use `dig` to debug:
   ```bash
   dig lookout-stg.timonier.io
   ```

### 4.10 Fixed NodePorts Benefits

The fixed NodePorts (32080, 32443) ensure:
- **Stable Configuration:** NodePorts don't change when Gateway is recreated
- **No ALB Updates:** Target groups remain valid after Gateway changes
- **Predictable Security Rules:** Security group rules never need updating
- **Consistent Port Mapping:** Always know which ports to use

To reconfigure NodePorts if needed:

```bash
# Change NodePort values
HTTP_NODEPORT=32090 HTTPS_NODEPORT=32453 ./scripts/setup-fixed-nodeports.sh

# Recreate Gateway to apply
kubectl delete gateway lookout-staging -n staging
kubectl apply -f k8s/argocd/staging-application.yaml

# Update ALB target groups with new ports
```

### 4.11 Cost Considerations

- **Application Load Balancer:** ~$16-20/month + data processing charges
- **ACM Certificate:** Free for public certificates
- **Route53:** $0.50/month per hosted zone + $0.40 per million queries

### 4.12 Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                           Internet                               │
└───────────────────────────────┬─────────────────────────────────┘
                                │
                ┌───────────────▼────────────────┐
                │  Route53: lookout-stg.timonier.io  │
                └───────────────┬────────────────┘
                                │
                ┌───────────────▼────────────────┐
                │   Application Load Balancer    │
                │  - HTTPS Listener (443)        │
                │  - HTTP Redirect (80→443)      │
                │  - SSL/TLS Termination         │
                └───────────────┬────────────────┘
                                │
                ┌───────────────▼────────────────┐
                │  Target Groups                 │
                │  - HTTP TG → Port 32080        │
                │  - HTTPS TG → Port 32443       │
                │  Health Check: /health         │
                └───────────────┬────────────────┘
                                │
                ┌───────────────▼────────────────┐
                │  EC2 Instance: 10.0.3.142      │
                │  ┌──────────────────────────┐  │
                │  │  Kind Cluster            │  │
                │  │  ┌────────────────────┐  │  │
                │  │  │ Envoy Gateway      │  │  │
                │  │  │ HTTP: :32080       │  │  │
                │  │  │ HTTPS: :32443      │  │  │
                │  │  └─────────┬──────────┘  │  │
                │  │            │              │  │
                │  │  ┌─────────▼──────────┐  │  │
                │  │  │  staging namespace │  │  │
                │  │  │  - HTTPRoute       │  │  │
                │  │  │  - Lookout App     │  │  │
                │  │  │  - Dgraph          │  │  │
                │  │  └────────────────────┘  │  │
                │  └──────────────────────────┘  │
                └─────────────────────────────────┘
```

---

## Chapter 5: Secrets Management with External Secrets Operator

This chapter explains how to securely manage secrets (like API keys) using AWS Secrets Manager and External Secrets Operator.

### 5.1 Overview

Lookout uses External Secrets Operator (ESO) to sync secrets from AWS Secrets Manager to Kubernetes:

#### Secret Sync Flow

```
┌──────────────────────┐
│ AWS Secrets Manager  │  ← Centralized secret storage
│  lookout/staging/    │
│  └─ nvd-api-key      │
└──────────┬───────────┘
           │
           │ External Secrets Operator (running in cluster)
           │ Syncs every 1 hour
           ▼
┌──────────────────────┐
│ Kubernetes Secret    │  ← Auto-created/updated
│  lookout-staging-    │
│  lookout-app         │
└──────────┬───────────┘
           │
           │ Mounted as env vars
           ▼
┌──────────────────────┐
│ Lookout Pod          │
│  env:                │
│    NVD_API_KEY=***   │
└──────────────────────┘
```

#### GitOps Integration

```
AWS Secrets Manager               Kubernetes Cluster
┌────────────────┐               ┌──────────────────┐
│ lookout/       │               │ External Secrets │
│ staging/       │◄──────────────┤ Operator (ESO)   │
│ nvd-api-key    │   Syncs       │                  │
└────────────────┘   every 1h    └────────┬─────────┘
                                          │
                                          │ Creates/Updates
                                          ▼
                                 ┌────────────────┐
                                 │ K8s Secret     │
                                 │ lookout-...    │
                                 └────────┬───────┘
                                          │
ArgoCD (GitOps)                           │ Mounts as env
┌────────────────┐                        │
│ Git Repository │                        ▼
│ ┌────────────┐ │             ┌────────────────────┐
│ │Helm Chart  │ │   Deploys   │ Lookout Pod        │
│ │values.yaml │ │────────────►│ containers:        │
│ │templates/  │ │             │   env:             │
│ └────────────┘ │             │   - NVD_API_KEY    │
└────────────────┘             └────────────────────┘
      │
      │ Config only
      │ (no secrets!)
      ▼
   Fully GitOps-friendly
   - App config in Git
   - Secrets in AWS
   - ESO syncs automatically
```

**Benefits:**
- ✅ Secrets never in Git
- ✅ Centralized management in AWS
- ✅ Automatic rotation support
- ✅ Audit trail via CloudTrail
- ✅ GitOps-friendly (config in Git, secrets in AWS)
- ✅ No SSH or GitHub Actions needed

### 5.2 Quick Start

#### Option 1: Automated Setup (Recommended)

The easiest way to set everything up:

```bash
cd ~/lookout
./scripts/setup-external-secrets.sh
```

This script will:
1. Install External Secrets Operator
2. Create IAM policy for Secrets Manager access
3. Create a dedicated IAM user (`lookout-external-secrets`) with access keys, stored as a K8s secret (`aws-credentials`)
4. Create secret in AWS Secrets Manager
5. Create SecretStore (using `secretRef` auth) and ExternalSecret resources
6. Verify the setup

#### Option 2: Manual Setup

If you prefer manual setup:

**A. Install External Secrets Operator**

```bash
helm repo add external-secrets https://charts.external-secrets.io
helm repo update

helm install external-secrets \
  external-secrets/external-secrets \
  --namespace external-secrets-system \
  --create-namespace \
  --set installCRDs=true
```

**B. Create IAM Policy**

```bash
cat > secrets-policy.json <<EOF
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "secretsmanager:GetSecretValue",
                "secretsmanager:DescribeSecret"
            ],
            "Resource": "arn:aws:secretsmanager:us-west-2:*:secret:lookout/*"
        }
    ]
}
EOF

aws iam create-policy \
  --policy-name LookoutSecretsManagerAccess \
  --policy-document file://secrets-policy.json
```

**C. Create IAM User and Bootstrap Credentials**

Since Kind-on-EC2 can't use IRSA (EKS-only) or IMDS (not accessible from Docker containers),
we use a dedicated IAM user with static credentials:

```bash
# Create IAM user
aws iam create-user --user-name lookout-external-secrets

# Attach the SecretsManager policy
aws iam attach-user-policy \
  --user-name lookout-external-secrets \
  --policy-arn "arn:aws:iam::$(aws sts get-caller-identity --query Account --output text):policy/LookoutSecretsManagerAccess"

# Create access keys
aws iam create-access-key --user-name lookout-external-secrets
# Save the AccessKeyId and SecretAccessKey from the output

# Create K8s secret with the credentials (bootstrap step)
kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: aws-credentials
  namespace: staging
type: Opaque
stringData:
  access-key-id: "YOUR_ACCESS_KEY_ID"
  secret-access-key: "YOUR_SECRET_ACCESS_KEY"
EOF
```

> **Important:** The `aws-credentials` secret is a bootstrap dependency — it must exist
> before the SecretStore can authenticate to AWS. This secret is NOT managed by External
> Secrets (it IS the credentials for External Secrets). It only needs to be created once
> per cluster setup. If the cluster is recreated, re-run the setup script or recreate
> this secret manually.

**D. Create Secret in AWS Secrets Manager**

```bash
aws secretsmanager create-secret \
  --name "lookout/staging/nvd-api-key" \
  --description "NVD API key for Lookout staging" \
  --secret-string '{"NVD_API_KEY":"your-api-key-here"}' \
  --region us-west-2
```

**E. Deploy via ArgoCD (GitOps)**

The SecretStore and ExternalSecret are already configured in the Helm chart:

```bash
# Push changes to Git
git add .
git commit -m "Enable External Secrets Operator"
git push

# ArgoCD will auto-sync
kubectl wait --for=condition=Synced application/lookout-staging -n argocd
```

### 5.3 Configuration

#### Helm Values

In `values.staging.yaml`:

```yaml
lookout-app:
  externalSecrets:
    enabled: true
    refreshInterval: "1h"  # How often to sync from AWS
    aws:
      region: "us-west-2"
      role: ""  # Set for EKS/IRSA; leave empty for Kind-on-EC2
      credentialsSecret: "aws-credentials"  # K8s secret with access-key-id and secret-access-key
    secretStore:
      name: "aws-secrets-manager"
    secrets:
      - secretKey: NVD_API_KEY  # Key in Kubernetes secret
        remoteKey: "lookout/staging/nvd-api-key"  # AWS secret name
        property: NVD_API_KEY  # JSON property in AWS secret
```

**Auth modes** (set one, leave the other empty):
- `credentialsSecret`: Name of a K8s secret containing `access-key-id` and `secret-access-key` (for Kind-on-EC2)
- `role`: IAM role ARN for IRSA (for EKS)

#### AWS Secret Format

AWS Secrets Manager secret should be in JSON format:

```json
{
  "NVD_API_KEY": "your-actual-api-key-here"
}
```

### 5.4 Managing Secrets

#### View Secret in AWS

```bash
aws secretsmanager get-secret-value \
  --secret-id "lookout/staging/nvd-api-key" \
  --region us-west-2 \
  --query 'SecretString' \
  --output text | jq .
```

#### Update Secret

```bash
aws secretsmanager update-secret \
  --secret-id "lookout/staging/nvd-api-key" \
  --secret-string '{"NVD_API_KEY":"new-api-key"}' \
  --region us-west-2
```

Secret will automatically sync to Kubernetes within 1 hour (or whatever `refreshInterval` is set to).

#### Force Immediate Sync

```bash
# Delete the Kubernetes secret (it will be recreated immediately)
kubectl delete secret lookout-staging-lookout-app -n staging

# Or restart the External Secrets Operator
kubectl rollout restart deployment external-secrets -n external-secrets-system
```

#### Rotate Secret

```bash
# 1. Get new API key from NVD
# 2. Update in AWS Secrets Manager
aws secretsmanager update-secret \
  --secret-id "lookout/staging/nvd-api-key" \
  --secret-string '{"NVD_API_KEY":"new-key"}' \
  --region us-west-2

# 3. Wait for sync (1 hour) or force sync
kubectl delete secret lookout-staging-lookout-app -n staging

# 4. Restart pods to pick up new value
kubectl rollout restart deployment/lookout-staging-lookout-app -n staging
```

### 5.5 Verification

#### Check External Secrets Operator

```bash
# Is ESO running?
kubectl get pods -n external-secrets-system

# Check ESO logs
kubectl logs -n external-secrets-system -l app.kubernetes.io/name=external-secrets
```

#### Check SecretStore

```bash
# View SecretStore
kubectl get secretstore -n staging

# Check status
kubectl describe secretstore aws-secrets-manager -n staging
```

#### Check ExternalSecret

```bash
# View ExternalSecret
kubectl get externalsecret -n staging

# Check sync status
kubectl describe externalsecret lookout-staging-lookout-app-external -n staging

# View conditions
kubectl get externalsecret lookout-staging-lookout-app-external -n staging -o jsonpath='{.status.conditions}'
```

#### Verify Kubernetes Secret Exists

```bash
# Check secret exists
kubectl get secret lookout-staging-lookout-app -n staging

# View secret keys (not values)
kubectl describe secret lookout-staging-lookout-app -n staging

# Decode secret value (for debugging)
kubectl get secret lookout-staging-lookout-app -n staging \
  -o jsonpath='{.data.NVD_API_KEY}' | base64 -d
```

#### Verify Pod Has Secret

```bash
# Check env vars in pod
kubectl exec -it deployment/lookout-staging-lookout-app -n staging -- env | grep NVD

# Should show: NVD_API_KEY=***
```

### 5.6 Troubleshooting

#### Secret Not Syncing

**Check ExternalSecret status:**
```bash
kubectl describe externalsecret lookout-staging-lookout-app-external -n staging
```

**Common issues:**
- `aws-credentials` K8s secret missing → Run setup script or create manually (see 5.2 Quick Start, Step C)
- IAM user doesn't have policy attached → Attach `LookoutSecretsManagerAccess` policy
- AWS secret doesn't exist → Create it in Secrets Manager
- Wrong region → Ensure secret is in us-west-2
- Wrong secret format → Must be valid JSON with the expected key
- Wrong API version → ESO v2.0+ uses `external-secrets.io/v1` (not `v1beta1`)

#### IAM Permission Errors

**Error:** `AccessDeniedException: User is not authorized to perform secretsmanager:GetSecretValue`

**Fix:**
```bash
# Verify IAM user has the policy
aws iam list-attached-user-policies --user-name lookout-external-secrets

# Attach policy if missing
aws iam attach-user-policy \
  --user-name lookout-external-secrets \
  --policy-arn arn:aws:iam::ACCOUNT_ID:policy/LookoutSecretsManagerAccess
```

#### SecretStore Shows InvalidProviderConfig

**Error:** `an IAM role must be associated with service account` or `no EC2 IMDS role found`

**Cause:** SecretStore is configured for IRSA/IMDS auth instead of `secretRef`.
Kind-on-EC2 can't use IRSA (EKS-only) or IMDS (not accessible from Docker containers).

**Fix:** Ensure `credentialsSecret` is set in Helm values and the `aws-credentials` K8s secret exists:
```bash
# Check if credentials secret exists
kubectl get secret aws-credentials -n staging

# If missing, create it (see 5.2 Quick Start, Step C)
```

#### Secret Not Appearing in Pod

**Check deployment references secret:**
```bash
kubectl get deployment lookout-staging-lookout-app -n staging -o yaml | grep -A5 secretRef
```

**Should show:**
```yaml
envFrom:
- configMapRef:
    name: lookout-staging-lookout-app
- secretRef:
    name: lookout-staging-lookout-app
```

**Restart deployment:**
```bash
kubectl rollout restart deployment/lookout-staging-lookout-app -n staging
```

#### External Secrets Operator Not Installed

```bash
# Check if installed
kubectl get namespace external-secrets-system

# If not, install
helm install external-secrets \
  external-secrets/external-secrets \
  --namespace external-secrets-system \
  --create-namespace \
  --set installCRDs=true
```

### 5.7 Adding More Secrets

#### Step 1: Add to AWS Secrets Manager

```bash
# Create new secret
aws secretsmanager create-secret \
  --name "lookout/staging/my-new-secret" \
  --secret-string '{"MY_SECRET":"value"}' \
  --region us-west-2

# Or update existing secret with new key
aws secretsmanager update-secret \
  --secret-id "lookout/staging/nvd-api-key" \
  --secret-string '{"NVD_API_KEY":"key","MY_SECRET":"value"}' \
  --region us-west-2
```

#### Step 2: Update Helm Values

Edit `values.staging.yaml`:

```yaml
lookout-app:
  externalSecrets:
    enabled: true
    secrets:
      - secretKey: NVD_API_KEY
        remoteKey: "lookout/staging/nvd-api-key"
        property: NVD_API_KEY
      - secretKey: MY_SECRET  # Add new secret
        remoteKey: "lookout/staging/my-new-secret"
        property: MY_SECRET
```

#### Step 3: Deploy

```bash
# Via GitOps (push to Git and ArgoCD will sync)
git add helm/lookout/values.staging.yaml
git commit -m "Add MY_SECRET to external secrets"
git push
```

### 5.8 Production Setup

For production environment:

#### Create Production Secret

```bash
aws secretsmanager create-secret \
  --name "lookout/production/nvd-api-key" \
  --description "NVD API key for Lookout production" \
  --secret-string '{"NVD_API_KEY":"production-key"}' \
  --region us-west-2
```

#### Update Production Values

In `values.production.yaml`:

```yaml
lookout-app:
  externalSecrets:
    enabled: true
    refreshInterval: "30m"  # More frequent sync for production
    aws:
      region: "us-west-2"
      role: ""  # Set IAM role ARN here if using EKS/IRSA
      credentialsSecret: "aws-credentials"  # Same bootstrap pattern as staging
    secrets:
      - secretKey: NVD_API_KEY
        remoteKey: "lookout/production/nvd-api-key"  # Different path
        property: NVD_API_KEY
```

> **Note:** The `aws-credentials` K8s secret must be bootstrapped in the production
> cluster as well, using the same setup script or manual process.

### 5.9 Security Best Practices

✅ **DO:**
- Use separate secrets for staging/production
- Enable AWS CloudTrail for audit logging
- Use a dedicated IAM user with least-privilege policy (only `secretsmanager:GetSecretValue` and `DescribeSecret`)
- Rotate IAM access keys and secrets regularly (every 90 days)
- Set up CloudWatch alarms for secret access
- Enable secret versioning in Secrets Manager
- Use IRSA instead of static credentials if migrating to EKS

❌ **DON'T:**
- Hardcode secrets in Helm values
- Commit secrets or AWS access keys to Git
- Use the same secret across environments
- Share secrets via chat/email
- Grant broader IAM permissions than needed
- Disable CloudTrail logging
- Reuse the ESO IAM user credentials for other purposes

### 5.10 Cost Considerations

**AWS Secrets Manager Pricing (us-west-2):**
- $0.40 per secret per month
- $0.05 per 10,000 API calls

**Example for Lookout staging:**
- 1 secret = $0.40/month
- Synced every 1 hour = ~720 API calls/month = $0.004
- **Total: ~$0.41/month**

**Optimization:**
- Increase `refreshInterval` if secrets don't change often
- Store multiple values in one secret (JSON format)

### 5.11 Monitoring

#### CloudWatch Metrics

Set up alerts for secret access:

```bash
aws cloudwatch put-metric-alarm \
  --alarm-name lookout-secrets-access-spike \
  --alarm-description "Alert on unusual secret access" \
  --metric-name GetSecretValue \
  --namespace AWS/SecretsManager \
  --statistic Sum \
  --period 300 \
  --evaluation-periods 1 \
  --threshold 100 \
  --comparison-operator GreaterThanThreshold
```

#### ExternalSecret Status

Monitor ExternalSecret sync status:

```bash
# Get sync status
kubectl get externalsecret -n staging \
  -o custom-columns=NAME:.metadata.name,STATUS:.status.conditions[0].type,REASON:.status.conditions[0].reason

# Should show:
# NAME                                         STATUS  REASON
# lookout-staging-lookout-app-external        Ready   SecretSynced
```

### 5.12 References

- [External Secrets Operator Documentation](https://external-secrets.io/)
- [AWS Secrets Manager Documentation](https://docs.aws.amazon.com/secretsmanager/)
- [Kubernetes Secrets](https://kubernetes.io/docs/concepts/configuration/secret/)
- [IAM Roles for EC2](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/iam-roles-for-amazon-ec2.html)

---

## Operations Guide

### Access the Cluster

```bash
# SSH to EC2
ssh ubuntu@10.0.3.142

# Check cluster status
kubectl cluster-info
kubectl get nodes

# Check all namespaces
kubectl get ns

# Check gateways
kubectl get gateway -A
```

### View Logs

```bash
# Application logs
kubectl logs -n staging -l app.kubernetes.io/name=lookout-app --tail=100 -f

# Envoy Gateway logs
kubectl logs -n envoy-gateway-system -l gateway.envoyproxy.io/owning-gateway-name=lookout-staging --tail=100 -f

# ArgoCD logs
kubectl logs -n argocd -l app.kubernetes.io/name=argocd-server --tail=100 -f

# Dgraph logs
kubectl logs -n staging -l app=dgraph-alpha --tail=100 -f
```

### Debug Pods

```bash
# Get pod name
kubectl get pods -n staging

# Describe pod
kubectl describe pod -n staging <pod-name>

# View pod events
kubectl get events -n staging --sort-by=.metadata.creationTimestamp

# Port forward to service
kubectl port-forward -n staging svc/lookout-staging-lookout-app 3000:3000
```

### Scale Deployments

```bash
# Scale up
kubectl scale deployment lookout-staging-lookout-app -n staging --replicas=3

# Scale down
kubectl scale deployment lookout-staging-lookout-app -n staging --replicas=1

# Check rollout status
kubectl rollout status deployment/lookout-staging-lookout-app -n staging
```

### Update Image

ArgoCD handles this automatically, but for manual updates:

```bash
# Update image tag
kubectl set image deployment/lookout-staging-lookout-app -n staging \
  lookout-app=ghcr.io/timoniersystems/lookout:v1.2.3

# Rollback if needed
kubectl rollout undo deployment/lookout-staging-lookout-app -n staging
```

### Manage Secrets

```bash
# List secrets
kubectl get secrets -n staging

# View secret (base64 encoded)
kubectl get secret lookout-tls -n staging -o yaml

# Create TLS secret
kubectl create secret tls lookout-tls-new \
  --cert=path/to/cert.pem \
  --key=path/to/key.pem \
  -n staging

# Delete secret
kubectl delete secret lookout-tls-old -n staging
```

### Health Checks

```bash
# Quick health check script
cat > ~/check-k8s.sh <<'EOF'
#!/bin/bash
echo "=== Cluster ==="
kubectl get nodes
echo ""
echo "=== Gateways ==="
kubectl get gateway -A
echo ""
echo "=== Staging Pods ==="
kubectl get pods -n staging
echo ""
echo "=== ArgoCD Applications ==="
kubectl get application -n argocd
echo ""
echo "=== Envoy Gateway ==="
kubectl get pods -n envoy-gateway-system
EOF

chmod +x ~/check-k8s.sh
~/check-k8s.sh
```

### Restart Services

```bash
# Restart all pods in namespace
kubectl rollout restart deployment -n staging

# Restart specific deployment
kubectl rollout restart deployment/lookout-staging-lookout-app -n staging

# Delete and recreate Gateway
kubectl delete gateway lookout-staging -n staging
kubectl apply -f k8s/argocd/staging-application.yaml
```

---

## Troubleshooting

### Gateway Not Accepting Connections

**Symptoms:**
- 502 Bad Gateway
- Connection refused
- Gateway not programmed

**Checks:**

```bash
# 1. Check Gateway status
kubectl describe gateway lookout-staging -n staging

# 2. Check TLS secret exists
kubectl get secret lookout-tls -n staging

# 3. Check Envoy Gateway pods
kubectl get pods -n envoy-gateway-system

# 4. Check Envoy logs
kubectl logs -n envoy-gateway-system -l gateway.envoyproxy.io/owning-gateway-name=lookout-staging

# 5. Verify EnvoyProxy configuration
kubectl get envoyproxy -n staging
kubectl describe envoyproxy custom-proxy-config -n staging

# 6. Check service has fixed NodePorts
kubectl get svc -n envoy-gateway-system | grep envoy-staging
```

**Solutions:**

```bash
# Recreate TLS secret
kubectl delete secret lookout-tls -n staging
./scripts/setup-gateway.sh

# Recreate EnvoyProxy with fixed NodePorts
./scripts/setup-fixed-nodeports.sh

# Recreate Gateway
kubectl delete gateway lookout-staging -n staging
# ArgoCD will recreate it, or manually:
kubectl apply -f helm/lookout/templates/gateway.yaml
```

### Application Not Syncing

**Symptoms:**
- ArgoCD shows "OutOfSync"
- Pods not deploying
- Image pull errors

**Checks:**

```bash
# 1. Check application status
kubectl describe application lookout-staging -n argocd

# 2. Check image pull secret
kubectl get secret ghcr-secret -n staging

# 3. Test image pull manually
kubectl run test-pull --image=ghcr.io/timoniersystems/lookout:main \
  --overrides='{"spec":{"imagePullSecrets":[{"name":"ghcr-secret"}]}}' \
  -n staging

# 4. Check repository credentials
kubectl get secret -n argocd | grep github

# 5. Check ArgoCD logs
kubectl logs -n argocd deployment/argocd-application-controller
```

**Solutions:**

```bash
# Recreate GitHub token (ensure 'repo' scope for private repos)
export GITHUB_TOKEN=<new-token>
./scripts/create-ghcr-secret.sh staging
./scripts/setup-argocd-github-repo.sh

# Manual sync
kubectl patch application lookout-staging -n argocd \
  --type merge -p '{"operation":{"sync":{}}}'

# Delete and recreate application
kubectl delete application lookout-staging -n argocd
kubectl apply -f k8s/argocd/staging-application.yaml
```

### Pods Not Starting

**Symptoms:**
- Pods in `Pending`, `CrashLoopBackOff`, or `ImagePullBackOff` state

**Checks:**

```bash
# 1. Describe the pod
kubectl describe pod -n staging <pod-name>

# 2. Check events
kubectl get events -n staging --sort-by=.metadata.creationTimestamp

# 3. Check logs (if pod started)
kubectl logs -n staging <pod-name>

# 4. Check resources
kubectl top nodes
kubectl top pods -n staging

# 5. Check image exists
docker pull ghcr.io/timoniersystems/lookout:main
```

**Common Issues:**

- **ImagePullBackOff:** Check `ghcr-secret`, verify token has `read:packages` scope
- **CrashLoopBackOff:** Check application logs, verify Dgraph is healthy
- **Pending:** Check node resources, verify PVCs are bound
- **CreateContainerConfigError:** Check secrets and configmaps exist

### NodePort Issues

**Symptoms:**
- ALB health checks failing
- Ports changed after Gateway recreation
- Cannot access via NodePort

**Checks:**

```bash
# 1. Verify fixed NodePorts are configured
kubectl get svc -n envoy-gateway-system | grep envoy-staging

# Expected: 8080:32080/TCP,8443:32443/TCP

# 2. Check EnvoyProxy configuration
kubectl get envoyproxy custom-proxy-config -n staging -o yaml

# 3. Test NodePorts from EC2
curl -H "Host: lookout-stg.timonier.io" http://localhost:32080/health
curl -k -H "Host: lookout-stg.timonier.io" https://localhost:32443/health

# 4. Check Kind port mappings
docker ps | grep lookout-control-plane
```

**Solutions:**

```bash
# Recreate EnvoyProxy with fixed NodePorts
./scripts/setup-fixed-nodeports.sh

# Recreate Gateway
kubectl delete gateway lookout-staging -n staging
kubectl apply -f k8s/argocd/staging-application.yaml

# Verify NodePorts
kubectl get svc -n envoy-gateway-system -l gateway.envoyproxy.io/owning-gateway-name=lookout-staging
```

### DNS/Routing Issues

**Symptoms:**
- Cannot resolve hostname
- Wrong backend receiving traffic
- HTTPRoute not working

**Checks:**

```bash
# 1. Check HTTPRoute configuration
kubectl get httproute -n staging
kubectl describe httproute lookout -n staging

# 2. Verify hostname in HTTPRoute
kubectl get httproute lookout -n staging -o jsonpath='{.spec.hostnames}'

# 3. Check backend service exists
kubectl get svc lookout-staging-lookout-app -n staging

# 4. Test with Host header
curl -H "Host: lookout-stg.timonier.io" http://localhost:32080/health

# 5. Check Envoy routing configuration
kubectl logs -n envoy-gateway-system -l gateway.envoyproxy.io/owning-gateway-name=lookout-staging | grep -i route
```

**Solutions:**

```bash
# Update HTTPRoute hostname
kubectl edit httproute lookout -n staging

# Recreate HTTPRoute
kubectl delete httproute lookout -n staging
# ArgoCD will recreate it

# Verify service endpoints
kubectl get endpoints lookout-staging-lookout-app -n staging
```

### Dgraph Connection Issues

**Symptoms:**
- Application can't connect to Dgraph
- GraphQL queries failing
- Database timeouts

**Checks:**

```bash
# 1. Check Dgraph pods
kubectl get pods -n staging -l app=dgraph-alpha
kubectl get pods -n staging -l app=dgraph-zero

# 2. Check Dgraph health
kubectl exec -n staging deploy/lookout-staging-dgraph-alpha -- \
  curl http://localhost:8080/health

# 3. Test from application pod
kubectl exec -n staging deployment/lookout-staging-lookout-app -- \
  nc -zv lookout-staging-dgraph-alpha 9080

# 4. Check service
kubectl get svc -n staging | grep dgraph

# 5. View Dgraph logs
kubectl logs -n staging -l app=dgraph-alpha
```

**Solutions:**

```bash
# Restart Dgraph
kubectl rollout restart statefulset/lookout-staging-dgraph-alpha -n staging
kubectl rollout restart statefulset/lookout-staging-dgraph-zero -n staging

# Check configuration
kubectl get configmap -n staging
kubectl describe configmap lookout-staging-dgraph -n staging
```

### ArgoCD UI Not Accessible

**Symptoms:**
- Cannot access ArgoCD UI via port-forward
- Login fails
- Certificate errors

**Checks:**

```bash
# 1. Check ArgoCD server pod
kubectl get pods -n argocd | grep server

# 2. Check service
kubectl get svc argocd-server -n argocd

# 3. Test locally on EC2
kubectl port-forward svc/argocd-server -n argocd 8080:443 &
curl -k https://localhost:8080

# 4. Get admin password
kubectl -n argocd get secret argocd-initial-admin-secret \
  -o jsonpath="{.data.password}" | base64 -d
```

**Solutions:**

```bash
# Reset admin password
kubectl -n argocd patch secret argocd-secret \
  -p '{"stringData": {"admin.password": "'$(htpasswd -nbBC 10 "" newpassword | tr -d ':\n' | sed 's/$2y/$2a/')'"}}'

# Restart ArgoCD server
kubectl rollout restart deployment/argocd-server -n argocd

# Check logs
kubectl logs -n argocd deployment/argocd-server
```

### Complete Cluster Reset

If all else fails, reset the cluster:

```bash
# 1. Delete kind cluster
kind delete cluster --name lookout

# 2. Recreate cluster
kind create cluster --config /tmp/kind-cluster-config.yaml

# 3. Reinstall components
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.2.1/standard-install.yaml

helm install eg oci://docker.io/envoyproxy/gateway-helm \
  --version v1.3.0 \
  --namespace envoy-gateway-system \
  --create-namespace

kubectl create namespace staging
./scripts/setup-gateway.sh
./scripts/setup-fixed-nodeports.sh
./scripts/setup-argocd.sh
./scripts/create-ghcr-secret.sh staging
./scripts/setup-argocd-github-repo.sh

kubectl apply -f k8s/argocd/staging-application.yaml
```

---

## References

- [Kind Documentation](https://kind.sigs.k8s.io/)
- [Gateway API Documentation](https://gateway-api.sigs.k8s.io/)
- [Envoy Gateway Documentation](https://gateway.envoyproxy.io/)
- [ArgoCD Documentation](https://argo-cd.readthedocs.io/)
- [GitHub Container Registry](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry)

---

## Quick Reference

### Useful Commands

```bash
# Cluster status
kubectl get nodes,namespaces,gateway -A,pods -A

# Application status
kubectl get application -n argocd
kubectl get pods,svc,httproute -n staging

# Logs
kubectl logs -n staging -l app.kubernetes.io/name=lookout-app -f
kubectl logs -n envoy-gateway-system -l gateway.envoyproxy.io/owning-gateway-name=lookout-staging -f

# Health checks
curl -H "Host: lookout-stg.timonier.io" http://localhost:32080/health
curl -k -H "Host: lookout-stg.timonier.io" https://localhost:32443/health

# ArgoCD access
ssh -L 8080:localhost:8080 ubuntu@10.0.3.142 \
  'kubectl port-forward svc/argocd-server -n argocd 8080:443'

# Sync application
kubectl patch application lookout-staging -n argocd \
  --type merge -p '{"operation":{"sync":{}}}'
```

### Port Mappings

| Service | Port | Protocol | Description |
|---------|------|----------|-------------|
| Gateway HTTP | 32080 | HTTP | Fixed NodePort for HTTP |
| Gateway HTTPS | 32443 | HTTPS | Fixed NodePort for HTTPS |
| ArgoCD UI | 8080 | HTTPS | Via port-forward |
| Lookout App | 3000 | HTTP | Internal ClusterIP |
| Dgraph Alpha | 9080 | gRPC | Internal ClusterIP |
| Dgraph Zero | 5080 | TCP | Internal ClusterIP |

### Credentials

**ArgoCD:**
- Username: `admin`
- Password: Get from setup script output (change after first login!)

**GitHub:**
- Token scopes: `read:packages` (public repos) or `repo` (private repos)

---

**Setup Date:** 2026-02-15
**Cluster:** lookout (Kind v0.27.0, K8s v1.32.2)
**Location:** EC2 instance 10.0.3.142
