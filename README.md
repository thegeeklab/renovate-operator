# renovate-operator

[![Build Status](https://ci.thegeeklab.de/api/badges/thegeeklab/renovate-operator/status.svg)](https://ci.thegeeklab.de/repos/thegeeklab/renovate-operator)
[![Go Report Card](https://goreportcard.com/badge/github.com/thegeeklab/renovate-operator)](https://goreportcard.com/report/github.com/thegeeklab/renovate-operator)
[![GitHub contributors](https://img.shields.io/github/contributors/thegeeklab/renovate-operator)](https://github.com/thegeeklab/renovate-operator/graphs/contributors)
[![License: MIT](https://img.shields.io/github/license/thegeeklab/renovate-operator)](https://github.com/thegeeklab/renovate-operator/blob/main/LICENSE)

A Kubernetes operator for automating [Renovate Bot](https://docs.renovatebot.com/) deployments with advanced parallel processing capabilities. This operator provides automated dependency updates for your repositories.

## ✨ Features

- **🔄 Automated Scheduling**: CronJob-based scheduling with configurable intervals
- **🔍 Repository Discovery**: Automatic discovery of repositories from Git platforms
- **🎯 Platform Support**: GitHub, Gitea, and other Git platforms

> **WARNING:** This project is still in development and is not yet ready for production use.

## Quick Start

### Prerequisites

- Kubernetes cluster (1.24+)
- kubectl configured
- Git platform credentials (GitHub, Gitea, etc.)

### Installation

**Build and push your image to the location specified by `IMG`:**

```Shell
make docker-build docker-push IMG=<some-registry>/renovate-operator:tag
```

> **NOTE:** This image ought to be published in the personal registry you specified.
> And it is required to have access to pull the image from the working environment.
> Make sure you have the proper permission to the registry if the above commands don’t work.

**Install the CRDs into the cluster:**

```Shell
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```Shell
make deploy IMG=<some-registry>/renovate-operator:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
> privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```Shell
kubectl apply -k config/samples/
```

> **NOTE**: Ensure that the samples has default values to test it out.

### Uninstall

**Delete the instances (CRs) from the cluster:**

```Shell
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```Shell
make uninstall
```

**UnDeploy the controller from the cluster:**

```Shell
make undeploy
```

## Project Distribution

Following the options to release and provide this solution to the users.

### By providing a bundle with all YAML files

1. Build the installer for the image built and published in the registry:

```Shell
make build-installer IMG=<some-registry>/renovate-operator:tag
```

> **NOTE:** The makefile target mentioned above generates an 'install.yaml'
> file in the dist directory. This file contains all the resources built
> with Kustomize, which are necessary to install this project without its
> dependencies.

1. Using the installer

Users can just run `kubectl apply -f <url-for-yaml-bundle>` to install
the project, i.e.:

```Shell
kubectl apply -f https://raw.githubusercontent.com/<org>/renovate-operator/<tag or branch>/dist/install.yaml
```

### By providing a Helm Chart

1. Build the chart using the optional helm plugin

```Shell
kubebuilder edit --plugins=helm/v1-alpha
```

1. See that a chart was generated under 'dist/chart', and users
   can obtain this solution from there.

> **NOTE:** If you change the project, you need to update the Helm Chart
> using the same command above to sync the latest changes. Furthermore,
> if you create webhooks, you need to use the above command with
> the '--force' flag and manually ensure that any custom configuration
> previously added to 'dist/chart/values.yaml' or 'dist/chart/manager/manager.yaml'
> is manually re-applied afterwards.

## Contributors

Special thanks to all [contributors](https://github.com/thegeeklab/renovate-operator/graphs/contributors). If you would like to contribute, please see the [instructions](https://github.com/thegeeklab/renovate-operator/blob/main/CONTRIBUTING.md).

This project is heavily inspired by [secustor/renovate-operator](https://github.com/secustor/renovate-operator/tree/master) from Sebastian Poxhofer. Thanks for your work.

## License

This project is licensed under the MIT License - see the [LICENSE](https://github.com/thegeeklab/renovate-operator/blob/main/LICENSE) file for details.
