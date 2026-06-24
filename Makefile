# renovate: datasource=github-releases depName=mvdan/gofumpt
GOFUMPT_PACKAGE_VERSION := v0.10.0
# renovate: datasource=github-releases depName=google/yamlfmt
YAMLFMT_PACKAGE_VERSION := v0.21.0
# renovate: datasource=github-releases depName=golangci/golangci-lint
GOLANGCI_LINT_PACKAGE_VERSION := v2.12.2
# renovate: datasource=go depName=github.com/a-h/templ
TEMPL_PACKAGE_VERSION := v0.3.1020
# renovate: datasource=github-releases depName=air-verse/air
AIR_PACKAGE_VERSION := v1.65.3
# renovate: datasource=github-releases depName=norwoodj/helm-docs
HELM_DOCS_PACKAGE_VERSION := v1.14.2
# renovate: datasource=docker depName=ghcr.io/helm-unittest/helm-unittest/unittest
HELM_UNITTEST_VERSION := 1.1.1

MOCKERY_PACKAGE ?= github.com/vektra/mockery/v3@latest

# Image URL to use all building image targets
IMG ?= docker.io/thegeeklab/renovate-operator:latest

# Toggle for Vite dev server during 'make run'
FRONTEND_DEV ?= false

# Kind Cluster Name
KIND_CLUSTER ?= renovate-operator

GO ?= go
SOURCES ?= $(shell find . -name "*.go" -type f ! -path "*/mocks/*" ! -name "*_templ.go")

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

TEST_FORMAT ?= pkgname
GOTEST_FLAGS = --format=$(TEST_FORMAT) -- -count=1 -cover -coverprofile=cover.out
GINKGO_FLAGS = -ginkgo.focus="$(FOCUS)" -ginkgo.skip="$(SKIP)" -ginkgo.v

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

.PHONY: frontend-deps
frontend-deps: ## Install frontend dependencies.
	@echo "Installing frontend dependencies..."
	npm install

.PHONY: eslint
eslint: ## Run eslint.
	npm run lint

.PHONY: frontend-build
frontend-build: frontend-deps ## Build the frontend assets for production.
	@echo "Building Vite assets for production..."
	npm run build

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases
	sed -i -e 's/validating-webhook-configuration/webhook-configuration/g' config/webhook/manifests.yaml
	sed -i -e 's/mutating-webhook-configuration/webhook-configuration/g' config/webhook/manifests.yaml
	sed -i -e 's/webhook-service/renovate-operator-webhook-service/g' config/webhook/manifests.yaml
	@$(MAKE) --no-print-directory yamlfmt

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object paths="./..."
	$(GO) run $(MOCKERY_PACKAGE)
	@$(MAKE) --no-print-directory templ
	@$(MAKE) --no-print-directory yamlfmt

.PHONY: templ
templ: templ-bin ## Generate templ components.
	$(TEMPL_BIN) generate --include-version=false --include-timestamp=false --path=./internal/frontend/view

.PHONY: templ-fmt
templ-fmt: templ-bin ## Format templ files.
	$(TEMPL_BIN) fmt ./internal/frontend/view

.PHONY: vet
vet: ## Run go vet against code.
	$(GO) vet ./...

.PHONY: fmt
fmt: gofumpt ## Run go fmt against code.
	$(GOFUMPT_BIN) -extra -w $(SOURCES)
	@$(MAKE) --no-print-directory templ-fmt

.PHONY: yamlfmt
yamlfmt: yamlfmt-bin ## Run yamlfmt.
	$(YAMLFMT_BIN) .

.PHONY: yamlfmt-dry
yamlfmt-dry: yamlfmt-bin
	$(YAMLFMT_BIN) -dry .

.PHONY: yamlfmt-lint
yamlfmt-lint: yamlfmt-bin
	$(YAMLFMT_BIN) -lint .

.PHONY: test
test: setup-envtest gotestsum ## Run tests without setup.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" $(GOTESTSUM_BIN) $(GOTEST_FLAGS) $$($(GO) list ./... | grep -v /e2e) $(GINKGO_FLAGS)

.PHONY: test-ci
test-ci: manifests generate fmt vet test ## Run tests with full setup.

.PHONY: kind-create
kind-create: ## Create a Kind cluster.
	$(call check-kind-installed)
	@kind get clusters | grep -q $(KIND_CLUSTER) || { \
		echo "Creating Kind cluster..."; \
		kind create cluster --config hack/kind.yaml --name $(KIND_CLUSTER) || exit 1; \
	}

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
		kind delete cluster --name $(KIND_CLUSTER) || exit 1; \
	} || echo "No Kind cluster named $(KIND_CLUSTER) exists."

.PHONY: test-e2e
test-e2e: manifests generate fmt vet ## Run the e2e tests. Expected an isolated environment using Kind.
	$(call check-kind-installed)
	$(call check-kind-cluster-running)
	$(GO) test ./test/e2e/ -v -ginkgo.v

.PHONY: golangci-lint
golangci-lint: golangci-lint-bin ## Run golangci-lint.
	$(GOLANGCI_LINT) run

.PHONY: lint
lint: yamlfmt-dry golangci-lint eslint

.PHONY: helm-docs
helm-docs: helm-docs-bin ## Generate helm documentation.
	$(HELM_DOCS_BIN) -c dist/chart/
	npx prettier --write "dist/chart/**/*.md"

.PHONY: helm-test
helm-test: ## Run helm unit tests.
	@helm plugin install https://github.com/helm-unittest/helm-unittest.git --version=$(HELM_UNITTEST_VERSION) --verify=false >/dev/null 2>&1 || true
	helm unittest --strict -f 'tests/**/*.yaml' dist/chart/

