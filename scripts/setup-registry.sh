#!/bin/bash
set -e

# Setup HTTPS Docker Registry for kind cluster
# This script sets up a local Docker registry with HTTPS and configures
# the kind cluster to trust the registry's self-signed certificate

echo "========================================="
echo "Setting up HTTPS Docker Registry for kind"
echo "========================================="
echo

# Step 1: Stop and remove existing registry if present
echo "Step 1: Cleaning up existing registry..."
docker stop registry 2>/dev/null || true
docker rm registry 2>/dev/null || true
echo "✅ Cleanup complete"
echo

# Step 2: Create directory for certificates
echo "Step 2: Creating certificate directory..."
mkdir -p ~/registry/certs
echo "✅ Certificate directory created"
echo

# Step 3: Generate self-signed certificate
echo "Step 3: Generating self-signed certificate..."
openssl req -newkey rsa:4096 -nodes -sha256 \
  -keyout ~/registry/certs/domain.key \
  -x509 -days 365 \
  -out ~/registry/certs/domain.crt \
  -subj '/CN=registry' \
  -addext 'subjectAltName=DNS:registry,DNS:localhost,IP:127.0.0.1' 2>/dev/null

echo "✅ Certificate generated"
echo

# Step 4: Start registry with TLS
echo "Step 4: Starting HTTPS registry..."
docker run -d \
  --name registry \
  --restart=always \
  -p 5000:5000 \
  -v ~/registry/certs:/certs \
  -e REGISTRY_HTTP_TLS_CERTIFICATE=/certs/domain.crt \
  -e REGISTRY_HTTP_TLS_KEY=/certs/domain.key \
  registry:2

echo "✅ Registry started on port 5000 with HTTPS"
echo

# Step 5: Connect registry to kind network
echo "Step 5: Connecting registry to kind network..."
docker network connect kind registry 2>/dev/null || echo "Already connected to kind network"
echo "✅ Registry connected to kind network"
echo

# Step 6: Configure kind nodes to trust the registry certificate
echo "Step 6: Configuring kind nodes to trust registry certificate..."

CLUSTER_NAME="${1:-lookout}"  # Default to 'lookout' if not specified

for node in $(kind get nodes --name "$CLUSTER_NAME"); do
  echo "  Configuring node: $node"

  # Create directories
  docker exec "$node" mkdir -p /etc/docker/certs.d/registry:5000
  docker exec "$node" mkdir -p /etc/containerd/certs.d/registry:5000

  # Copy certificate to system trust store
  docker cp ~/registry/certs/domain.crt "$node":/usr/local/share/ca-certificates/registry.crt
  docker exec "$node" update-ca-certificates

  # Copy certificate for containerd
  docker cp ~/registry/certs/domain.crt "$node":/etc/containerd/certs.d/registry:5000/ca.crt

  echo "  ✅ Node $node configured"
done

echo "✅ All nodes configured"
echo

# Step 7: Restart containerd on kind nodes
echo "Step 7: Restarting containerd on kind nodes..."
for node in $(kind get nodes --name "$CLUSTER_NAME"); do
  echo "  Restarting containerd on: $node"
  docker exec "$node" sh -c 'pkill -9 containerd' || true
done

# Wait for cluster to recover
echo "  Waiting for cluster to recover..."
sleep 10

echo "✅ Containerd restarted"
echo

# Step 8: Verify registry is accessible
echo "Step 8: Verifying registry..."
sleep 2

if curl -sk https://localhost:5000/v2/_catalog > /dev/null 2>&1; then
  echo "✅ Registry is accessible"
  echo "   Catalog: $(curl -sk https://localhost:5000/v2/_catalog)"
else
  echo "❌ Warning: Registry might not be accessible yet"
fi

echo
echo "========================================="
echo "✅ Registry Setup Complete!"
echo "========================================="
echo
echo "Registry URL (from host): localhost:5000"
echo "Registry URL (from kind): registry:5000"
echo
echo "To push an image:"
echo "  docker tag <image> localhost:5000/<name>:<tag>"
echo "  docker push localhost:5000/<name>:<tag>"
echo
echo "To list images:"
echo "  curl -sk https://localhost:5000/v2/_catalog"
echo
echo "To list tags for an image:"
echo "  curl -sk https://localhost:5000/v2/<image>/tags/list"
echo
