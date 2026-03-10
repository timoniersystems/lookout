#!/bin/bash
set -e

# Setup ArgoCD Image Updater
# This automatically updates images when new versions are pushed to ghcr.io

echo "🔄 Setting up ArgoCD Image Updater..."

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
    echo "❌ kubectl is not installed"
    exit 1
fi

# Install ArgoCD Image Updater
echo "📥 Installing ArgoCD Image Updater..."
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj-labs/argocd-image-updater/stable/manifests/install.yaml

# Wait for Image Updater to be ready
echo "⏳ Waiting for Image Updater to be ready..."
kubectl wait --for=condition=available --timeout=120s \
    deployment/argocd-image-updater -n argocd

echo "✅ ArgoCD Image Updater installed successfully!"
echo ""
echo "📝 Image Updater is configured via annotations on the Application:"
echo "   - argocd-image-updater.argoproj.io/image-list: specifies which images to track"
echo "   - argocd-image-updater.argoproj.io/update-strategy: how to update (digest, latest-tag, etc)"
echo ""
echo "Example annotations are already included in staging-application.yaml"
