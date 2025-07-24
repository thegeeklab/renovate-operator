# Installation Guide

This guide covers installation and deployment of the renovate-operator.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Installation Methods](#installation-methods)
- [Development Setup](#development-setup)
- [Verification](#verification)

## Prerequisites

### Kubernetes Requirements

- **Kubernetes Version**: 1.24+ (recommended 1.26+)
- **RBAC**: Enabled (required for operator functionality)
- **Custom Resource Definitions**: Support for CRD v1
- **Admission Controllers**: ValidatingAdmissionWebhook, MutatingAdmissionWebhook (for webhooks)

### Resource Requirements

#### Minimum Requirements

- **CPU**: 200m (operator) + 500m per Renovate job
- **Memory**: 512Mi (operator) + 1Gi per Renovate job
- **Storage**: 10Gi (for logs and temporary data)

#### Recommended Production

- **CPU**: 1 core (operator) + 1 core per Renovate job
- **Memory**: 2Gi (operator) + 2Gi per Renovate job
- **Storage**: 50Gi (persistent logs and caching)

### Network Requirements

- **Outbound HTTPS (443)**: Access to Git platforms (GitHub, Gitea, etc.)
- **Outbound HTTP (80)**: Optional, for insecure connections
- **Inbound HTTP (8080)**: Metrics endpoint
- **Inbound HTTP (9443)**: Webhook endpoint (if using webhooks)

## Installation Methods

### Build from Source

Build and install from the main branch:

```bash
# Clone the repository
git clone https://github.com/thegeeklab/renovate-operator.git
cd renovate-operator

# Install CRDs and deploy operator
make install  # Install CRDs
make deploy   # Deploy operator

# Verify installation
kubectl get pods -n renovate
kubectl get crd | grep renovate
```

### Kustomize

```bash
# Clone the repository
git clone https://github.com/thegeeklab/renovate-operator.git
cd renovate-operator

# Install CRDs
make install

# Deploy operator
make deploy IMG=ghcr.io/thegeeklab/renovate-operator:latest
```

## Development Setup

### Run Outside Cluster

```bash
# Install CRDs
make install

# Create sample configurations
kubectl apply -k config/samples/

# Run the operator locally
make run
```

### Run in Cluster

```bash
# Build and deploy
make docker-build docker-push IMG=dev/renovate-operator:latest
make deploy IMG=dev/renovate-operator:latest

# View logs
kubectl logs -n renovate deployment/renovate-operator-controller-manager -f
```

## Verification

After installation, verify the operator is working:

```bash
# Check operator status
kubectl get pods -n renovate

# Check CRDs are installed
kubectl get crd | grep renovate

# Check operator logs
kubectl logs -n renovate deployment/renovate-operator-controller-manager -f

# Test with a simple Renovator
kubectl apply -f - <<EOF
apiVersion: renovate.thegeeklab.de/v1beta1
kind: Renovator
metadata:
  name: test-renovator
spec:
  schedule: "0 2 * * *"
  renovate:
    platform:
      type: github
      endpoint: https://api.github.com
      token:
        secretKeyRef:
          name: github-token
          key: token
  discovery:
    filter:
      - "test-org/*"
EOF

# Check status
kubectl get renovator test-renovator -o yaml
```
