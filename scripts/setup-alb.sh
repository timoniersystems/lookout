#!/bin/bash
# Note: Not using 'set -e' to allow idempotent re-runs

# AWS Application Load Balancer Setup Script
# This script automates the creation of an ALB for Lookout with multi-environment support
# Prerequisites: AWS CLI configured, kubectl access to Kind cluster
# This script is idempotent and can be run multiple times safely
#
# Features:
# - HTTPS listener with Protocol=HTTPS to port 32443 (end-to-end encryption)
# - HTTP listener with 301 redirect to HTTPS
# - Host-based routing for multi-environment support (optional)
# - Automatic detection and correction of misconfigured target groups
#
# Environments:
# - Staging: lookout-stg.timonier.io (always configured)
# - Production: lookout-prod.timonier.io, lookout.timonier.io (when ENABLE_PROD=true)

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "🚀 Setting up AWS Application Load Balancer for Lookout"
if [ "$ENABLE_PROD" == "true" ]; then
    echo "   Mode: Multi-environment (Staging + Production)"
else
    echo "   Mode: Single environment (Staging only)"
fi
echo "   This script is idempotent and safe to re-run"
echo ""

# Configuration - UPDATE THESE VALUES
if [ -z "$AWS_REGION" ]; then
    echo -e "${YELLOW}AWS_REGION not set. Using default: us-east-1${NC}"
    export AWS_REGION=us-east-1
fi

if [ -z "$VPC_ID" ]; then
    echo -e "${RED}ERROR: VPC_ID environment variable not set${NC}"
    echo "Usage: VPC_ID=vpc-xxxxx EC2_INSTANCE_ID=i-xxxxx SUBNET_1=subnet-xxxxx SUBNET_2=subnet-xxxxx CERTIFICATE_ARN=arn:aws:... HOSTED_ZONE_ID=Z0xxxxx $0"
    exit 1
fi

# Validate required variables
REQUIRED_VARS=(VPC_ID EC2_INSTANCE_ID SUBNET_1 SUBNET_2 SUBNET_3 CERTIFICATE_ARN HOSTED_ZONE_ID)
for var in "${REQUIRED_VARS[@]}"; do
    if [ -z "${!var}" ]; then
        echo -e "${RED}ERROR: $var environment variable not set${NC}"
        exit 1
    fi
done

echo "Configuration:"
echo "  AWS Region: $AWS_REGION"
echo "  VPC ID: $VPC_ID"
echo "  EC2 Instance: $EC2_INSTANCE_ID"
echo "  Subnets: $SUBNET_1, $SUBNET_2, $SUBNET_3"
echo "  Certificate ARN: $CERTIFICATE_ARN"
echo "  Hosted Zone ID: $HOSTED_ZONE_ID"
echo ""

# Verify fixed NodePorts are configured
echo "🔍 Verifying fixed NodePorts..."
if ! kubectl get svc -n envoy-gateway-system -l gateway.envoyproxy.io/owning-gateway-name=lookout-staging 2>/dev/null | grep -q "32080.*32443"; then
    echo -e "${RED}ERROR: Fixed NodePorts not configured. Run ./scripts/setup-fixed-nodeports.sh first${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Fixed NodePorts verified (32080, 32443)${NC}"
echo ""

# Create ALB Security Group
echo "🔐 Creating ALB security group..."
ALB_SG_ID=$(aws ec2 create-security-group \
    --group-name lookout-alb-sg \
    --description "Security group for Lookout ALB" \
    --vpc-id $VPC_ID \
    --region $AWS_REGION \
    --output text --query 'GroupId' 2>/dev/null || \
    aws ec2 describe-security-groups \
        --filters "Name=group-name,Values=lookout-alb-sg" "Name=vpc-id,Values=$VPC_ID" \
        --region $AWS_REGION \
        --query 'SecurityGroups[0].GroupId' \
        --output text)

echo -e "${GREEN}✓ ALB Security Group: $ALB_SG_ID${NC}"

# Add inbound rules to ALB security group
echo "🔐 Configuring ALB security group rules..."
aws ec2 authorize-security-group-ingress \
    --group-id $ALB_SG_ID \
    --ip-permissions \
    IpProtocol=tcp,FromPort=80,ToPort=80,IpRanges='[{CidrIp=0.0.0.0/0,Description="Allow HTTP from anywhere"}]' \
    IpProtocol=tcp,FromPort=443,ToPort=443,IpRanges='[{CidrIp=0.0.0.0/0,Description="Allow HTTPS from anywhere"}]' \
    --region $AWS_REGION 2>/dev/null || echo "  (rules already exist)"

