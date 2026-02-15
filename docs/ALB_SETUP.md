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
- Domain `timonier.io` managed in Route53 (or ability to create DNS records)
- SSL certificate for `*.timonier.io` or `lookout-stg.timonier.io`

## Step 1: Identify the Envoy Gateway Service

First, find the NodePort or port that Envoy Gateway is exposing:

```bash
ssh ubuntu@<EC2_INSTANCE_IP> 'kubectl get svc -n envoy-gateway-system'
```

The Envoy Gateway should have a service that exposes ports for HTTP (80) and HTTPS (443). For Kind clusters, this will typically be a NodePort service.

If not already exposed, you may need to create a NodePort service or configure the gateway to use a specific port.

## Step 2: Create Target Group

1. **Navigate to EC2 Console** → **Target Groups** → **Create target group**

2. **Basic Configuration:**
   - Target type: `Instances`
   - Target group name: `lookout-staging-tg`
   - Protocol: `HTTP`
   - Port: Get the NodePort from Step 1 (likely `30000-32767` range) or use the Kind cluster's gateway port
   - VPC: Select the VPC where your EC2 instance is running

3. **Health Check Settings:**
   - Health check protocol: `HTTP`
   - Health check path: `/health` (or another health endpoint exposed by your app)
   - Advanced health check settings:
     - Healthy threshold: `2`
     - Unhealthy threshold: `2`
     - Timeout: `5 seconds`
     - Interval: `10 seconds`
     - Success codes: `200`

4. **Register Targets:**
   - Select your EC2 instance (`<EC2_INSTANCE_IP>`)
   - Port: The NodePort from Step 1
   - Click "Include as pending below"
   - Click "Create target group"

## Step 3: Configure Security Groups

### EC2 Instance Security Group

Ensure the security group attached to your EC2 instance allows:

```
Inbound Rules:
- Type: Custom TCP
  Port: <NodePort from Step 1>
  Source: <ALB Security Group>
  Description: Allow traffic from ALB
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
   - Default action: Forward to `lookout-staging-tg`

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
   EC2 → Target Groups → lookout-staging-tg → Targets tab
   # Status should show "healthy"
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

1. Check EC2 security group allows traffic from ALB
2. Verify the NodePort is correct
3. Check health check path is accessible:
   ```bash
   ssh ubuntu@<EC2_INSTANCE_IP> 'curl -I http://localhost:<NodePort>/health'
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

## Alternative: Using NodePort Directly

If you want to simplify the setup without Envoy Gateway:

1. Create a NodePort service for the Lookout app:
   ```bash
   ssh ubuntu@<EC2_INSTANCE_IP> 'kubectl patch svc lookout-staging-lookout-app -n staging -p "{\"spec\":{\"type\":\"NodePort\",\"ports\":[{\"port\":3000,\"nodePort\":30080}]}}"'
   ```

2. Configure target group to use port `30080`

3. Update EC2 security group to allow port `30080` from ALB

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
                │  Target Group                  │
                │  Health Check: /health         │
                └───────────────┬────────────────┘
                                │
                ┌───────────────▼────────────────┐
                │  EC2 Instance: <EC2_INSTANCE_IP>      │
                │  ┌──────────────────────────┐  │
                │  │  Kind Cluster            │  │
                │  │  ┌────────────────────┐  │  │
                │  │  │ Envoy Gateway      │  │  │
                │  │  │ NodePort: 30XXX    │  │  │
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
