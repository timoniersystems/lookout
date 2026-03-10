#!/bin/bash
set -e

# Deployment script for Lookout to EC2 kind cluster
# Usage:
#   EC2_HOST=ubuntu@<your-ec2-ip> ./scripts/deploy.sh staging
#   EC2_HOST=ubuntu@<your-ec2-ip> ./scripts/deploy.sh production
#
# Or set environment variables:
#   export EC2_HOST=ubuntu@<your-ec2-ip>
#   ./scripts/deploy.sh staging

# Configuration
EC2_HOST="${EC2_HOST:-}"
EC2_DIR="${EC2_DIR:-lookout}"
REGISTRY="${REGISTRY:-localhost:5000}"
IMAGE_NAME="${IMAGE_NAME:-lookout}"

# Validate EC2_HOST is set
if [[ -z "$EC2_HOST" ]]; then
    echo "❌ Error: EC2_HOST environment variable is not set"
    echo ""
    echo "Usage:"
    echo "  EC2_HOST=ubuntu@<your-ec2-ip> $0 <staging|production>"
    echo ""
    echo "Or export it first:"
    echo "  export EC2_HOST=ubuntu@<your-ec2-ip>"
    echo "  $0 <staging|production>"
    exit 1
fi

ENVIRONMENT=$1

# Validate input
if [[ "$ENVIRONMENT" != "staging" && "$ENVIRONMENT" != "production" ]]; then
    echo "❌ Error: Environment must be 'staging' or 'production'"
    echo "Usage: EC2_HOST=<host> $0 <staging|production>"
    exit 1
fi

# Determine image tag based on environment
if [[ "$ENVIRONMENT" == "staging" ]]; then
    # Staging: use 'main' tag
    IMAGE_TAG="main"
    NAMESPACE="staging"
    HELM_RELEASE="lookout-staging"
    VALUES_FILE="values.staging.yaml"

    # Verify we're on main branch
    CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
    if [[ "$CURRENT_BRANCH" != "main" ]]; then
        echo "Warning: You're on branch '$CURRENT_BRANCH', not 'main'"
        read -p "Continue anyway? (y/N) " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            exit 1
        fi
    fi
else
    # Production: use git tag
    IMAGE_TAG=$(git describe --tags --exact-match 2>/dev/null || echo "")

    if [[ -z "$IMAGE_TAG" ]]; then
        echo "Error: No git tag found on current commit"
        echo "Production deployments require a git tag"
        echo "Create a tag with: git tag -a v1.0.0 -m 'Version 1.0.0'"
        exit 1
    fi

    NAMESPACE="production"
    HELM_RELEASE="lookout-production"
    VALUES_FILE="values.production.yaml"
fi

echo "========================================="
echo "Deploying to $ENVIRONMENT"
echo "Environment: $ENVIRONMENT"
echo "Image tag: $IMAGE_TAG"
echo "Namespace: $NAMESPACE"
echo "========================================="
echo

# Step 1: Rsync code to EC2
echo "📦 Step 1: Syncing code to EC2..."
rsync -avz --delete \
    --exclude '.git/' \
    --exclude 'dgraph/' \
    --exclude 'vendor/' \
    --exclude 'node_modules/' \
    --exclude '.env' \
    ./ ${EC2_HOST}:${EC2_DIR}/

echo "✅ Code synced to EC2"
echo

# Step 2: Build Docker image on EC2
echo "🔨 Step 2: Building Docker image on EC2..."
ssh ${EC2_HOST} "cd ${EC2_DIR} && docker compose build"

echo "✅ Docker image built"
echo

# Step 3: Tag and push to local registry
echo "🏷️  Step 3: Tagging and pushing to local registry..."
ssh ${EC2_HOST} "cd ${EC2_DIR} && \
    docker tag lookout-lookout:latest ${REGISTRY}/${IMAGE_NAME}:${IMAGE_TAG} && \
    docker tag lookout-lookout:latest ${REGISTRY}/${IMAGE_NAME}:latest && \
    docker push ${REGISTRY}/${IMAGE_NAME}:${IMAGE_TAG} && \
    docker push ${REGISTRY}/${IMAGE_NAME}:latest"

echo "✅ Image pushed to registry as ${REGISTRY}/${IMAGE_NAME}:${IMAGE_TAG}"
echo

# Step 4: Deploy with Helm
echo "🚀 Step 4: Deploying to Kubernetes with Helm..."
ssh ${EC2_HOST} "cd ${EC2_DIR} && \
    helm upgrade --install ${HELM_RELEASE} ./helm/lookout \
        -f ./helm/lookout/${VALUES_FILE} \
        --set lookout-app.image.tag=${IMAGE_TAG} \
        -n ${NAMESPACE} \
        --create-namespace \
        --wait \
        --timeout 5m"

echo "✅ Deployed to ${NAMESPACE} namespace"
echo

# Step 5: Verify deployment
echo "🔍 Step 5: Verifying deployment..."
ssh ${EC2_HOST} "kubectl get pods -n ${NAMESPACE} -l app=lookout"

echo
echo "========================================="
echo "✅ Deployment complete!"
echo "Environment: $ENVIRONMENT"
echo "Image: ${REGISTRY}/${IMAGE_NAME}:${IMAGE_TAG}"
echo "Namespace: $NAMESPACE"
echo "========================================="
echo
echo "To check status:"
echo "  ssh ${EC2_HOST} 'kubectl get pods -n ${NAMESPACE}'"
echo
echo "To view logs:"
echo "  ssh ${EC2_HOST} 'kubectl logs -n ${NAMESPACE} -l app=lookout -f'"
echo
echo "To get service info:"
echo "  ssh ${EC2_HOST} 'kubectl get svc -n ${NAMESPACE}'"
