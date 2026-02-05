# API Reference

This document provides a comprehensive reference for all Custom Resource Definitions (CRDs) and their configurations in the renovate-operator.

## Table of Contents

- [Renovator CRD](#renovator-crd)
- [GitRepo CRD](#gitrepo-crd)
- [Common Types](#common-types)
- [Status Conditions](#status-conditions)

## Renovator CRD

The primary Custom Resource for configuring a Renovate instance.

### API Version and Kind

```yaml
apiVersion: renovate.thegeeklab.de/v1beta1
kind: Renovator
```

### Metadata

Standard Kubernetes metadata fields are supported:

```yaml
metadata:
  name: string # Required: Renovator instance name
  namespace: string # Optional: Kubernetes namespace
  labels: map[string]string # Optional: Kubernetes labels
  annotations:
    renovate.thegeeklab.de/operation: "discover" # Optional: Trigger immediate Git repo discovery
```

### Spec

The `RenovatorSpec` defines the desired state of a Renovator.

#### Root Fields

| Field             | Type          | Required | Default                                           | Description                      |
| ----------------- | ------------- | -------- | ------------------------------------------------- | -------------------------------- |
| `suspend`         | boolean       | No       | `false`                                           | Suspend all operations           |
| `schedule`        | string        | Yes      | -                                                 | Cron schedule for execution      |
| `image`           | string        | No       | `"docker.io/thegeeklab/renovate-operator:latest"` | Operator container image         |
| `imagePullPolicy` | string        | No       | `"IfNotPresent"`                                  | Image pull policy                |
| `renovate`        | RenovateSpec  | Yes      | -                                                 | Renovate configuration           |
| `discovery`       | DiscoverySpec | Yes      | -                                                 | Repository discovery settings    |
| `scheduler`       | SchedulerSpec | No       | -                                                 | Parallel execution configuration |
| `logging`         | LoggingSpec   | No       | -                                                 | Logging configuration            |

#### RenovateSpec

Configuration for the Renovate bot behavior.

```yaml
renovate:
  image: string                    # Optional: Renovate container image (default: "ghcr.io/renovatebot/renovate:latest")
  imagePullPolicy: string          # Optional: Image pull policy (default: "IfNotPresent")

  platform:
    type: PlatformType              # Required: Platform type (github|gitea)
    endpoint: string                # Required: Platform API endpoint
    token: EnvVarSource            # Required: Authentication token

  dryRun: DryRun                   # Optional: Dry run mode (extract|lookup|full)
  onboarding: *bool                # Optional: Enable onboarding (default: true)
  prHourlyLimit: int               # Optional: PR rate limit (default: 10)
  addLabels: []string              # Optional: Labels to add to PRs

  githubToken: *EnvVarSource       # Optional: GitHub token reference for enhanced limits
```

##### PlatformType

Supported platform types:

| Value    | Description                     |
| -------- | ------------------------------- |
| `github` | GitHub.com or GitHub Enterprise |
| `gitea`  | Gitea instances                 |

##### DryRun

Dry run modes for testing:

| Value     | Description                       |
| --------- | --------------------------------- |
| `extract` | Only extract dependencies         |
| `lookup`  | Extract and lookup updates        |
| `full`    | Full dry run without creating PRs |

##### PlatformSpec

```yaml
platform:
  type: github | gitea # Required: Platform type
  endpoint: string # Required: API endpoint URL
  token: EnvVarSource # Required: Authentication token reference
```

#### DiscoverySpec

Configuration for repository discovery.

```yaml
discovery:
  suspend: *bool                  # Optional: Suspend discovery
  schedule: string                # Optional: Discovery schedule
  filter: []string                # Optional: Repository filters
```

| Field      | Type     | Default         | Description                  |
| ---------- | -------- | --------------- | ---------------------------- |
| `suspend`  | boolean  | `false`         | Suspend discovery operations |
| `schedule` | string   | `"0 */2 * * *"` | Cron schedule for discovery  |
| `filter`   | []string | `[]`            | Repository filter patterns   |

##### Filter Patterns

Repository filters support glob patterns:

```yaml
filter:
  - "octocat/*" # Include all repos in octocat
  - "!octocat/archived-*" # Exclude archived repos (! prefix)
  - "team-*/backend-*" # Include specific patterns
```

#### SchedulerSpec

Configuration for parallel job execution.

```yaml
scheduler:
  strategy: SchedulerStrategy # Optional: Execution strategy
  instances: int32 # Optional: Parallel instances
  batchSize: int # Optional: Repositories per batch
```

| Field       | Type  | Default  | Validation      | Description                |
| ----------- | ----- | -------- | --------------- | -------------------------- |
| `strategy`  | enum  | `"none"` | `none`, `batch` | Execution strategy         |
| `instances` | int32 | `1`      | 1-100           | Number of parallel workers |
| `batchSize` | int   | auto     | 1-1000          | Repositories per batch     |

##### SchedulerStrategy

| Value   | Description                          |
| ------- | ------------------------------------ |
| `none`  | Sequential processing (single batch) |
| `batch` | Parallel processing with batching    |

#### LoggingSpec

Configuration for logging behavior.

```yaml
logging:
  level: LogLevel # Optional: Log level (default: info)
```

##### LogLevel

| Value   | Description                   |
| ------- | ----------------------------- |
| `trace` | Most verbose logging          |
| `debug` | Debug information             |
| `info`  | General information (default) |
| `warn`  | Warning messages              |
| `error` | Error messages only           |
| `fatal` | Fatal errors only             |

### Status

The `RenovatorStatus` reflects the observed state of a Renovator.

```yaml
status:
  ready: boolean                  # Overall readiness status
  failed: int                     # Number of failed operations
  conditions: []Condition         # Detailed status conditions
  specHash: string                # Hash of current spec
  repositories: []string          # List of discovered repositories
```

#### Status Fields

| Field          | Type               | Description                                   |
| -------------- | ------------------ | --------------------------------------------- |
| `ready`        | boolean            | Whether the Renovator is ready                |
| `failed`       | int                | Number of failed operations                   |
| `conditions`   | []metav1.Condition | Detailed status conditions                    |
| `specHash`     | string             | Hash of the current spec for change detection |
| `repositories` | []string           | List of discovered repositories               |

## GitRepo CRD

Represents a discovered repository that can be processed by Renovate.

### API Version and Kind

```yaml
apiVersion: renovate.thegeeklab.de/v1beta1
kind: GitRepo
```

### Spec

```yaml
spec:
  name: string # Required: Repository name (e.g., "owner/repo")
  webhookId: string # Optional: Webhook ID for repository
```

### Status

```yaml
status:
  ready: boolean                  # Whether repository is ready
  failed: int                     # Number of failed operations
  conditions: []Condition         # Status conditions
  specHash: string                # Hash of current spec
```

## Common Types

### EnvVarSource

Reference to environment variable sources (commonly used for secrets).

```yaml
# From secret
secretKeyRef:
  name: string # Secret name
  key: string # Key within secret
  optional: boolean # Whether the secret/key is optional

# From config map
configMapKeyRef:
  name: string # ConfigMap name
  key: string # Key within ConfigMap
  optional: boolean # Whether the ConfigMap/key is optional

# Direct value (not recommended for sensitive data)
value: string # Direct value
```

### LocalObjectReference

Reference to objects in the same namespace.

```yaml
name: string # Object name
```

## Status Conditions

Standard Kubernetes condition types used across CRDs.

### Condition Types

| Type          | Description                         |
| ------------- | ----------------------------------- |
| `Ready`       | Resource is ready for use           |
| `Progressing` | Resource is being processed         |
| `Degraded`    | Resource is degraded but functional |
| `Available`   | Resource is available               |

### Condition Status

| Status    | Description                 |
| --------- | --------------------------- |
| `True`    | Condition is met            |
| `False`   | Condition is not met        |
| `Unknown` | Condition status is unknown |

### Example Conditions

```yaml
conditions:
  - type: Ready
    status: "True"
    lastTransitionTime: "2023-10-01T10:00:00Z"
    reason: "RenovatorReady"
    message: "Renovator is ready and operational"

  - type: Progressing
    status: "False"
    lastTransitionTime: "2023-10-01T10:00:00Z"
    reason: "JobCompleted"
    message: "Last renovation job completed successfully"
```

### GitRepo Example

```yaml
apiVersion: renovate.thegeeklab.de/v1beta1
kind: GitRepo
metadata:
  name: mycompany-api-service
  namespace: renovate
  labels:
    renovator: production-renovator
    platform: github
spec:
  name: "mycompany/api-service"
  webhookId: "webhook-123"
status:
  ready: true
  failed: 0
  specHash: "abc123def456"
  conditions:
    - type: Ready
      status: "True"
      lastTransitionTime: "2023-10-01T10:00:00Z"
      reason: "RepositoryReady"
      message: "Repository is ready for processing"
```

### Supported Annotations

The following annotations are supported for the Renovator CRD:

| Annotation Key                     | Description                          | Values     |
| ---------------------------------- | ------------------------------------ | ---------- |
| `renovate.thegeeklab.de/operation` | Trigger immediate git repo discovery | `discover` |
