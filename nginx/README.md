# Nginx Reverse Proxy with TLS

This directory contains nginx configuration for TLS termination and reverse proxy.

## Quick Start

### Development (Self-Signed Certificates)

```bash
# Generate self-signed certificates
make certs

# Start all services including nginx
make up

# Access via HTTPS (will show browser warning for self-signed cert)
open https://localhost:7443
```

### Production (Let's Encrypt)

For production, use Let's Encrypt with certbot:

```bash
# Install certbot
sudo apt-get install certbot python3-certbot-nginx  # Debian/Ubuntu
# or
brew install certbot  # macOS

# Get certificate (replace with your domain)
sudo certbot certonly --webroot \
  -w /var/www/certbot \
  -d yourdomain.com \
  -d www.yourdomain.com

# Copy certificates to nginx/certs/
sudo cp /etc/letsencrypt/live/yourdomain.com/fullchain.pem nginx/certs/cert.pem
sudo cp /etc/letsencrypt/live/yourdomain.com/privkey.pem nginx/certs/key.pem

# Update nginx/conf.d/lookout.conf with your domain
# Change: server_name localhost;
# To:     server_name yourdomain.com www.yourdomain.com;
```

## Configuration Files

- `nginx.conf` - Main nginx configuration (worker settings, gzip, etc.)
- `conf.d/lookout.conf` - Lookout-specific virtual host configuration
- `certs/` - TLS certificates (not committed to git)

## Security Features

✅ **TLS 1.2/1.3 only** - Modern protocol versions
✅ **Strong cipher suites** - ECDHE-based ciphers
✅ **HSTS** - HTTP Strict Transport Security with preload
✅ **Security headers** - X-Frame-Options, CSP, etc.
✅ **OCSP stapling** - Fast certificate validation
✅ **Rate limiting** - 10 requests/second per IP for uploads
✅ **HTTP → HTTPS redirect** - Automatic upgrade

## Performance Features

✅ **HTTP/2** - Enabled for better performance
✅ **Gzip compression** - Reduces bandwidth
✅ **Connection keepalive** - Reuse connections
✅ **Static file caching** - 1 year cache for /static/
✅ **Upstream keepalive** - Connection pooling to backend

## Endpoints

- **HTTP (Port 7080):** Redirects to HTTPS
- **HTTPS (Port 7443):** Main application
  - `/` - Home page
  - `/health` - Health check (no logging)
  - `/static/*` - Static files (cached)
  - `/upload` - Rate limited (10 req/s)

## Monitoring

Check nginx logs:
```bash
# Access logs
docker logs lookout-nginx

# Follow logs in real-time
docker logs -f lookout-nginx

# Check config syntax
docker exec lookout-nginx nginx -t

# Reload config without downtime
docker exec lookout-nginx nginx -s reload
```

## Troubleshooting

### Browser shows "Not Secure" warning
This is expected with self-signed certificates in development. For production, use Let's Encrypt.

### Connection refused
```bash
# Check if nginx is running
docker ps | grep nginx

# Check nginx logs
docker logs lookout-nginx

# Verify certificates exist
ls -la nginx/certs/
```

### 502 Bad Gateway
The backend (lookout-app) is not responding. Check:
```bash
# Is lookout-app running?
docker ps | grep lookout-app

# Check lookout-app logs
docker logs lookout-app

# Test via nginx (lookout-app uses distroless with no curl)
curl -k https://localhost:7443/health
```

## Alternative: Caddy (Easier Auto-TLS)

For simpler automatic TLS with Let's Encrypt, consider using Caddy instead of nginx:

```yaml
# docker-compose.yml
caddy:
  image: caddy:2-alpine
  ports:
    - "80:80"
    - "443:443"
  volumes:
    - ./Caddyfile:/etc/caddy/Caddyfile
    - caddy_data:/data
    - caddy_config:/config
```

```caddyfile
# Caddyfile
yourdomain.com {
    reverse_proxy lookout:3000

    # Automatic HTTPS with Let's Encrypt!
    tls {
        dns cloudflare {env.CLOUDFLARE_API_TOKEN}
    }
}
```

Caddy automatically:
- Gets Let's Encrypt certificates
- Renews certificates before expiry
- Redirects HTTP to HTTPS
- Sets security headers

## Cloud Alternatives

For cloud deployments, use managed load balancers instead:

- **AWS:** Application Load Balancer (ALB) + ACM certificates
- **GCP:** Cloud Load Balancing + Google-managed certificates
- **Azure:** Application Gateway + Key Vault certificates
- **Cloudflare:** Automatic TLS at the edge

These handle TLS termination, certificates, and renewal automatically.
