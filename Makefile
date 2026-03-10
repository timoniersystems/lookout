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
	./scripts/setup-registry.sh

kind-nodeports: ## Configure fixed NodePorts 32080/32443 on EnvoyProxy
	HTTP_NODEPORT=32080 HTTPS_NODEPORT=32443 ./scripts/setup-fixed-nodeports.sh

kind-forward: ## Set up socat port-forwarding from EC2 host to kind NodePorts
	./scripts/setup-kind-nodeport-forwarding.sh

##@ Gateway

gateway: ## Install GatewayClass and TLS cert secret (NAMESPACE, DOMAIN, CERT_DAYS)
	NAMESPACE=$(NAMESPACE) ./scripts/setup-gateway.sh

health-route: ## Create /health HTTPRoute for ALB health checks
	./scripts/setup-health-httproute.sh

##@ ArgoCD

argocd: ## Install ArgoCD on kind cluster
	./scripts/setup-argocd.sh

argocd-github: ## Configure ArgoCD with GitHub repo credentials (GITHUB_USERNAME, GITHUB_TOKEN)
	./scripts/setup-argocd-github-repo.sh

argocd-image-updater: ## Install ArgoCD Image Updater
	./scripts/setup-argocd-image-updater.sh

##@ Secrets & Auth

external-secrets: ## Install External Secrets Operator and sync NVD API key from AWS
	./scripts/setup-external-secrets.sh

basic-auth: ## Store htpasswd in AWS Secrets Manager and sync to k8s (NAMESPACE, BASIC_AUTH_PASSWORD)
	NAMESPACE=$(NAMESPACE) ./scripts/setup-basic-auth.sh

ghcr-secret: ## Create ghcr.io image pull secret in k8s (GITHUB_USERNAME, GITHUB_TOKEN, NAMESPACE)
	./scripts/create-ghcr-secret.sh $(NAMESPACE)

##@ AWS / ALB

alb: ## Create AWS ALB with HTTPS and target groups (requires VPC_ID, EC2_INSTANCE_ID, SUBNET_*, CERTIFICATE_ARN, HOSTED_ZONE_ID)
	./scripts/setup-alb.sh

##@ Deployment

sync: ## Rsync project to EC2 instance (EC2_HOST required)
	./scripts/sync-to-ec2.sh

deploy-staging: ## Deploy staging environment to EC2 kind cluster (EC2_HOST required)
	./scripts/deploy.sh staging

deploy-production: ## Deploy production environment to EC2 kind cluster (EC2_HOST required)
	./scripts/deploy.sh production

##@ Full Cluster Setup

cluster-setup: kind-registry gateway kind-nodeports argocd argocd-github argocd-image-updater external-secrets basic-auth ghcr-secret health-route ## Run full cluster bootstrap in order
