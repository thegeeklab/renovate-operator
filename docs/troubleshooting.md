# Troubleshooting Guide

This guide helps you diagnose and resolve common issues with the renovate-operator.

## Table of Contents

- [Common Issues](#common-issues)
- [Diagnostic Commands](#diagnostic-commands)
- [Error Messages](#error-messages)
- [Performance Issues](#performance-issues)
- [Logging and Monitoring](#logging-and-monitoring)

## Common Issues

### 1. Operator Not Starting

#### Symptoms

- Operator pod in `CrashLoopBackOff` or `Error` state
- No reconciliation happening
- CRDs not being processed

#### Diagnosis

```bash
# Check operator pod status
kubectl get pods -n renovate-system

# Check operator logs
kubectl logs -n renovate-system deployment/renovate-operator-controller-manager -f

# Check events
kubectl get events -n renovate-system --sort-by='.lastTimestamp'
```

#### Common Causes and Solutions

**Missing CRDs:**

```bash
# Install CRDs from source
make install
```

**RBAC Issues:**

```bash
# Check service account permissions
kubectl auth can-i create renovators --as=system:serviceaccount:renovate-system:renovate-operator-controller-manager

# Deploy operator with correct RBAC from source
make deploy
```

**Resource Limits:**

```bash
# Check resource usage
kubectl top pods -n renovate-system

# Increase resource limits
kubectl patch deployment renovate-operator-controller-manager -n renovate-system -p '{"spec":{"template":{"spec":{"containers":[{"name":"manager","resources":{"limits":{"memory":"4Gi","cpu":"2"},"requests":{"memory":"2Gi","cpu":"1"}}}]}}}}'
```

### 2. No Repositories Being Discovered

#### Symptoms

- No GitRepo CRs created
- Discovery reconciler not finding repositories
- Empty repository list in Renovator status

#### Diagnosis

```bash
# Check Renovator status
kubectl get renovator my-renovator -o yaml

# Check GitRepo CRs
kubectl get gitrepos -A

# Check discovery logs
kubectl logs -n renovate-system deployment/renovate-operator-controller-manager | grep discovery
```

#### Solutions

**Incorrect Platform Configuration:**

```yaml
# Verify platform settings
apiVersion: renovate.thegeeklab.de/v1beta1
kind: Renovator
spec:
  renovate:
    platform:
      type: github # Correct type
      endpoint: https://api.github.com # Correct endpoint
      token:
        secretKeyRef:
          name: github-secret
          key: token
```

**Filter Issues:**

```yaml
# Check filter patterns
discovery:
  filter:
    - "myorg/*" # Include pattern
    - "!myorg/archived-*" # Exclude pattern (note the !)
```

**Token Permissions:**

```bash
# Test token manually
curl -H "Authorization: token YOUR_TOKEN" https://api.github.com/user/repos

# Check token scopes
curl -H "Authorization: token YOUR_TOKEN" -I https://api.github.com/user/repos
# Look for X-OAuth-Scopes header
```

### 3. Jobs Not Running

#### Symptoms

- CronJob created but no Jobs executing
- Jobs stuck in Pending state
- No Renovate containers running

#### Diagnosis

```bash
# Check CronJob
kubectl get cronjobs -A

# Check Jobs
kubectl get jobs -A

# Check CronJob events
kubectl describe cronjob my-renovator-runner -n renovate-system
```

#### Solutions

**CronJob Suspended:**

```bash
# Check if suspended
kubectl get cronjob my-renovator-runner -n renovate-system -o yaml | grep suspend

# Unsuspend if needed
kubectl patch cronjob my-renovator-runner -n renovate-system -p '{"spec":{"suspend":false}}'
```

**Resource Constraints:**

```bash
# Check node resources
kubectl describe nodes

# Check resource quotas
kubectl describe quota -n renovate-system

# Check limit ranges
kubectl describe limitrange -n renovate-system
```

**Image Pull Issues:**

```bash
# Check image pull secrets
kubectl get secrets -n renovate-system

# Test image pull
kubectl run test-renovate --image=ghcr.io/renovatebot/renovate:latest --dry-run=client -o yaml
```

### 4. Authentication Failures

#### Symptoms

- Jobs failing with authentication errors
- "Bad credentials" or similar errors
- Rate limit errors

#### Diagnosis

```bash
# Check job logs
kubectl logs -l app.kubernetes.io/name=renovator-runner -n renovate-system

# Check secret content
kubectl get secret github-secret -n renovate-system -o yaml
echo "BASE64_TOKEN" | base64 -d
```

#### Solutions

**Token Validation:**

```bash
# Test token manually
curl -H "Authorization: token YOUR_TOKEN" https://api.github.com/user

# Check rate limits
curl -H "Authorization: token YOUR_TOKEN" https://api.github.com/rate_limit
```

**Secret Update:**

```bash
# Update secret
kubectl patch secret github-secret -n renovate-system \
  --type='json' \
  -p='[{"op": "replace", "path": "/data/token", "value":"'$(echo -n NEW_TOKEN | base64)'"}]'
```

**GitHub App Configuration:**

```yaml
# Ensure correct GitHub App setup
renovate:
  githubAppId:
    secretKeyRef:
      name: github-app-secret
      key: app-id
  githubAppPrivateKey:
    secretKeyRef:
      name: github-app-secret
      key: private-key
  githubAppInstallationId:
    secretKeyRef:
      name: github-app-secret
      key: installation-id
```

## Diagnostic Commands

### Essential Debugging Commands

#### Operator Health Check

```bash
#!/bin/bash
# operator-health-check.sh

echo "=== Operator Health Check ==="

echo "1. Checking operator deployment..."
kubectl get deployment -n renovate-system renovate-operator-controller-manager

echo "2. Checking operator pods..."
kubectl get pods -n renovate-system -l control-plane=controller-manager

echo "3. Checking CRDs..."
kubectl get crd | grep renovate

echo "4. Checking RBAC..."
kubectl get serviceaccount,role,rolebinding,clusterrole,clusterrolebinding -n renovate-system

echo "5. Checking recent events..."
kubectl get events -n renovate-system --sort-by='.lastTimestamp' | tail -10

echo "6. Checking operator logs (last 50 lines)..."
kubectl logs -n renovate-system deployment/renovate-operator-controller-manager --tail=50
```

#### Renovator Status Check

```bash
#!/bin/bash
# renovator-status-check.sh

RENOVATOR_NAME=${1:-"my-renovator"}
NAMESPACE=${2:-"renovate-system"}

echo "=== Renovator Status Check: $RENOVATOR_NAME ==="

echo "1. Renovator resource status..."
kubectl get renovator $RENOVATOR_NAME -n $NAMESPACE -o yaml

echo "2. Associated GitRepos..."
kubectl get gitrepos -n $NAMESPACE --show-labels

echo "3. CronJob status..."
kubectl get cronjob ${RENOVATOR_NAME}-runner -n $NAMESPACE -o yaml

echo "4. Recent Jobs..."
kubectl get jobs -n $NAMESPACE -l renovator.renovate/name=$RENOVATOR_NAME --sort-by='.metadata.creationTimestamp'

echo "5. Running Pods..."
kubectl get pods -n $NAMESPACE -l app.kubernetes.io/name=renovator-runner
```

#### Network Connectivity Test

```bash
#!/bin/bash
# network-test.sh

NAMESPACE=${1:-"renovate-system"}
ENDPOINT=${2:-"https://api.github.com"}

echo "=== Network Connectivity Test ==="

# Create test pod
kubectl run network-test --image=curlimages/curl --rm -it -n $NAMESPACE -- sh -c "
  echo 'Testing connectivity to $ENDPOINT'
  curl -v -I $ENDPOINT
  echo 'DNS resolution test:'
  nslookup api.github.com
  echo 'Network test completed'
"
```

### Advanced Diagnostics

#### Resource Usage Analysis

```bash
#!/bin/bash
# resource-analysis.sh

echo "=== Resource Usage Analysis ==="

echo "1. Node resource usage..."
kubectl top nodes

echo "2. Pod resource usage..."
kubectl top pods -n renovate-system

echo "3. Resource quotas..."
kubectl describe quota -n renovate-system

echo "4. Limit ranges..."
kubectl describe limitrange -n renovate-system

echo "5. Persistent volumes..."
kubectl get pv,pvc -n renovate-system
```

#### Log Analysis

```bash
#!/bin/bash
# log-analysis.sh

NAMESPACE=${1:-"renovate-system"}
SINCE=${2:-"1h"}

echo "=== Log Analysis ==="

echo "1. Operator logs with errors..."
kubectl logs -n $NAMESPACE deployment/renovate-operator-controller-manager --since=$SINCE | grep -i error

echo "2. Operator logs with warnings..."
kubectl logs -n $NAMESPACE deployment/renovate-operator-controller-manager --since=$SINCE | grep -i warn

echo "3. Job logs..."
kubectl logs -n $NAMESPACE -l app.kubernetes.io/name=renovator-runner --since=$SINCE --tail=100

echo "4. Recent events..."
kubectl get events -n $NAMESPACE --since=$SINCE --sort-by='.lastTimestamp'
```

## Error Messages

### Common Error Messages and Solutions

#### "failed to create renovator job"

**Error:**

```text
ERROR controller-runtime.manager.controller.renovator failed to create renovator job: admission webhook "validation.renovate.thegeeklab.de" denied the request
```

**Solution:**

```bash
# Check webhook configuration
kubectl get validatingadmissionwebhooks,mutatingadmissionwebhooks

# Remove webhook if not needed
kubectl delete validatingadmissionwebhook renovate-operator-validating-webhook
```

#### "x509: certificate signed by unknown authority"

**Error:**

```text
ERROR x509: certificate signed by unknown authority
```

**Solution:**

Custom CA certificates would need to be configured at the operator deployment level, not in the Renovator CRD.

#### "rate limit exceeded"

**Error:**

```text
ERROR GitHub API rate limit exceeded
```

**Solution:**

```yaml
# Reduce parallel instances and PR limits
spec:
  runner:
    instances: 2 # Reduce from higher number
  renovate:
    prHourlyLimit: 5 # Reduce from higher number
```

#### "repository not found"

**Error:**

```text
ERROR Repository not found or access denied
```

**Solution:**

```bash
# Check token permissions
curl -H "Authorization: token YOUR_TOKEN" https://api.github.com/repos/owner/repo

# Verify repository exists and token has access
```

#### "context deadline exceeded"

**Error:**

```text
ERROR context deadline exceeded
```

**Solution:**

```yaml
# Configure longer timeouts in renovate settings
spec:
  renovate:
    env:
      - name: RENOVATE_TIMEOUT
        value: "300000" # 5 minutes
```

## Performance Issues

### Slow Repository Discovery

#### Symptoms

- Discovery taking very long
- Timeout errors during discovery
- High memory usage during discovery

#### Solutions

**Optimize Filter Patterns:**

```yaml
# Use more specific patterns
discovery:
  filter:
    - "myorg/active-*" # Specific prefix
    - "!myorg/archived-*" # Exclude archived
    - "!myorg/test-*" # Exclude test repos
```

**Adjust Discovery Schedule:**

```yaml
# Less frequent discovery
discovery:
  schedule: "0 0 * * 0" # Weekly instead of daily
```

**Increase Resources:**

```yaml
# More resources for discovery
resources:
  limits:
    cpu: "4"
    memory: "8Gi"
  requests:
    cpu: "2"
    memory: "4Gi"
```

### Slow Job Execution

#### Symptoms

- Jobs taking very long to complete
- High resource usage
- Jobs timing out

#### Solutions

**Optimize Batch Configuration:**

```yaml
runner:
  strategy: batch
  instances: 6 # More parallel workers
  batchSize: 15 # Smaller batches
```

**Increase Job Resources:**

```yaml
resources:
  limits:
    cpu: "2"
    memory: "4Gi"
  requests:
    cpu: "1"
    memory: "2Gi"
```

**Use Dry Run for Testing:**

```yaml
renovate:
  dryRun: extract # Fastest mode
```

### Memory Issues

#### Symptoms

- OOMKilled pods
- High memory usage
- Slow performance

#### Solutions

**Increase Memory Limits:**

```yaml
resources:
  limits:
    memory: "8Gi"
  requests:
    memory: "4Gi"
```

**Reduce Batch Sizes:**

```yaml
runner:
  batchSize: 10 # Smaller batches use less memory
```

**Optimize Renovate Configuration:**

Node.js memory optimization would need to be configured at the container image level.

## Logging and Monitoring

### Enable Debug Logging

```yaml
apiVersion: renovate.thegeeklab.de/v1beta1
kind: Renovator
metadata:
  name: debug-renovator
spec:
  logging:
    level: debug
```

### Structured Log Analysis

```bash
# Extract error logs (if logs are in JSON format)
kubectl logs -n renovate-system deployment/renovate-operator-controller-manager | \
  grep '"level":"error"' | \
  jq -r '"\(.ts) \(.msg)"' 2>/dev/null || \
  kubectl logs -n renovate-system deployment/renovate-operator-controller-manager | grep -i error

# Count error types
kubectl logs -n renovate-system deployment/renovate-operator-controller-manager | \
  grep -i error | \
  head -20
```

### Monitoring Setup

#### Prometheus Metrics

```yaml
# ServiceMonitor for metrics
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: renovate-operator-metrics
spec:
  selector:
    matchLabels:
      control-plane: controller-manager
  endpoints:
    - port: metrics
      interval: 30s
```

#### Grafana Alerts

```yaml
# Alert for failed jobs
- alert: RenovateJobFailureRate
  expr: rate(renovate_operator_jobs_failed_total[5m]) / rate(renovate_operator_jobs_total[5m]) > 0.2
  for: 10m
  labels:
    severity: warning
  annotations:
    summary: "High Renovate job failure rate"
    description: "Renovate job failure rate is above 20%"
```

### Emergency Procedures

#### Stop All Operations

```bash
# Suspend all Renovators
kubectl get renovators -A -o name | \
  xargs -I {} kubectl patch {} --type='json' -p='[{"op": "replace", "path": "/spec/suspend", "value": true}]'

# Scale down operator
kubectl scale deployment renovate-operator-controller-manager -n renovate-system --replicas=0
```

#### Clean Up Failed Jobs

```bash
# Delete failed jobs
kubectl delete jobs -l app.kubernetes.io/name=renovator-runner --field-selector=status.conditions[0].type=Failed

# Clean up completed jobs
kubectl delete jobs -l app.kubernetes.io/name=renovator-runner --field-selector=status.conditions[0].type=Complete
```

#### Recovery Procedures

```bash
# Restart operator
kubectl rollout restart deployment renovate-operator-controller-manager -n renovate-system

# Re-enable Renovators
kubectl get renovators -A -o name | \
  xargs -I {} kubectl patch {} --type='json' -p='[{"op": "replace", "path": "/spec/suspend", "value": false}]'
```

For additional support, check the [GitHub Issues](https://github.com/thegeeklab/renovate-operator/issues) or join the community discussions.
