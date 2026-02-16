# AWS Application Load Balancer Setup for Lookout Staging

This guide explains how to configure an AWS Application Load Balancer (ALB) to route traffic for `lookout-stg.timonier.io` to the Kind cluster running on EC2.

## Overview

The traffic flow will be:
```
Internet → Route53 (lookout-stg.timonier.io) → ALB → EC2 Instance → Kind Cluster → Envoy Gateway → Lookout App
```

## Prerequisites

- AWS account with appropriate IAM permissions
- EC2 instance at `<EC2_INSTANCE_IP>` with Kind cluster running
- **Envoy Gateway configured with fixed NodePorts** (see [GATEWAY_SETUP.md](GATEWAY_SETUP.md))
  - HTTP NodePort: `32080`
  - HTTPS NodePort: `32443`
- Domain `timonier.io` managed in Route53 (or ability to create DNS records)
- SSL certificate for `*.timonier.io` or `lookout-stg.timonier.io` in AWS Certificate Manager

**Important:** Before configuring the ALB, ensure the Gateway is set up with fixed NodePorts by running:
```bash
ssh ubuntu@<EC2_INSTANCE_IP> 'cd lookout && ./scripts/setup-fixed-nodeports.sh'
```

## Step 1: Configure Fixed NodePorts for Envoy Gateway

The Envoy Gateway has been configured with **fixed NodePorts** to ensure they don't change when the Gateway is recreated:

- **HTTP NodePort:** `32080` (Gateway port 8080)
- **HTTPS NodePort:** `32443` (Gateway port 8443)

These ports are configured via the `scripts/setup-fixed-nodeports.sh` script and will remain stable across Gateway recreations.

To verify the NodePorts:

```bash
ssh ubuntu@<EC2_INSTANCE_IP> 'kubectl get svc -n envoy-gateway-system -l gateway.envoyproxy.io/owning-gateway-name=lookout-staging'
```

Expected output:
```
NAME                                     TYPE           PORT(S)
envoy-staging-lookout-staging-*          LoadBalancer   8080:32080/TCP,8443:32443/TCP
```

**Note:** If you need to reconfigure the Gateway or NodePorts, see [GATEWAY_SETUP.md](GATEWAY_SETUP.md) for detailed instructions.

## Step 2: Create Target Groups

You'll need **two target groups** - one for HTTP and one for HTTPS:

### HTTP Target Group

1. **Navigate to EC2 Console** → **Target Groups** → **Create target group**

2. **Basic Configuration:**
   - Target type: `Instances`
   - Target group name: `lookout-staging-http-tg`
   - Protocol: `HTTP`
   - Port: `32080` (fixed HTTP NodePort)
   - VPC: Select the VPC where your EC2 instance is running

3. **Health Check Settings:**
   - Health check protocol: `HTTP`
   - Health check path: `/health`
   - Advanced health check settings:
     - Healthy threshold: `2`
     - Unhealthy threshold: `2`
     - Timeout: `5 seconds`
     - Interval: `10 seconds`
     - Success codes: `200`

4. **Register Targets:**
   - Select your EC2 instance
   - Port: `32080`
   - Click "Include as pending below"
   - Click "Create target group"

### HTTPS Target Group

Repeat the above steps with:
- Target group name: `lookout-staging-https-tg`
- Protocol: `HTTPS` (or `HTTP` if Envoy Gateway terminates TLS)
- Port: `32443` (fixed HTTPS NodePort)
- Health check path: `/health`
- Register same EC2 instance on port `32443`

**Note:** The Envoy Gateway terminates TLS, so you can use HTTP protocol for the target group even though it's on the HTTPS port. The connection from ALB to the target uses the NodePort (32080/32443).

## Step 3: Configure Security Groups

### EC2 Instance Security Group

Ensure the security group attached to your EC2 instance allows traffic from the ALB on both NodePorts:

```
Inbound Rules:
- Type: Custom TCP
  Port: 32080
  Source: <ALB Security Group>
  Description: Allow HTTP traffic from ALB

- Type: Custom TCP
  Port: 32443
  Source: <ALB Security Group>
  Description: Allow HTTPS traffic from ALB
```

### Create ALB Security Group

1. Navigate to **EC2** → **Security Groups** → **Create security group**

2. **Basic details:**
   - Security group name: `lookout-alb-sg`
   - Description: Security group for Lookout ALB
   - VPC: Same VPC as EC2 instance

