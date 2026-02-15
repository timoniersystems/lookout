# Lookout Deployment Guide

This guide explains how to deploy Lookout to the EC2 kind cluster.

## Prerequisites

- SSH access to EC2 instance at `<EC2_INSTANCE_IP>`
- Git repository with committed changes
- For production: Git tag on the commit you want to deploy

## Deployment Environments

### Staging
- **Namespace**: `staging`
- **Image Tag**: `main`
- **Source**: Current main branch
- **Replicas**: 2
- **Resource Limits**: Lower than production

### Production
- **Namespace**: `production`
- **Image Tag**: Git tag (e.g., `v1.0.0`)
- **Source**: Tagged commit
- **Replicas**: 3
- **Resource Limits**: Production-grade

## Deployment Script

The [`../scripts/deploy.sh`](../scripts/deploy.sh) script automates the entire deployment process.

**Prerequisites:** Set the `EC2_HOST` environment variable:

```bash
export EC2_HOST=<EC2_USER>@<EC2_INSTANCE_IP>
```

### Deploy to Staging

```bash
# Make sure you're on main branch
git checkout main
git pull

# Deploy to staging
./scripts/deploy.sh staging
```

Or set EC2_HOST inline:

```bash
EC2_HOST=ubuntu@<EC2_INSTANCE_IP> ./scripts/deploy.sh staging
```

This will:
1. Sync code to EC2
2. Build Docker image
3. Tag as `registry:5000/lookout:main`
4. Push to local registry
5. Deploy to `staging` namespace with Helm

### Deploy to Production

```bash
# Create a git tag for the release
git tag -a v1.0.0 -m "Release version 1.0.0"
git push origin v1.0.0

# Checkout the tag
git checkout v1.0.0

# Deploy to production (with EC2_HOST set)
./scripts/deploy.sh production
```

This will:
1. Verify git tag exists on current commit
2. Sync code to EC2
3. Build Docker image
4. Tag as `registry:5000/lookout:v1.0.0` and `registry:5000/lookout:latest`
5. Push to local registry
6. Deploy to `production` namespace with Helm

## Manual Deployment Steps

If you need to deploy manually:

### 1. Sync Code

```bash
rsync -avz --delete \
    --exclude '.git/' \
    --exclude 'dgraph/' \
    --exclude 'vendor/' \
    ./ <EC2_USER>@<EC2_INSTANCE_IP>:lookout/
```

### 2. Build Image on EC2

```bash
ssh <EC2_USER>@<EC2_INSTANCE_IP> "cd lookout && docker compose build"
```

### 3. Tag and Push to Registry

For staging (main):
```bash
ssh <EC2_USER>@<EC2_INSTANCE_IP> "cd lookout && \
    docker tag lookout-lookout:latest localhost:5000/lookout:main && \
    docker push localhost:5000/lookout:main"
```

For production (version tag):
```bash
VERSION=v1.0.0
ssh <EC2_USER>@<EC2_INSTANCE_IP> "cd lookout && \
    docker tag lookout-lookout:latest localhost:5000/lookout:${VERSION} && \
    docker push localhost:5000/lookout:${VERSION}"
```

### 4. Deploy with Helm

Staging:
```bash
ssh <EC2_USER>@<EC2_INSTANCE_IP> "cd lookout && \
    helm upgrade --install lookout-staging ./helm/lookout \
        -f ./helm/lookout/values.staging.yaml \
        --set lookout-app.image.tag=main \
        -n staging --create-namespace --wait"
```

Production:
```bash
VERSION=v1.0.0
ssh <EC2_USER>@<EC2_INSTANCE_IP> "cd lookout && \
    helm upgrade --install lookout-production ./helm/lookout \
        -f ./helm/lookout/values.production.yaml \
        --set lookout-app.image.tag=${VERSION} \
        -n production --create-namespace --wait"
```

## Verification

### Check Pods

