#!/bin/bash
set -e

# Create GitHub Container Registry (ghcr.io) pull secret
# This allows Kubernetes to pull images from ghcr.io

NAMESPACE="${1:-staging}"

if [[ -z "$GITHUB_USERNAME" ]] || [[ -z "$GITHUB_TOKEN" ]]; then
    echo "❌ Error: Required environment variables not set"
    echo ""
    echo "Usage:"
    echo "  export GITHUB_USERNAME=<your-github-username>"
    echo "  export GITHUB_TOKEN=<your-github-token>"
    echo "  $0 [namespace]"
    echo ""
    echo "To create a GitHub token:"
    echo "  1. Go to https://github.com/settings/tokens"
    echo "  2. Generate new token (classic)"
    echo "  3. Select 'read:packages' scope"
    exit 1
fi

echo "🔐 Creating ghcr.io pull secret in namespace: $NAMESPACE"

# Create namespace if it doesn't exist
kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -

# Create docker-registry secret
kubectl create secret docker-registry ghcr-secret \
    --docker-server=ghcr.io \
    --docker-username="$GITHUB_USERNAME" \
    --docker-password="$GITHUB_TOKEN" \
    --docker-email="$GITHUB_USERNAME@users.noreply.github.com" \
    --namespace="$NAMESPACE" \
    --dry-run=client -o yaml | kubectl apply -f -

echo "✅ Secret 'ghcr-secret' created in namespace '$NAMESPACE'"
echo ""
echo "📝 To use this secret in your deployments, add:"
echo "   imagePullSecrets:"
echo "     - name: ghcr-secret"
