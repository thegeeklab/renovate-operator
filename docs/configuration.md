# Configuration Guide

This guide provides comprehensive information about configuring the renovate-operator for various use cases and environments.

## Table of Contents

- [Renovator Configuration](#renovator-configuration)
- [Renovate Settings](#renovate-settings)
- [Discovery Configuration](#discovery-configuration)
- [Runner Configuration](#runner-configuration)
- [Complete Example](#complete-example)

## Renovator Configuration

The `Renovator` Custom Resource is the main configuration object for the operator.

### Basic Structure

```yaml
apiVersion: renovate.thegeeklab.de/v1beta1
kind: Renovator
metadata:
  name: my-renovator
  namespace: renovate
spec:
  # Global settings
  suspend: false
  schedule: "0 2 * * *"
  image: "docker.io/thegeeklab/renovate-operator:latest"
  imagePullPolicy: IfNotPresent

  # Renovate configuration (includes platform settings)
  renovate:
    image: "ghcr.io/renovatebot/renovate:latest" # Renovate bot image
    platform:
      type: github
      endpoint: https://api.github.com
      token:
        secretKeyRef:
          name: github-secret
          key: token

  # Discovery settings
  discovery:
    schedule: "0 */2 * * *"
    filter: []

  # Runner configuration
  runner:
    instances: 1

  # Logging configuration
  logging:
    level: info
```

### Global Settings

| Field             | Type    | Default                                           | Description                 |
| ----------------- | ------- | ------------------------------------------------- | --------------------------- |
| `suspend`         | boolean | `false`                                           | Suspend all operations      |
| `schedule`        | string  | -                                                 | Cron schedule for execution |
| `image`           | string  | `"docker.io/thegeeklab/renovate-operator:latest"` | Operator container image    |
| `imagePullPolicy` | string  | `"IfNotPresent"`                                  | Image pull policy           |

#### Schedule Examples

```yaml
# Every day at 2 AM
schedule: "0 2 * * *"

# Every 6 hours
schedule: "0 */6 * * *"

# Weekdays only at 1 AM
schedule: "0 1 * * 1-5"

# Every Sunday at 3 AM
schedule: "0 3 * * 0"
```

## Renovate Settings

Configure Renovate behavior including platform connections, dry run modes, rate limiting, and other bot-specific settings.

```yaml
spec:
  renovate:
    platform:
      type: github
      endpoint: https://api.github.com
      token:
        secretKeyRef:
          name: github-secret
          key: token

    # Dry run modes
    dryRun: full # Options: extract, lookup, full

    # Onboarding
    onboarding: true

    # Rate limiting
    prHourlyLimit: 10

    # Labels
    addLabels:
      - "dependencies"
      - "renovate"
```

| Field           | Type    | Default | Description                                |
| --------------- | ------- | ------- | ------------------------------------------ |
| `dryRun`        | enum    | `false` | Dry run mode (`extract`, `lookup`, `full`) |
| `onboarding`    | boolean | `true`  | Enable repository onboarding               |
| `prHourlyLimit` | integer | `10`    | Pull requests per hour limit               |
| `addLabels`     | array   | `[]`    | Labels to add to PRs                       |

### GitHub Configuration

Create platform secret:

```bash
# GitHub
kubectl create secret generic github-secret \
  --from-literal=token=ghp_your_github_token_here
```

```yaml
spec:
  renovate:
    platform:
      type: github
      endpoint: https://api.github.com
      token:
        secretKeyRef:
          name: github-secret
          key: token
    githubToken:
      secretKeyRef:
        name: github-secret
        key: token
```

#### Dry Run Modes

```yaml
# Extract mode - only extract dependencies
spec:
  renovate:
    dryRun: extract

# Lookup mode - extract and lookup updates
spec:
  renovate:
    dryRun: lookup

# Full mode - extract, lookup, but don't create PRs
spec:
  renovate:
    dryRun: full
```

## Discovery Configuration

Control how repositories are discovered and filtered.

### Basic Discovery

```yaml
spec:
  discovery:
    suspend: false
    schedule: "0 1 * * *"
    filter:
      - "octocat/*"
      - "!octocat/archived-*"
```

| Field      | Type    | Default         | Description                                       |
| ---------- | ------- | --------------- | ------------------------------------------------- |
| `suspend`  | boolean | `false`         | Suspend discovery operations                      |
| `schedule` | string  | `"0 */2 * * *"` | Discovery schedule (independent of main schedule) |
| `filter`   | array   | `[]`            | Repository filters (glob patterns)                |

### Filter Patterns

```yaml
spec:
  discovery:
    filter:
      # Include all repositories in 'octocat'
      - "octocat/*"

      # Include specific repositories
      - "octocat/important-repo"
      - "anotherorg/critical-app"

      # Exclude patterns (use ! prefix)
      - "!octocat/archived-*"
      - "!*/test-*"
      - "!octocat/legacy-system"

      # Include only certain types
      - "octocat/*-api"
      - "octocat/*-service"
```

### Advanced Discovery

```yaml
spec:
  discovery:
    suspend: false
    schedule: "0 */4 * * *" # Every 4 hours
    filter:
      - "company/*"
      - "!company/archived-*"
      - "!company/legacy-*"
      - "!*/test-*"
```

## Runner Configuration

Configure parallel processing and job execution. The Renovate Operator uses Indexed Jobs for efficient parallel processing.

| Field       | Type    | Default | Description                        |
| ----------- | ------- | ------- | ---------------------------------- |
| `instances` | integer | `1`     | Number of parallel workers (1-100) |

### Parallel Processing Configuration

The renovate-operator uses Indexed Jobs for parallel processing. The `instances` field controls the number of parallel workers:

```yaml
spec:
  runner:
    instances: 4 # Allows 4 jobs to run concurrently
```

### How Parallel Processing Works

The operator uses Indexed Jobs for efficient parallel processing:

1. **Repository Discovery**: Discovers all repositories matching your filters
2. **Indexed Job Creation**: Creates a single Indexed Job with:
   - `completions`: Total number of repositories to process
   - `parallelism`: Set to `runner.instances` value
   - `completionMode: Indexed`
3. **Parallel Execution**: Limits concurrent workers based on the `instances` configuration

## Complete Example

```yaml
apiVersion: renovate.thegeeklab.de/v1beta1
kind: Renovator
metadata:
  name: renovator
  namespace: renovate
spec:
  # Global settings
  suspend: false
  schedule: "0 2 * * 1-5" # Weekdays at 2 AM
  image: "docker.io/thegeeklab/renovate-operator:latest" # Use specific version in production
  imagePullPolicy: IfNotPresent

  # Platform configuration
  renovate:
    image: "ghcr.io/renovatebot/renovate:latest" # Pin specific version in production
    platform:
      type: github
      endpoint: https://api.github.com
      token:
        secretKeyRef:
          name: github-token
          key: token

    # Renovate behavior
    dryRun: false
    onboarding: true
    prHourlyLimit: 8
    addLabels:
      - "dependencies"
      - "renovate"
      - "automated"

    # GitHub token for enhanced rate limits
    githubToken:
      secretKeyRef:
        name: github-token
        key: token

  # Discovery configuration
  discovery:
    suspend: false
    schedule: "0 1 * * *" # Daily at 1 AM
    filter:
      - "mycompany/*"
      - "!mycompany/archived-*"
      - "!mycompany/legacy-*"
      - "!mycompany/*-backup"
      - "!*/test-*"

  # Runner configuration
  runner:
    instances: 6 # 6 parallel jobs

  # Logging
  logging:
    level: info

  # Resource management
  resources:
    limits:
      cpu: "2"
      memory: "4Gi"
    requests:
      cpu: "1"
      memory: "2Gi"
```