```bash
# Staging
ssh <EC2_USER>@<EC2_INSTANCE_IP> 'kubectl get pods -n staging'

# Production
ssh <EC2_USER>@<EC2_INSTANCE_IP> 'kubectl get pods -n production'
```

### Check Services

```bash
# Staging
ssh <EC2_USER>@<EC2_INSTANCE_IP> 'kubectl get svc -n staging'

# Production
ssh <EC2_USER>@<EC2_INSTANCE_IP> 'kubectl get svc -n production'
```

### View Logs

```bash
# Staging
ssh <EC2_USER>@<EC2_INSTANCE_IP> 'kubectl logs -n staging -l app=lookout -f'

# Production
ssh <EC2_USER>@<EC2_INSTANCE_IP> 'kubectl logs -n production -l app=lookout -f'
```

### Test Health Endpoint

```bash
# Get the service endpoint and test
ssh <EC2_USER>@<EC2_INSTANCE_IP> 'kubectl get httproute -n staging'
```

## Rollback

To rollback to a previous version:

```bash
# List Helm releases
ssh <EC2_USER>@<EC2_INSTANCE_IP> 'helm list -n production'

# Rollback to previous release
ssh <EC2_USER>@<EC2_INSTANCE_IP> 'helm rollback lookout-production -n production'

# Or rollback to specific revision
ssh <EC2_USER>@<EC2_INSTANCE_IP> 'helm rollback lookout-production 2 -n production'
```

## Troubleshooting

### Image Pull Errors

If pods can't pull the image:
```bash
# Check if registry is running
ssh <EC2_USER>@<EC2_INSTANCE_IP> 'docker ps | grep registry'

# Check registry contents
ssh <EC2_USER>@<EC2_INSTANCE_IP> 'curl -s http://localhost:5000/v2/_catalog'

# Check specific image tags
ssh <EC2_USER>@<EC2_INSTANCE_IP> 'curl -s http://localhost:5000/v2/lookout/tags/list'
```

### Pod Not Starting

```bash
# Describe the pod
ssh <EC2_USER>@<EC2_INSTANCE_IP> 'kubectl describe pod -n staging <pod-name>'

# Check events
ssh <EC2_USER>@<EC2_INSTANCE_IP> 'kubectl get events -n staging --sort-by=.metadata.creationTimestamp'
```

### Database Connection Issues

```bash
# Check dgraph pods
ssh <EC2_USER>@<EC2_INSTANCE_IP> 'kubectl get pods -n staging -l app=dgraph'

# Check dgraph logs
ssh <EC2_USER>@<EC2_INSTANCE_IP> 'kubectl logs -n staging -l app=dgraph-alpha'
```

## Local Registry Information

- **Registry URL**: `localhost:5000` (from EC2)
- **Registry URL (from kind)**: `registry:5000`
- **Container Name**: `registry`
- **Auto-restart**: Enabled
- **Connected to**: `kind` network
- **TLS**: HTTPS with self-signed certificate

### Registry Setup

The local Docker registry is configured with HTTPS using a self-signed certificate. The setup process is documented in the [`../scripts/setup-registry.sh`](../scripts/setup-registry.sh) script.

To set up the registry (if not already configured):

```bash
# On EC2 instance
cd lookout
./scripts/setup-registry.sh
```

The script will:
1. Create certificates directory
2. Generate self-signed TLS certificate
3. Start registry container with HTTPS
4. Connect registry to kind network
5. Install certificate in kind nodes
6. Restart containerd to recognize the certificate

## Git Tagging Strategy

### Semantic Versioning

Use semantic versioning for production releases:
- `v1.0.0` - Major version (breaking changes)
- `v1.1.0` - Minor version (new features)
- `v1.1.1` - Patch version (bug fixes)

### Creating Tags

```bash
# Create annotated tag
git tag -a v1.0.0 -m "Release version 1.0.0"

# Push tag to remote
git push origin v1.0.0

# List all tags
git tag -l

# Delete tag if needed
git tag -d v1.0.0
git push origin --delete v1.0.0
```