# Get EC2 instance security group
echo "🔐 Configuring EC2 security group..."
EC2_SG_ID=$(aws ec2 describe-instances \
    --instance-ids $EC2_INSTANCE_ID \
    --region $AWS_REGION \
    --query 'Reservations[0].Instances[0].SecurityGroups[0].GroupId' \
    --output text)

echo -e "${GREEN}✓ EC2 Security Group: $EC2_SG_ID${NC}"

# Add inbound rules to EC2 security group
aws ec2 authorize-security-group-ingress \
    --group-id $EC2_SG_ID \
    --ip-permissions \
    IpProtocol=tcp,FromPort=32080,ToPort=32080,UserIdGroupPairs="[{GroupId=$ALB_SG_ID,Description='Allow HTTP from ALB'}]" \
    IpProtocol=tcp,FromPort=32443,ToPort=32443,UserIdGroupPairs="[{GroupId=$ALB_SG_ID,Description='Allow HTTPS from ALB'}]" \
    --region $AWS_REGION 2>/dev/null || echo "  (rules already exist)"

# Create HTTP Target Group
echo "🎯 Creating HTTP target group..."
HTTP_TG_ARN=$(aws elbv2 create-target-group \
    --name lookout-http-tg \
    --protocol HTTP \
    --port 32080 \
    --vpc-id $VPC_ID \
    --health-check-protocol HTTP \
    --health-check-path /health \
    --health-check-interval-seconds 10 \
    --health-check-timeout-seconds 5 \
    --healthy-threshold-count 2 \
    --unhealthy-threshold-count 2 \
    --matcher HttpCode=200 \
    --region $AWS_REGION \
    --query 'TargetGroups[0].TargetGroupArn' \
    --output text 2>/dev/null || \
    aws elbv2 describe-target-groups \
        --names lookout-http-tg \
        --region $AWS_REGION \
        --query 'TargetGroups[0].TargetGroupArn' \
        --output text)

echo -e "${GREEN}✓ HTTP Target Group: $HTTP_TG_ARN${NC}"

# Ensure HTTP target group has correct settings
echo "🔧 Updating HTTP target group health check settings..."
aws elbv2 modify-target-group \
    --target-group-arn $HTTP_TG_ARN \
    --health-check-protocol HTTP \
    --health-check-path /health \
    --health-check-port 32080 \
    --health-check-interval-seconds 10 \
    --health-check-timeout-seconds 5 \
    --healthy-threshold-count 2 \
    --unhealthy-threshold-count 2 \
    --matcher HttpCode=200 \
    --region $AWS_REGION > /dev/null

# Create HTTPS Target Group for Staging
echo "🎯 Creating HTTPS target group for staging..."

# Check if target group exists and has wrong protocol
EXISTING_TG_PROTOCOL=$(aws elbv2 describe-target-groups \
    --names lookout-https-tg \
    --region $AWS_REGION \
    --query 'TargetGroups[0].Protocol' \
    --output text 2>/dev/null)

if [ "$EXISTING_TG_PROTOCOL" == "HTTP" ]; then
    echo -e "${YELLOW}⚠ Existing target group has wrong protocol (HTTP). Deleting...${NC}"
    EXISTING_TG_ARN=$(aws elbv2 describe-target-groups \
        --names lookout-https-tg \
        --region $AWS_REGION \
        --query 'TargetGroups[0].TargetGroupArn' \
        --output text)
    aws elbv2 delete-target-group \
        --target-group-arn $EXISTING_TG_ARN \
        --region $AWS_REGION
    echo -e "${GREEN}✓ Deleted old target group${NC}"
    sleep 2
fi

HTTPS_TG_ARN=$(aws elbv2 create-target-group \
    --name lookout-https-tg \
    --protocol HTTPS \
    --port 32443 \
    --vpc-id $VPC_ID \
    --health-check-protocol HTTPS \
    --health-check-path /health \
    --health-check-interval-seconds 10 \
    --health-check-timeout-seconds 5 \
    --healthy-threshold-count 2 \
    --unhealthy-threshold-count 2 \
    --matcher HttpCode=200 \
    --region $AWS_REGION \
    --query 'TargetGroups[0].TargetGroupArn' \
    --output text 2>/dev/null || \
    aws elbv2 describe-target-groups \
        --names lookout-https-tg \
        --region $AWS_REGION \
        --query 'TargetGroups[0].TargetGroupArn' \
        --output text)

