# AGENTS.md - Development Guidelines

<!-- cSpell:ignore mirrord envtest golangci deepcopy gofumpt Errorf logr keyup afterend subpackages noctx Gomega gotestsum Codegen cspell -->

This file contains comprehensive guidelines for AI coding agents working in the renovate-operator repository.

## AI Agent Tool Usage (Mandatory)

**AI agents MUST use `make` targets for all build, test, lint, generate, and dependency operations.** Never invoke underlying tools directly (e.g. `go build`, `go test`, `go vet`, `go mod`, `npm install`, `npm run ...`, `npx ...`, `eslint`, `tsc`, `cspell`).

- ✅ Correct: `make build`, `make test`, `make lint`, `make frontend-deps`, `make cspell`
- ❌ Incorrect: `go build ./...`, `go test ./...`, `npm install`, `npx cspell ...`, `eslint ...`, `tsc --noEmit`

Rationale: make targets encapsulate the exact tool versions, flags, and environment variables (e.g. `KUBEBUILDER_ASSETS` for envtest) that CI runs. Bypassing them causes drift from CI and breaks reproducibility.

If a needed operation has no make target, **add one first** instead of running the tool directly.

## Build/Test/Lint Commands

### Development

- **Full build**: `make build` (manifests + generate + fmt + vet + frontend + go)
- **Run locally**: `make run` (uses mirrord + air for hot reload)
- **Run with Vite dev server**: `FRONTEND_DEV=true make run`
- **Build Go binaries only**: `make build-go`
- **Build frontend assets**: `make frontend-build`
- **Build container image**: `make docker-build IMG=docker.io/thegeeklab/renovate-operator:latest`

### Testing

- **All unit tests**: `make test` (uses envtest, excludes e2e)
- **Single test**: `make test FOCUS="TestName"` (uses Ginkgo focus)
- **Skip tests**: `make test SKIP="TestName"` (uses Ginkgo skip)
- **E2E tests**: `make test-e2e` (requires Kind cluster with image loaded) — **DO NOT RUN**: E2E tests are not fully implemented; never run them autonomously
- **Helm unit tests**: `make helm-test`
- **Test with race detector**: `go test -race ./...` (only for packages not requiring envtest) — acceptable exception; envtest-based packages still need `make test`

### Linting

- **Lint all**: `make lint` (runs yamlfmt-dry, golangci-lint, eslint, typecheck, cspell)
- **Go lint only**: `make golangci-lint`
- **Frontend lint**: `make eslint`
- **TypeScript check**: `make typecheck`
- **YAML format check**: `make yamlfmt-lint`
- **Spell check**: `make cspell`

### Code Generation

- **Generate all**: `make generate` (deepcopy, mocks, icons, templ, yamlfmt)
- **Generate CRDs/RBAC/webhooks**: `make manifests`
- **Generate templ only**: `make templ`
- **Generate icons**: `make gen-icons`
- **Format Go code**: `make fmt` (uses gofumpt with -extra)
- **Format YAML**: `make yamlfmt`

### Dependencies

- **Update all Go deps**: `go get -u ./...`
- **Tidy Go modules**: `go mod tidy`
- **Update npm deps**: `npm update`
- **Install frontend deps**: `make frontend-deps`

## Code Style & Conventions

### Consistency

- **Follow existing patterns**: When implementing new features, always check how similar functionality is implemented elsewhere in the codebase and follow the same patterns.
- **Inline over abstraction**: Prefer inline code that matches existing patterns over creating helper methods, unless the abstraction is already established in the codebase.

### Imports

- **Order**: Standard library first, blank line, third-party packages, blank line, local packages
- **Example**:

  ```go
  import (
      "context"
      "fmt"

      "k8s.io/apimachinery/pkg/runtime"
      "sigs.k8s.io/controller-runtime/pkg/client"

      "github.com/thegeeklab/renovate-operator/api/v1beta1"
      "github.com/thegeeklab/renovate-operator/pkg/util/k8s"
  )
  ```

### Naming

