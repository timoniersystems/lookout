#!/bin/bash
set -e

# Setup Envoy Gateway and TLS certificates
# This script configures Gateway API with Envoy Gateway and creates self-signed TLS certificates

NAMESPACE="${NAMESPACE:-staging}"
DOMAIN="${DOMAIN:-lookout-stg.timonier.io}"
CERT_DAYS="${CERT_DAYS:-365}"

echo "🚀 Setting up Envoy Gateway and TLS certificates..."

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
    echo "❌ kubectl is not installed"
    exit 1
fi

# Check if openssl is available
if ! command -v openssl &> /dev/null; then
    echo "❌ openssl is not installed"
    exit 1
fi

# Create GatewayClass if it doesn't exist
echo "📦 Creating GatewayClass 'eg'..."
cat <<EOF | kubectl apply -f -
apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata:
  name: eg
spec:
  controllerName: gateway.envoyproxy.io/gatewayclass-controller
EOF

# Generate self-signed TLS certificate
echo "🔐 Generating self-signed TLS certificate for ${DOMAIN}..."
TMP_DIR=$(mktemp -d)
trap "rm -rf ${TMP_DIR}" EXIT

openssl req -x509 -newkey rsa:4096 \
  -keyout "${TMP_DIR}/tls.key" \
  -out "${TMP_DIR}/tls.crt" \
  -days ${CERT_DAYS} \
  -nodes \
  -subj "/CN=${DOMAIN}/O=Timonier Systems" \
  -addext "subjectAltName=DNS:${DOMAIN},DNS:*.${DOMAIN}" \
  2>/dev/null

# Create or update TLS secret
echo "📝 Creating TLS secret 'lookout-tls' in namespace ${NAMESPACE}..."
kubectl create namespace ${NAMESPACE} --dry-run=client -o yaml | kubectl apply -f -

kubectl create secret tls lookout-tls \
  --cert="${TMP_DIR}/tls.crt" \
  --key="${TMP_DIR}/tls.key" \
  --namespace=${NAMESPACE} \
  --dry-run=client -o yaml | kubectl apply -f -

echo "✅ Gateway setup completed successfully!"
echo ""
echo "📋 Certificate Details:"
echo "   Domain: ${DOMAIN}"
echo "   Wildcard: *.${DOMAIN}"
echo "   Valid for: ${CERT_DAYS} days"
echo "   Secret: lookout-tls (namespace: ${NAMESPACE})"
echo ""
echo "🔍 Verify setup:"
echo "   kubectl get gatewayclass eg"
echo "   kubectl get secret lookout-tls -n ${NAMESPACE}"
echo "   kubectl get gateway -n ${NAMESPACE}"
echo ""
echo "📦 Next steps:"
echo "   1. Deploy Gateway: kubectl apply -f helm/lookout/templates/gateway.yaml"
echo "   2. Check Gateway status: kubectl get gateway -n ${NAMESPACE}"
echo "   3. Access via NodePort or configure LoadBalancer"
echo ""
