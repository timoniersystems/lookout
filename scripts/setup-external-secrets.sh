#!/bin/bash
set -e

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${BLUE}🔐 Setting up External Secrets Operator${NC}\n"

# Configuration
AWS_REGION="${AWS_REGION:-us-west-2}"
SECRET_NAME="lookout/staging/nvd-api-key"
NAMESPACE="staging"

# Ensure namespace exists
if ! kubectl get namespace ${NAMESPACE} &> /dev/null; then
    echo "Creating namespace: ${NAMESPACE}"
    kubectl create namespace ${NAMESPACE}
fi

# Step 1: Install External Secrets Operator
echo -e "${YELLOW}📦 Step 1: Installing External Secrets Operator${NC}"

# Check if already installed via Helm
if helm list -n external-secrets-system 2>/dev/null | grep -q external-secrets; then
    echo "External Secrets Operator already installed (Helm release found)"
    echo "Upgrading to ensure latest version..."
    helm repo add external-secrets https://charts.external-secrets.io 2>/dev/null || true
    helm repo update
    helm upgrade external-secrets \
        external-secrets/external-secrets \
        --namespace external-secrets-system \
        --set installCRDs=true \
        --reuse-values
else
    echo "Installing External Secrets Operator..."
    helm repo add external-secrets https://charts.external-secrets.io 2>/dev/null || true
    helm repo update

    helm install external-secrets \
        external-secrets/external-secrets \
        --namespace external-secrets-system \
        --create-namespace \
        --set installCRDs=true

    echo "Waiting for External Secrets Operator to be ready..."
    kubectl wait --for=condition=ready pod \
        -l app.kubernetes.io/name=external-secrets \
        -n external-secrets-system \
        --timeout=120s
fi

# Verify CRDs are installed
echo "Verifying CRDs are available..."
for i in {1..30}; do
    if kubectl get crd secretstores.external-secrets.io &> /dev/null && \
       kubectl get crd externalsecrets.external-secrets.io &> /dev/null; then
        echo -e "${GREEN}✓ CRDs are ready${NC}"
        break
    fi
    if [ $i -eq 30 ]; then
        echo -e "${YELLOW}⚠ CRDs not found after waiting. Installing manually...${NC}"
        kubectl apply -f https://raw.githubusercontent.com/external-secrets/external-secrets/main/deploy/crds/bundle.yaml
        sleep 5
    fi
    echo "Waiting for CRDs... ($i/30)"
    sleep 2
done

echo -e "${GREEN}✓ External Secrets Operator ready${NC}\n"

# Step 2: Create IAM policy for Secrets Manager access
echo -e "${YELLOW}📋 Step 2: Creating IAM policy for Secrets Manager${NC}"
POLICY_NAME="LookoutSecretsManagerAccess"
POLICY_ARN=$(aws iam list-policies --query "Policies[?PolicyName=='${POLICY_NAME}'].Arn" --output text)

if [ -z "$POLICY_ARN" ]; then
    cat > /tmp/secrets-policy.json <<EOF
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "secretsmanager:GetSecretValue",
                "secretsmanager:DescribeSecret"
            ],
            "Resource": "arn:aws:secretsmanager:${AWS_REGION}:*:secret:lookout/*"
        }
    ]
}
EOF

    POLICY_ARN=$(aws iam create-policy \
        --policy-name "${POLICY_NAME}" \
        --policy-document file:///tmp/secrets-policy.json \
        --query 'Policy.Arn' \
        --output text)

    rm /tmp/secrets-policy.json
    echo -e "${GREEN}✓ Created IAM policy: ${POLICY_ARN}${NC}"
else
    echo -e "Policy already exists: ${POLICY_ARN}"
fi
echo ""

# Step 3: Attach policy to EC2 instance role
echo -e "${YELLOW}🔗 Step 3: Attaching policy to EC2 instance role${NC}"
INSTANCE_ID=$(ec2-metadata --instance-id 2>/dev/null | cut -d ' ' -f 2 || aws ec2 describe-instances --filters "Name=tag:Name,Values=*kind*" --query 'Reservations[0].Instances[0].InstanceId' --output text --region ${AWS_REGION})

# Get the instance profile ARN, extract the profile name, then get the role name from the profile
PROFILE_ARN=$(aws ec2 describe-instances --instance-ids ${INSTANCE_ID} --query 'Reservations[0].Instances[0].IamInstanceProfile.Arn' --output text --region ${AWS_REGION})
PROFILE_NAME=$(echo ${PROFILE_ARN} | cut -d'/' -f2)
ROLE_NAME=$(aws iam get-instance-profile --instance-profile-name ${PROFILE_NAME} --query 'InstanceProfile.Roles[0].RoleName' --output text 2>/dev/null)