echo -e "${GREEN}✓ HTTPS Target Group: $HTTPS_TG_ARN${NC}"

# Ensure HTTPS target group has correct settings
echo "🔧 Updating HTTPS target group health check settings..."
aws elbv2 modify-target-group \
    --target-group-arn $HTTPS_TG_ARN \
    --health-check-protocol HTTPS \
    --health-check-path /health \
    --health-check-port 32443 \
    --health-check-interval-seconds 10 \
    --health-check-timeout-seconds 5 \
    --healthy-threshold-count 2 \
    --unhealthy-threshold-count 2 \
    --matcher HttpCode=200 \
    --region $AWS_REGION > /dev/null

# Register EC2 instance with target groups
echo "🎯 Registering EC2 instance with target groups..."

# Deregister first to ensure clean state, then re-register
aws elbv2 deregister-targets \
    --target-group-arn $HTTP_TG_ARN \
    --targets Id=$EC2_INSTANCE_ID \
    --region $AWS_REGION 2>/dev/null || true

aws elbv2 register-targets \
    --target-group-arn $HTTP_TG_ARN \
    --targets Id=$EC2_INSTANCE_ID,Port=32080 \
    --region $AWS_REGION

aws elbv2 deregister-targets \
    --target-group-arn $HTTPS_TG_ARN \
    --targets Id=$EC2_INSTANCE_ID \
    --region $AWS_REGION 2>/dev/null || true

aws elbv2 register-targets \
    --target-group-arn $HTTPS_TG_ARN \
    --targets Id=$EC2_INSTANCE_ID,Port=32443 \
    --region $AWS_REGION

echo -e "${GREEN}✓ Targets registered${NC}"

# Create Application Load Balancer
echo "⚖️  Creating Application Load Balancer..."
ALB_ARN=$(aws elbv2 create-load-balancer \
    --name lookout-alb \
    --subnets $SUBNET_1 $SUBNET_2 $SUBNET_3 \
    --security-groups $ALB_SG_ID \
    --scheme internet-facing \
    --type application \
    --ip-address-type ipv4 \
    --region $AWS_REGION \
    --query 'LoadBalancers[0].LoadBalancerArn' \
    --output text 2>/dev/null || \
    aws elbv2 describe-load-balancers \
        --names lookout-alb \
        --region $AWS_REGION \
        --query 'LoadBalancers[0].LoadBalancerArn' \
        --output text)

echo -e "${GREEN}✓ ALB ARN: $ALB_ARN${NC}"

# Update ALB security groups (in case they changed)
echo "🔧 Updating ALB security groups..."
aws elbv2 set-security-groups \
    --load-balancer-arn $ALB_ARN \
    --security-groups $ALB_SG_ID \
    --region $AWS_REGION > /dev/null || echo "  (security groups already set)"

# Get ALB DNS name and Hosted Zone ID
ALB_DNS=$(aws elbv2 describe-load-balancers \
    --load-balancer-arns $ALB_ARN \
    --region $AWS_REGION \
    --query 'LoadBalancers[0].DNSName' \
    --output text)

ALB_ZONE_ID=$(aws elbv2 describe-load-balancers \
    --load-balancer-arns $ALB_ARN \
    --region $AWS_REGION \
    --query 'LoadBalancers[0].CanonicalHostedZoneId' \
    --output text)

echo "  ALB DNS: $ALB_DNS"
echo "  ALB Zone ID: $ALB_ZONE_ID"

# Wait for ALB to be active
echo "⏳ Waiting for ALB to become active..."
aws elbv2 wait load-balancer-available \
    --load-balancer-arns $ALB_ARN \
    --region $AWS_REGION

echo -e "${GREEN}✓ ALB is active${NC}"

# Create HTTP listener with redirect to HTTPS
echo "🎧 Creating HTTP listener (redirect to HTTPS)..."
HTTP_LISTENER_ARN=$(aws elbv2 describe-listeners \
    --load-balancer-arn $ALB_ARN \
    --region $AWS_REGION \
    --query 'Listeners[?Port==`80`].ListenerArn' \
    --output text 2>/dev/null)

if [ -z "$HTTP_LISTENER_ARN" ]; then
    HTTP_LISTENER_ARN=$(aws elbv2 create-listener \
        --load-balancer-arn $ALB_ARN \
        --protocol HTTP \
        --port 80 \
        --default-actions Type=redirect,RedirectConfig='{Protocol=HTTPS,Port=443,StatusCode=HTTP_301}' \
        --region $AWS_REGION \
        --query 'Listeners[0].ListenerArn' \
        --output text)
