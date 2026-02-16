#!/bin/bash
set -e

# Configure Fixed NodePorts for Gateway
# This ensures NodePorts don't change when Gateway is recreated, preventing ALB reconfiguration

HTTP_NODEPORT="${HTTP_NODEPORT:-32080}"
HTTPS_NODEPORT="${HTTPS_NODEPORT:-32443}"
NAMESPACE="${NAMESPACE:-staging}"

echo "🔧 Configuring fixed NodePorts for Envoy Gateway..."
echo "   Namespace: ${NAMESPACE}"
echo "   HTTP NodePort: ${HTTP_NODEPORT}"
echo "   HTTPS NodePort: ${HTTPS_NODEPORT}"

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
    echo "❌ kubectl is not installed"
    exit 1
fi

# Create EnvoyProxy infrastructure configuration
echo "📦 Creating EnvoyProxy configuration with fixed NodePorts..."
cat <<EOF | kubectl apply -f -
apiVersion: gateway.envoyproxy.io/v1alpha1
kind: EnvoyProxy
metadata:
  name: custom-proxy-config
  namespace: ${NAMESPACE}
spec:
  provider:
    type: Kubernetes
    kubernetes:
      envoyService:
        type: LoadBalancer
        annotations:
          service.beta.kubernetes.io/aws-load-balancer-type: "nlb"
          service.beta.kubernetes.io/aws-load-balancer-scheme: "internet-facing"
        patch:
          type: StrategicMerge
          value:
            spec:
              ports:
              - name: http
                port: 8080
                targetPort: 8080
                protocol: TCP
                nodePort: ${HTTP_NODEPORT}
              - name: https
                port: 8443
                targetPort: 8443
                protocol: TCP
                nodePort: ${HTTPS_NODEPORT}
EOF

echo "✅ Fixed NodePorts configured successfully!"
echo ""
echo "📋 Configuration:"
echo "   HTTP:  8080 → NodePort ${HTTP_NODEPORT}"
echo "   HTTPS: 8443 → NodePort ${HTTPS_NODEPORT}"
echo ""
echo "🔄 Recreate Gateway to apply changes:"
echo "   kubectl delete gateway lookout-staging -n staging"
echo "   kubectl apply -f lookout/k8s/argocd/staging-application.yaml"
echo ""
echo "📝 Configure ALB with these fixed ports:"
echo "   Target Group HTTP:  Port ${HTTP_NODEPORT}"
echo "   Target Group HTTPS: Port ${HTTPS_NODEPORT}"
echo ""
