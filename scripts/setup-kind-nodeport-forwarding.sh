#!/bin/bash
# Setup Kind NodePort forwarding for ALB integration
# This script sets up port forwarding from EC2 host to Kind container
# Required because Kind NodePorts are not exposed to the host by default

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "🔧 Setting up Kind NodePort forwarding for ALB integration"
echo ""

# Get Kind container name
KIND_CONTAINER=$(docker ps --filter name=control-plane --format "{{.Names}}" | head -1)
if [ -z "$KIND_CONTAINER" ]; then
    echo -e "${RED}ERROR: Kind control plane container not found${NC}"
    echo "Make sure your Kind cluster is running"
    exit 1
fi

echo "Found Kind container: $KIND_CONTAINER"

# Get Kind container IP
KIND_IP=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' $KIND_CONTAINER)
if [ -z "$KIND_IP" ]; then
    echo -e "${RED}ERROR: Could not get Kind container IP${NC}"
    exit 1
fi

echo "Kind container IP: $KIND_IP"
echo ""

# Check if socat is installed
if ! command -v socat &> /dev/null; then
    echo "Installing socat..."
    sudo apt-get update && sudo apt-get install -y socat
fi

# Kill existing socat processes
echo "Stopping existing port forwarding processes..."
sudo pkill -f "socat.*32080" 2>/dev/null || true
sudo pkill -f "socat.*32443" 2>/dev/null || true
sleep 1

# Create port forwarding for 32080 (HTTP)
echo "Setting up port forwarding for 32080 (HTTP)..."
sudo nohup socat TCP4-LISTEN:32080,fork,reuseaddr TCP4:${KIND_IP}:32080 > /tmp/socat-32080.log 2>&1 &

# Create port forwarding for 32443 (HTTPS)
echo "Setting up port forwarding for 32443 (HTTPS)..."
sudo nohup socat TCP4-LISTEN:32443,fork,reuseaddr TCP4:${KIND_IP}:32443 > /tmp/socat-32443.log 2>&1 &

sleep 2

# Verify processes are running
if ps aux | grep -q "[s]ocat.*32080"; then
    echo -e "${GREEN}✓ Port 32080 forwarding active${NC}"
else
    echo -e "${RED}✗ Port 32080 forwarding failed${NC}"
    cat /tmp/socat-32080.log
fi

if ps aux | grep -q "[s]ocat.*32443"; then
    echo -e "${GREEN}✓ Port 32443 forwarding active${NC}"
else
    echo -e "${RED}✗ Port 32443 forwarding failed${NC}"
    cat /tmp/socat-32443.log
fi

echo ""
echo "Testing port forwarding..."

# Test HTTP port (32080)
if curl -s -f -H "Host: lookout-stg.timonier.io" http://localhost:32080/health > /dev/null 2>&1; then
    echo -e "${GREEN}✓ HTTP port 32080 is accessible${NC}"
else
    echo -e "${YELLOW}⚠ HTTP port 32080 test failed (may be normal if app not yet deployed)${NC}"
fi

# Test HTTPS port (32443)
if curl -s -f -k -H "Host: lookout-stg.timonier.io" https://localhost:32443/health > /dev/null 2>&1; then
    echo -e "${GREEN}✓ HTTPS port 32443 is accessible${NC}"
else
    echo -e "${YELLOW}⚠ HTTPS port 32443 test failed (may be normal if app not yet deployed)${NC}"
fi

echo ""
echo -e "${GREEN}✅ NodePort forwarding setup complete!${NC}"
echo ""
echo "Port forwarding processes:"
ps aux | grep "[s]ocat.*3204"
echo ""
echo "To stop port forwarding:"
echo "  sudo pkill -f 'socat.*32080'"
echo "  sudo pkill -f 'socat.*32443'"
echo ""
echo "To make this persistent across reboots, add to /etc/rc.local or create a systemd service"