fi

echo -e "${GREEN}✓ HTTP Listener: $HTTP_LISTENER_ARN${NC}"

# Create HTTPS listener
echo "🎧 Creating HTTPS listener..."
HTTPS_LISTENER_ARN=$(aws elbv2 describe-listeners \
    --load-balancer-arn $ALB_ARN \
    --region $AWS_REGION \
    --query 'Listeners[?Port==`443`].ListenerArn' \
    --output text 2>/dev/null)

if [ -z "$HTTPS_LISTENER_ARN" ]; then
    HTTPS_LISTENER_ARN=$(aws elbv2 create-listener \
        --load-balancer-arn $ALB_ARN \
        --protocol HTTPS \
        --port 443 \
        --certificates CertificateArn=$CERTIFICATE_ARN \
        --default-actions Type=forward,TargetGroupArn=$HTTPS_TG_ARN \
        --region $AWS_REGION \
        --query 'Listeners[0].ListenerArn' \
        --output text)
else
    # Update existing HTTPS listener with correct certificate and target group
    echo "🔧 Updating HTTPS listener configuration..."
    aws elbv2 modify-listener \
        --listener-arn $HTTPS_LISTENER_ARN \
        --certificates CertificateArn=$CERTIFICATE_ARN \
        --default-actions Type=forward,TargetGroupArn=$HTTPS_TG_ARN \
        --region $AWS_REGION > /dev/null || echo "  (listener already configured)"
fi

echo -e "${GREEN}✓ HTTPS Listener: $HTTPS_LISTENER_ARN${NC}"

# Configure Host-Based Routing (Virtual Hosts)
# Set default production hostname if not provided
if [ -z "$PROD_HOSTNAME" ]; then
    PROD_HOSTNAME="lookout-prod.timonier.io"
fi