3. **Inbound rules:**
   ```
   - Type: HTTP
     Port: 80
     Source: 0.0.0.0/0
     Description: Allow HTTP from anywhere

   - Type: HTTPS
     Port: 443
     Source: 0.0.0.0/0
     Description: Allow HTTPS from anywhere
   ```

4. **Outbound rules:**
   ```
   - Type: All traffic
     Destination: 0.0.0.0/0
   ```

## Step 4: Request or Import SSL Certificate

### Option A: Use AWS Certificate Manager (ACM)

1. Navigate to **Certificate Manager** → **Request certificate**
2. Choose "Request a public certificate"
3. Domain names:
   - `lookout-stg.timonier.io` (or `*.timonier.io` for wildcard)
4. Validation method: DNS validation (recommended)
5. Follow ACM instructions to add DNS validation records to Route53
6. Wait for certificate to be issued (usually a few minutes)

### Option B: Import Existing Certificate

If you already have an SSL certificate:
1. Navigate to **Certificate Manager** → **Import certificate**
2. Provide certificate body, private key, and certificate chain

## Step 5: Create Application Load Balancer

1. **Navigate to EC2 Console** → **Load Balancers** → **Create Load Balancer**

2. **Choose Load Balancer Type:** Application Load Balancer

3. **Basic Configuration:**
   - Load balancer name: `lookout-staging-alb`
   - Scheme: `Internet-facing`
   - IP address type: `IPv4`

4. **Network Mapping:**
   - VPC: Select the same VPC as your EC2 instance
   - Availability Zones: Select at least 2 AZs for high availability
   - Select public subnets in each AZ

5. **Security Groups:**
   - Select the `lookout-alb-sg` security group created in Step 3
   - Remove the default security group

6. **Listeners and Routing:**

   **HTTP Listener (Port 80):**
   - Protocol: HTTP
   - Port: 80
   - Default action: Redirect to HTTPS
     - Protocol: HTTPS
     - Port: 443
     - Status code: 301 (Permanent redirect)

   **HTTPS Listener (Port 443):**
   - Protocol: HTTPS
   - Port: 443
   - Default SSL/TLS certificate: Select the certificate from Step 4
   - Default action: Forward to `lookout-staging-https-tg`

   **Note:** You can also forward HTTP traffic directly to `lookout-staging-http-tg` instead of redirecting, depending on your requirements.

7. **Review and Create**

## Step 6: Configure DNS in Route53

1. **Navigate to Route53** → **Hosted zones** → Select `timonier.io`

2. **Create Record:**
   - Record name: `lookout-stg`
   - Record type: `A - Routes traffic to an IPv4 address`
   - Alias: Yes
   - Route traffic to:
     - Alias to Application and Classic Load Balancer
     - Region: Select your region
     - Load balancer: Select `lookout-staging-alb`
   - Routing policy: Simple routing
   - Click "Create records"

## Step 7: Configure Listener Rules (Optional - for path-based routing)

If you need more sophisticated routing:

1. **Navigate to Load Balancers** → Select `lookout-staging-alb` → **Listeners** tab

2. **Select HTTPS:443** → **View/edit rules**

3. **Add rules** for specific paths or headers:
   ```
   IF:
     - Host header is lookout-stg.timonier.io
   THEN:
     - Forward to lookout-staging-tg
   ```

## Step 8: Verify the Setup

1. **Check Target Health:**
   ```bash
   # In AWS Console
   EC2 → Target Groups → lookout-staging-http-tg → Targets tab
   EC2 → Target Groups → lookout-staging-https-tg → Targets tab
   # Both should show "healthy"
   ```

2. **Test DNS Resolution:**
   ```bash
   nslookup lookout-stg.timonier.io
   # Should return ALB's IP addresses
   ```

3. **Test HTTPS Access:**
   ```bash
   curl -I https://lookout-stg.timonier.io
   # Should return 200 OK or redirect
   ```

4. **Test in Browser:**
   - Open https://lookout-stg.timonier.io in your browser
   - Should load the Lookout application

## Troubleshooting

### Target is Unhealthy

1. Check EC2 security group allows traffic from ALB on ports 32080 and 32443
2. Verify the Gateway service is running with fixed NodePorts:
   ```bash
   ssh ubuntu@<EC2_INSTANCE_IP> 'kubectl get svc -n envoy-gateway-system'
   ```
