# renovate: datasource=github-releases depName=mvdan/gofumpt
GOFUMPT_PACKAGE_VERSION := v0.8.0
# renovate: datasource=github-releases depName=golangci/golangci-lint
GOLANGCI_LINT_PACKAGE_VERSION := v2.1.6

GOFUMPT_PACKAGE ?= mvdan.cc/gofumpt@$(GOFUMPT_PACKAGE_VERSION)
GOLANGCI_LINT_PACKAGE ?= github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_PACKAGE_VERSION)

# Image URL to use all building image targets
IMG ?= docker.io/thegeeklab/renovate-operator:devel
# To use a custom image, run: make deploy IMG=myregistry/myimage:tag
# or export IMG=myregistry/myimage:tag before running make commands

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

.PHONY: all
all: build

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: deps
deps:
	$(GO) mod download
	$(GO) install $(GOFUMPT_PACKAGE)
	$(GO) install $(GOLANGCI_LINT_PACKAGE)

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object paths="./..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	$(GO) run $(GOFUMPT_PACKAGE) -extra -w $(SOURCES)

.PHONY: vet
vet: ## Run go vet against code.
	$(GO) vet ./...

.PHONY: test
test: manifests generate fmt vet setup-envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" $(GO) test $$($(GO) list ./... | grep -v /e2e) -coverprofile cover.out

# The default setup assumes Kind is pre-installed and builds/loads the Manager Docker image locally.
# Prometheus and CertManager are installed by default; skip with:
# - PROMETHEUS_INSTALL_SKIP=true
# - CERT_MANAGER_INSTALL_SKIP=true
.PHONY: test-e2e
test-e2e: manifests generate fmt vet ## Run the e2e tests. Expected an isolated environment using Kind.
	@command -v kind >/dev/null 2>&1 || { \
		echo "Kind is not installed. Please install Kind manually."; \
		exit 1; \
	}
	@kind get clusters | grep -q $(KIND_CLUSTER) || { \
		echo "No Kind cluster is running. Please start a Kind cluster before running the e2e tests."; \
		exit 1; \
	}
	$(GO) test ./test/e2e/ -v -ginkgo.v

.PHONY: golangci-lint
golangci-lint:
	$(GO) run $(GOLANGCI_LINT_PACKAGE) run

.PHONY: lint
lint: golangci-lint

##@ Build

.PHONY: build
build: manifests generate fmt vet ## Build binaries.
	$(GO) build -o bin/manager cmd/main.go
	$(GO) build -o bin/discovery discovery/cmd/main.go
	$(GO) build -o bin/dispatcher dispatcher/cmd/main.go
	$(GO) build -o bin/job-scheduler jobscheduler/cmd/main.go

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	$(GO) run ./cmd/main.go

.PHONY: docker-build
docker-build: ## Build container image.
	docker build --tag ${IMG} -f Containerfile.multiarch .

PLATFORMS ?= linux/amd64,linux/arm64
.PHONY: docker-buildx
docker-buildx: ## Build container image for cross-platform support.
	docker buildx build --platform=$(PLATFORMS) --tag ${IMG} -f Containerfile.multiarch .

.PHONY: docker-push
docker-push: docker-build ## Build and push container image.
	docker push ${IMG}

.PHONY: docker-buildx-push
docker-buildx-push: ## Build and push container image for cross-platform support.
	docker buildx build --push --platform=$(PLATFORMS) --tag ${IMG} -f Containerfile.multiarch .

.PHONY: build-installer
build-installer: manifests generate kustomize ## Generate a consolidated YAML with CRDs and deployment.
	mkdir -p dist
	@cp config/manager/kustomization.yaml config/manager/kustomization.yaml.bak
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default > dist/install.yaml
	@mv config/manager/kustomization.yaml.bak config/manager/kustomization.yaml
	@echo "Generated installer at dist/install.yaml with image: ${IMG}"

##@ Deployment

IGNORE_NOT_FOUND ?= false

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
    # K8s adds last-applied-configuration annotation to the CRDs.
	# which causes is invalid: metadata.annotations: Too long: may not be more than 262144 bytes
	# so we need to apply server-side
	$(KUSTOMIZE) build config/crd | $(KUBECTL) apply --server-side -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with IGNORE_NOT_FOUND=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) delete --ignore-not-found=${IGNORE_NOT_FOUND} -f -

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	@cp config/manager/kustomization.yaml config/manager/kustomization.yaml.bak
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | $(KUBECTL) apply -f -
	@mv config/manager/kustomization.yaml.bak config/manager/kustomization.yaml
	@echo "Deployed with image: ${IMG}"
# kubectl patch deployment renovate-operator-controller-manager -n renovate-operator-system --patch "{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"manager\",\"image\":\"${IMG}\"}],\"imagePullSecrets\":[{\"name\":\"regcred\"}]}}}}"

.PHONY: verify-image
verify-image: manifests kustomize ## Verify what image would be deployed without actually deploying.
	@echo "Current IMG setting: ${IMG}"
	@echo "---"
	@echo "Current kustomization.yaml image settings:"
	@grep -A2 "name: controller" config/manager/kustomization.yaml || true
	@echo "---"
	@echo "Simulating deploy with IMG=${IMG}:"
	@cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	@echo "Updated kustomization.yaml:"
	@grep -A2 "name: controller" config/manager/kustomization.yaml || true
	@echo "---"
	@echo "Image in generated manifest:"
	@$(KUSTOMIZE) build config/default | grep "image:" | grep -v "imagePullPolicy" | head -1 || echo "Could not find image in manifest"
	@git checkout config/manager/kustomization.yaml 2>/dev/null || true

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
