#!/bin/bash
# Generate self-signed certificates for local development
# For production, use Let's Encrypt or proper CA-signed certificates

CERTS_DIR="nginx/certs"

echo "🔐 Generating self-signed TLS certificates for development..."

# Create certs directory if it doesn't exist
mkdir -p "$CERTS_DIR"

# Generate private key
openssl genrsa -out "$CERTS_DIR/key.pem" 2048

# Generate certificate
openssl req -new -x509 -sha256 \
    -key "$CERTS_DIR/key.pem" \
    -out "$CERTS_DIR/cert.pem" \
    -days 365 \
    -subj "/C=US/ST=State/L=City/O=Organization/OU=Department/CN=localhost"

echo "✅ Certificates generated in $CERTS_DIR/"
echo ""
echo "⚠️  Note: These are self-signed certificates for DEVELOPMENT ONLY"
echo "   Your browser will show a security warning - this is expected."
echo ""
echo "📌 For production, use:"
echo "   - Let's Encrypt (certbot)"
echo "   - Cloud provider certificates (AWS ACM, GCP, etc.)"
echo "   - CA-signed certificates"