- **Exported**: PascalCase (e.g., `Renovator`, `GitRepo`, `NewReconciler`)
- **Unexported**: camelCase (e.g., `defaultSchedule`, `getRepositories`)
- **Acronyms**: Use uppercase for exported (e.g., `API`, `URL`, `ID`), lowercase for unexported (e.g., `api`, `url`, `id`)
- **CRD types**: Match Kubernetes naming (e.g., `Renovator`, `Discovery`, `GitRepo`)
- **Interfaces**: Name after what they do (e.g., `Provider`, `Scheduler`)

### Error Handling

- **Always check errors**: Never ignore error return values
- **Wrap errors**: Use `fmt.Errorf` with `%w` for error context: `fmt.Errorf("failed to reconcile: %w", err)`
- **No dynamic errors**: `err113` linter forbids `errors.New` with dynamic messages in non-test code
- **Log then return**: Log errors with context before returning
- **Example**:

  ```go
  if err := r.Client.Get(ctx, key, obj); err != nil {
      return fmt.Errorf("failed to get resource: %w", err)
  }
  ```

### Logging

- **Use controller-runtime logger**: `log.FromContext(ctx)`
- **Example**: `log.Info("reconciling renovator", "name", req.NamespacedName)`
- **Error logs**: `log.Error(err, "reconcile failed", "name", req.NamespacedName)`
- **Inject logger**: Pass `logr.Logger` to structs via dependency injection

### Comments

- **Document exports**: All exported functions, types, methods need doc comments
- **Format**: Start with the name: `// Reconciler reconciles a Renovator object.`
- **Single line**: Use `//` for all comments
- **No unnecessary comments**: Do not add comments unless asked

### Interface Implementation

- **Compile-time checks**: Always add compile-time interface checks for structs that implement interfaces
- **Format**: `var _ InterfaceName = (*StructName)(nil)`
- **Placement**: Place the check immediately after the struct definition
- **Example**:

  ```go
  type GitHubProvider struct {
      client *github.Client
  }

  var _ provider.Provider = (*GitHubProvider)(nil)
  ```

### YAML Struct Tags

- **Must be kebab-case**: Enforced by `tagliatelle` linter
- **Example**:

  ```go
  type RenovatorSpec struct {
      Schedule string `yaml:"schedule"`
      PlatformConfig PlatformConfig `yaml:"platform-config"`
  }
  ```

## Templ Syntax

- **Components**: `templ ComponentName(params) { <html>content</html> }`
- **Expressions**: Use `{ variable }` for interpolation, `{ function() }` for function calls
- **Composition**: Call other components with `@ComponentName(args)`
- **All tags must be closed**: Use `<div></div>` or `<br/>` (self-closing)
- **Parameters**: Accept Go types as parameters: `templ Button(text string, disabled bool)`
- **File structure**: Package declaration, imports, then templ components
- **Generated files**: `*_templ.go` files are auto-generated, edit only `*.templ` files
- **Format**: Run `make templ-fmt` to format templ files

## HTMX Patterns

- **Basic requests**: `hx-get="/path"`, `hx-post="/path"`, `hx-put="/path"`, `hx-delete="/path"`
- **Triggers**: `hx-trigger="click"` (default), `hx-trigger="change"`, `hx-trigger="keyup delay:500ms"`
- **Targets**: `hx-target="#result"`, `hx-target="closest tr"`, `hx-target="next .error"`
- **Swapping**: `hx-swap="innerHTML"` (default), `hx-swap="outerHTML"`, `hx-swap="afterend"`
- **SSE**: Use `hx-ext="sse"` with `sse-connect="/events"` and `sse-swap="message"`
- **Indicators**: Add `class="htmx-indicator"` to show/hide loading states
- **Forms**: Include form values automatically, use `hx-include` for additional inputs

## Tailwind CSS

