# TLS Setup Guide

Complete guide for setting up TLS/HTTPS with nginx reverse proxy.

## Architecture

```
Internet → Nginx (TLS Termination) → Lookout UI (HTTP)
           ↓
        Port 443/HTTPS              Port 3000/HTTP
        (Public)                    (Internal only)
```

**Benefits:**
- ✅ Separation of concerns (TLS vs application logic)
- ✅ Better performance (nginx optimized for TLS)
- ✅ Easier certificate management
- ✅ Additional features (rate limiting, caching, load balancing)
- ✅ Security headers automatically added
- ✅ HTTP → HTTPS redirect

---

## Quick Start

### 1. Development (Self-Signed Certificates)

```bash
# Generate development certificates
./scripts/generate-certs.sh

# Start all services
docker-compose up -d

# Access via HTTPS
open https://localhost:7443

# Or via HTTP (redirects to HTTPS)
open http://localhost:7080
```

**Note:** Browser will show security warning for self-signed certificates - this is expected.

### 2. Production (Let's Encrypt)

```bash
# Install certbot
sudo apt-get install certbot python3-certbot-nginx

# Get certificate
sudo certbot certonly --standalone \
  -d yourdomain.com \
  -d www.yourdomain.com \
  --email you@yourdomain.com \
  --agree-tos

# Copy certificates
sudo cp /etc/letsencrypt/live/yourdomain.com/fullchain.pem nginx/certs/cert.pem
sudo cp /etc/letsencrypt/live/yourdomain.com/privkey.pem nginx/certs/key.pem

# Update nginx config with your domain
sed -i 's/server_name localhost;/server_name yourdomain.com www.yourdomain.com;/' nginx/conf.d/lookout.conf

# Restart nginx
docker-compose restart nginx
```

---

## Security Features

### TLS Configuration

✅ **TLS 1.2 & 1.3 only** - Older versions disabled
✅ **Strong cipher suites** - ECDHE-based, forward secrecy
✅ **OCSP stapling** - Faster certificate validation
✅ **Session caching** - Improves TLS handshake performance

### Security Headers

All responses include:
- **HSTS** - Force HTTPS for 2 years + preload
- **X-Frame-Options** - Prevent clickjacking
- **X-Content-Type-Options** - Prevent MIME sniffing
- **X-XSS-Protection** - XSS filter
- **Referrer-Policy** - Control referrer information
- **CSP** - Content Security Policy

### Rate Limiting

- **Upload endpoints:** 10 requests/second per IP
- **Burst:** Up to 5 requests
- **Status:** Returns 429 (Too Many Requests)

---

## Performance Features

✅ **HTTP/2** - Multiplexing, header compression
✅ **Gzip compression** - Reduces bandwidth by ~70%
✅ **Connection pooling** - Keepalive to backend
✅ **Static file caching** - 1 year cache headers
✅ **No buffering** - Real-time responses

---

## Endpoints

| URL | Description | Features |
|-----|-------------|----------|
| `http://localhost:7080/` | Redirects to HTTPS | 301 Permanent Redirect |
| `https://localhost:7443/` | Home page | Full app access |
| `https://localhost:7443/health` | Health check | No logging, fast response |
| `https://localhost:7443/ready` | Readiness probe | Kubernetes compatible |
| `https://localhost:7443/static/*` | Static files | 1 year cache |
| `https://localhost:7443/upload` | Upload endpoint | Rate limited |

---

## Testing

### Check TLS Configuration

```bash
# Test HTTPS endpoint
curl -k https://localhost:7443/health

# Check security headers
curl -k -I https://localhost:7443/ | grep -E "Strict-Transport|X-Frame|X-Content"

# Test HTTP redirect
curl -I http://localhost:7080/

# Check TLS version and cipher
openssl s_client -connect localhost:7443 -tls1_3

# SSL Labs test (production only)
# https://www.ssllabs.com/ssltest/analyze.html?d=yourdomain.com
```

### Performance Testing

```bash
# Load test with ab (Apache Bench)
ab -n 1000 -c 10 https://localhost:7443/health

# Check compression
curl -k -H "Accept-Encoding: gzip" -I https://localhost:7443/

# Response time
curl -k -w "@curl-format.txt" -o /dev/null -s https://localhost:7443/
```

Create `curl-format.txt`:
```
    time_namelookup:  %{time_namelookup}s\n
       time_connect:  %{time_connect}s\n
    time_appconnect:  %{time_appconnect}s\n
   time_pretransfer:  %{time_pretransfer}s\n
      time_redirect:  %{time_redirect}s\n
 time_starttransfer:  %{time_starttransfer}s\n
                    ----------\n
         time_total:  %{time_total}s\n
```

---

## Certificate Management

### Automated Renewal (Let's Encrypt)

Add to crontab:
```bash
# Check for renewal twice daily
0 0,12 * * * certbot renew --quiet --post-hook "docker-compose -f /path/to/lookout/docker-compose.yml restart nginx"
```

Or use systemd timer:
```bash
sudo systemctl enable certbot-renew.timer
sudo systemctl start certbot-renew.timer
```

### Manual Renewal

```bash
# Renew certificates
sudo certbot renew

# Copy new certificates
sudo cp /etc/letsencrypt/live/yourdomain.com/fullchain.pem nginx/certs/cert.pem
sudo cp /etc/letsencrypt/live/yourdomain.com/privkey.pem nginx/certs/key.pem

# Reload nginx (no downtime)
docker exec lookout-nginx nginx -s reload
```

---

## Monitoring

### Check Nginx Status

