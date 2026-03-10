SHELL := /bin/bash
.SHELLFLAGS := -eu -o pipefail -c
.ONESHELL:

ifeq ($(OS),Windows_NT)
    detected_OS := Windows
    EXE_EXT := .exe
    INSTALL_DIR := C:\Program Files\lookout
    INSTALL_CMD := copy
    RM := del /Q
else
    detected_OS := $(shell uname -s)
    EXE_EXT :=
    INSTALL_DIR := $(HOME)/.local/bin
    INSTALL_CMD := install -m 755
    RM := rm -f
endif

# Kubernetes config - must be set in the environment
export KUBECONFIG

# Deployment target namespace (staging or production)
NAMESPACE ?= staging

.PHONY: help \
        build build-cli build-ui install install-cli install-ui clean \
        test test-integration test-all test-verbose test-coverage \
        up up-standalone down \
        certs \
        kind-registry kind-nodeports kind-forward \
        gateway health-route \
        argocd argocd-github argocd-image-updater \
        external-secrets basic-auth ghcr-secret \
        alb \
        sync deploy-staging deploy-production \
        cluster-setup

CLI_BINARY=lookout$(EXE_EXT)
UI_BINARY=lookout-ui$(EXE_EXT)

GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test

##@ Help

help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} \
	/^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-26s\033[0m %s\n", $$1, $$2 } \
	/^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) }' $(MAKEFILE_LIST)

##@ Build

build: build-cli build-ui ## Build all binaries

build-cli: ## Build CLI binary (cmd/cli)
	CGO_ENABLED=1 $(GOBUILD) -o $(CLI_BINARY) ./cmd/cli

build-ui: ## Build UI binary (cmd/ui)
	CGO_ENABLED=1 $(GOBUILD) -o $(UI_BINARY) ./cmd/ui

install: install-cli install-ui ## Install all binaries to $(INSTALL_DIR)

install-cli: build-cli ## Install CLI binary
	mkdir -p $(INSTALL_DIR)
	$(INSTALL_CMD) $(CLI_BINARY) $(INSTALL_DIR)

install-ui: build-ui ## Install UI binary
	mkdir -p $(INSTALL_DIR)
	$(INSTALL_CMD) $(UI_BINARY) $(INSTALL_DIR)

clean: ## Remove build artifacts
	$(GOCLEAN)
	$(RM) $(CLI_BINARY) $(UI_BINARY)

##@ Test

test: ## Run unit tests
	$(GOTEST) -v -short ./...

test-integration: ## Run integration tests (requires Dgraph)
	$(GOTEST) -v -tags=integration ./...

test-all: ## Run all tests including integration
	$(GOTEST) -v -tags=integration ./...

test-verbose: ## Run tests with verbose output
	$(GOTEST) -v -count=1 ./...

test-coverage: ## Generate HTML coverage report
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

##@ Local Development (Docker Compose)

up: ## Start full stack (Dgraph + app + nginx)
	docker compose up -d

up-standalone: ## Start Dgraph only (no app)
	docker compose --profile standalone up -d dgraph

down: ## Stop and remove all containers
	docker compose down

certs: ## Generate self-signed TLS certs for nginx (nginx/certs/)
	./scripts/generate-certs.sh

##@ Kind Cluster

kind-registry: ## Set up HTTPS Docker registry for kind (CLUSTER_NAME=lookout)
	set -e
	CLUSTER_NAME="$${CLUSTER_NAME:-lookout}"
	echo "========================================="
	echo "Setting up HTTPS Docker Registry for kind"
	echo "========================================="
	echo
	echo "Step 1: Cleaning up existing registry..."
	docker stop registry 2>/dev/null || true
	docker rm registry 2>/dev/null || true
	echo "✅ Cleanup complete"
	echo
	echo "Step 2: Creating certificate directory..."
	mkdir -p ~/registry/certs
	echo "✅ Certificate directory created"
	echo
	echo "Step 3: Generating self-signed certificate..."
	openssl req -newkey rsa:4096 -nodes -sha256 \
	  -keyout ~/registry/certs/domain.key \
	  -x509 -days 365 \
	  -out ~/registry/certs/domain.crt \
	  -subj '/CN=registry' \
	  -addext 'subjectAltName=DNS:registry,DNS:localhost,IP:127.0.0.1' 2>/dev/null
	echo "✅ Certificate generated"
	echo
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
	echo "Step 5: Connecting registry to kind network..."
	docker network connect kind registry 2>/dev/null || echo "Already connected to kind network"
	echo "✅ Registry connected to kind network"
	echo
	echo "Step 6: Configuring kind nodes to trust registry certificate..."
	for node in $$(kind get nodes --name "$$CLUSTER_NAME"); do
	  echo "  Configuring node: $$node"
	  docker exec "$$node" mkdir -p /etc/docker/certs.d/registry:5000
	  docker exec "$$node" mkdir -p /etc/containerd/certs.d/registry:5000
	  docker cp ~/registry/certs/domain.crt "$$node":/usr/local/share/ca-certificates/registry.crt
	  docker exec "$$node" update-ca-certificates
	  docker cp ~/registry/certs/domain.crt "$$node":/etc/containerd/certs.d/registry:5000/ca.crt
	  echo "  ✅ Node $$node configured"
	done
	echo "✅ All nodes configured"
	echo
	echo "Step 7: Restarting containerd on kind nodes..."
	for node in $$(kind get nodes --name "$$CLUSTER_NAME"); do
	  echo "  Restarting containerd on: $$node"
	  docker exec "$$node" sh -c 'pkill -9 containerd' || true
	done
	echo "  Waiting for cluster to recover..."
	sleep 10
	echo "✅ Containerd restarted"
	echo
	echo "Step 8: Verifying registry..."
	sleep 2
	if curl -sk https://localhost:5000/v2/_catalog > /dev/null 2>&1; then
	  echo "✅ Registry is accessible"
	  echo "   Catalog: $$(curl -sk https://localhost:5000/v2/_catalog)"
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

kind-nodeports: ## Configure fixed NodePorts 32080/32443 on EnvoyProxy
	set -e
	HTTP_NODEPORT=32080
	HTTPS_NODEPORT=32443
	NAMESPACE_VAL="$${NAMESPACE:-$(NAMESPACE)}"
	echo "🔧 Configuring fixed NodePorts for Envoy Gateway..."
	echo "   Namespace: $${NAMESPACE_VAL}"
	echo "   HTTP NodePort: $${HTTP_NODEPORT}"
	echo "   HTTPS NodePort: $${HTTPS_NODEPORT}"
	if ! command -v kubectl &> /dev/null; then
	    echo "❌ kubectl is not installed"
	    exit 1
	fi
	echo "📦 Creating EnvoyProxy configuration with fixed NodePorts..."
	cat <<EOF | kubectl apply -f -
	apiVersion: gateway.envoyproxy.io/v1alpha1
	kind: EnvoyProxy
	metadata:
	  name: custom-proxy-config
	  namespace: $${NAMESPACE_VAL}
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
	                nodePort: $${HTTP_NODEPORT}
	              - name: https
	                port: 8443
	                targetPort: 8443
	                protocol: TCP
	                nodePort: $${HTTPS_NODEPORT}
	EOF
	echo "✅ Fixed NodePorts configured successfully!"
	echo ""
	echo "📋 Configuration:"
	echo "   HTTP:  8080 → NodePort $${HTTP_NODEPORT}"
	echo "   HTTPS: 8443 → NodePort $${HTTPS_NODEPORT}"
	echo ""
	echo "🔄 Recreate Gateway to apply changes:"
	echo "   kubectl delete gateway lookout -n staging"
	echo "   kubectl apply -f lookout/k8s/argocd/staging-application.yaml"
	echo ""
	echo "📝 Configure ALB with these fixed ports:"
	echo "   Target Group HTTP:  Port $${HTTP_NODEPORT}"
	echo "   Target Group HTTPS: Port $${HTTPS_NODEPORT}"
	echo ""

kind-forward: ## Set up socat port-forwarding from EC2 host to kind NodePorts
	RED='\033[0;31m'
	GREEN='\033[0;32m'
	YELLOW='\033[1;33m'
	NC='\033[0m'
	echo "🔧 Setting up Kind NodePort forwarding for ALB integration"
	echo ""
	KIND_CONTAINER=$$(docker ps --filter name=control-plane --format "{{.Names}}" | head -1)
	if [ -z "$$KIND_CONTAINER" ]; then
	    echo -e "$${RED}ERROR: Kind control plane container not found$${NC}"
	    echo "Make sure your Kind cluster is running"
	    exit 1
	fi
	echo "Found Kind container: $$KIND_CONTAINER"
	KIND_IP=$$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' $$KIND_CONTAINER)
	if [ -z "$$KIND_IP" ]; then
	    echo -e "$${RED}ERROR: Could not get Kind container IP$${NC}"
	    exit 1
	fi
	echo "Kind container IP: $$KIND_IP"
	echo ""
	if ! command -v socat &> /dev/null; then
	    echo "Installing socat..."
	    sudo apt-get update && sudo apt-get install -y socat
	fi
	echo "Stopping existing port forwarding processes..."
	sudo pkill -f "socat.*32080" 2>/dev/null || true
	sudo pkill -f "socat.*32443" 2>/dev/null || true
	sleep 1
	echo "Setting up port forwarding for 32080 (HTTP)..."
	sudo nohup socat TCP4-LISTEN:32080,fork,reuseaddr TCP4:$${KIND_IP}:32080 > /tmp/socat-32080.log 2>&1 &
	echo "Setting up port forwarding for 32443 (HTTPS)..."
	sudo nohup socat TCP4-LISTEN:32443,fork,reuseaddr TCP4:$${KIND_IP}:32443 > /tmp/socat-32443.log 2>&1 &
	sleep 2
	if ps aux | grep -q "[s]ocat.*32080"; then
	    echo -e "$${GREEN}✓ Port 32080 forwarding active$${NC}"
	else
	    echo -e "$${RED}✗ Port 32080 forwarding failed$${NC}"
	    cat /tmp/socat-32080.log
	fi
	if ps aux | grep -q "[s]ocat.*32443"; then
	    echo -e "$${GREEN}✓ Port 32443 forwarding active$${NC}"
	else
	    echo -e "$${RED}✗ Port 32443 forwarding failed$${NC}"
	    cat /tmp/socat-32443.log
	fi
	echo ""
	echo "Testing port forwarding..."
	if curl -s -f -H "Host: lookout-stg.timonier.io" http://localhost:32080/health > /dev/null 2>&1; then
	    echo -e "$${GREEN}✓ HTTP port 32080 is accessible$${NC}"
	else
	    echo -e "$${YELLOW}⚠ HTTP port 32080 test failed (may be normal if app not yet deployed)$${NC}"
	fi
	if curl -s -f -k -H "Host: lookout-stg.timonier.io" https://localhost:32443/health > /dev/null 2>&1; then
	    echo -e "$${GREEN}✓ HTTPS port 32443 is accessible$${NC}"
	else
	    echo -e "$${YELLOW}⚠ HTTPS port 32443 test failed (may be normal if app not yet deployed)$${NC}"
	fi
	echo ""
	echo -e "$${GREEN}✅ NodePort forwarding setup complete!$${NC}"
	echo ""
	echo "Port forwarding processes:"
	ps aux | grep "[s]ocat.*3204"
	echo ""
	echo "To stop port forwarding:"
	echo "  sudo pkill -f 'socat.*32080'"
	echo "  sudo pkill -f 'socat.*32443'"
	echo ""
	echo "To make this persistent across reboots, add to /etc/rc.local or create a systemd service"

##@ Gateway

gateway: ## Install GatewayClass and TLS cert secret (NAMESPACE, DOMAIN, CERT_DAYS)
	set -e
	NAMESPACE_VAL="$(NAMESPACE)"
	DOMAIN="$${DOMAIN:-lookout-stg.timonier.io}"
	CERT_DAYS="$${CERT_DAYS:-365}"
	echo "🚀 Setting up Envoy Gateway and TLS certificates..."
	if ! command -v kubectl &> /dev/null; then
	    echo "❌ kubectl is not installed"
	    exit 1
	fi
	if ! command -v openssl &> /dev/null; then
	    echo "❌ openssl is not installed"
	    exit 1
	fi
	echo "📦 Creating GatewayClass 'eg'..."
	cat <<EOF | kubectl apply -f -
	apiVersion: gateway.networking.k8s.io/v1
	kind: GatewayClass
	metadata:
	  name: eg
	spec:
	  controllerName: gateway.envoyproxy.io/gatewayclass-controller
	EOF
	echo "🔐 Generating self-signed TLS certificate for $${DOMAIN}..."
	TMP_DIR=$$(mktemp -d)
	trap "rm -rf $${TMP_DIR}" EXIT
	openssl req -x509 -newkey rsa:4096 \
	  -keyout "$${TMP_DIR}/tls.key" \
	  -out "$${TMP_DIR}/tls.crt" \
	  -days $${CERT_DAYS} \
	  -nodes \
	  -subj "/CN=$${DOMAIN}/O=Timonier Systems" \
	  -addext "subjectAltName=DNS:$${DOMAIN},DNS:*.$${DOMAIN}" \
	  2>/dev/null
	echo "📝 Creating TLS secret 'lookout-tls' in namespace $${NAMESPACE_VAL}..."
	kubectl create namespace $${NAMESPACE_VAL} --dry-run=client -o yaml | kubectl apply -f -
	kubectl create secret tls lookout-tls \
	  --cert="$${TMP_DIR}/tls.crt" \
	  --key="$${TMP_DIR}/tls.key" \
	  --namespace=$${NAMESPACE_VAL} \
	  --dry-run=client -o yaml | kubectl apply -f -
	echo "✅ Gateway setup completed successfully!"
	echo ""
	echo "📋 Certificate Details:"
	echo "   Domain: $${DOMAIN}"
	echo "   Wildcard: *.$${DOMAIN}"
	echo "   Valid for: $${CERT_DAYS} days"
	echo "   Secret: lookout-tls (namespace: $${NAMESPACE_VAL})"
	echo ""
	echo "🔍 Verify setup:"
	echo "   kubectl get gatewayclass eg"
	echo "   kubectl get secret lookout-tls -n $${NAMESPACE_VAL}"
	echo "   kubectl get gateway -n $${NAMESPACE_VAL}"
	echo ""
	echo "📦 Next steps:"
	echo "   1. Deploy Gateway: kubectl apply -f helm/lookout/templates/gateway.yaml"
	echo "   2. Check Gateway status: kubectl get gateway -n $${NAMESPACE_VAL}"
	echo "   3. Access via NodePort or configure LoadBalancer"
	echo ""

health-route: ## Create /health HTTPRoute for ALB health checks
	RED='\033[0;31m'
	GREEN='\033[0;32m'
	YELLOW='\033[1;33m'
	NC='\033[0m'
	echo "🔧 Setting up health check HTTPRoute for ALB integration"
	echo ""
	if ! command -v kubectl &> /dev/null; then
	    echo -e "$${RED}ERROR: kubectl not found$${NC}"
	    exit 1
	fi
	if ! kubectl get gateway lookout -n staging &> /dev/null; then
	    echo -e "$${RED}ERROR: Gateway 'lookout' not found in staging namespace$${NC}"
	    echo "Make sure the Gateway is deployed first"
	    exit 1
	fi
	echo "Creating health check HTTPRoute..."
	cat <<EOF | kubectl apply -f -
	apiVersion: gateway.networking.k8s.io/v1
	kind: HTTPRoute
	metadata:
	  name: lookout-health
	  namespace: staging
	  labels:
	    app: lookout
	    component: health-check
	spec:
	  parentRefs:
	  - name: lookout
	  rules:
	  - matches:
	    - path:
	        type: Exact
	        value: /health
	    backendRefs:
	    - name: lookout-staging-lookout-app
	      port: 3000
	EOF
	if [ $$? -eq 0 ]; then
	    echo -e "$${GREEN}✓ Health check HTTPRoute created$${NC}"
	    echo ""
	    echo "Testing health endpoint without Host header..."
	    sleep 3
	    EC2_IP=$$(hostname -I | awk '{print $$1}')
	    if curl -k -s -f https://$${EC2_IP}:32443/health > /dev/null 2>&1; then
	        echo -e "$${GREEN}✓ Health endpoint accessible without Host header$${NC}"
	        curl -k -s https://$${EC2_IP}:32443/health
	    else
	        echo -e "$${YELLOW}⚠ Health endpoint test failed (may need to wait for route to propagate)$${NC}"
	    fi
	else
	    echo -e "$${RED}✗ Failed to create health check HTTPRoute$${NC}"
	    exit 1
	fi
	echo ""
	echo -e "$${GREEN}✅ Health check HTTPRoute setup complete!$${NC}"
	echo ""
	echo "This allows ALB health checks to work without sending the Host header"
	echo "The HTTPRoute accepts requests to /health from any hostname"

##@ ArgoCD

argocd: ## Install ArgoCD on kind cluster
	set -e
	echo "🚀 Setting up ArgoCD on kind cluster..."
	if ! command -v kubectl &> /dev/null; then
	    echo "❌ kubectl is not installed"
	    exit 1
	fi
	if ! kubectl cluster-info | grep -q "kind-lookout"; then
	    echo "❌ Not connected to kind-lookout cluster"
	    echo "Run: kubectl config use-context kind-lookout"
	    exit 1
	fi
	echo "📦 Creating argocd namespace..."
	kubectl create namespace argocd --dry-run=client -o yaml | kubectl apply -f -
	echo "📥 Installing ArgoCD..."
	kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
	echo "🔧 Fixing ApplicationSets CRD (using server-side apply to avoid annotation size limits)..."
	kubectl apply --server-side -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/crds/applicationset-crd.yaml
	echo "⏳ Waiting for ArgoCD to be ready..."
	kubectl wait --for=condition=available --timeout=300s \
	    deployment/argocd-server -n argocd
	echo "✅ ArgoCD installed successfully!"
	echo ""
	echo "📋 ArgoCD Admin Password:"
	kubectl -n argocd get secret argocd-initial-admin-secret \
	    -o jsonpath="{.data.password}" | base64 -d
	echo ""
	echo ""
	echo "🌐 To access ArgoCD UI:"
	echo "   1. Port forward: kubectl port-forward svc/argocd-server -n argocd 8080:443"
	echo "   2. Open: https://localhost:8080"
	echo "   3. Login: admin / <password above>"
	echo ""
	echo "🔧 To change password:"
	echo "   argocd account update-password"
	echo ""
	echo "📦 Next steps:"
	echo "   1. Create GitHub Container Registry secret: make ghcr-secret"
	echo "   2. Deploy staging application: kubectl apply -f k8s/argocd/staging-application.yaml"
	echo ""

argocd-github: ## Configure ArgoCD with GitHub repo credentials (GITHUB_USERNAME, GITHUB_TOKEN)
	set -e
	echo "🔐 Setting up GitHub repository access for ArgoCD..."
	GITHUB_USERNAME="$${GITHUB_USERNAME:-}"
	GITHUB_TOKEN="$${GITHUB_TOKEN:-}"
	REPO_URL="$${REPO_URL:-https://github.com/timoniersystems/lookout.git}"
	if [[ -z "$$GITHUB_USERNAME" ]]; then
	    echo "❌ Error: GITHUB_USERNAME environment variable is not set"
	    echo ""
	    echo "Usage:"
	    echo "  GITHUB_USERNAME=<your-username> GITHUB_TOKEN=<your-token> make argocd-github"
	    echo ""
	    echo "Or export them first:"
	    echo "  export GITHUB_USERNAME=<your-username>"
	    echo "  export GITHUB_TOKEN=<your-token>"
	    echo "  make argocd-github"
	    exit 1
	fi
	if [[ -z "$$GITHUB_TOKEN" ]]; then
	    echo "❌ Error: GITHUB_TOKEN environment variable is not set"
	    echo ""
	    echo "To create a GitHub token:"
	    echo "  1. Go to https://github.com/settings/tokens"
	    echo "  2. Click 'Generate new token (classic)'"
	    echo "  3. Select scopes: 'repo' (full control of private repositories)"
	    echo "  4. Copy the token"
	    exit 1
	fi
	if ! command -v kubectl &> /dev/null; then
	    echo "❌ kubectl is not installed"
	    exit 1
	fi
	if ! kubectl get namespace argocd &> /dev/null; then
	    echo "❌ ArgoCD namespace not found"
	    echo "Run: make argocd first"
	    exit 1
	fi
	echo "📦 Adding repository credentials to ArgoCD..."
	kubectl create secret generic github-repo-creds \
	    --namespace argocd \
	    --from-literal=username="$$GITHUB_USERNAME" \
	    --from-literal=password="$$GITHUB_TOKEN" \
	    --from-literal=url="$$REPO_URL" \
	    --dry-run=client -o yaml | kubectl apply -f -
	kubectl label secret github-repo-creds \
	    --namespace argocd \
	    argocd.argoproj.io/secret-type=repository \
	    --overwrite
	echo "✅ GitHub repository credentials configured successfully!"
	echo ""
	echo "📝 Repository configured:"
	echo "   URL: $$REPO_URL"
	echo "   Username: $$GITHUB_USERNAME"
	echo ""
	echo "🔄 ArgoCD will now be able to access this private repository"
	echo ""
	echo "📦 Next steps:"
	echo "   1. Verify: kubectl get secret github-repo-creds -n argocd"
	echo "   2. Deploy application: kubectl apply -f k8s/argocd/staging-application.yaml"
	echo ""

argocd-image-updater: ## Install ArgoCD Image Updater
	set -e
	echo "🔄 Setting up ArgoCD Image Updater..."
	if ! command -v kubectl &> /dev/null; then
	    echo "❌ kubectl is not installed"
	    exit 1
	fi
	echo "📥 Installing ArgoCD Image Updater..."
	kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj-labs/argocd-image-updater/stable/manifests/install.yaml
	echo "⏳ Waiting for Image Updater to be ready..."
	kubectl wait --for=condition=available --timeout=120s \
	    deployment/argocd-image-updater -n argocd
	echo "✅ ArgoCD Image Updater installed successfully!"
	echo ""
	echo "📝 Image Updater is configured via annotations on the Application:"
	echo "   - argocd-image-updater.argoproj.io/image-list: specifies which images to track"
	echo "   - argocd-image-updater.argoproj.io/update-strategy: how to update (digest, latest-tag, etc)"
	echo ""
	echo "Example annotations are already included in staging-application.yaml"

##@ Secrets & Auth

external-secrets: ## Install External Secrets Operator and sync NVD API key from AWS
	set -e
	GREEN='\033[0;32m'
	BLUE='\033[0;34m'
	YELLOW='\033[1;33m'
	NC='\033[0m'
	echo -e "$${BLUE}🔐 Setting up External Secrets Operator$${NC}\n"
	AWS_REGION="$${AWS_REGION:-us-west-2}"
	SECRET_NAME="lookout/staging/nvd-api-key"
	NAMESPACE_VAL="staging"
	if ! kubectl get namespace $${NAMESPACE_VAL} &> /dev/null; then
	    echo "Creating namespace: $${NAMESPACE_VAL}"
	    kubectl create namespace $${NAMESPACE_VAL}
	fi
	echo -e "$${YELLOW}📦 Step 1: Installing External Secrets Operator$${NC}"
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
	echo "Verifying CRDs are available..."
	for i in {1..30}; do
	    if kubectl get crd secretstores.external-secrets.io &> /dev/null && \
	       kubectl get crd externalsecrets.external-secrets.io &> /dev/null; then
	        echo -e "$${GREEN}✓ CRDs are ready$${NC}"
	        break
	    fi
	    if [ $$i -eq 30 ]; then
	        echo -e "$${YELLOW}⚠ CRDs not found after 60s$${NC}"
	        echo "Installed CRDs:"
	        kubectl get crd | grep external-secrets || echo "  (none)"
	        exit 1
	    fi
	    echo "Waiting for CRDs... ($$i/30)"
	    sleep 2
	done
	echo "Waiting for API registration..."
	for i in {1..30}; do
	    if kubectl api-resources --api-group=external-secrets.io 2>/dev/null | grep -q SecretStore; then
	        echo -e "$${GREEN}✓ API resources registered$${NC}"
	        break
	    fi
	    if [ $$i -eq 30 ]; then
	        echo -e "$${YELLOW}⚠ API resources not registered after 60s$${NC}"
	        kubectl api-resources --api-group=external-secrets.io || true
	        exit 1
	    fi
	    echo "Waiting for API registration... ($$i/30)"
	    sleep 2
	done
	echo "External Secrets Operator pods:"
	kubectl get pods -n external-secrets-system
	echo -e "$${GREEN}✓ External Secrets Operator ready$${NC}\n"
	echo -e "$${YELLOW}📋 Step 2: Creating IAM policy for Secrets Manager$${NC}"
	POLICY_NAME="LookoutSecretsManagerAccess"
	POLICY_ARN=$$(aws iam list-policies --query "Policies[?PolicyName=='$${POLICY_NAME}'].Arn" --output text)
	if [ -z "$$POLICY_ARN" ]; then
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
	            "Resource": "arn:aws:secretsmanager:$${AWS_REGION}:*:secret:lookout/*"
	        }
	    ]
	}
	EOF
	    POLICY_ARN=$$(aws iam create-policy \
	        --policy-name "$${POLICY_NAME}" \
	        --policy-document file:///tmp/secrets-policy.json \
	        --query 'Policy.Arn' \
	        --output text)
	    rm /tmp/secrets-policy.json
	    echo -e "$${GREEN}✓ Created IAM policy: $${POLICY_ARN}$${NC}"
	else
	    echo -e "Policy already exists: $${POLICY_ARN}"
	fi
	echo ""
	echo -e "$${YELLOW}🔗 Step 3: Setting up IAM user and K8s credentials$${NC}"
	IAM_USER="lookout-external-secrets"
	if aws iam get-user --user-name "$${IAM_USER}" &> /dev/null; then
	    echo "IAM user $${IAM_USER} already exists"
	else
	    echo "Creating IAM user: $${IAM_USER}"
	    aws iam create-user --user-name "$${IAM_USER}" > /dev/null
	fi
	if aws iam list-attached-user-policies --user-name "$${IAM_USER}" --query "AttachedPolicies[?PolicyArn=='$${POLICY_ARN}'].PolicyArn" --output text | grep -q "$${POLICY_ARN}"; then
	    echo "Policy already attached to user: $${IAM_USER}"
	else
	    echo "Attaching policy to user: $${IAM_USER}"
	    aws iam attach-user-policy \
	        --user-name "$${IAM_USER}" \
	        --policy-arn "$${POLICY_ARN}"
	fi
	if kubectl get secret aws-credentials -n $${NAMESPACE_VAL} &> /dev/null; then
	    echo "K8s secret aws-credentials already exists"
	else
	    echo "Creating access key for IAM user..."
	    ACCESS_KEY_JSON=$$(aws iam create-access-key --user-name "$${IAM_USER}")
	    ACCESS_KEY_ID=$$(echo "$$ACCESS_KEY_JSON" | python3 -c "import sys,json; print(json.load(sys.stdin)['AccessKey']['AccessKeyId'])")
	    SECRET_ACCESS_KEY=$$(echo "$$ACCESS_KEY_JSON" | python3 -c "import sys,json; print(json.load(sys.stdin)['AccessKey']['SecretAccessKey'])")
	    kubectl apply -f - <<CREDEOF
	apiVersion: v1
	kind: Secret
	metadata:
	  name: aws-credentials
	  namespace: $${NAMESPACE_VAL}
	type: Opaque
	stringData:
	  access-key-id: "$${ACCESS_KEY_ID}"
	  secret-access-key: "$${SECRET_ACCESS_KEY}"
	CREDEOF
	    echo -e "$${GREEN}✓ AWS credentials secret created in K8s$${NC}"
	fi
	echo -e "$${GREEN}✓ IAM user and credentials configured$${NC}"
	echo ""
	echo -e "$${YELLOW}🔑 Step 4: Creating secret in AWS Secrets Manager$${NC}"
	read -p "Enter your NVD API Key (or press Enter to skip): " NVD_API_KEY
	if [ -n "$$NVD_API_KEY" ]; then
	    if aws secretsmanager describe-secret --secret-id "$${SECRET_NAME}" --region $${AWS_REGION} &> /dev/null; then
	        echo "Secret already exists, updating..."
	        aws secretsmanager update-secret \
	            --secret-id "$${SECRET_NAME}" \
	            --secret-string "{\"NVD_API_KEY\":\"$${NVD_API_KEY}\"}" \
	            --region $${AWS_REGION} > /dev/null
	    else
	        echo "Creating new secret..."
	        aws secretsmanager create-secret \
	            --name "$${SECRET_NAME}" \
	            --description "NVD API key for Lookout staging environment" \
	            --secret-string "{\"NVD_API_KEY\":\"$${NVD_API_KEY}\"}" \
	            --region $${AWS_REGION} > /dev/null
	    fi
	    echo -e "$${GREEN}✓ Secret created/updated in AWS Secrets Manager$${NC}"
	else
	    echo -e "$${YELLOW}⚠ Skipped - you can create the secret manually later$${NC}"
	fi
	echo ""
	echo -e "$${YELLOW}🏪 Step 5: Creating SecretStore$${NC}"
	kubectl apply -f - <<EOF
	apiVersion: external-secrets.io/v1
	kind: SecretStore
	metadata:
	  name: aws-secrets-manager
	  namespace: $${NAMESPACE_VAL}
	spec:
	  provider:
	    aws:
	      service: SecretsManager
	      region: $${AWS_REGION}
	      auth:
	        secretRef:
	          accessKeyIDSecretRef:
	            name: aws-credentials
	            key: access-key-id
	          secretAccessKeySecretRef:
	            name: aws-credentials
	            key: secret-access-key
	EOF
	echo -e "$${GREEN}✓ SecretStore created$${NC}\n"
	echo -e "$${YELLOW}🔄 Step 6: Creating ExternalSecret$${NC}"
	kubectl apply -f - <<EOF
	apiVersion: external-secrets.io/v1
	kind: ExternalSecret
	metadata:
	  name: lookout-nvd-api-key
	  namespace: $${NAMESPACE_VAL}
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
	        key: $${SECRET_NAME}
	        property: NVD_API_KEY
	EOF
	echo -e "$${GREEN}✓ ExternalSecret created$${NC}\n"
	echo -e "$${YELLOW}✅ Step 7: Verifying setup$${NC}"
	echo "SecretStore status:"
	kubectl get secretstore -n $${NAMESPACE_VAL}
	echo ""
	echo "ExternalSecret status:"
	kubectl get externalsecret -n $${NAMESPACE_VAL}
	echo ""
	echo "Waiting for secret to sync..."
	sleep 5
	if kubectl get secret lookout-staging-lookout-app -n $${NAMESPACE_VAL} &> /dev/null; then
	    echo -e "$${GREEN}✓ Secret successfully synced from AWS Secrets Manager$${NC}"
	    kubectl get secret lookout-staging-lookout-app -n $${NAMESPACE_VAL} -o jsonpath='{.data.NVD_API_KEY}' | base64 -d | wc -c | xargs echo "Secret length:"
	else
	    echo -e "$${YELLOW}⚠ Secret not yet synced. Check ExternalSecret status:$${NC}"
	    kubectl describe externalsecret lookout-nvd-api-key -n $${NAMESPACE_VAL}
	fi
	echo ""
	echo -e "$${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$${NC}"
	echo -e "$${GREEN}✅ External Secrets setup complete!$${NC}"
	echo -e "$${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$${NC}"
	echo ""
	echo "📋 Next steps:"
	echo "  1. Verify secret exists: kubectl get secret lookout-staging-lookout-app -n staging"
	echo "  2. Restart deployment: kubectl rollout restart deployment/lookout-staging-lookout-app -n staging"
	echo "  3. Check logs: kubectl logs -n staging deployment/lookout-staging-lookout-app"
	echo ""
	echo "🔧 To update the secret:"
	echo "  aws secretsmanager update-secret \\"
	echo "    --secret-id $${SECRET_NAME} \\"
	echo "    --secret-string '{\"NVD_API_KEY\":\"your-new-key\"}' \\"
	echo "    --region $${AWS_REGION}"
	echo ""