- **Utility-first**: Use small, single-purpose classes like `text-center`, `bg-blue-500`, `p-4`
- **No inline styles**: Avoid `style="..."` attributes in templ and JS; use Tailwind utility classes instead
- **Responsive**: Prefix utilities with breakpoints: `sm:text-left`, `md:flex`, `lg:grid-cols-3`
- **States**: Use state prefixes: `hover:bg-blue-700`, `focus:ring-2`, `disabled:opacity-50`
- **Spacing**: Use consistent scale: `p-4` (padding), `m-2` (margin), `gap-6` (gap)
- **Colors**: Use semantic names: `bg-red-500`, `text-gray-700`, `border-blue-200`
- **Layout**: Common patterns: `flex items-center justify-between`, `grid grid-cols-2 gap-4`
- **Typography**: Size and weight: `text-xl font-bold`, `text-sm text-gray-600`
- **Version**: Tailwind CSS v4 via `@tailwindcss/vite` plugin

## Architecture & Patterns

- **Operator pattern**: Uses kubebuilder with controller-runtime reconcilers
- **API types**: Define CRDs in `api/v1beta1/` with kubebuilder markers
- **Reconcilers**: Implement `Reconcile()` method in `internal/controller/`, use `pkg/util/reconciler` helpers
- **Provider abstraction**: Git platform logic in `internal/provider/` with per-platform subpackages (`github/`, `gitea/`)
- **Webhook receivers**: Handle platform webhooks in `internal/receiver/` with per-platform subpackages
- **Frontend**: Served by manager binary on port `8082`, static assets Vite-bundled and embedded
- **Context**: Always pass `context.Context` as first parameter; `noctx` linter enforces this for HTTP calls
- **Error wrapping**: Use `fmt.Errorf("...: %w", err)` for error context
- **Kubernetes helpers**: Use `pkg/util/k8s/` for naming/metadata helpers; `pkg/util/` for general utilities

## Creating New CRDs

All new CRDs must be created using kubebuilder scaffolding. Do not manually create API types or controllers.

### Create a new API with controller (default)

```bash
kubebuilder create api \
  --group renovate \
  --version v1beta1 \
  --kind <KindName>
```

### Create a new API without controller

```bash
kubebuilder create api \
  --group renovate \
  --version v1beta1 \
  --kind <KindName> \
  --controller=false
```

### Add webhooks to an existing API

```bash
kubebuilder create webhook \
  --group renovate \
  --version v1beta1 \
  --kind <KindName> \
  --defaulting \
  --validation
```

### After creating a new CRD

1. Edit the generated types in `api/v1beta1/<kind>_types.go`
2. Run `make generate` to generate deepcopy methods
3. Run `make manifests` to generate CRD YAML, RBAC, and webhook configs
4. Implement the reconciler in `internal/controller/<kind>_controller.go`

### Project configuration

- **Domain**: `thegeeklab.de`
- **Group**: `renovate`
- **Version**: `v1beta1`
- **Layout**: `go.kubebuilder.io/v4`

The `PROJECT` file is auto-updated by kubebuilder and tracks all registered resources.

## Environment Variables

- **KUBECONFIG**: Path to kubeconfig file (default: `~/.kube/config`)
- **FRONTEND_DEV**: Enable Vite dev server during `make run` (default: `false`)
- **IMG**: Container image tag for `make docker-build` (default: `docker.io/thegeeklab/renovate-operator:latest`)
- **KIND_CLUSTER**: Kind cluster name (default: `renovate-operator`)

## Testing

- **Framework**: Ginkgo v2 + Gomega, with `testify` assertions in some places
- **Test runner**: `gotestsum` (wrapped by `make test`)
- **Run all unit tests**: `make test` (sets up envtest with KUBEBUILDER_ASSETS)
- **Single package (with envtest)**: Must use `make test` — envtest setup is only configured there
- **Single test**: `make test FOCUS="TestName"` (uses Ginkgo focus)
- **Skip tests**: `make test SKIP="TestName"` (uses Ginkgo skip)
- **E2E tests**: `make test-e2e` (requires Kind cluster with image loaded) — **DO NOT RUN**: E2E tests are not fully implemented; never run them autonomously
- **Helm tests**: `make helm-test` (uses helm-unittest)
- **Fixtures**: Placed in `fixtures/` subdirectories next to the tests that use them
- **Mocks**: Generated by mockery via `make generate`; placed in `mocks/` directories
- **envtest**: Uses `setup-envtest` for Kubernetes API testing (configured via `make test`)
- **Note**: Do not run `go test` directly on packages that use envtest — they require the KUBEBUILDER_ASSETS environment variable set by `make test`

