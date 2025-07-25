# renovate-operator

[![Build Status](https://ci.thegeeklab.de/api/badges/thegeeklab/renovate-operator/status.svg)](https://ci.thegeeklab.de/repos/thegeeklab/renovate-operator)
[![Go Report Card](https://goreportcard.com/badge/github.com/thegeeklab/renovate-operator)](https://goreportcard.com/report/github.com/thegeeklab/renovate-operator)
[![GitHub contributors](https://img.shields.io/github/contributors/thegeeklab/renovate-operator)](https://github.com/thegeeklab/renovate-operator/graphs/contributors)
[![License: MIT](https://img.shields.io/github/license/thegeeklab/renovate-operator)](https://github.com/thegeeklab/renovate-operator/blob/main/LICENSE)

A Kubernetes operator for automating [Renovate Bot](https://docs.renovatebot.com/) deployments with advanced parallel processing capabilities. This operator provides automated dependency updates for your repositories with intelligent batching and resource management.

## ✨ Features

- **🚀 Parallel Processing**: Intelligent repository batching for faster dependency updates
- **🔄 Automated Scheduling**: CronJob-based scheduling with configurable intervals
- **🔍 Repository Discovery**: Automatic discovery of repositories from Git platforms
- **🎯 Platform Support**: GitHub, Gitea, and more Git platforms
- **📊 Resource Management**: Efficient Kubernetes resource utilization
- **🛡️ Security**: Secure credential management with Kubernetes secrets
- **📈 Scalability**: Handle hundreds of repositories efficiently

> **WARNING:** This project is still in development and is not yet ready for production use.

## Quick Start

### Prerequisites

- Kubernetes cluster (1.24+)
- kubectl configured
- Git platform credentials (GitHub, Gitea, etc.)

### Installation

1. **Install the operator:**

   ```bash
   # Clone and build from source
   git clone https://github.com/thegeeklab/renovate-operator.git
   cd renovate-operator
   make install  # Install CRDs
   make deploy   # Deploy operator
   ```

2. **Create a secret with your platform credentials:**

   ```bash
   kubectl create secret generic renovate-secret \
     --from-literal=token=<your-github-token>
   ```

3. **Create your first Renovator:**

   ```yaml
   apiVersion: renovate.thegeeklab.de/v1beta1
   kind: Renovator
   metadata:
     name: my-renovator
   spec:
     schedule: "0 2 * * *" # Daily at 2 AM
     runner:
       strategy: batch # Enable parallel processing
       instances: 3 # Run 3 parallel workers
     renovate:
       platform:
         type: github
         endpoint: https://api.github.com
         token:
           secretKeyRef:
             name: renovate-secret
             key: token
     discovery:
       filter:
         - "octocat/*" # Discover all repos in 'octocat'
   ```

4. **Apply the configuration:**

   ```bash
   kubectl apply -f renovator.yaml
   ```

## 📚 Documentation

- **[Installation Guide](docs/installation.md)** - How to install and deploy the operator
- **[Configuration Guide](docs/configuration.md)** - Complete configuration reference
- **[API Reference](docs/api-reference.md)** - Complete API documentation
- **[Architecture Overview](docs/architecture.md)** - Understanding the operator components

## 🏗️ Development

### Building from Source

1. **Clone the repository:**

   ```bash
   git clone https://github.com/thegeeklab/renovate-operator.git
   cd renovate-operator
   ```

2. **Build and push your image:**

   ```bash
   make docker-build docker-push IMG=<some-registry>/renovate-operator:tag
   ```

3. **Deploy to cluster:**

   ```bash
   make install  # Install CRDs
   make deploy IMG=<some-registry>/renovate-operator:tag
   ```

4. **Create test instances:**

   ```bash
   kubectl apply -k config/samples/
   ```

### Running Tests

```bash
# Run unit tests
make test

# Run integration tests
make test-integration

# Run end-to-end tests
make test-e2e
```

### Local Development

```bash
# Install dependencies
make install

# Run locally (outside cluster)
make run

# Generate code and manifests
make generate
make manifests
```

## 🔧 Configuration Examples

### Basic Configuration

```yaml
apiVersion: renovate.thegeeklab.de/v1beta1
kind: Renovator
metadata:
  name: basic-renovator
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
```

## Contributors

Special thanks to all [contributors](https://github.com/thegeeklab/renovate-operator/graphs/contributors). If you would like to contribute, please see the [instructions](https://github.com/thegeeklab/renovate-operator/blob/main/CONTRIBUTING.md).

This project is heavily inspired by [secustor/renovate-operator](https://github.com/secustor/renovate-operator/tree/master) from Sebastian Poxhofer. Thanks for your work.

## License

This project is licensed under the MIT License - see the [LICENSE](https://github.com/thegeeklab/renovate-operator/blob/main/LICENSE) file for details.