basic-auth: ## Store htpasswd in AWS Secrets Manager and sync to k8s (NAMESPACE, BASIC_AUTH_PASSWORD)
	set -e
	GREEN='\033[0;32m'
	BLUE='\033[0;34m'
	YELLOW='\033[1;33m'
	RED='\033[0;31m'
	NC='\033[0m'
	AWS_REGION="$${AWS_REGION:-us-west-2}"
	NAMESPACE_VAL="$(NAMESPACE)"
	SECRET_NAME="$${SECRET_NAME:-lookout/$${NAMESPACE_VAL}/basic-auth}"
	echo -e "$${BLUE}🔐 Setting up Basic Authentication for $${NAMESPACE_VAL}$${NC}\n"
	echo -e "$${YELLOW}📋 Step 1: Checking prerequisites$${NC}"
	if ! command -v htpasswd &> /dev/null; then
	    echo -e "$${RED}✗ htpasswd not found$${NC}"
	    echo "  Install it with:"
	    echo "    Ubuntu/Debian: sudo apt-get install apache2-utils"
	    echo "    macOS:         brew install httpd"
	    exit 1
	fi
	echo -e "$${GREEN}✓ htpasswd found$${NC}"
	if ! command -v aws &> /dev/null; then
	    echo -e "$${RED}✗ AWS CLI not found$${NC}"
	    exit 1
	fi
	echo -e "$${GREEN}✓ AWS CLI found$${NC}"
	if ! aws sts get-caller-identity &> /dev/null; then
	    echo -e "$${RED}✗ AWS credentials not configured or expired$${NC}"
	    exit 1
	fi
	echo -e "$${GREEN}✓ AWS credentials valid$${NC}"
	echo ""
	echo -e "$${YELLOW}🔑 Step 2: Generating credentials$${NC}"
	DEFAULT_USERNAME="$${NAMESPACE_VAL}"
	read -p "Enter username [$${DEFAULT_USERNAME}]: " USERNAME
	USERNAME="$${USERNAME:-$${DEFAULT_USERNAME}}"
	if [ -n "$${BASIC_AUTH_PASSWORD:-}" ]; then
	    PASSWORD="$$BASIC_AUTH_PASSWORD"
	    echo "Using password from BASIC_AUTH_PASSWORD environment variable"
	else
	    read -s -p "Enter password: " PASSWORD
	    echo ""
	    if [ -z "$$PASSWORD" ]; then
	        echo -e "$${RED}✗ Password cannot be empty$${NC}"
	        exit 1
	    fi
	    read -s -p "Confirm password: " PASSWORD_CONFIRM
	    echo ""
	    if [ "$$PASSWORD" != "$$PASSWORD_CONFIRM" ]; then
	        echo -e "$${RED}✗ Passwords do not match$${NC}"
	        exit 1
	    fi
	fi
	HTPASSWD_ENTRY=$$(htpasswd -nbs "$$USERNAME" "$$PASSWORD")
	echo -e "$${GREEN}✓ Generated htpasswd entry for user: $${USERNAME}$${NC}"
	echo ""
	echo -e "$${YELLOW}☁️  Step 3: Storing credentials in AWS Secrets Manager$${NC}"
	HTPASSWD_JSON=$$(python3 -c "import json,sys; print(json.dumps({'htpasswd': sys.stdin.read().strip()}))" <<< "$$HTPASSWD_ENTRY")
	if aws secretsmanager describe-secret --secret-id "$${SECRET_NAME}" --region "$${AWS_REGION}" &> /dev/null; then
	    echo "Secret already exists, updating..."
	    aws secretsmanager update-secret \
	        --secret-id "$${SECRET_NAME}" \
	        --secret-string "$${HTPASSWD_JSON}" \
	        --region "$${AWS_REGION}" > /dev/null
	    echo -e "$${GREEN}✓ Secret updated in AWS Secrets Manager$${NC}"
	else
	    echo "Creating new secret..."
	    aws secretsmanager create-secret \
	        --name "$${SECRET_NAME}" \
	        --description "Basic auth credentials for Lookout $${NAMESPACE_VAL}" \
	        --secret-string "$${HTPASSWD_JSON}" \
	        --region "$${AWS_REGION}" > /dev/null
	    echo -e "$${GREEN}✓ Secret created in AWS Secrets Manager$${NC}"
	fi
	echo ""
	echo -e "$${YELLOW}✅ Step 4: Verifying secret in AWS$${NC}"
	STORED_VALUE=$$(aws secretsmanager get-secret-value \
	    --secret-id "$${SECRET_NAME}" \
	    --region "$${AWS_REGION}" \
	    --query 'SecretString' \
	    --output text)
	STORED_HTPASSWD=$$(echo "$$STORED_VALUE" | python3 -c "import sys,json; print(json.load(sys.stdin)['htpasswd'])")
	if [ -n "$$STORED_HTPASSWD" ]; then
	    echo -e "$${GREEN}✓ Secret verified in AWS Secrets Manager$${NC}"
	    echo "  Secret name: $${SECRET_NAME}"
	    echo "  Username:    $${USERNAME}"
	    echo "  Hash:        $${STORED_HTPASSWD:0:20}..."
	else
	    echo -e "$${RED}✗ Failed to verify secret$${NC}"
	    exit 1
	fi
	echo ""
	echo -e "$${YELLOW}🔄 Step 5: Syncing to Kubernetes$${NC}"
	if command -v kubectl &> /dev/null && kubectl cluster-info &> /dev/null; then
	    RELEASE_NAME="lookout-$${NAMESPACE_VAL}"
	    BASIC_AUTH_SECRET_NAME="$${RELEASE_NAME}-basic-auth"
	    if kubectl get externalsecret -n "$${NAMESPACE_VAL}" 2>/dev/null | grep -q basic-auth; then
	        echo "ExternalSecret for basic-auth found, forcing sync..."
	        kubectl delete secret -n "$${NAMESPACE_VAL}" -l "reconcile.external-secrets.io/managed=true" --field-selector "metadata.name=$${BASIC_AUTH_SECRET_NAME}" 2>/dev/null || true
	        echo "Waiting for sync..."
	        sleep 10
	        if kubectl get secret "$${BASIC_AUTH_SECRET_NAME}" -n "$${NAMESPACE_VAL}" &> /dev/null; then
	            echo -e "$${GREEN}✓ Secret synced to Kubernetes$${NC}"
	        else
	            echo -e "$${YELLOW}⚠ Secret not yet synced. It will sync within the configured refreshInterval (default: 1h)$${NC}"
	            echo "  Check status: kubectl get externalsecret -n $${NAMESPACE_VAL}"
	        fi
	    else
	        echo -e "$${YELLOW}⚠ ExternalSecret for basic-auth not found in cluster$${NC}"
	        echo "  Deploy the Helm chart first, then the secret will sync automatically."
	        echo "  Or run: argocd app sync $${RELEASE_NAME}"
	    fi
	else
	    echo -e "$${YELLOW}⚠ kubectl not available or cluster not reachable$${NC}"
	    echo "  The secret will sync automatically when the ExternalSecret is deployed."
	fi
	echo ""
	echo -e "$${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$${NC}"
	echo -e "$${GREEN}✅ Basic auth setup complete!$${NC}"
	echo -e "$${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$${NC}"
	echo ""
	if [ "$${NAMESPACE_VAL}" = "production" ]; then
	    DOMAIN="lookout-prod.timonier.io"
	else
	    DOMAIN="lookout-stg.timonier.io"
	fi
	echo "📋 Access the $${NAMESPACE_VAL} site:"
	echo "  Browser: https://$${DOMAIN} (enter $${USERNAME} / your password)"
	echo "  curl:    curl -u $${USERNAME}:PASSWORD https://$${DOMAIN}/"
	echo ""
	echo "🔧 To update the password later:"
	echo "  make basic-auth"
	echo ""
	echo "🔧 To update non-interactively:"
	echo "  BASIC_AUTH_PASSWORD='newpass' make basic-auth"
	echo ""

