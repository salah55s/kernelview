# KernelView — Build System
# eBPF-powered Kubernetes Autopilot

SHELL := /bin/bash
.DEFAULT_GOAL := help

# Go parameters
GOBIN := $(shell go env GOPATH)/bin
GOOS := linux
GOARCH := amd64

# Docker
REGISTRY ?= ghcr.io/kernelview
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
DOCKER_TAG ?= $(VERSION)

# BPF
CLANG ?= clang
CLANG_FLAGS := -O2 -g -target bpf -D__TARGET_ARCH_x86
BPF_SOURCES := $(wildcard bpf/*.c)
BPF_OBJECTS := $(BPF_SOURCES:.c=.o)

# Binaries
BINARIES := agent collector correlator operator

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ============================================================
# Build
# ============================================================

.PHONY: build
build: build-agent build-collector build-correlator build-operator ## Build all Go binaries

.PHONY: build-agent
build-agent: ## Build the eBPF agent binary
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags="-s -w -X main.version=$(VERSION)" -o bin/kernelview-agent ./cmd/agent/

.PHONY: build-collector
build-collector: ## Build the collector binary
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags="-s -w -X main.version=$(VERSION)" -o bin/kernelview-collector ./cmd/collector/

.PHONY: build-correlator
build-correlator: ## Build the AI correlator binary
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags="-s -w -X main.version=$(VERSION)" -o bin/kernelview-correlator ./cmd/correlator/

.PHONY: build-operator
build-operator: ## Build the remediation operator binary
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags="-s -w -X main.version=$(VERSION)" -o bin/kernelview-operator ./cmd/operator/

# ============================================================
# eBPF Programs
# ============================================================

.PHONY: build-bpf
build-bpf: $(BPF_OBJECTS) ## Compile all eBPF C programs to .o

bpf/%.o: bpf/%.c bpf/headers/*.h
	$(CLANG) $(CLANG_FLAGS) -I bpf/headers -c $< -o $@

.PHONY: generate-bpf
generate-bpf: build-bpf ## Generate Go bindings for eBPF objects using bpf2go
	go generate ./internal/agent/bpf/...

# ============================================================
# Code Generation
# ============================================================

.PHONY: generate-proto
generate-proto: ## Generate Go code from protobuf definitions
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/*.proto

.PHONY: generate-crd
generate-crd: ## Generate CRD manifests from Go types
	controller-gen crd paths="./api/..." output:crd:dir=deploy/helm/kernelview/templates

.PHONY: generate
generate: generate-proto generate-bpf generate-crd ## Run all code generation

# ============================================================
# Docker
# ============================================================

.PHONY: docker-build
docker-build: ## Build all Docker images
	docker build -t $(REGISTRY)/kernelview-agent:$(DOCKER_TAG) -f Dockerfile.agent .
	docker build -t $(REGISTRY)/kernelview-collector:$(DOCKER_TAG) -f Dockerfile.collector .
	docker build -t $(REGISTRY)/kernelview-correlator:$(DOCKER_TAG) -f Dockerfile.correlator .
	docker build -t $(REGISTRY)/kernelview-operator:$(DOCKER_TAG) -f Dockerfile.operator .

.PHONY: docker-push
docker-push: ## Push all Docker images
	docker push $(REGISTRY)/kernelview-agent:$(DOCKER_TAG)
	docker push $(REGISTRY)/kernelview-collector:$(DOCKER_TAG)
	docker push $(REGISTRY)/kernelview-correlator:$(DOCKER_TAG)
	docker push $(REGISTRY)/kernelview-operator:$(DOCKER_TAG)

# ============================================================
# Testing
# ============================================================

.PHONY: test
test: ## Run all Go tests
	go test -race -coverprofile=coverage.out ./...

.PHONY: test-bpf
test-bpf: ## Run eBPF-specific tests (requires root & kernel 5.10+)
	sudo go test -v ./internal/agent/bpf/... -tags=bpf

.PHONY: test-integration
test-integration: ## Run integration tests (requires kind cluster)
	go test -v -timeout 10m ./test/integration/...

.PHONY: test-chaos
test-chaos: ## Run chaos tests
	go test -v -timeout 20m ./test/chaos/...

.PHONY: coverage
coverage: test ## Show test coverage
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# ============================================================
# Lint & Format
# ============================================================

.PHONY: lint
lint: ## Run linters
	golangci-lint run ./...

.PHONY: fmt
fmt: ## Format all Go files
	gofmt -s -w .

.PHONY: vet
vet: ## Run go vet
	go vet ./...

# ============================================================
# Helm
# ============================================================

.PHONY: helm-lint
helm-lint: ## Lint the Helm chart
	helm lint deploy/helm/kernelview/

.PHONY: helm-install
helm-install: ## Install KernelView to the current cluster
	helm install kernelview deploy/helm/kernelview/ \
		--namespace kernelview \
		--create-namespace

.PHONY: helm-upgrade
helm-upgrade: ## Upgrade KernelView on the current cluster
	helm upgrade kernelview deploy/helm/kernelview/ \
		--namespace kernelview

.PHONY: helm-uninstall
helm-uninstall: ## Uninstall KernelView from the cluster
	helm uninstall kernelview --namespace kernelview

# ============================================================
# Development
# ============================================================

.PHONY: dev-deploy
dev-deploy: docker-build ## Deploy to local kind cluster
	kind load docker-image $(REGISTRY)/kernelview-agent:$(DOCKER_TAG)
	kind load docker-image $(REGISTRY)/kernelview-collector:$(DOCKER_TAG)
	kind load docker-image $(REGISTRY)/kernelview-correlator:$(DOCKER_TAG)
	kind load docker-image $(REGISTRY)/kernelview-operator:$(DOCKER_TAG)
	$(MAKE) helm-install

.PHONY: dev-kind
dev-kind: ## Create a 3-node kind cluster for development
	kind create cluster --config test/kind-config.yaml --name kernelview-dev

.PHONY: dev-kind-delete
dev-kind-delete: ## Delete the development kind cluster
	kind delete cluster --name kernelview-dev

# ============================================================
# Clean
# ============================================================

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf bin/
	rm -f bpf/*.o
	rm -f coverage.out coverage.html
