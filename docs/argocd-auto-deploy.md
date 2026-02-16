# ArgoCD Auto-Deploy Setup

## Overview

ArgoCD is now configured to automatically deploy new Docker images when they are pushed to GHCR, even if they use the same tag (e.g., `main`). This is achieved using ArgoCD Image Updater, which watches the container registry for new image digests.

## How It Works

1. **GitHub Actions** builds and pushes a new Docker image to GHCR with tag `main`
2. **ArgoCD Image Updater** (runs every 2 minutes) checks GHCR for new image digests
3. When a new digest is detected, Image Updater updates the Application spec with the new digest
4. **ArgoCD** (with auto-sync enabled) automatically deploys the new image to Kubernetes

## Components Installed

### ArgoCD Image Updater
- **Namespace**: `argocd`
- **Deployment**: `argocd-image-updater`
- **Check Interval**: 2 minutes
- **Strategy**: `digest` - watches for changes in image SHA256 digest

### Configuration
```yaml
# Application annotations
argocd-image-updater.argoproj.io/image-list: lookout=ghcr.io/timoniersystems/lookout:main
argocd-image-updater.argoproj.io/lookout.helm.image-name: lookout-app.image.repository
argocd-image-updater.argoproj.io/lookout.helm.image-tag: lookout-app.image.tag
argocd-image-updater.argoproj.io/lookout.force-update: "true"
argocd-image-updater.argoproj.io/lookout.update-strategy: digest
argocd-image-updater.argoproj.io/lookout.pull-secret: pullsecret:argocd/ghcr-secret
argocd-image-updater.argoproj.io/write-back-method: argocd

# Auto-sync policy
spec.syncPolicy.automated:
  prune: true
  selfHeal: true
  allowEmpty: false
```

## Verification

### Check Image Updater Status
```bash
# Check if Image Updater is running
kubectl get pods -n argocd -l app=argocd-image-updater

# View Image Updater logs
kubectl logs -n argocd -l app=argocd-image-updater --tail=100

# Look for lines like:
# "Setting new image to ghcr.io/timoniersystems/lookout:main@sha256:..."
# "Successfully updated the live application spec"
```

### Check Current Image Digest
```bash
# Check what image is deployed
kubectl get deployment -n staging lookout-staging-lookout-app \
  -o jsonpath='{.spec.template.spec.containers[0].image}'

# Should show something like:
# ghcr.io/timoniersystems/lookout:main@sha256:32ae185ebc207a2fe9881570a51ad1d9247a49ec69ed6daa1fd4036153123b59
```

### Check Application Sync Status
```bash
# Check if auto-sync is working
kubectl get application lookout-staging -n argocd \
  -o jsonpath='{.status.sync.status}' && echo

# Should show: Synced
```

### Check Application Health
```bash
kubectl get application lookout-staging -n argocd \
  -o jsonpath='{.status.health.status}' && echo

# Should show: Healthy (or Progressing during deployment)
```

## Testing Auto-Deploy

### Trigger a New Build
1. Push code changes to the `main` branch
2. GitHub Actions will build and push a new Docker image
3. Wait up to 2 minutes for Image Updater to detect the change
4. ArgoCD will automatically deploy the new image

### Force Image Updater Check
```bash
# Restart Image Updater to trigger immediate check
kubectl rollout restart deployment argocd-image-updater -n argocd

# Watch the logs
kubectl logs -n argocd -l app=argocd-image-updater -f
```

### Monitor Deployment
```bash
# Watch application status
kubectl get application lookout-staging -n argocd -w

# Watch pods for rolling update
kubectl get pods -n staging -l app.kubernetes.io/name=lookout-app -w

# Check pod events
kubectl describe pod -n staging -l app.kubernetes.io/name=lookout-app
```

## Troubleshooting

### Image Updater Not Detecting Changes
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

### Application Not Auto-Syncing
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

### Check Application Differences
```bash
# View what would be synced
kubectl get application lookout-staging -n argocd \
  -o jsonpath='{.status.sync.revision}'

# Force refresh
kubectl patch application lookout-staging -n argocd \
  --type merge -p '{"metadata":{"annotations":{"argocd.argoproj.io/refresh":"hard"}}}'
```

## Configuration Files

- Image Updater deployment: `/tmp/argocd-image-updater-install.yaml` (on EC2)
- Image Updater RBAC: `/tmp/argocd-image-updater-rbac-fix.yaml` (on EC2)

## Timeline

The complete deployment timeline after pushing code:
1. **0-5 min**: GitHub Actions builds and pushes Docker image
2. **0-2 min**: Image Updater detects new digest (next check cycle)
3. **0-1 min**: ArgoCD syncs the change
4. **1-2 min**: Kubernetes rolling update completes

**Total time**: Approximately 5-10 minutes from code push to deployment

## Security

- Image Updater uses digest verification for image integrity
- GHCR credentials are stored in Kubernetes secrets
- Auto-sync includes `prune: true` to remove orphaned resources
- `selfHeal: true` ensures desired state is maintained

## Secrets

The following secrets are required:
- `ghcr-secret` in namespace `argocd` - GHCR credentials for Image Updater
- `ghcr-secret` in namespace `staging` - GHCR credentials for pulling images
- `argocd-image-updater-secret` in namespace `argocd` - ArgoCD API token

These secrets are already configured and should not need manual updates.