# Check if multi-environment support is enabled
if [ "$ENABLE_PROD" == "true" ]; then
    echo ""
    echo "🔀 Configuring host-based routing for multiple environments..."

    # Create HTTPS target group for prod (if it doesn't exist)
    echo "🎯 Creating HTTPS target group for production..."

    # Check if prod target group exists with wrong protocol
    EXISTING_PROD_TG_PROTOCOL=$(aws elbv2 describe-target-groups \
        --names lookout-prod-https-tg \
        --region $AWS_REGION \
        --query 'TargetGroups[0].Protocol' \
        --output text 2>/dev/null)

    if [ "$EXISTING_PROD_TG_PROTOCOL" == "HTTP" ]; then
        echo -e "${YELLOW}⚠ Existing prod target group has wrong protocol (HTTP). Deleting...${NC}"
        EXISTING_PROD_TG_ARN=$(aws elbv2 describe-target-groups \
            --names lookout-prod-https-tg \
            --region $AWS_REGION \
            --query 'TargetGroups[0].TargetGroupArn' \
            --output text)
        aws elbv2 delete-target-group \
            --target-group-arn $EXISTING_PROD_TG_ARN \
            --region $AWS_REGION
        echo -e "${GREEN}✓ Deleted old prod target group${NC}"
        sleep 2
    fi

    PROD_TG_ARN=$(aws elbv2 create-target-group \
        --name lookout-prod-https-tg \
        --protocol HTTPS \
        --port 32443 \
        --vpc-id $VPC_ID \
        --health-check-protocol HTTPS \
        --health-check-path /health \
        --health-check-interval-seconds 10 \
        --health-check-timeout-seconds 5 \
        --healthy-threshold-count 2 \
        --unhealthy-threshold-count 2 \
        --matcher HttpCode=200 \
        --region $AWS_REGION \
        --query 'TargetGroups[0].TargetGroupArn' \
        --output text 2>/dev/null || \
        aws elbv2 describe-target-groups \
            --names lookout-prod-https-tg \
            --region $AWS_REGION \
            --query 'TargetGroups[0].TargetGroupArn' \
            --output text)

    echo -e "${GREEN}✓ Prod HTTPS Target Group: $PROD_TG_ARN${NC}"

    # Register EC2 instance with prod target group
    aws elbv2 deregister-targets \
        --target-group-arn $PROD_TG_ARN \
        --targets Id=$EC2_INSTANCE_ID \
        --region $AWS_REGION 2>/dev/null || true

    aws elbv2 register-targets \
        --target-group-arn $PROD_TG_ARN \
        --targets Id=$EC2_INSTANCE_ID,Port=32443 \
        --region $AWS_REGION

    # Create listener rules for host-based routing
    echo "🔀 Creating host-based routing rules..."

    # Rule for staging hostname (priority 1)
    STAGING_RULE=$(aws elbv2 describe-rules \
        --listener-arn $HTTPS_LISTENER_ARN \
        --region $AWS_REGION \
        --query 'Rules[?Priority==`1`].RuleArn' \
        --output text 2>/dev/null)

    if [ -z "$STAGING_RULE" ]; then
        aws elbv2 create-rule \
            --listener-arn $HTTPS_LISTENER_ARN \
            --priority 1 \
            --conditions Field=host-header,Values=lookout-stg.timonier.io \
            --actions Type=forward,TargetGroupArn=$HTTPS_TG_ARN \
            --region $AWS_REGION > /dev/null
        echo -e "${GREEN}✓ Created routing rule for lookout-stg.timonier.io${NC}"
    else
        aws elbv2 modify-rule \
            --rule-arn $STAGING_RULE \
            --conditions Field=host-header,Values=lookout-stg.timonier.io \
            --actions Type=forward,TargetGroupArn=$HTTPS_TG_ARN \
            --region $AWS_REGION > /dev/null
        echo -e "${GREEN}✓ Updated routing rule for lookout-stg.timonier.io${NC}"
    fi

    # Rule for prod hostnames (priority 2) - supports both lookout-prod and lookout
    PROD_RULE=$(aws elbv2 describe-rules \
        --listener-arn $HTTPS_LISTENER_ARN \
        --region $AWS_REGION \
        --query 'Rules[?Priority==`2`].RuleArn' \
        --output text 2>/dev/null)

    if [ -z "$PROD_RULE" ]; then
        aws elbv2 create-rule \
            --listener-arn $HTTPS_LISTENER_ARN \
            --priority 2 \
            --conditions Field=host-header,Values=lookout-prod.timonier.io,lookout.timonier.io \
            --actions Type=forward,TargetGroupArn=$PROD_TG_ARN \
            --region $AWS_REGION > /dev/null
        echo -e "${GREEN}✓ Created routing rule for production (lookout-prod.timonier.io, lookout.timonier.io)${NC}"
    else
        aws elbv2 modify-rule \
            --rule-arn $PROD_RULE \
            --conditions Field=host-header,Values=lookout-prod.timonier.io,lookout.timonier.io \
            --actions Type=forward,TargetGroupArn=$PROD_TG_ARN \
            --region $AWS_REGION > /dev/null
        echo -e "${GREEN}✓ Updated routing rule for production (lookout-prod.timonier.io, lookout.timonier.io)${NC}"
    fi

    echo ""
    echo -e "${GREEN}✅ Host-based routing configured!${NC}"
    echo "  Staging: lookout-stg.timonier.io → Port 32443 (staging target group)"
    echo "  Production: lookout-prod.timonier.io, lookout.timonier.io → Port 32443 (prod target group)"
else
    echo ""
    echo -e "${YELLOW}ℹ Single environment mode (staging only)${NC}"
    echo "  To enable multi-environment support with host-based routing:"
    echo "  export ENABLE_PROD=true"
    echo "  This will configure:"
    echo "    - lookout-stg.timonier.io → staging"
    echo "    - lookout-prod.timonier.io, lookout.timonier.io → production"
fi

# Create Route53 A record
# echo "🌐 Creating Route53 A record..."
# cat > /tmp/route53-change.json <<EOF
# {
#   "Changes": [
#     {
#       "Action": "UPSERT",
#       "ResourceRecordSet": {
#         "Name": "lookout-stg.timonier.io",
#         "Type": "A",
#         "AliasTarget": {
#           "HostedZoneId": "$ALB_ZONE_ID",
#           "DNSName": "$ALB_DNS",
#           "EvaluateTargetHealth": true
#         }
#       }
#     }
#   ]
# }
# EOF

# aws route53 change-resource-record-sets \
#     --hosted-zone-id $HOSTED_ZONE_ID \
#     --change-batch file:///tmp/route53-change.json \
#     --region $AWS_REGION > /dev/null

# rm /tmp/route53-change.json

# echo -e "${GREEN}✓ Route53 record created for lookout-stg.timonier.io${NC}"

# Verify target health
echo ""
echo "🏥 Checking target health..."
sleep 5

HTTP_HEALTH=$(aws elbv2 describe-target-health \
    --target-group-arn $HTTP_TG_ARN \
    --region $AWS_REGION \
    --query 'TargetHealthDescriptions[0].TargetHealth.State' \
    --output text)

