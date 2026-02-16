#!/bin/bash
set -e

# Setup ArgoCD GitHub Repository Access
# This script configures ArgoCD to access a private GitHub repository

echo "🔐 Setting up GitHub repository access for ArgoCD..."

# Validate environment variables
GITHUB_USERNAME="${GITHUB_USERNAME:-}"
GITHUB_TOKEN="${GITHUB_TOKEN:-}"
REPO_URL="${REPO_URL:-https://github.com/timoniersystems/lookout.git}"

if [[ -z "$GITHUB_USERNAME" ]]; then
    echo "❌ Error: GITHUB_USERNAME environment variable is not set"
    echo ""
    echo "Usage:"
    echo "  GITHUB_USERNAME=<your-username> GITHUB_TOKEN=<your-token> $0"
    echo ""
    echo "Or export them first:"
    echo "  export GITHUB_USERNAME=<your-username>"
    echo "  export GITHUB_TOKEN=<your-token>"
    echo "  $0"
    exit 1
fi

if [[ -z "$GITHUB_TOKEN" ]]; then
    echo "❌ Error: GITHUB_TOKEN environment variable is not set"
    echo ""
    echo "To create a GitHub token:"
    echo "  1. Go to https://github.com/settings/tokens"
    echo "  2. Click 'Generate new token (classic)'"
    echo "  3. Select scopes: 'repo' (full control of private repositories)"
    echo "  4. Copy the token"
    exit 1
fi

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
    echo "❌ kubectl is not installed"
    exit 1
fi

# Check if ArgoCD namespace exists
if ! kubectl get namespace argocd &> /dev/null; then
    echo "❌ ArgoCD namespace not found"
    echo "Run: ./scripts/setup-argocd.sh first"
    exit 1
fi

# Add repository credentials to ArgoCD
echo "📦 Adding repository credentials to ArgoCD..."
kubectl create secret generic github-repo-creds \
    --namespace argocd \
    --from-literal=username="$GITHUB_USERNAME" \
    --from-literal=password="$GITHUB_TOKEN" \
    --from-literal=url="$REPO_URL" \
    --dry-run=client -o yaml | kubectl apply -f -

# Label the secret so ArgoCD recognizes it as repository credentials
kubectl label secret github-repo-creds \
    --namespace argocd \
    argocd.argoproj.io/secret-type=repository \
    --overwrite

echo "✅ GitHub repository credentials configured successfully!"
echo ""
echo "📝 Repository configured:"
echo "   URL: $REPO_URL"
echo "   Username: $GITHUB_USERNAME"
echo ""
echo "🔄 ArgoCD will now be able to access this private repository"
echo ""
echo "📦 Next steps:"
echo "   1. Verify: kubectl get secret github-repo-creds -n argocd"
echo "   2. Deploy application: kubectl apply -f k8s/argocd/staging-application.yaml"
echo ""
