#!/bin/bash
set -e

# Sync code to EC2 instance
# Usage: EC2_HOST=ubuntu@<your-ec2-ip> ./scripts/sync-to-ec2.sh

EC2_HOST="${EC2_HOST:-}"
EC2_DIR="${EC2_DIR:-lookout}"

# Validate EC2_HOST is set
if [[ -z "$EC2_HOST" ]]; then
    echo "❌ Error: EC2_HOST environment variable is not set"
    echo ""
    echo "Usage:"
    echo "  EC2_HOST=ubuntu@<your-ec2-ip> $0"
    echo ""
    echo "Or export it first:"
    echo "  export EC2_HOST=ubuntu@<your-ec2-ip>"
    echo "  $0"
    exit 1
fi

echo "🔄 Syncing code to ${EC2_HOST}:${EC2_DIR}..."

# Sync code to EC2
rsync -avz --delete \
    --exclude '.git/' \
    --exclude 'dgraph/' \
    --exclude 'vendor/' \
    --exclude '.env' \
    --exclude 'nginx/certs/' \
    --exclude 'nginx/auth/.htpasswd' \
    ./ "${EC2_HOST}:${EC2_DIR}/"

echo "✅ Code synced successfully!"
echo ""
echo "📋 Next steps on EC2 instance:"
echo "   1. SSH: ssh ${EC2_HOST}"
echo "   2. Setup ArgoCD: cd ${EC2_DIR} && ./scripts/setup-argocd.sh"
