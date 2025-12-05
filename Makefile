# renovate: datasource=github-releases depName=mvdan/gofumpt
GOFUMPT_PACKAGE_VERSION := v0.9.2
# renovate: datasource=github-releases depName=golangci/golangci-lint
GOLANGCI_LINT_PACKAGE_VERSION := v2.7.1
# renovate: datasource=github-releases depName=cert-manager/cert-manager
CERT_MANAGER_VERSION := v1.19.1

GOFUMPT_PACKAGE ?= mvdan.cc/gofumpt@$(GOFUMPT_PACKAGE_VERSION)
GOTESTSUM_PACKAGE ?= gotest.tools/gotestsum@latest

# Image URL to use all building image targets
IMG ?= docker.io/thegeeklab/renovate-operator:devel

GO ?= go
SOURCES ?= $(shell find . -name "*.go" -type f)

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

# Check if kind is installed
define check-kind-installed
@command -v kind >/dev/null 2>&1 || { \
	echo "Kind is not installed. Please install Kind manually."; \
	exit 1; \
}
endef

# Check if kind cluster is running
define check-kind-cluster-running
@kind get clusters | grep -q $(KIND_CLUSTER) || { \
	echo "No Kind cluster is running. Please start a Kind cluster before running this command."; \
	exit 1; \
}
endef

.PHONY: all
all: build

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: deps
deps:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(shell go env GOPATH)/bin $(GOLANGCI_LINT_PACKAGE_VERSION)
	$(GO) mod download
	$(GO) install $(GOFUMPT_PACKAGE)
	$(GO) install $(GOTESTSUM_PACKAGE)

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object paths="./..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	$(shell go env GOPATH)/bin/gofumpt -extra -w $(SOURCES)

.PHONY: vet
vet: ## Run go vet against code.
	$(GO) vet ./...

.PHONY: test
test: manifests generate fmt vet setup-envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" $(shell go env GOPATH)/bin/gotestsum $$($(GO) list ./... | grep -v /e2e) -coverprofile cover.out

.PHONY: kind-create
kind-create: ## Create a Kind cluster from hack/kind.yaml and optionally deploy cert-manager.
	$(call check-kind-installed)
	@kind get clusters | grep -q renovate-operator || { \
		echo "Creating Kind cluster..."; \
		kind create cluster --config hack/kind.yaml --name $(KIND_CLUSTER) || { \
			echo "Failed to create Kind cluster."; \
			exit 1; \
		}; \
	}
	@if [ "$(CERT_MANAGER_INSTALL_SKIP)" != "true" ]; then \
		echo "Installing cert-manager..."; \
		$(KUBECTL) apply -f https://github.com/cert-manager/cert-manager/releases/download/$(CERT_MANAGER_VERSION)/cert-manager.yaml || { \
			echo "Failed to install cert-manager."; \
			exit 1; \
		}; \
		$(KUBECTL) wait --for=condition=Available --timeout=300s deployment/cert-manager -n cert-manager || { \
			echo "cert-manager deployment not available after 300 seconds."; \
			exit 1; \
		}; \
	else \
		echo "Skipping cert-manager installation as requested."; \
	fi

.PHONY: kind-load
kind-load: ## Load the manager image into the Kind cluster.
	$(call check-kind-installed)
	$(call check-kind-cluster-running)
	kind load docker-image ${IMG} --name $(KIND_CLUSTER)

.PHONY: kind-delete
kind-delete: ## Delete the Kind cluster.
	$(call check-kind-installed)
	@kind get clusters | grep -q $(KIND_CLUSTER) && { \
		echo "Deleting Kind cluster..."; \
		kind delete cluster --name $(KIND_CLUSTER) || { \
			echo "Failed to delete Kind cluster."; \
			exit 1; \
		}; \
	} || echo "No Kind cluster named $(KIND_CLUSTER) exists."

# The default setup assumes Kind is pre-installed and builds/loads the Manager Docker image locally.
# Prometheus and CertManager are installed by default; skip with:
# - PROMETHEUS_INSTALL_SKIP=true
# - CERT_MANAGER_INSTALL_SKIP=true
.PHONY: test-e2e
test-e2e: manifests generate fmt vet ## Run the e2e tests. Expected an isolated environment using Kind.
	$(call check-kind-installed)
	$(call check-kind-cluster-running)
	$(GO) test ./test/e2e/ -v -ginkgo.v

.PHONY: golangci-lint
golangci-lint:
	$(shell go env GOPATH)/bin/golangci-lint run

.PHONY: lint
lint: golangci-lint

##@ Build

.PHONY: build
build: manifests generate fmt vet ## Build binaries.
	$(GO) build -o bin/manager cmd/renovate-operator/main.go
	$(GO) build -o bin/discovery cmd/discovery/main.go
	$(GO) build -o bin/dispatcher cmd/dispatcher/main.go

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	$(GO) run ./cmd/renovate-operator/main.go

.PHONY: docker-build
docker-build: ## Build container image.
	docker build --tag ${IMG} -f Containerfile.multiarch .

PLATFORMS ?= linux/amd64,linux/arm64
.PHONY: docker-buildx
docker-buildx: ## Build container image for cross-platform support.
	docker buildx build --platform=$(PLATFORMS) --tag ${IMG} -f Containerfile.multiarch .

.PHONY: build-installer
build-installer: manifests generate kustomize ## Generate a consolidated YAML with CRDs and deployment.
	mkdir -p dist
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default > dist/install.yaml

##@ Deployment

IGNORE_NOT_FOUND ?= false

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with IGNORE_NOT_FOUND=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) delete --ignore-not-found=${IGNORE_NOT_FOUND} -f -

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | $(KUBECTL) apply -f -

.PHONY: undeploy
undeploy: kustomize ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with IGNORE_NOT_FOUND=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | $(KUBECTL) delete --ignore-not-found=$(IGNORE_NOT_FOUND) -f -

##@ Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUBECTL ?= kubectl
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint

## Tool Versions
KUSTOMIZE_VERSION ?= v5.5.0
CONTROLLER_TOOLS_VERSION ?= v0.17.1
#ENVTEST_VERSION is the version of controller-runtime release branch to fetch the envtest setup script (i.e. release-0.20)
ENVTEST_VERSION ?= $(shell go list -m -f "{{ .Version }}" sigs.k8s.io/controller-runtime | awk -F'[v.]' '{printf "release-%d.%d", $$2, $$3}')
#ENVTEST_K8S_VERSION is the version of Kubernetes to use for setting up ENVTEST binaries (i.e. 1.31)
ENVTEST_K8S_VERSION ?= $(shell go list -m -f "{{ .Version }}" k8s.io/api | awk -F'[v.]' '{printf "1.%d", $$3}')

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

.PHONY: setup-envtest
setup-envtest: envtest ## Download the binaries required for ENVTEST in the local bin directory.
	@echo "Setting up envtest binaries for Kubernetes version $(ENVTEST_K8S_VERSION)..."
	@$(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path || { \
		echo "Error: Failed to set up envtest binaries for version $(ENVTEST_K8S_VERSION)."; \
		exit 1; \
	}

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): $(LOCALBIN)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/v2/cmd/golangci-lint,$(GOLANGCI_LINT_PACKAGE_VERSION))

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f "$(1)-$(3)" ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
rm -f $(1) || true ;\
GOBIN=$(LOCALBIN) $(GO) install $${package} ;\
mv $(1) $(1)-$(3) ;\
} ;\
ln -sf $(1)-$(3) $(1)
endef