ghcr-secret: ## Create ghcr.io image pull secret in k8s (GITHUB_USERNAME, GITHUB_TOKEN, NAMESPACE)
	set -e
	NAMESPACE_VAL="$(NAMESPACE)"
	if [[ -z "$${GITHUB_USERNAME:-}" ]] || [[ -z "$${GITHUB_TOKEN:-}" ]]; then
	    echo "❌ Error: Required environment variables not set"
	    echo ""
	    echo "Usage:"
	    echo "  export GITHUB_USERNAME=<your-github-username>"
	    echo "  export GITHUB_TOKEN=<your-github-token>"
	    echo "  make ghcr-secret [NAMESPACE=<namespace>]"
	    echo ""
	    echo "To create a GitHub token:"
	    echo "  1. Go to https://github.com/settings/tokens"
	    echo "  2. Generate new token (classic)"
	    echo "  3. Select 'read:packages' scope"
	    exit 1
	fi
	echo "🔐 Creating ghcr.io pull secret in namespace: $${NAMESPACE_VAL}"
	kubectl create namespace "$$NAMESPACE_VAL" --dry-run=client -o yaml | kubectl apply -f -
	kubectl create secret docker-registry ghcr-secret \
	    --docker-server=ghcr.io \
	    --docker-username="$$GITHUB_USERNAME" \
	    --docker-password="$$GITHUB_TOKEN" \
	    --docker-email="$$GITHUB_USERNAME@users.noreply.github.com" \
	    --namespace="$$NAMESPACE_VAL" \
	    --dry-run=client -o yaml | kubectl apply -f -
	echo "✅ Secret 'ghcr-secret' created in namespace '$${NAMESPACE_VAL}'"
	echo ""
	echo "📝 To use this secret in your deployments, add:"
	echo "   imagePullSecrets:"
	echo "     - name: ghcr-secret"