```bash
# Is nginx running?
docker ps | grep nginx

# Check nginx logs
docker logs -f lookout-nginx

# Access logs only
docker logs lookout-nginx 2>/dev/null

# Error logs only
docker logs lookout-nginx 2>&1 >/dev/null

# Check config syntax
docker exec lookout-nginx nginx -t

# Reload config (no downtime)
docker exec lookout-nginx nginx -s reload
```

### Metrics

```bash
# Request rate
docker logs lookout-nginx | grep -oP '\d{2}/\w+/\d{4}:\d{2}:\d{2}' | uniq -c

# Status codes
docker logs lookout-nginx | grep -oP ' \d{3} ' | sort | uniq -c

# Top IPs
docker logs lookout-nginx | grep -oP '^\d+\.\d+\.\d+\.\d+' | sort | uniq -c | sort -rn | head -10
```

---

## Troubleshooting

### Connection Refused

```bash
# Check if nginx is listening
docker exec lookout-nginx netstat -tlnp

# Verify port bindings
docker port lookout-nginx

# Test from inside nginx container
docker exec lookout-nginx wget -O- http://lookout:3000/health
```

### 502 Bad Gateway

Backend (lookout-app) is down or unreachable:
```bash
# Check backend
docker ps | grep lookout-app
docker logs lookout-app

# Test backend directly
docker exec lookout-nginx wget -O- http://lookout:3000/health

# Check network
docker network inspect lookout_lookout-net | grep lookout
```

### Certificate Errors

```bash
# Check certificates exist
ls -la nginx/certs/

# Verify certificate
openssl x509 -in nginx/certs/cert.pem -text -noout

# Check private key
openssl rsa -in nginx/certs/key.pem -check

# Verify cert matches key
openssl x509 -noout -modulus -in nginx/certs/cert.pem | openssl md5
openssl rsa -noout -modulus -in nginx/certs/key.pem | openssl md5
# (should match)
```

### Performance Issues

```bash
# Check worker connections
docker exec lookout-nginx grep -E "worker_processes|worker_connections" /etc/nginx/nginx.conf

# Monitor active connections
docker exec lookout-nginx sh -c "while true; do netstat -an | grep :443 | wc -l; sleep 1; done"

# Check for errors
docker logs lookout-nginx | grep -i error
```

---

## Alternatives to Nginx

### Caddy (Easiest - Auto TLS)

```yaml
caddy:
  image: caddy:2-alpine
  ports:
    - "80:80"
    - "443:443"
  volumes:
    - ./Caddyfile:/etc/caddy/Caddyfile
    - caddy_data:/data
```

```caddyfile
yourdomain.com {
    reverse_proxy lookout:3000
    # Automatic HTTPS!
}
```

**Pros:** Automatic Let's Encrypt, simple config
**Cons:** Less battle-tested than nginx

### Traefik (Best for Microservices)

```yaml
traefik:
  image: traefik:v2.10
  command:
    - "--api.insecure=true"
    - "--providers.docker=true"
    - "--entrypoints.web.address=:80"
    - "--entrypoints.websecure.address=:443"
    - "--certificatesresolvers.le.acme.email=you@example.com"
    - "--certificatesresolvers.le.acme.storage=/letsencrypt/acme.json"
    - "--certificatesresolvers.le.acme.httpchallenge.entrypoint=web"

lookout:
  labels:
    - "traefik.http.routers.lookout.rule=Host(`yourdomain.com`)"
    - "traefik.http.routers.lookout.tls.certresolver=le"
```

**Pros:** Service discovery, automatic HTTPS
**Cons:** More complex setup

### Cloud Load Balancers (Production)

**AWS:**
```bash
# Application Load Balancer + ACM
aws elbv2 create-load-balancer \
  --name lookout-alb \
  --subnets subnet-xxx \
  --security-groups sg-xxx

aws acm request-certificate \
  --domain-name yourdomain.com \
  --validation-method DNS
```

**GCP:**
```bash
# Google Cloud Load Balancing
gcloud compute ssl-certificates create lookout-cert \
  --domains=yourdomain.com

gcloud compute target-https-proxies create lookout-proxy \
  --ssl-certificates=lookout-cert
```

**Pros:** Managed certificates, auto-renewal, DDoS protection
**Cons:** Vendor lock-in, cost

---

## Best Practices Checklist

✅ Use TLS 1.2/1.3 only
✅ Strong cipher suites with forward secrecy
✅ HSTS with preload
✅ Automated certificate renewal
✅ Monitor certificate expiry
✅ Security headers (CSP, X-Frame-Options, etc.)
✅ Rate limiting on upload endpoints
✅ HTTP → HTTPS redirect
✅ Regular security updates (nginx image)
✅ Log monitoring
✅ Performance tuning (keepalive, gzip)

---

## Security Hardening

### Additional Headers

Add to `nginx/conf.d/lookout.conf`:
```nginx
# Additional security
add_header Permissions-Policy "geolocation=(), microphone=(), camera=()" always;
add_header X-Permitted-Cross-Domain-Policies "none" always;
```

### Disable Unnecessary Methods

```nginx
if ($request_method !~ ^(GET|HEAD|POST)$ ) {
    return 444;
}
```

### IP Whitelisting (if needed)

```nginx
geo $allowed_ip {
    default 0;
    1.2.3.4 1;  # Allow specific IP
    10.0.0.0/8 1;  # Allow private network
}

server {
    if ($allowed_ip = 0) {
        return 403;
    }
}
```

---

## Resources

- [Mozilla SSL Configuration Generator](https://ssl-config.mozilla.org/)
- [SSL Labs Server Test](https://www.ssllabs.com/ssltest/)
- [Security Headers Check](https://securityheaders.com/)
- [Let's Encrypt Documentation](https://letsencrypt.org/docs/)
- [Nginx Documentation](https://nginx.org/en/docs/)
