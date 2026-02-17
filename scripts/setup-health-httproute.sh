#!/bin/bash
# Setup health check HTTPRoute for ALB integration
# This creates an HTTPRoute that accepts /health requests without hostname restrictions
# Required because ALB health checks don't send the Host header

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "🔧 Setting up health check HTTPRoute for ALB integration"
echo ""

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
    echo -e "${RED}ERROR: kubectl not found${NC}"
    exit 1
fi

# Check if lookout Gateway exists
if ! kubectl get gateway lookout -n staging &> /dev/null; then
    echo -e "${RED}ERROR: Gateway 'lookout' not found in staging namespace${NC}"
    echo "Make sure the Gateway is deployed first"
    exit 1
fi

echo "Creating health check HTTPRoute..."

cat <<EOF | kubectl apply -f -
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: lookout-health
  namespace: staging
  labels:
    app: lookout
    component: health-check
spec:
  parentRefs:
  - name: lookout
  rules:
  - matches:
    - path:
        type: Exact
        value: /health
    backendRefs:
    - name: lookout-lookout-app
      port: 3000
EOF

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Health check HTTPRoute created${NC}"
    echo ""
    echo "Testing health endpoint without Host header..."
    sleep 3

    # Get EC2 private IP
    EC2_IP=$(hostname -I | awk '{print $1}')

    # Test with HTTPS
    if curl -k -s -f https://${EC2_IP}:32443/health > /dev/null 2>&1; then
        echo -e "${GREEN}✓ Health endpoint accessible without Host header${NC}"
        curl -k -s https://${EC2_IP}:32443/health
    else
        echo -e "${YELLOW}⚠ Health endpoint test failed (may need to wait for route to propagate)${NC}"
    fi
else
    echo -e "${RED}✗ Failed to create health check HTTPRoute${NC}"
    exit 1
fi

echo ""
echo -e "${GREEN}✅ Health check HTTPRoute setup complete!${NC}"
echo ""
echo "This allows ALB health checks to work without sending the Host header"
echo "The HTTPRoute accepts requests to /health from any hostname"