HTTPS_HEALTH=$(aws elbv2 describe-target-health \
    --target-group-arn $HTTPS_TG_ARN \
    --region $AWS_REGION \
    --query 'TargetHealthDescriptions[0].TargetHealth.State' \
    --output text)

echo "  HTTP Target Group (staging): $HTTP_HEALTH"
echo "  HTTPS Target Group (staging): $HTTPS_HEALTH"

if [ "$ENABLE_PROD" == "true" ]; then
    PROD_HTTPS_HEALTH=$(aws elbv2 describe-target-health \
        --target-group-arn $PROD_TG_ARN \
        --region $AWS_REGION \
        --query 'TargetHealthDescriptions[0].TargetHealth.State' \
        --output text)
    echo "  HTTPS Target Group (prod): $PROD_HTTPS_HEALTH"

    if [ "$HTTP_HEALTH" == "healthy" ] && [ "$HTTPS_HEALTH" == "healthy" ] && [ "$PROD_HTTPS_HEALTH" == "healthy" ]; then
        echo -e "${GREEN}✓ All targets healthy${NC}"
    elif [ "$HTTP_HEALTH" == "initial" ] || [ "$HTTPS_HEALTH" == "initial" ] || [ "$PROD_HTTPS_HEALTH" == "initial" ]; then
        echo -e "${YELLOW}⚠ Targets in 'initial' state. This is normal and they should become healthy in ~30 seconds${NC}"
    else
        echo -e "${YELLOW}⚠ Some targets not yet healthy. Check security groups and Gateway configuration${NC}"
    fi
else
    if [ "$HTTP_HEALTH" == "healthy" ] && [ "$HTTPS_HEALTH" == "healthy" ]; then
        echo -e "${GREEN}✓ All targets healthy${NC}"
    elif [ "$HTTP_HEALTH" == "initial" ] || [ "$HTTPS_HEALTH" == "initial" ]; then
        echo -e "${YELLOW}⚠ Targets in 'initial' state. This is normal and they should become healthy in ~30 seconds${NC}"
    else
        echo -e "${YELLOW}⚠ Targets not yet healthy. Check security groups and Gateway configuration${NC}"
    fi
fi

# Summary
echo ""
echo -e "${GREEN}✅ ALB setup complete!${NC}"
echo ""
echo "📋 Summary:"
echo "  ALB DNS: $ALB_DNS"
if [ "$ENABLE_PROD" == "true" ]; then
    echo "  Domains (multi-environment):"
    echo "    - lookout-stg.timonier.io (staging)"
    echo "    - lookout-prod.timonier.io (production)"
    echo "    - lookout.timonier.io (production alias)"
else
    echo "  Domain: lookout-stg.timonier.io (staging only)"
fi
echo "  HTTP Listener: Port 80 → Redirect to HTTPS (301)"
echo "  HTTPS Listener: Port 443 → Forward to HTTPS targets (Protocol=HTTPS, Port=32443)"
if [ "$ENABLE_PROD" == "true" ]; then
    echo "  Target Groups:"
    echo "    - HTTP: lookout-http-tg (32080)"
    echo "    - HTTPS Staging: lookout-https-tg (32443)"
    echo "    - HTTPS Production: lookout-prod-https-tg (32443)"
    echo "  Host-Based Routing:"
    echo "    - lookout-stg.timonier.io → staging target group"
    echo "    - lookout-prod.timonier.io, lookout.timonier.io → prod target group"
else
    echo "  Target Groups: HTTP (32080), HTTPS (32443)"
fi
echo ""
echo "🔍 Next steps:"
echo "  1. Wait 5-10 minutes for DNS propagation"
if [ "$ENABLE_PROD" == "true" ]; then
    echo "  2. Test staging: curl -I https://lookout-stg.timonier.io/health"
    echo "  3. Test production: curl -I https://lookout-prod.timonier.io/health"
    echo "  4. Test prod alias: curl -I https://lookout.timonier.io/health"
else
    echo "  2. Test: curl -I https://lookout-stg.timonier.io/health"
fi
echo "  3. Check target health in AWS Console"
echo ""
if [ "$ENABLE_PROD" == "true" ]; then
    echo "🌐 Access your applications:"
    echo "  Staging: https://lookout-stg.timonier.io"
    echo "  Production: https://lookout-prod.timonier.io or https://lookout.timonier.io"
else
    echo "🌐 Access your application at: https://lookout-stg.timonier.io"
fi
