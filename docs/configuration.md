# Configuration Guide

This guide provides comprehensive information about configuring the renovate-operator for various use cases and environments.

## Table of Contents

- [Renovator Configuration](#renovator-configuration)
- [Renovate Settings](#renovate-settings)
- [Discovery Configuration](#discovery-configuration)
- [Scheduler Configuration](#scheduler-configuration)
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

  # Scheduler configuration
  scheduler:
    strategy: none
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

## Scheduler Configuration

Configure parallel processing and job execution. The Renovate Operator supports efficient parallel processing of repositories through its built-in batching strategy, allowing you to significantly reduce total runtime for large repository sets.

| Field       | Type    | Default | Description                            |
| ----------- | ------- | ------- | -------------------------------------- |
| `strategy`  | enum    | `none`  | Execution strategy (`none` or `batch`) |
| `instances` | integer | `1`     | Number of parallel workers (1-100)     |
| `batchSize` | integer | auto    | Repositories per batch (1-1000)        |

### Scheduler Strategies

#### None Strategy (Sequential)

```yaml
spec:
  scheduler:
    strategy: none
    instances: 1
```

#### Batch Strategy (Parallel Processing)

Basic parallel processing:

```yaml
spec:
  scheduler:
    strategy: batch
    instances: 4 # 4 parallel workers
    batchSize: 20 # 20 repositories per batch
```

Auto-calculated batch sizes (recommended for dynamic environments):

```yaml
spec:
  scheduler:
    strategy: batch
    instances: 5
    # batchSize omitted - automatically calculated based on repo count and instances
```

### How Parallel Processing Works

The operator uses Kubernetes [Indexed Jobs](https://kubernetes.io/docs/concepts/workloads/controllers/job/#completion-mode) to achieve parallel processing:

1. **Repository Discovery**: Discovers all repositories matching your filters
2. **Batch Creation**: Divides repositories into batches based on your configuration
3. **Parallel Execution**: Creates a single Kubernetes Job with multiple parallel pods
4. **Index-based Processing**: Each pod processes a specific batch using `JOB_COMPLETION_INDEX`

#### Batch Size Calculation

When `batchSize` is not specified, the operator automatically calculates optimal batch sizes:

- **Target**: 3 batches per instance for optimal parallelization
- **Formula**: `batchSize = totalRepositories / (instances × 3)`
- **Bounds**: Minimum 1 repository, maximum 50 repositories per batch
- **Example**: 120 repositories with 4 instances = 10 repositories per batch (12 total batches)

### Performance Tuning Guidelines

Choose appropriate configuration based on your repository count and infrastructure:

**Performance Considerations:**

- **Choose appropriate instance count**: Don't exceed your cluster's capacity
- **Optimize batch size**: 10-50 repositories per batch works well
- **Consider platform rate limits**: Adjust `prHourlyLimit` accordingly
- **Monitor resource usage**: Use `kubectl top pods` to check resource consumption

#### Best Practices

1. **Start conservative**: Begin with 2-3 instances
2. **Monitor performance**: Check job completion times
3. **Gradually increase**: Add more instances if resources allow
4. **Fine-tune batch size**: Adjust based on your specific needs

#### Migration from Sequential Processing

If you're currently using `strategy: none` and experiencing long runtimes:

1. Start with `instances: 2` and monitor
2. Check job completion times and resource usage
3. Gradually increase instances if cluster resources allow
4. Adjust batch size based on performance

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

  # Scheduler configuration
  scheduler:
    strategy: batch
    instances: 6
    batchSize: 25

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
