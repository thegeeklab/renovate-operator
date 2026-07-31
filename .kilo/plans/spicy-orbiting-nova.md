# Warning Badge on Renovator List View

## Context

The renovator list view shows GitRepo cards with job status (Succeeded/Running/Failed) derived from Kubernetes Job conditions. However, a job can succeed while still emitting WARN-level log entries. The parser already detects these warnings (`WarnCount`/`ErrorCount` in `ParseLogsResult`) during PR activity aggregation, but the counts are discarded. We need to surface them as a warning badge on the RenovatorCard, following the same pattern as the existing PR badge.

## Approach

Propagate warning/error counts from the existing log parsing pipeline through to a new HTMX badge on the RenovatorCard. The data already flows through `parseJobPRActivity` -> `computePRActivityForRenovator` -> `GetPRActivityForRenovator` -> `buildRenovatorSummaries` -> `WebView`. We extend each layer with two new fields (`WarnCount`, `ErrorCount`) and add a new badge component + HTMX endpoint.

## Changes

### 1. `internal/frontend/data_factory.go`

- Add `WarnCount int` and `ErrorCount int` to `PRActivitySummary`
- Add `WarnCount int` and `ErrorCount int` to `prJobSample`
- In `parseJobPRActivity`: propagate `res.WarnCount` and `res.ErrorCount` from `ParseLogsResult` into `prJobSample`
- In `computePRActivityForRenovator`: aggregate `sample.WarnCount` and `sample.ErrorCount` into the summary

### 2. `internal/frontend/viewmodel/viewmodel.go`

- Add `WarnCount int` and `ErrorCount int` to `WebView`

### 3. `internal/frontend/web.go`

- In `buildRenovatorSummaries`: pass `prActivity.WarnCount` and `prActivity.ErrorCount` into `WebView`
- Add `HandleRenovatorWarnings` handler (mirrors `HandleRenovatorPRs`): fetches `GetPRActivityForRenovator`, renders `RenovatorWarningsBadge` partial
- Register route `GET /renovators/warnings` in `RegisterRoutes`

### 4. `internal/frontend/sanitize/sanitize.go`

- Add `RenovatorWarningsURL(namespace, renovatorUID string) string` (mirrors `RenovatorPRsURL`)

### 5. `internal/frontend/view/renovator_list.templ`

- Add `RenovatorWarningsBadge` templ component:
  - Hidden (no badge) when `warnCount == 0 && errorCount == 0`
  - Yellow/amber pill with `IconTriangleAlert` when `warnCount > 0 && errorCount == 0`
  - Red pill with `IconTriangleAlert` when `errorCount > 0`
  - Auto-refreshes via `hx-trigger="sse:job-updated throttle:15s"` (same as PR badge)
  - Tooltip shows e.g. "3 warnings" or "2 errors, 1 warning"
- Render `@RenovatorWarningsBadge(v.Namespace, v.Renovator, v.WarnCount, v.ErrorCount)` in `RenovatorCard` between the PRs badge and the Runner column

### 6. Tests (TDD)

- `internal/frontend/sanitize/sanitize_test.go`: test `RenovatorWarningsURL` output
- `internal/frontend/web_test.go`: test `HandleRenovatorWarnings` handler (missing params -> 400, valid request -> 200)
- `internal/frontend/data_factory_test.go`: no new tests needed; existing `GetPRActivityForRenovator` tests cover the aggregation path, and the new fields are additive

## Verification

1. `make generate` (regenerate templ files)
2. `make test` (all unit tests pass)
3. `make lint` (no lint errors)
4. Manual: run `make run`, verify warning badge appears on RenovatorCard when jobs have WARN-level log entries
