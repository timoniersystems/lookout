# Kubernetes Deployment Guide

Complete guide for deploying Lookout on Kubernetes with kind cluster, Gateway API, ArgoCD, and AWS ALB.

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Chapter 1: Kind Cluster Setup](#chapter-1-kind-cluster-setup)
- [Chapter 2: Gateway API Setup](#chapter-2-gateway-api-setup)
- [Chapter 3: ArgoCD GitOps Setup](#chapter-3-argocd-gitops-setup)
- [Chapter 4: AWS ALB Integration](#chapter-4-aws-alb-integration)
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

### 4.3 Step 1: Create Target Groups

You'll need **two target groups** - one for HTTP and one for HTTPS.

#### HTTP Target Group

1. Navigate to **EC2 Console** → **Target Groups** → **Create target group**

2. **Basic Configuration:**
   - Target type: `Instances`
   - Target group name: `lookout-staging-http-tg`
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
- Target group name: `lookout-staging-https-tg`
- Protocol: `HTTP` (Envoy Gateway terminates TLS)
- Port: `32443` (fixed HTTPS NodePort)
- Health check path: `/health`
- Register same EC2 instance on port `32443`

**Note:** Use HTTP protocol for the target group even though it's on the HTTPS port, because Envoy Gateway terminates TLS.

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
   - Load balancer name: `lookout-staging-alb`
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
   - Default action: Forward to `lookout-staging-https-tg`

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
     - Load balancer: Select `lookout-staging-alb`
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
export CERTIFICATE_ARN=arn:aws:acm:us-east-1:xxxxx:certificate/xxxxx
export HOSTED_ZONE_ID=Z0xxxxx

# Run the script
./scripts/setup-alb.sh
```

The script will:
- ✅ Verify fixed NodePorts are configured
- ✅ Create ALB and target security groups
- ✅ Create HTTP and HTTPS target groups (ports 32080, 32443)
- ✅ Register EC2 instance with target groups
- ✅ Create Application Load Balancer
- ✅ Configure HTTP→HTTPS redirect listener
- ✅ Configure HTTPS listener with ACM certificate
- ✅ Create Route53 A record (alias)
- ✅ Verify target health

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
  --name lookout-staging-http-tg \
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

# Create HTTPS target group
HTTPS_TG_ARN=$(aws elbv2 create-target-group \
  --name lookout-staging-https-tg \
  --protocol HTTP \
  --port 32443 \
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
  --name lookout-staging-alb \
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
# EC2 → Target Groups → lookout-staging-http-tg → Targets tab
# EC2 → Target Groups → lookout-staging-https-tg → Targets tab
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