3. Check health check path is accessible:
   ```bash
   # Test HTTP NodePort
   ssh ubuntu@<EC2_INSTANCE_IP> 'curl -I -H "Host: lookout-stg.timonier.io" http://localhost:32080/health'

   # Test HTTPS NodePort
   ssh ubuntu@<EC2_INSTANCE_IP> 'curl -I -k -H "Host: lookout-stg.timonier.io" https://localhost:32443/health'
   ```
4. Check Kind cluster status:
   ```bash
   ssh ubuntu@<EC2_INSTANCE_IP> 'kubectl get pods -n staging'
   ```

### SSL Certificate Issues

1. Ensure certificate covers `lookout-stg.timonier.io`
2. Check certificate status in ACM (must be "Issued")
3. Verify DNS validation records exist in Route53

### 502 Bad Gateway

1. Check target health in target group
2. Verify Envoy Gateway is running:
   ```bash
   ssh ubuntu@<EC2_INSTANCE_IP> 'kubectl get pods -n envoy-gateway-system'
   ```
3. Check HTTPRoute configuration:
   ```bash
   ssh ubuntu@<EC2_INSTANCE_IP> 'kubectl get httproute -n staging -o yaml'
   ```

### DNS Not Resolving

1. Wait 5-10 minutes for DNS propagation
2. Check Route53 record is correctly configured
3. Verify ALB is in "active" state
4. Use `dig` to debug:
   ```bash
   dig lookout-stg.timonier.io
   ```

## Fixed NodePorts: Why They Matter

The Envoy Gateway is configured with **fixed NodePorts** (32080 and 32443) to ensure:

1. **Stable ALB Configuration:** NodePorts don't change when the Gateway is recreated
2. **No Target Group Updates:** You don't need to reconfigure ALB target groups
3. **Predictable Port Mapping:** Always know which ports to allow in security groups

This configuration is managed by:
- `scripts/setup-fixed-nodeports.sh` - Creates EnvoyProxy with fixed ports
- `helm/lookout/values.staging.yaml` - References the EnvoyProxy infrastructure config
- `k8s/gateway/envoyproxy-config.yaml` - EnvoyProxy resource definition

If you need to change the NodePorts, update the `HTTP_NODEPORT` and `HTTPS_NODEPORT` environment variables when running the setup script:

```bash
HTTP_NODEPORT=32090 HTTPS_NODEPORT=32453 ./scripts/setup-fixed-nodeports.sh
```

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                           Internet                               │
└───────────────────────────────┬─────────────────────────────────┘
                                │
                ┌───────────────▼────────────────┐
                │  Route53: lookout-stg.timonier.io  │
                └───────────────┬────────────────┘
                                │
                ┌───────────────▼────────────────┐
                │   Application Load Balancer    │
                │  - HTTPS Listener (443)        │
                │  - HTTP Redirect (80→443)      │
                │  - SSL/TLS Termination         │
                └───────────────┬────────────────┘
                                │
                ┌───────────────▼────────────────┐
                │  Target Groups                 │
                │  - HTTP TG → Port 32080        │
                │  - HTTPS TG → Port 32443       │
                │  Health Check: /health         │
                └───────────────┬────────────────┘
                                │
                ┌───────────────▼────────────────┐
                │  EC2 Instance: <EC2_INSTANCE_IP>      │
                │  ┌──────────────────────────┐  │
                │  │  Kind Cluster            │  │
                │  │  ┌────────────────────┐  │  │
                │  │  │ Envoy Gateway      │  │  │
                │  │  │ HTTP: :32080       │  │  │
                │  │  │ HTTPS: :32443      │  │  │
                │  │  └─────────┬──────────┘  │  │
                │  │            │              │  │
                │  │  ┌─────────▼──────────┐  │  │
                │  │  │  staging namespace │  │  │
                │  │  │  - HTTPRoute       │  │  │
                │  │  │  - Lookout App     │  │  │
                │  │  │  - Dgraph          │  │  │
                │  │  └────────────────────┘  │  │
                │  └──────────────────────────┘  │
                └─────────────────────────────────┘
```

## Cost Considerations

- **Application Load Balancer:** ~$16-20/month + data processing charges
- **ACM Certificate:** Free for public certificates
- **Route53:** $0.50/month per hosted zone + $0.40 per million queries

## Next Steps

After the ALB is configured:

1. Set up monitoring with CloudWatch for ALB metrics
2. Configure ALB access logs to S3 for audit trail
3. Set up alerts for unhealthy targets
4. Consider enabling WAF for additional security
5. Configure proper backup and disaster recovery for the Kind cluster data