##@ AWS / ALB

alb: ## Create AWS ALB with HTTPS and target groups (requires VPC_ID, EC2_INSTANCE_ID, SUBNET_*, CERTIFICATE_ARN, HOSTED_ZONE_ID)
	RED='\033[0;31m'
	GREEN='\033[0;32m'
	YELLOW='\033[1;33m'
	NC='\033[0m'
	echo "🚀 Setting up AWS Application Load Balancer for Lookout"
	if [ "$${ENABLE_PROD:-}" == "true" ]; then
	    echo "   Mode: Multi-environment (Staging + Production)"
	else
	    echo "   Mode: Single environment (Staging only)"
	fi
	echo "   This script is idempotent and safe to re-run"
	echo ""
	if [ -z "$${AWS_REGION:-}" ]; then
	    echo -e "$${YELLOW}AWS_REGION not set. Using default: us-east-1$${NC}"
	    export AWS_REGION=us-east-1
	fi
	if [ -z "$${VPC_ID:-}" ]; then
	    echo -e "$${RED}ERROR: VPC_ID environment variable not set$${NC}"
	    echo "Usage: VPC_ID=vpc-xxxxx EC2_INSTANCE_ID=i-xxxxx SUBNET_1=subnet-xxxxx SUBNET_2=subnet-xxxxx CERTIFICATE_ARN=arn:aws:... HOSTED_ZONE_ID=Z0xxxxx make alb"
	    exit 1
	fi
	REQUIRED_VARS=(VPC_ID EC2_INSTANCE_ID SUBNET_1 SUBNET_2 SUBNET_3 CERTIFICATE_ARN HOSTED_ZONE_ID)
	for var in "$${REQUIRED_VARS[@]}"; do
	    if [ -z "$${!var}" ]; then
	        echo -e "$${RED}ERROR: $$var environment variable not set$${NC}"
	        exit 1
	    fi
	done
	echo "Configuration:"
	echo "  AWS Region: $$AWS_REGION"
	echo "  VPC ID: $$VPC_ID"
	echo "  EC2 Instance: $$EC2_INSTANCE_ID"
	echo "  Subnets: $$SUBNET_1, $$SUBNET_2, $$SUBNET_3"
	echo "  Certificate ARN: $$CERTIFICATE_ARN"
	echo "  Hosted Zone ID: $$HOSTED_ZONE_ID"
	echo ""
	echo "🔍 Verifying fixed NodePorts..."
	if ! kubectl get svc -n envoy-gateway-system -l gateway.envoyproxy.io/owning-gateway-name=lookout 2>/dev/null | grep -q "32080.*32443"; then
	    echo -e "$${RED}ERROR: Fixed NodePorts not configured. Run make kind-nodeports first$${NC}"
	    exit 1
	fi
	echo -e "$${GREEN}✓ Fixed NodePorts verified (32080, 32443)$${NC}"
	echo ""
	echo "🔐 Creating ALB security group..."
	ALB_SG_ID=$$(aws ec2 create-security-group \
	    --group-name lookout-alb-sg \
	    --description "Security group for Lookout ALB" \
	    --vpc-id $$VPC_ID \
	    --region $$AWS_REGION \
	    --output text --query 'GroupId' 2>/dev/null || \
	    aws ec2 describe-security-groups \
	        --filters "Name=group-name,Values=lookout-alb-sg" "Name=vpc-id,Values=$$VPC_ID" \
	        --region $$AWS_REGION \
	        --query 'SecurityGroups[0].GroupId' \
	        --output text)
	echo -e "$${GREEN}✓ ALB Security Group: $$ALB_SG_ID$${NC}"
	echo "🔐 Configuring ALB security group rules..."
	aws ec2 authorize-security-group-ingress \
	    --group-id $$ALB_SG_ID \
	    --ip-permissions \
	    IpProtocol=tcp,FromPort=80,ToPort=80,IpRanges='[{CidrIp=0.0.0.0/0,Description="Allow HTTP from anywhere"}]' \
	    IpProtocol=tcp,FromPort=443,ToPort=443,IpRanges='[{CidrIp=0.0.0.0/0,Description="Allow HTTPS from anywhere"}]' \
	    --region $$AWS_REGION 2>/dev/null || echo "  (rules already exist)"
	echo "🔐 Configuring EC2 security group..."
	EC2_SG_ID=$$(aws ec2 describe-instances \
	    --instance-ids $$EC2_INSTANCE_ID \
	    --region $$AWS_REGION \
	    --query 'Reservations[0].Instances[0].SecurityGroups[0].GroupId' \
	    --output text)
	echo -e "$${GREEN}✓ EC2 Security Group: $$EC2_SG_ID$${NC}"
	aws ec2 authorize-security-group-ingress \
	    --group-id $$EC2_SG_ID \
	    --ip-permissions \
	    IpProtocol=tcp,FromPort=32080,ToPort=32080,UserIdGroupPairs="[{GroupId=$$ALB_SG_ID,Description='Allow HTTP from ALB'}]" \
	    IpProtocol=tcp,FromPort=32443,ToPort=32443,UserIdGroupPairs="[{GroupId=$$ALB_SG_ID,Description='Allow HTTPS from ALB'}]" \
	    --region $$AWS_REGION 2>/dev/null || echo "  (rules already exist)"
	echo "🎯 Creating HTTP target group..."
	HTTP_TG_ARN=$$(aws elbv2 create-target-group \
	    --name lookout-http-tg \
	    --protocol HTTP \
	    --port 32080 \
	    --vpc-id $$VPC_ID \
	    --health-check-protocol HTTP \
	    --health-check-path /health \
	    --health-check-interval-seconds 10 \
	    --health-check-timeout-seconds 5 \
	    --healthy-threshold-count 2 \
	    --unhealthy-threshold-count 2 \
	    --matcher HttpCode=200 \
	    --region $$AWS_REGION \
	    --query 'TargetGroups[0].TargetGroupArn' \
	    --output text 2>/dev/null || \
	    aws elbv2 describe-target-groups \
	        --names lookout-http-tg \
	        --region $$AWS_REGION \
	        --query 'TargetGroups[0].TargetGroupArn' \
	        --output text)
	echo -e "$${GREEN}✓ HTTP Target Group: $$HTTP_TG_ARN$${NC}"
	echo "🔧 Updating HTTP target group health check settings..."
	aws elbv2 modify-target-group \
	    --target-group-arn $$HTTP_TG_ARN \
	    --health-check-protocol HTTP \
	    --health-check-path /health \
	    --health-check-port 32080 \
	    --health-check-interval-seconds 10 \
	    --health-check-timeout-seconds 5 \
	    --healthy-threshold-count 2 \
	    --unhealthy-threshold-count 2 \
	    --matcher HttpCode=200 \
	    --region $$AWS_REGION > /dev/null
	echo "🎯 Creating HTTPS target group for staging..."
	EXISTING_TG_PROTOCOL=$$(aws elbv2 describe-target-groups \
	    --names lookout-https-tg \
	    --region $$AWS_REGION \
	    --query 'TargetGroups[0].Protocol' \
	    --output text 2>/dev/null)
	if [ "$$EXISTING_TG_PROTOCOL" == "HTTP" ]; then
	    echo -e "$${YELLOW}⚠ Existing target group has wrong protocol (HTTP). Deleting...$${NC}"
	    EXISTING_TG_ARN=$$(aws elbv2 describe-target-groups \
	        --names lookout-https-tg \
	        --region $$AWS_REGION \
	        --query 'TargetGroups[0].TargetGroupArn' \
	        --output text)
	    aws elbv2 delete-target-group \
	        --target-group-arn $$EXISTING_TG_ARN \
	        --region $$AWS_REGION
	    echo -e "$${GREEN}✓ Deleted old target group$${NC}"
	    sleep 2
	fi
	HTTPS_TG_ARN=$$(aws elbv2 create-target-group \
	    --name lookout-https-tg \
	    --protocol HTTPS \
	    --port 32443 \
	    --vpc-id $$VPC_ID \
	    --health-check-protocol HTTPS \
	    --health-check-path /health \
	    --health-check-interval-seconds 10 \
	    --health-check-timeout-seconds 5 \
	    --healthy-threshold-count 2 \
	    --unhealthy-threshold-count 2 \
	    --matcher HttpCode=200 \
	    --region $$AWS_REGION \
	    --query 'TargetGroups[0].TargetGroupArn' \
	    --output text 2>/dev/null || \
	    aws elbv2 describe-target-groups \
	        --names lookout-https-tg \
	        --region $$AWS_REGION \
	        --query 'TargetGroups[0].TargetGroupArn' \
	        --output text)
	echo -e "$${GREEN}✓ HTTPS Target Group: $$HTTPS_TG_ARN$${NC}"
	echo "🔧 Updating HTTPS target group health check settings..."
	aws elbv2 modify-target-group \
	    --target-group-arn $$HTTPS_TG_ARN \
	    --health-check-protocol HTTPS \
	    --health-check-path /health \
	    --health-check-port 32443 \
	    --health-check-interval-seconds 10 \
	    --health-check-timeout-seconds 5 \
	    --healthy-threshold-count 2 \
	    --unhealthy-threshold-count 2 \
	    --matcher HttpCode=200 \
	    --region $$AWS_REGION > /dev/null
	echo "🎯 Registering EC2 instance with target groups..."
	aws elbv2 deregister-targets \
	    --target-group-arn $$HTTP_TG_ARN \
	    --targets Id=$$EC2_INSTANCE_ID \
	    --region $$AWS_REGION 2>/dev/null || true
	aws elbv2 register-targets \
	    --target-group-arn $$HTTP_TG_ARN \
	    --targets Id=$$EC2_INSTANCE_ID,Port=32080 \
	    --region $$AWS_REGION
	aws elbv2 deregister-targets \
	    --target-group-arn $$HTTPS_TG_ARN \
	    --targets Id=$$EC2_INSTANCE_ID \
	    --region $$AWS_REGION 2>/dev/null || true
	aws elbv2 register-targets \
	    --target-group-arn $$HTTPS_TG_ARN \
	    --targets Id=$$EC2_INSTANCE_ID,Port=32443 \
	    --region $$AWS_REGION
	echo -e "$${GREEN}✓ Targets registered$${NC}"
	echo "⚖️  Creating Application Load Balancer..."
	ALB_ARN=$$(aws elbv2 create-load-balancer \
	    --name lookout-alb \
	    --subnets $$SUBNET_1 $$SUBNET_2 $$SUBNET_3 \
	    --security-groups $$ALB_SG_ID \
	    --scheme internet-facing \
	    --type application \
	    --ip-address-type ipv4 \
	    --region $$AWS_REGION \
	    --query 'LoadBalancers[0].LoadBalancerArn' \
	    --output text 2>/dev/null || \
	    aws elbv2 describe-load-balancers \
	        --names lookout-alb \
	        --region $$AWS_REGION \
	        --query 'LoadBalancers[0].LoadBalancerArn' \
	        --output text)
	echo -e "$${GREEN}✓ ALB ARN: $$ALB_ARN$${NC}"
	echo "🔧 Configuring ALB idle timeout..."
	aws elbv2 modify-load-balancer-attributes \
	    --load-balancer-arn $$ALB_ARN \
	    --attributes Key=idle_timeout.timeout_seconds,Value=600 \
	    --region $$AWS_REGION > /dev/null
	echo -e "$${GREEN}✓ ALB idle timeout set to 600 seconds$${NC}"
	echo "🔧 Updating ALB security groups..."
	aws elbv2 set-security-groups \
	    --load-balancer-arn $$ALB_ARN \
	    --security-groups $$ALB_SG_ID \
	    --region $$AWS_REGION > /dev/null || echo "  (security groups already set)"
	ALB_DNS=$$(aws elbv2 describe-load-balancers \
	    --load-balancer-arns $$ALB_ARN \
	    --region $$AWS_REGION \
	    --query 'LoadBalancers[0].DNSName' \
	    --output text)
	ALB_ZONE_ID=$$(aws elbv2 describe-load-balancers \
	    --load-balancer-arns $$ALB_ARN \
	    --region $$AWS_REGION \
	    --query 'LoadBalancers[0].CanonicalHostedZoneId' \
	    --output text)
	echo "  ALB DNS: $$ALB_DNS"
	echo "  ALB Zone ID: $$ALB_ZONE_ID"
	echo "⏳ Waiting for ALB to become active..."
	aws elbv2 wait load-balancer-available \
	    --load-balancer-arns $$ALB_ARN \
	    --region $$AWS_REGION
	echo -e "$${GREEN}✓ ALB is active$${NC}"
	echo "🎧 Creating HTTP listener (redirect to HTTPS)..."
	HTTP_LISTENER_ARN=$$(aws elbv2 describe-listeners \
	    --load-balancer-arn $$ALB_ARN \
	    --region $$AWS_REGION \
	    --query 'Listeners[?Port==`80`].ListenerArn' \
	    --output text 2>/dev/null)
	if [ -z "$$HTTP_LISTENER_ARN" ]; then
	    HTTP_LISTENER_ARN=$$(aws elbv2 create-listener \
	        --load-balancer-arn $$ALB_ARN \
	        --protocol HTTP \
	        --port 80 \
	        --default-actions Type=redirect,RedirectConfig='{Protocol=HTTPS,Port=443,StatusCode=HTTP_301}' \
	        --region $$AWS_REGION \
	        --query 'Listeners[0].ListenerArn' \
	        --output text)
	fi
	echo -e "$${GREEN}✓ HTTP Listener: $$HTTP_LISTENER_ARN$${NC}"
	echo "🎧 Creating HTTPS listener..."
	HTTPS_LISTENER_ARN=$$(aws elbv2 describe-listeners \
	    --load-balancer-arn $$ALB_ARN \
	    --region $$AWS_REGION \
	    --query 'Listeners[?Port==`443`].ListenerArn' \
	    --output text 2>/dev/null)
	if [ -z "$$HTTPS_LISTENER_ARN" ]; then
	    HTTPS_LISTENER_ARN=$$(aws elbv2 create-listener \
	        --load-balancer-arn $$ALB_ARN \
	        --protocol HTTPS \
	        --port 443 \
	        --certificates CertificateArn=$$CERTIFICATE_ARN \
	        --default-actions Type=forward,TargetGroupArn=$$HTTPS_TG_ARN \
	        --region $$AWS_REGION \
	        --query 'Listeners[0].ListenerArn' \
	        --output text)
	else
	    echo "🔧 Updating HTTPS listener configuration..."
	    aws elbv2 modify-listener \
	        --listener-arn $$HTTPS_LISTENER_ARN \
	        --certificates CertificateArn=$$CERTIFICATE_ARN \
	        --default-actions Type=forward,TargetGroupArn=$$HTTPS_TG_ARN \
	        --region $$AWS_REGION > /dev/null || echo "  (listener already configured)"
	fi
	echo -e "$${GREEN}✓ HTTPS Listener: $$HTTPS_LISTENER_ARN$${NC}"
	if [ "$${ENABLE_PROD:-}" == "true" ]; then
	    echo ""
	    echo "🔀 Configuring host-based routing for multiple environments..."
	    echo "   Using shared target groups - routing based on hostname"
	    echo "🔀 Creating host-based routing rules..."
	    STAGING_RULE=$$(aws elbv2 describe-rules \
	        --listener-arn $$HTTPS_LISTENER_ARN \
	        --region $$AWS_REGION \
	        --query 'Rules[?Priority==`1`].RuleArn' \
	        --output text 2>/dev/null)
	    if [ -z "$$STAGING_RULE" ]; then
	        aws elbv2 create-rule \
	            --listener-arn $$HTTPS_LISTENER_ARN \
	            --priority 1 \
	            --conditions Field=host-header,Values=lookout-stg.timonier.io \
	            --actions Type=forward,TargetGroupArn=$$HTTPS_TG_ARN \
	            --region $$AWS_REGION > /dev/null
	        echo -e "$${GREEN}✓ Created routing rule for lookout-stg.timonier.io$${NC}"
	    else
	        aws elbv2 modify-rule \
	            --rule-arn $$STAGING_RULE \
	            --conditions Field=host-header,Values=lookout-stg.timonier.io \
	            --actions Type=forward,TargetGroupArn=$$HTTPS_TG_ARN \
	            --region $$AWS_REGION > /dev/null
	        echo -e "$${GREEN}✓ Updated routing rule for lookout-stg.timonier.io$${NC}"
	    fi
	    PROD_RULE=$$(aws elbv2 describe-rules \
	        --listener-arn $$HTTPS_LISTENER_ARN \
	        --region $$AWS_REGION \
	        --query 'Rules[?Priority==`2`].RuleArn' \
	        --output text 2>/dev/null)
	    if [ -z "$$PROD_RULE" ]; then
	        aws elbv2 create-rule \
	            --listener-arn $$HTTPS_LISTENER_ARN \
	            --priority 2 \
	            --conditions Field=host-header,Values=lookout-prod.timonier.io,lookout.timonier.io \
	            --actions Type=forward,TargetGroupArn=$$HTTPS_TG_ARN \
	            --region $$AWS_REGION > /dev/null
	        echo -e "$${GREEN}✓ Created routing rule for production (lookout-prod.timonier.io, lookout.timonier.io)$${NC}"
	    else
	        aws elbv2 modify-rule \
	            --rule-arn $$PROD_RULE \
	            --conditions Field=host-header,Values=lookout-prod.timonier.io,lookout.timonier.io \
	            --actions Type=forward,TargetGroupArn=$$HTTPS_TG_ARN \
	            --region $$AWS_REGION > /dev/null
	        echo -e "$${GREEN}✓ Updated routing rule for production (lookout-prod.timonier.io, lookout.timonier.io)$${NC}"
	    fi
	    echo ""
	    echo -e "$${GREEN}✅ Host-based routing configured!$${NC}"
	    echo "  All hostnames use the same target groups"
	    echo "  - lookout-stg.timonier.io → lookout-https-tg"
	    echo "  - lookout-prod.timonier.io, lookout.timonier.io → lookout-https-tg"
	else
	    echo ""
	    echo -e "$${YELLOW}ℹ Single environment mode$${NC}"
	    echo "  To enable multi-environment support with host-based routing:"
	    echo "  export ENABLE_PROD=true"
	    echo "  This will configure:"
	    echo "    - lookout-stg.timonier.io"
	    echo "    - lookout-prod.timonier.io, lookout.timonier.io"
	fi
	echo ""
	echo "🏥 Checking target health..."
	sleep 5
	HTTP_HEALTH=$$(aws elbv2 describe-target-health \
	    --target-group-arn $$HTTP_TG_ARN \
	    --region $$AWS_REGION \
	    --query 'TargetHealthDescriptions[0].TargetHealth.State' \
	    --output text)
	HTTPS_HEALTH=$$(aws elbv2 describe-target-health \
	    --target-group-arn $$HTTPS_TG_ARN \
	    --region $$AWS_REGION \
	    --query 'TargetHealthDescriptions[0].TargetHealth.State' \
	    --output text)
	echo "  HTTP Target Group: $$HTTP_HEALTH"
	echo "  HTTPS Target Group: $$HTTPS_HEALTH"
	if [ "$$HTTP_HEALTH" == "healthy" ] && [ "$$HTTPS_HEALTH" == "healthy" ]; then
	    echo -e "$${GREEN}✓ All targets healthy$${NC}"
	elif [ "$$HTTP_HEALTH" == "initial" ] || [ "$$HTTPS_HEALTH" == "initial" ]; then
	    echo -e "$${YELLOW}⚠ Targets in 'initial' state. This is normal and they should become healthy in ~30 seconds$${NC}"
	else
	    echo -e "$${YELLOW}⚠ Targets not yet healthy. Check security groups and Gateway configuration$${NC}"
	fi
	echo ""
	echo -e "$${GREEN}✅ ALB setup complete!$${NC}"
	echo ""
	echo "📋 Summary:"
	echo "  ALB DNS: $$ALB_DNS"
	if [ "$${ENABLE_PROD:-}" == "true" ]; then
	    echo "  Domains (multi-environment):"
	    echo "    - lookout-stg.timonier.io (staging)"
	    echo "    - lookout-prod.timonier.io (production)"
	    echo "    - lookout.timonier.io (production alias)"
	else
	    echo "  Domain: lookout-stg.timonier.io (staging only)"
	fi
	echo "  HTTP Listener: Port 80 → Redirect to HTTPS (301)"
	echo "  HTTPS Listener: Port 443 → Forward to HTTPS targets (Protocol=HTTPS, Port=32443)"
	echo "  Target Groups (shared across all environments):"
	echo "    - HTTP: lookout-http-tg (32080)"
	echo "    - HTTPS: lookout-https-tg (32443)"
	if [ "$${ENABLE_PROD:-}" == "true" ]; then
	    echo "  Host-Based Routing (same target groups):"
	    echo "    - lookout-stg.timonier.io → lookout-https-tg"
	    echo "    - lookout-prod.timonier.io, lookout.timonier.io → lookout-https-tg"
	fi
	echo ""
	echo "🔍 Next steps:"
	echo "  1. Wait 5-10 minutes for DNS propagation"
	if [ "$${ENABLE_PROD:-}" == "true" ]; then
	    echo "  2. Test staging: curl -I https://lookout-stg.timonier.io/health"
	    echo "  3. Test production: curl -I https://lookout-prod.timonier.io/health"
	    echo "  4. Test prod alias: curl -I https://lookout.timonier.io/health"
	else
	    echo "  2. Test: curl -I https://lookout-stg.timonier.io/health"
	fi
	echo "  3. Check target health in AWS Console"
	echo ""
	if [ "$${ENABLE_PROD:-}" == "true" ]; then
	    echo "🌐 Access your applications:"
	    echo "  Staging: https://lookout-stg.timonier.io"
	    echo "  Production: https://lookout-prod.timonier.io or https://lookout.timonier.io"
	else
	    echo "🌐 Access your application at: https://lookout-stg.timonier.io"
	fi

