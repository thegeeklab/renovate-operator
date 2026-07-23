# renovate-operator

[![Build Status](https://ci.thegeeklab.de/api/badges/thegeeklab/renovate-operator/status.svg)](https://ci.thegeeklab.de/repos/thegeeklab/renovate-operator)
[![GitHub contributors](https://img.shields.io/github/contributors/thegeeklab/renovate-operator)](https://github.com/thegeeklab/renovate-operator/graphs/contributors)
[![License: MIT](https://img.shields.io/github/license/thegeeklab/renovate-operator)](https://github.com/thegeeklab/renovate-operator/blob/main/LICENSE)

A Kubernetes operator for automating [Renovate Bot](https://docs.renovatebot.com/) deployments. This operator manages Renovate runs across repositories discovered from Git platforms, with a built-in web dashboard for monitoring.

<!-- markdownlint-disable MD033 -->
<p align="center">
  <img src="https://raw.githubusercontent.com/thegeeklab/renovate-operator/main/images/dashboard.png" alt="Dashboard" width="45%" />
  &nbsp;
  <img src="https://raw.githubusercontent.com/thegeeklab/renovate-operator/main/images/gitrepo.png" alt="GitRepo View" width="45%" />
</p>
<!-- markdownlint-enable MD033 -->

## Features

- **Automated Scheduling**: Cron-based scheduling for discovery and Renovate runs
- **Repository Discovery**: Automatic discovery of repositories from Git platforms
- **Per-Repository Jobs**: One Kubernetes Job per repository, all running concurrently
- **Web Dashboard**: Real-time monitoring with Server-Sent Events, job log viewer
- **OAuth2 Login**: Secure web UI access via platform OIDC
- **Webhook Triggers**: Trigger Renovate runs from platform webhook events

### Supported Platforms

- [x] Renovate platform
  - [x] Gitea
  - [x] GitHub
  - [ ] Forgejo
  - [x] GitLab
- [x] OAuth2 authentication
  - [x] Gitea
  - [x] GitHub
  - [ ] Forgejo
  - [x] GitLab

## Quick Start

### Prerequisites

- Kubernetes cluster (1.24+)
- `kubectl` configured to access your cluster
- Access token of a Git platform

### Installation

#### Helm (OCI)

Container images are published to:

- **Docker Hub**: `docker.io/thegeeklab/renovate-operator`
- **Quay.io**: `quay.io/thegeeklab/renovate-operator`

The Helm chart is published to:

- **Docker Hub**: `oci://docker.io/thegeeklab/renovate-operator-chart`
- **Quay.io**: `oci://quay.io/thegeeklab/renovate-operator-chart`

Install using Quay.io:

```bash
helm install renovate-operator oci://quay.io/thegeeklab/renovate-operator-chart \
  --namespace renovate-system --create-namespace
```

#### Static Manifest

Download `install.yaml` from the [GitHub Releases](https://github.com/thegeeklab/renovate-operator/releases) page, then:

```bash
kubectl apply --server-side --force-conflicts -f install.yaml
```

### Create Your First Renovator

1. **Create a Gitea token secret**:

   ```bash
   kubectl create secret generic gitea-token \
     --from-literal=token=your_gitea_token_here \
     --namespace renovate-system
   ```

1. **Create a Renovator resource**:

   ```yaml
   apiVersion: renovate.thegeeklab.de/v1beta1
   kind: Renovator
   metadata:
     name: my-renovator
     namespace: renovate-system
   spec:
     schedule: "0 2 * * *"

     renovate:
       platform:
         type: gitea
         endpoint: https://gitea.example.com
         token:
           secretKeyRef:
             name: gitea-token
             key: token

     discovery:
       schedule: "0 */2 * * *"
       filter:
         - "your-org/*"
         - "!your-org/archived-*"

     runner:
       schedule: "0 3 * * *"
   ```

   See [config/samples/](https://github.com/thegeeklab/renovate-operator/tree/main/config/samples) for a full example with all available options.

   ```bash
   kubectl apply -f renovator.yaml
   ```

1. **Verify**:

   ```bash
   kubectl get renovator my-renovator -n renovate-system
   kubectl get gitrepos -n renovate-system
   kubectl logs -n renovate-system deployment/renovate-operator-controller-manager
   ```

### Access the Web UI

The frontend is served by the manager pod on port `8082` by default (configurable via `--frontend-bind-address`). There is no dedicated Kubernetes Service for the frontend in the default installation, so use port-forward directly to the pod:

```bash
kubectl port-forward -n renovate-system \
  deployment/renovate-operator-controller-manager 8082:8082
```

Then open `<http://localhost:8082>` in your browser.

## Uninstallation

> **WARNING:** `GitRepo` and `AuthProvider` resources have finalizers, so the operator must be running while they are deleted. `GitRepo` resources also call the Git platform API to deregister webhooks. Deleting them with the operator unreachable can block deletion and leave orphaned webhooks registered on the platform.

Delete resources in this order:

```bash
# 1. Delete GitRepo resources and wait for finalizers to clear
kubectl delete gitrepos --all -n renovate-system
kubectl wait gitrepos --all --for=delete --timeout=120s -n renovate-system

# 2. Delete AuthProvider resources and wait for finalizers to clear
kubectl delete authprovider --all -n renovate-system
kubectl wait authprovider --all --for=delete --timeout=120s -n renovate-system

# 3. Delete Renovator instances
kubectl delete renovator --all -n renovate-system

# 4. Remove the operator
# If using Helm
helm uninstall renovate-operator -n renovate-system

# If using static manifest
kubectl delete -f install.yaml
```

## Contributors

Special thanks to all [contributors](https://github.com/thegeeklab/renovate-operator/graphs/contributors). If you would like to contribute, please see the [instructions](https://github.com/thegeeklab/renovate-operator/blob/main/CONTRIBUTING.md).

This project is heavily inspired by [secustor/renovate-operator](https://github.com/secustor/renovate-operator/tree/master) from Sebastian Poxhofer.

## License

This project is licensed under the MIT License - see the [LICENSE](https://github.com/thegeeklab/renovate-operator/blob/main/LICENSE) file for details.
