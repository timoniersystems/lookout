#!/bin/bash
set -e

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${BLUE}🔐 Setting up Basic Authentication for Staging${NC}\n"

# Configuration
AWS_REGION="${AWS_REGION:-us-west-2}"
SECRET_NAME="lookout/staging/basic-auth"
NAMESPACE="${NAMESPACE:-staging}"

# Step 1: Check prerequisites
echo -e "${YELLOW}📋 Step 1: Checking prerequisites${NC}"

if ! command -v htpasswd &> /dev/null; then
    echo -e "${RED}✗ htpasswd not found${NC}"
    echo "  Install it with:"
    echo "    Ubuntu/Debian: sudo apt-get install apache2-utils"
    echo "    macOS:         brew install httpd"
    exit 1
fi
echo -e "${GREEN}✓ htpasswd found${NC}"

if ! command -v aws &> /dev/null; then
    echo -e "${RED}✗ AWS CLI not found${NC}"
    exit 1
fi
echo -e "${GREEN}✓ AWS CLI found${NC}"

# Verify AWS credentials work
if ! aws sts get-caller-identity &> /dev/null; then
    echo -e "${RED}✗ AWS credentials not configured or expired${NC}"
    exit 1
fi
echo -e "${GREEN}✓ AWS credentials valid${NC}"
echo ""

# Step 2: Get username and password
echo -e "${YELLOW}🔑 Step 2: Generating credentials${NC}"

read -p "Enter username [staging]: " USERNAME
USERNAME="${USERNAME:-staging}"

# Check if password was passed via environment variable (for non-interactive use)
if [ -n "$BASIC_AUTH_PASSWORD" ]; then
    PASSWORD="$BASIC_AUTH_PASSWORD"
    echo "Using password from BASIC_AUTH_PASSWORD environment variable"
else
    read -s -p "Enter password: " PASSWORD
    echo ""
    if [ -z "$PASSWORD" ]; then
        echo -e "${RED}✗ Password cannot be empty${NC}"
        exit 1
    fi
    read -s -p "Confirm password: " PASSWORD_CONFIRM
    echo ""
    if [ "$PASSWORD" != "$PASSWORD_CONFIRM" ]; then
        echo -e "${RED}✗ Passwords do not match${NC}"
        exit 1
    fi
fi

# Generate SHA-hashed htpasswd entry (Envoy requires {SHA} format, not bcrypt)
HTPASSWD_ENTRY=$(htpasswd -nbs "$USERNAME" "$PASSWORD")
echo -e "${GREEN}✓ Generated htpasswd entry for user: ${USERNAME}${NC}"
echo ""

# Step 3: Store in AWS Secrets Manager
echo -e "${YELLOW}☁️  Step 3: Storing credentials in AWS Secrets Manager${NC}"

# Build the JSON payload - use python3 to safely escape special characters
HTPASSWD_JSON=$(python3 -c "import json,sys; print(json.dumps({'htpasswd': sys.stdin.read().strip()}))" <<< "$HTPASSWD_ENTRY")

if aws secretsmanager describe-secret --secret-id "${SECRET_NAME}" --region "${AWS_REGION}" &> /dev/null; then
    echo "Secret already exists, updating..."
    aws secretsmanager update-secret \
        --secret-id "${SECRET_NAME}" \
        --secret-string "${HTPASSWD_JSON}" \
        --region "${AWS_REGION}" > /dev/null
    echo -e "${GREEN}✓ Secret updated in AWS Secrets Manager${NC}"
else
    echo "Creating new secret..."
    aws secretsmanager create-secret \
        --name "${SECRET_NAME}" \
        --description "Basic auth credentials for Lookout staging" \
        --secret-string "${HTPASSWD_JSON}" \
        --region "${AWS_REGION}" > /dev/null
    echo -e "${GREEN}✓ Secret created in AWS Secrets Manager${NC}"
fi
echo ""

# Step 4: Verify the secret in AWS
echo -e "${YELLOW}✅ Step 4: Verifying secret in AWS${NC}"
STORED_VALUE=$(aws secretsmanager get-secret-value \
    --secret-id "${SECRET_NAME}" \
    --region "${AWS_REGION}" \
    --query 'SecretString' \
    --output text)
STORED_HTPASSWD=$(echo "$STORED_VALUE" | python3 -c "import sys,json; print(json.load(sys.stdin)['htpasswd'])")

if [ -n "$STORED_HTPASSWD" ]; then
    echo -e "${GREEN}✓ Secret verified in AWS Secrets Manager${NC}"
    echo "  Secret name: ${SECRET_NAME}"
    echo "  Username:    ${USERNAME}"
    echo "  Hash:        ${STORED_HTPASSWD:0:20}..."
else
    echo -e "${RED}✗ Failed to verify secret${NC}"
    exit 1
fi
echo ""

# Step 5: Sync to Kubernetes (if kubectl is available and cluster is reachable)
echo -e "${YELLOW}🔄 Step 5: Syncing to Kubernetes${NC}"

if command -v kubectl &> /dev/null && kubectl cluster-info &> /dev/null; then
    # Check if the ExternalSecret exists
    if kubectl get externalsecret -n "${NAMESPACE}" 2>/dev/null | grep -q basic-auth; then
        echo "ExternalSecret for basic-auth found, forcing sync..."
        # Delete the K8s secret to trigger immediate re-sync
        kubectl delete secret -n "${NAMESPACE}" -l "reconcile.external-secrets.io/managed=true" --field-selector "metadata.name=lookout-staging-basic-auth" 2>/dev/null || true
        echo "Waiting for sync..."
        sleep 10

        if kubectl get secret lookout-staging-basic-auth -n "${NAMESPACE}" &> /dev/null; then
            echo -e "${GREEN}✓ Secret synced to Kubernetes${NC}"
        else
            echo -e "${YELLOW}⚠ Secret not yet synced. It will sync within the configured refreshInterval (default: 1h)${NC}"
            echo "  Check status: kubectl get externalsecret -n ${NAMESPACE}"
        fi
    else
        echo -e "${YELLOW}⚠ ExternalSecret for basic-auth not found in cluster${NC}"
        echo "  Deploy the Helm chart first, then the secret will sync automatically."
        echo "  Or run: argocd app sync lookout-staging"
    fi
else
    echo -e "${YELLOW}⚠ kubectl not available or cluster not reachable${NC}"
    echo "  The secret will sync automatically when the ExternalSecret is deployed."
fi
echo ""

# Done
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}✅ Basic auth setup complete!${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo "📋 Access the staging site:"
echo "  Browser: https://lookout-stg.timonier.io (enter ${USERNAME} / your password)"
echo "  curl:    curl -u ${USERNAME}:PASSWORD https://lookout-stg.timonier.io/"
echo ""
echo "🔧 To update the password later:"
echo "  ./scripts/setup-basic-auth.sh"
echo ""
echo "🔧 To update non-interactively:"
echo "  BASIC_AUTH_PASSWORD='newpass' ./scripts/setup-basic-auth.sh"
echo ""