##@ Build

.PHONY: build-go
build-go: ## Build the Go binaries.
	$(GO) build -o bin/manager cmd/main.go
	$(GO) build -o bin/discovery cmd/discovery/main.go

.PHONY: build
build: manifests generate fmt vet frontend-build build-go ## Build binaries and frontend assets.

.PHONY: run
run: manifests generate fmt vet air-bin ## Run a controller from your host.
	@$(KUBECTL) set env deployments -n renovate-system renovate-operator-controller-manager ENABLE_CONTROLLERS=false
ifeq ($(FRONTEND_DEV),true)
	@npm install
	@npm run dev & VITE_PID=$$!; \
	trap "kill $$VITE_PID 2>/dev/null" EXIT INT TERM; \
	NODE_ENV=development mirrord exec -f mirrord.json -- $(AIR_BIN) -c .air.toml
else
	mirrord exec -f mirrord.json -- $(AIR_BIN) -c .air.toml
endif

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
	@$(MAKE) --no-print-directory yamlfmt

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
	$(KUSTOMIZE) build --enable-helm config/dev | $(KUBECTL) apply -f -
	@$(MAKE) --no-print-directory yamlfmt

.PHONY: undeploy
undeploy: kustomize ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with IGNORE_NOT_FOUND=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build --enable-helm config/dev | $(KUBECTL) delete --ignore-not-found=$(IGNORE_NOT_FOUND) -f -

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
GOLANGCI_LINT ?= $(LOCALBIN)/golangci-lint
GOFUMPT_BIN ?= $(LOCALBIN)/gofumpt
YAMLFMT_BIN ?= $(LOCALBIN)/yamlfmt
GOTESTSUM_BIN ?= $(LOCALBIN)/gotestsum
TEMPL_BIN ?= $(LOCALBIN)/templ
AIR_BIN ?= $(LOCALBIN)/air
HELM_DOCS_BIN ?= $(LOCALBIN)/helm-docs

## Tool Versions
KUSTOMIZE_VERSION ?= v5.8.1
CONTROLLER_TOOLS_VERSION ?= v0.17.1
# ENVTEST_VERSION is the version of controller-runtime release branch to fetch the envtest setup script (i.e. release-0.20)
ENVTEST_VERSION ?= $(shell go list -m -f "{{ .Version }}" sigs.k8s.io/controller-runtime | awk -F'[v.]' '{printf "release-%d.%d", $$2, $$3}')
# ENVTEST_K8S_VERSION is the version of Kubernetes to use for setting up ENVTEST binaries (i.e. 1.31)
ENVTEST_K8S_VERSION ?= $(shell go list -m -f "{{ .Version }}" k8s.io/api | awk -F'[v.]' '{printf "1.%d", $$3}')

# go-install-tool will 'go install' any package if the target binary is missing or not executable
# $1 - target path with name of binary (e.g., $(LOCALBIN)/yamlfmt)
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

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): $(LOCALBIN)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

.PHONY: setup-envtest
setup-envtest: envtest ## Download the binaries required for ENVTEST in the local bin directory.
	@echo "Setting up envtest binaries for Kubernetes version $(ENVTEST_K8S_VERSION)..."
	@$(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path || { \
		echo "Error: Failed to set up envtest binaries for version $(ENVTEST_K8S_VERSION)."; \
		exit 1; \
	}

.PHONY: golangci-lint-bin
golangci-lint-bin: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/v2/cmd/golangci-lint,$(GOLANGCI_LINT_PACKAGE_VERSION))

.PHONY: gofumpt
gofumpt: $(GOFUMPT_BIN) ## Download gofumpt locally if necessary.
$(GOFUMPT_BIN): $(LOCALBIN)
	$(call go-install-tool,$(GOFUMPT_BIN),mvdan.cc/gofumpt,$(GOFUMPT_PACKAGE_VERSION))

.PHONY: yamlfmt-bin
yamlfmt-bin: $(YAMLFMT_BIN) ## Download yamlfmt locally if necessary.
$(YAMLFMT_BIN): $(LOCALBIN)
	$(call go-install-tool,$(YAMLFMT_BIN),github.com/google/yamlfmt/cmd/yamlfmt,$(YAMLFMT_PACKAGE_VERSION))

.PHONY: gotestsum
gotestsum: $(GOTESTSUM_BIN) ## Download gotestsum locally if necessary.
$(GOTESTSUM_BIN): $(LOCALBIN)
	$(call go-install-tool,$(GOTESTSUM_BIN),gotest.tools/gotestsum,latest)

.PHONY: templ-bin
templ-bin: $(TEMPL_BIN) ## Download templ locally if necessary.
$(TEMPL_BIN): $(LOCALBIN)
	$(call go-install-tool,$(TEMPL_BIN),github.com/a-h/templ/cmd/templ,$(TEMPL_PACKAGE_VERSION))

.PHONY: air-bin
air-bin: $(AIR_BIN) ## Download air locally if necessary.
$(AIR_BIN): $(LOCALBIN)
	$(call go-install-tool,$(AIR_BIN),github.com/air-verse/air,$(AIR_PACKAGE_VERSION))

.PHONY: helm-docs-bin
helm-docs-bin: $(HELM_DOCS_BIN) ## Download helm-docs locally if necessary.
$(HELM_DOCS_BIN): $(LOCALBIN)
	$(call go-install-tool,$(HELM_DOCS_BIN),github.com/norwoodj/helm-docs/cmd/helm-docs,$(HELM_DOCS_PACKAGE_VERSION))