##@ Deployment

sync: ## Rsync project to EC2 instance (EC2_HOST required)
	set -e
	EC2_HOST="$${EC2_HOST:-}"
	EC2_DIR="$${EC2_DIR:-lookout}"
	if [[ -z "$$EC2_HOST" ]]; then
	    echo "❌ Error: EC2_HOST environment variable is not set"
	    echo ""
	    echo "Usage:"
	    echo "  EC2_HOST=ubuntu@<your-ec2-ip> make sync"
	    echo ""
	    echo "Or export it first:"
	    echo "  export EC2_HOST=ubuntu@<your-ec2-ip>"
	    echo "  make sync"
	    exit 1
	fi
	echo "🔄 Syncing code to $${EC2_HOST}:$${EC2_DIR}..."
	rsync -avz --delete \
	    --exclude '.git/' \
	    --exclude 'dgraph/' \
	    --exclude 'vendor/' \
	    --exclude '.env' \
	    --exclude 'nginx/certs/' \
	    --exclude 'nginx/auth/.htpasswd' \
	    ./ "$${EC2_HOST}:$${EC2_DIR}/"
	echo "✅ Code synced successfully!"
	echo ""
	echo "📋 Next steps on EC2 instance:"
	echo "   1. SSH: ssh $${EC2_HOST}"
	echo "   2. Setup ArgoCD: cd $${EC2_DIR} && make argocd"

