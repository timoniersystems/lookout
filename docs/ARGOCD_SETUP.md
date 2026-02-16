# ArgoCD Setup Guide

This guide explains how to set up ArgoCD on the kind cluster to automatically deploy Lookout from GitHub Container Registry.

## Architecture

```
┌─────────────────────────────────────────────────┐
│  GitHub Actions (CI/CD)                         │
│  - Builds Docker image                          │
│  - Pushes to ghcr.io/timoniersystems/lookout   │
│  - Tags: main, sha, latest                     │
└────────────────┬────────────────────────────────┘
                 │
                 │ Push image
                 ▼
┌─────────────────────────────────────────────────┐
│  GitHub Container Registry (ghcr.io)            │
│  - Stores Docker images                         │
└────────────────┬────────────────────────────────┘
                 │
                 │ Pull image
                 ▼
┌─────────────────────────────────────────────────┐
│  Kind Cluster on EC2 (10.0.3.142)              │
│                                                 │
│  ┌──────────────────────────────────────────┐  │
│  │ ArgoCD                                   │  │
│  │  - Watches Git repo (main branch)        │  │
│  │  - Watches ghcr.io for new images        │  │
│  │  - Auto-syncs to staging namespace       │  │
│  └──────────────────────────────────────────┘  │
│                                                 │
│  ┌──────────────────────────────────────────┐  │
│  │ Staging Namespace                        │  │
│  │  - Lookout app (main tag)                │  │
│  │  - Dgraph database                       │  │
│  │  - Nginx proxy                           │  │
│  └──────────────────────────────────────────┘  │
└─────────────────────────────────────────────────┘
```

## Prerequisites

- SSH access to EC2 instance at `10.0.3.142`
- kubectl configured for kind-lookout cluster
- GitHub Personal Access Token with `read:packages` and `repo` scopes
- Helm charts already exist in `helm/lookout/`
- Envoy Gateway installed (for Gateway API support)

## Setup Steps

### 1. Setup Gateway API and TLS (Optional but Recommended)

If you want to use Gateway API with HTTPS support, run:

```bash
cd ~/lookout
./scripts/setup-gateway.sh
```

This will:
- Create the Envoy Gateway GatewayClass
- Generate a self-signed TLS certificate
- Create the `lookout-tls` secret in staging namespace

See [GATEWAY_SETUP.md](GATEWAY_SETUP.md) for detailed Gateway configuration.

### 2. Install ArgoCD

SSH to the EC2 instance and run:

```bash
cd ~/lookout
./scripts/setup-argocd.sh
```

This will:
- Install ArgoCD in the `argocd` namespace
- Wait for all pods to be ready
- Display the initial admin password

Save the admin password shown in the output!

### 3. Access ArgoCD UI (Optional)

From your local machine, create an SSH tunnel:

```bash
ssh -L 8080:localhost:8080 ubuntu@10.0.3.142 \
  'kubectl port-forward svc/argocd-server -n argocd 8080:443'
```

Then open https://localhost:8080 and login:
- Username: `admin`
- Password: (from setup step)

### 4. Create GitHub Container Registry Secret

On the EC2 instance, create a secret to pull images from ghcr.io:

```bash
# Set your GitHub credentials
export GITHUB_USERNAME=<your-github-username>
export GITHUB_TOKEN=<your-github-token>

# Create secret in staging namespace
./scripts/create-ghcr-secret.sh staging
```

**To create a GitHub token:**
1. Go to https://github.com/settings/tokens
2. Click "Generate new token (classic)"
3. Select scope: `read:packages`
4. Copy the token

### 5. Deploy ArgoCD Application

Deploy the staging application configuration:

```bash
kubectl apply -f k8s/argocd/staging-application.yaml
```

This creates an ArgoCD Application that:
- Watches the `main` branch of the GitHub repo
- Deploys the Helm chart from `helm/lookout/`
- Uses `values.staging.yaml` for configuration
- Pulls images from `ghcr.io/timoniersystems/lookout:main`
- Deploys to the `staging` namespace
- Auto-syncs when changes are detected

### 6. Install ArgoCD Image Updater (Optional)

For automatic image updates when new images are pushed:

```bash
./scripts/setup-argocd-image-updater.sh
```

This enables ArgoCD to automatically detect new images in ghcr.io and update the deployment.

## Verification

### Check ArgoCD Application Status

```bash
# List applications
kubectl get applications -n argocd

# Get application details
kubectl describe application lookout-staging -n argocd

# Check application health
kubectl get application lookout-staging -n argocd -o jsonpath='{.status.health.status}'
```

### Check Staging Deployment

```bash
# Check pods in staging namespace
kubectl get pods -n staging

# Check services
kubectl get svc -n staging

# Check logs
kubectl logs -n staging -l app=lookout -f
```

