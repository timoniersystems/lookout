#!/bin/bash
set -e

# Setup ArgoCD on kind cluster
# This script installs ArgoCD and configures it for the lookout application

echo "🚀 Setting up ArgoCD on kind cluster..."

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
    echo "❌ kubectl is not installed"
    exit 1
fi

# Check if we're connected to the kind cluster
if ! kubectl cluster-info | grep -q "kind-lookout"; then
    echo "❌ Not connected to kind-lookout cluster"
    echo "Run: kubectl config use-context kind-lookout"
    exit 1
fi

# Create argocd namespace
echo "📦 Creating argocd namespace..."
kubectl create namespace argocd --dry-run=client -o yaml | kubectl apply -f -

# Install ArgoCD
echo "📥 Installing ArgoCD..."
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml

# Wait for ArgoCD to be ready
echo "⏳ Waiting for ArgoCD to be ready..."
kubectl wait --for=condition=available --timeout=300s \
    deployment/argocd-server -n argocd

echo "✅ ArgoCD installed successfully!"

# Get initial admin password
echo ""
echo "📋 ArgoCD Admin Password:"
kubectl -n argocd get secret argocd-initial-admin-secret \
    -o jsonpath="{.data.password}" | base64 -d
echo ""
echo ""

# Instructions for accessing ArgoCD
echo "🌐 To access ArgoCD UI:"
echo "   1. Port forward: kubectl port-forward svc/argocd-server -n argocd 8080:443"
echo "   2. Open: https://localhost:8080"
echo "   3. Login: admin / <password above>"
echo ""
echo "🔧 To change password:"
echo "   argocd account update-password"
echo ""
echo "📦 Next steps:"
echo "   1. Create GitHub Container Registry secret: ./scripts/create-ghcr-secret.sh"
echo "   2. Deploy staging application: kubectl apply -f k8s/argocd/staging-application.yaml"
echo ""