if [ -n "$ROLE_NAME" ]; then
    # Check if policy is already attached
    if aws iam list-attached-role-policies --role-name "${ROLE_NAME}" --query "AttachedPolicies[?PolicyArn=='${POLICY_ARN}'].PolicyArn" --output text | grep -q "${POLICY_ARN}"; then
        echo "Policy already attached to role: ${ROLE_NAME}"
    else
        echo "Attaching policy to role: ${ROLE_NAME}"
        aws iam attach-role-policy \
            --role-name "${ROLE_NAME}" \
            --policy-arn "${POLICY_ARN}"
    fi
    echo -e "${GREEN}✓ Policy attached to role: ${ROLE_NAME}${NC}"
else
    echo -e "${YELLOW}⚠ No IAM role found on instance. You'll need to configure IRSA or use access keys${NC}"
fi
echo ""

# Step 4: Create secret in AWS Secrets Manager
echo -e "${YELLOW}🔑 Step 4: Creating secret in AWS Secrets Manager${NC}"
read -p "Enter your NVD API Key (or press Enter to skip): " NVD_API_KEY

if [ -n "$NVD_API_KEY" ]; then
    if aws secretsmanager describe-secret --secret-id "${SECRET_NAME}" --region ${AWS_REGION} &> /dev/null; then
        echo "Secret already exists, updating..."
        aws secretsmanager update-secret \
            --secret-id "${SECRET_NAME}" \
            --secret-string "{\"NVD_API_KEY\":\"${NVD_API_KEY}\"}" \
            --region ${AWS_REGION} > /dev/null
    else
        echo "Creating new secret..."
        aws secretsmanager create-secret \
            --name "${SECRET_NAME}" \
            --description "NVD API key for Lookout staging environment" \
            --secret-string "{\"NVD_API_KEY\":\"${NVD_API_KEY}\"}" \
            --region ${AWS_REGION} > /dev/null
    fi
    echo -e "${GREEN}✓ Secret created/updated in AWS Secrets Manager${NC}"
else
    echo -e "${YELLOW}⚠ Skipped - you can create the secret manually later${NC}"
fi
echo ""

# Step 5: Create SecretStore
echo -e "${YELLOW}🏪 Step 5: Creating SecretStore${NC}"
kubectl apply -f - <<EOF
apiVersion: external-secrets.io/v1beta1
kind: SecretStore
metadata:
  name: aws-secrets-manager
  namespace: ${NAMESPACE}
spec:
  provider:
    aws:
      service: SecretsManager
      region: ${AWS_REGION}
      auth:
        jwt:
          serviceAccountRef:
            name: default
EOF
echo -e "${GREEN}✓ SecretStore created${NC}\n"

# Step 6: Create ExternalSecret
echo -e "${YELLOW}🔄 Step 6: Creating ExternalSecret${NC}"
kubectl apply -f - <<EOF
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: lookout-nvd-api-key
  namespace: ${NAMESPACE}
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: aws-secrets-manager
    kind: SecretStore
  target:
    name: lookout-staging-lookout-app
    creationPolicy: Owner
    template:
      engineVersion: v2
      data:
        NVD_API_KEY: "{{ .NVD_API_KEY }}"
  data:
    - secretKey: NVD_API_KEY
      remoteRef:
        key: ${SECRET_NAME}
        property: NVD_API_KEY
EOF
echo -e "${GREEN}✓ ExternalSecret created${NC}\n"

# Step 7: Verify setup
echo -e "${YELLOW}✅ Step 7: Verifying setup${NC}"
echo "Waiting for secret to sync..."
sleep 5

if kubectl get secret lookout-staging-lookout-app -n ${NAMESPACE} &> /dev/null; then
    echo -e "${GREEN}✓ Secret successfully synced from AWS Secrets Manager${NC}"
    kubectl get secret lookout-staging-lookout-app -n ${NAMESPACE} -o jsonpath='{.data.NVD_API_KEY}' | base64 -d | wc -c | xargs echo "Secret length:"
else
    echo -e "${YELLOW}⚠ Secret not yet synced. Check ExternalSecret status:${NC}"
    kubectl describe externalsecret lookout-nvd-api-key -n ${NAMESPACE}
fi

echo ""
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}✅ External Secrets setup complete!${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo "📋 Next steps:"
echo "  1. Verify secret exists: kubectl get secret lookout-staging-lookout-app -n staging"
echo "  2. Restart deployment: kubectl rollout restart deployment/lookout-staging-lookout-app -n staging"
echo "  3. Check logs: kubectl logs -n staging deployment/lookout-staging-lookout-app"
echo ""
echo "🔧 To update the secret:"
echo "  aws secretsmanager update-secret \\"
echo "    --secret-id ${SECRET_NAME} \\"
echo "    --secret-string '{\"NVD_API_KEY\":\"your-new-key\"}' \\"
echo "    --region ${AWS_REGION}"
echo ""