deploy-staging: ## Deploy staging environment to EC2 kind cluster (EC2_HOST required)
	set -e
	ENVIRONMENT=staging
	EC2_HOST="$${EC2_HOST:-}"
	EC2_DIR="$${EC2_DIR:-lookout}"
	REGISTRY="$${REGISTRY:-localhost:5000}"
	IMAGE_NAME="$${IMAGE_NAME:-lookout}"
	if [[ -z "$$EC2_HOST" ]]; then
	    echo "❌ Error: EC2_HOST environment variable is not set"
	    echo ""
	    echo "Usage:"
	    echo "  EC2_HOST=ubuntu@<your-ec2-ip> make deploy-staging"
	    echo ""
	    echo "Or export it first:"
	    echo "  export EC2_HOST=ubuntu@<your-ec2-ip>"
	    echo "  make deploy-staging"
	    exit 1
	fi
	IMAGE_TAG="main"
	NAMESPACE_VAL="staging"
	HELM_RELEASE="lookout-staging"
	VALUES_FILE="values.staging.yaml"
	CURRENT_BRANCH=$$(git rev-parse --abbrev-ref HEAD)
	if [[ "$$CURRENT_BRANCH" != "main" ]]; then
	    echo "Warning: You're on branch '$$CURRENT_BRANCH', not 'main'"
	    read -p "Continue anyway? (y/N) " -n 1 -r
	    echo
	    if [[ ! $$REPLY =~ ^[Yy]$$ ]]; then
	        exit 1
	    fi
	fi
	echo "========================================="
	echo "Deploying to $$ENVIRONMENT"
	echo "Environment: $$ENVIRONMENT"
	echo "Image tag: $$IMAGE_TAG"
	echo "Namespace: $$NAMESPACE_VAL"
	echo "========================================="
	echo
	echo "📦 Step 1: Syncing code to EC2..."
	rsync -avz --delete \
	    --exclude '.git/' \
	    --exclude 'dgraph/' \
	    --exclude 'vendor/' \
	    --exclude 'node_modules/' \
	    --exclude '.env' \
	    ./ $${EC2_HOST}:$${EC2_DIR}/
	echo "✅ Code synced to EC2"
	echo
	echo "🔨 Step 2: Building Docker image on EC2..."
	ssh $${EC2_HOST} "cd $${EC2_DIR} && docker compose build"
	echo "✅ Docker image built"
	echo
	echo "🏷️  Step 3: Tagging and pushing to local registry..."
	ssh $${EC2_HOST} "cd $${EC2_DIR} && \
	    docker tag lookout-lookout:latest $${REGISTRY}/$${IMAGE_NAME}:$${IMAGE_TAG} && \
	    docker tag lookout-lookout:latest $${REGISTRY}/$${IMAGE_NAME}:latest && \
	    docker push $${REGISTRY}/$${IMAGE_NAME}:$${IMAGE_TAG} && \
	    docker push $${REGISTRY}/$${IMAGE_NAME}:latest"
	echo "✅ Image pushed to registry as $${REGISTRY}/$${IMAGE_NAME}:$${IMAGE_TAG}"
	echo
	echo "🚀 Step 4: Deploying to Kubernetes with Helm..."
	ssh $${EC2_HOST} "cd $${EC2_DIR} && \
	    helm upgrade --install $${HELM_RELEASE} ./helm/lookout \
	        -f ./helm/lookout/$${VALUES_FILE} \
	        --set lookout-app.image.tag=$${IMAGE_TAG} \
	        -n $${NAMESPACE_VAL} \
	        --create-namespace \
	        --wait \
	        --timeout 5m"
	echo "✅ Deployed to $${NAMESPACE_VAL} namespace"
	echo
	echo "🔍 Step 5: Verifying deployment..."
	ssh $${EC2_HOST} "kubectl get pods -n $${NAMESPACE_VAL} -l app=lookout"
	echo
	echo "========================================="
	echo "✅ Deployment complete!"
	echo "Environment: $$ENVIRONMENT"
	echo "Image: $${REGISTRY}/$${IMAGE_NAME}:$${IMAGE_TAG}"
	echo "Namespace: $${NAMESPACE_VAL}"
	echo "========================================="
	echo
	echo "To check status:"
	echo "  ssh $${EC2_HOST} 'kubectl get pods -n $${NAMESPACE_VAL}'"
	echo
	echo "To view logs:"
	echo "  ssh $${EC2_HOST} 'kubectl logs -n $${NAMESPACE_VAL} -l app=lookout -f'"
	echo
	echo "To get service info:"
	echo "  ssh $${EC2_HOST} 'kubectl get svc -n $${NAMESPACE_VAL}'"