## Project Structure

- `api/v1beta1/`: CRD types, deepcopy, groupversion, constants, resource helpers
- `cmd/`: `main.go` (manager), `discovery/main.go` (discovery binary)
- `internal/`:
  - `component/`: Reusable internal components
  - `controller/`: Kubebuilder reconcilers for each CRD
  - `frontend/`: Web dashboard
    - `auth/`: OIDC/OAuth2 providers (GitHub, Gitea), session, middleware
    - `view/`: templ components (`*_templ.go` are generated; edit `*.templ` only)
    - `viewmodel/`: View models
    - `static/`: JS/TS assets (Vite-bundled)
  - `metadata/`: Shared metadata helpers
  - `parser/`: Renovate log parser
  - `provider/`: Git platform abstraction (GitHub, Gitea)
  - `receiver/`: Webhook receivers (GitHub, Gitea, Renovate)
  - `resource/`: Resource builders for Kubernetes objects
  - `scheduler/`: Cron scheduling
  - `webhook/`: Admission webhooks
- `pkg/`: Public-ish utilities (discovery, util/k8s, util/reconciler)
- `config/`: Kubebuilder manifests (crd, rbac, manager, webhook, samples)
- `dist/`: Build output (Helm chart, install.yaml)
- `hack/`: Codegen helpers (e.g., gen-icons.go)
- `test/`: e2e tests and test utilities

## Generated Files (Do Not Edit)

These files are auto-generated and must not be edited by hand:

- `api/v1beta1/zz_generated.deepcopy.go` — run `make generate`
- `internal/frontend/view/*_templ.go` — run `make templ` (or `make generate`)
- `internal/frontend/view/icons.templ` — run `make gen-icons` (pulls from lucide-static npm package)
- `internal/provider/mocks/mock_*.go` — generated by mockery via `make generate` (config: `.mockery.yaml`)
- `config/crd/bases/*.yaml`, `config/rbac/*.yaml`, `config/webhook/manifests.yaml` — run `make manifests`

After editing any `*.templ` file, always run `make templ`. After editing any `api/v1beta1/*_types.go`, run `make generate`.

## Common Pitfalls

- **Stale generated files**: Forgetting to run `make generate` after changing API types — deepcopy and CRDs will be stale
- **Editing generated templ files**: `*_templ.go` files are regenerated; edit the `.templ` source instead
- **envtest failures**: Running `make test` without Go 1.26+ or with a stale `./bin/` — run `make setup-envtest` if envtest fails
- **E2E test setup**: E2E tests require a Kind cluster with the image loaded: `make kind-create && make docker-build kind-load test-e2e` — **DO NOT RUN**: E2E tests are not fully implemented; never run them autonomously
- **mirrord configuration**: The `run` target uses `mirrord`; ensure `mirrord.json` is configured for your environment
- **Line length**: `lll` linter is enabled — keep lines reasonably short (avoid very long lines)
- **YAML tag casing**: YAML struct tags must be `kebab-case` (enforced by `tagliatelle`)
- **Defaulting webhooks**: When adding new fields to API types (especially pointer types like `*bool`, `*int32`), check if a defaulting webhook exists in `internal/webhook/v1beta1/` and update it to set appropriate defaults. This ensures consistent behavior and proper handling of unset fields.

## CI

Woodpecker CI pipelines in `.woodpecker/`:

- `test.yaml` — runs `make test-ci` (manifests, generate, fmt, vet, test)
- `static.yaml` — linting (golangci-lint, eslint, typecheck, yamlfmt, cspell)
- `build-container.yaml` — multi-arch container build
- `build-package.yaml` — release packaging
- `docs.yaml` — documentation builds