### Trigger Manual Sync

If you want to manually trigger a sync:

```bash
# Using kubectl
kubectl patch application lookout-staging -n argocd \
  --type merge -p '{"operation":{"initiatedBy":{"username":"admin"},"sync":{}}}'

# Or using ArgoCD CLI
argocd app sync lookout-staging
```

## Configuration

### Staging Application Settings

The staging application is configured in `k8s/argocd/staging-application.yaml`:

- **Repository**: `https://github.com/timoniersystems/lookout.git`
- **Branch**: `main`
- **Path**: `helm/lookout`
- **Values**: `values.staging.yaml`
- **Image**: `ghcr.io/timoniersystems/lookout:main`
- **Namespace**: `staging`

### Auto-Sync Policy

The application is configured with automated sync:
- **Prune**: Removes resources deleted from Git
- **Self-Heal**: Reverts manual changes
- **Retry**: Up to 5 attempts with exponential backoff

### Image Update Strategy

With ArgoCD Image Updater installed, the application:
- Checks for new images tagged `main` in ghcr.io
- Automatically updates the deployment when new images are found
- Uses the pull secret `ghcr-secret` for authentication

## Workflow

### Development Flow

1. **Developer pushes to `main` branch**
   ```bash
   git push origin main
   ```

2. **GitHub Actions builds and pushes image**
   - Runs tests
   - Builds Docker image
   - Pushes to `ghcr.io/timoniersystems/lookout:main`
   - Pushes to `ghcr.io/timoniersystems/lookout:<sha>`

3. **ArgoCD detects changes**
   - Git sync: Detects Helm chart changes
   - Image updater: Detects new image with `main` tag

4. **ArgoCD deploys to staging**
   - Applies Helm chart changes
   - Updates image to new version
   - Waits for health checks
   - Marks deployment as healthy

5. **Verify deployment**
   ```bash
   # Check in ArgoCD UI or
   kubectl get pods -n staging
   curl http://10.0.3.142:10080/health
   ```

## Troubleshooting

### Application Not Syncing

Check application status:
```bash
kubectl describe application lookout-staging -n argocd
```

Common issues:
- Image pull failures: Check `ghcr-secret` exists in staging namespace
- Helm errors: Validate `helm/lookout/values.staging.yaml`
- Git access: Ensure repo is public or credentials are configured

### Image Pull Errors

Verify the secret:
```bash
kubectl get secret ghcr-secret -n staging
```

Re-create if needed:
```bash
./scripts/create-ghcr-secret.sh staging
```

### Check ArgoCD Logs

```bash
# ArgoCD server logs
kubectl logs -n argocd deployment/argocd-server

# ArgoCD application controller logs
kubectl logs -n argocd deployment/argocd-application-controller

# Image updater logs (if installed)
kubectl logs -n argocd deployment/argocd-image-updater
```

### Force Sync

If auto-sync isn't working:
```bash
kubectl patch application lookout-staging -n argocd \
  --type merge -p '{"metadata":{"annotations":{"argocd.argoproj.io/sync-wave":"0"}}}'
```

## ArgoCD CLI (Optional)

Install the ArgoCD CLI for easier management:

```bash
# On macOS
brew install argocd

# On Linux
curl -sSL -o /usr/local/bin/argocd \
  https://github.com/argoproj/argo-cd/releases/latest/download/argocd-linux-amd64
chmod +x /usr/local/bin/argocd
```

Login to ArgoCD:
```bash
# Port forward first
kubectl port-forward svc/argocd-server -n argocd 8080:443 &

# Login
argocd login localhost:8080
```

Common CLI commands:
```bash
# List applications
argocd app list

# Get application details
argocd app get lookout-staging

# Sync application
argocd app sync lookout-staging

# View application history
argocd app history lookout-staging

# Rollback application
argocd app rollback lookout-staging <revision>
```

## Security Notes

1. **GitHub Token**: Store securely, rotate regularly
2. **ArgoCD Admin Password**: Change after initial setup
3. **Image Pull Secrets**: Keep GitHub token with minimal permissions (`read:packages` only)
4. **RBAC**: Consider setting up ArgoCD RBAC for multi-user access

## Next Steps

After ArgoCD is set up:

1. **Create production application**: Copy `staging-application.yaml` and modify for production
2. **Set up notifications**: Configure Slack/email notifications for deployment events
3. **Enable sync waves**: Add sync-wave annotations for ordered deployments
4. **Configure health checks**: Customize health checks for your applications
5. **Set up RBAC**: Create projects and users for team access

## References

- [ArgoCD Documentation](https://argo-cd.readthedocs.io/)
- [ArgoCD Image Updater](https://argocd-image-updater.readthedocs.io/)
- [GitHub Container Registry](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry)