deploy-production: ## Deploy production environment to EC2 kind cluster (EC2_HOST required)
	set -e
	ENVIRONMENT=production
	EC2_HOST="$${EC2_HOST:-}"
	EC2_DIR="$${EC2_DIR:-lookout}"
	REGISTRY="$${REGISTRY:-localhost:5000}"
	IMAGE_NAME="$${IMAGE_NAME:-lookout}"
	if [[ -z "$$EC2_HOST" ]]; then
	    echo "❌ Error: EC2_HOST environment variable is not set"
	    echo ""
	    echo "Usage:"
	    echo "  EC2_HOST=ubuntu@<your-ec2-ip> make deploy-production"
	    echo ""
	    echo "Or export it first:"
	    echo "  export EC2_HOST=ubuntu@<your-ec2-ip>"
	    echo "  make deploy-production"
	    exit 1
	fi
	IMAGE_TAG=$$(git describe --tags --exact-match 2>/dev/null || echo "")
	if [[ -z "$$IMAGE_TAG" ]]; then
	    echo "Error: No git tag found on current commit"
	    echo "Production deployments require a git tag"
	    echo "Create a tag with: git tag -a v1.0.0 -m 'Version 1.0.0'"
	    exit 1
	fi
	NAMESPACE_VAL="production"
	HELM_RELEASE="lookout-production"
	VALUES_FILE="values.production.yaml"
	echo "========================================="
	echo "Deploying to $$ENVIRONMENT"
	echo "Environment: $$ENVIRONMENT"
	echo "Image tag: $$IMAGE_TAG"
	echo "Namespace: $$NAMESPACE_VAL"
	echo "========================================="
	echo
	echo "📦 Step 1: Syncing code to EC2..."
	rsync -avz --delete \
	    --exclude '.git/' \
	    --exclude 'dgraph/' \
	    --exclude 'vendor/' \
	    --exclude 'node_modules/' \
	    --exclude '.env' \
	    ./ $${EC2_HOST}:$${EC2_DIR}/
	echo "✅ Code synced to EC2"
	echo
	echo "🔨 Step 2: Building Docker image on EC2..."
	ssh $${EC2_HOST} "cd $${EC2_DIR} && docker compose build"
	echo "✅ Docker image built"
	echo
	echo "🏷️  Step 3: Tagging and pushing to local registry..."
	ssh $${EC2_HOST} "cd $${EC2_DIR} && \
	    docker tag lookout-lookout:latest $${REGISTRY}/$${IMAGE_NAME}:$${IMAGE_TAG} && \
	    docker tag lookout-lookout:latest $${REGISTRY}/$${IMAGE_NAME}:latest && \
	    docker push $${REGISTRY}/$${IMAGE_NAME}:$${IMAGE_TAG} && \
	    docker push $${REGISTRY}/$${IMAGE_NAME}:latest"
	echo "✅ Image pushed to registry as $${REGISTRY}/$${IMAGE_NAME}:$${IMAGE_TAG}"
	echo
	echo "🚀 Step 4: Deploying to Kubernetes with Helm..."
	ssh $${EC2_HOST} "cd $${EC2_DIR} && \
	    helm upgrade --install $${HELM_RELEASE} ./helm/lookout \
	        -f ./helm/lookout/$${VALUES_FILE} \
	        --set lookout-app.image.tag=$${IMAGE_TAG} \
	        -n $${NAMESPACE_VAL} \
	        --create-namespace \
	        --wait \
	        --timeout 5m"
	echo "✅ Deployed to $${NAMESPACE_VAL} namespace"
	echo
	echo "🔍 Step 5: Verifying deployment..."
	ssh $${EC2_HOST} "kubectl get pods -n $${NAMESPACE_VAL} -l app=lookout"
	echo
	echo "========================================="
	echo "✅ Deployment complete!"
	echo "Environment: $$ENVIRONMENT"
	echo "Image: $${REGISTRY}/$${IMAGE_NAME}:$${IMAGE_TAG}"
	echo "Namespace: $${NAMESPACE_VAL}"
	echo "========================================="
	echo
	echo "To check status:"
	echo "  ssh $${EC2_HOST} 'kubectl get pods -n $${NAMESPACE_VAL}'"
	echo
	echo "To view logs:"
	echo "  ssh $${EC2_HOST} 'kubectl logs -n $${NAMESPACE_VAL} -l app=lookout -f'"
	echo
	echo "To get service info:"
	echo "  ssh $${EC2_HOST} 'kubectl get svc -n $${NAMESPACE_VAL}'"

##@ Full Cluster Setup

cluster-setup: kind-registry gateway kind-nodeports argocd argocd-github argocd-image-updater external-secrets basic-auth ghcr-secret health-route ## Run full cluster bootstrap in order
