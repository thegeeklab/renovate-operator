# renovate-operator

// TODO(user): Add simple overview of use/purpose

## Description

// TODO(user): An in-depth paragraph about your project and overview of use

## Getting Started

### Prerequisites

- go version v1.23.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### To Deploy on the cluster

**Build and push your image to the location specified by `IMG`:**

```Shell
make docker-build docker-push IMG=<some-registry>/renovate-operator:tag
```

**NOTE:** This image ought to be published in the personal registry you specified.
And it is required to have access to pull the image from the working environment.
Make sure you have the proper permission to the registry if the above commands don’t work.

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

### To Uninstall

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

**NOTE:** The makefile target mentioned above generates an 'install.yaml'
file in the dist directory. This file contains all the resources built
with Kustomize, which are necessary to install this project without its
dependencies.

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

**NOTE:** If you change the project, you need to update the Helm Chart
using the same command above to sync the latest changes. Furthermore,
if you create webhooks, you need to use the above command with
the '--force' flag and manually ensure that any custom configuration
previously added to 'dist/chart/values.yaml' or 'dist/chart/manager/manager.yaml'
is manually re-applied afterwards.

## Contributing

// TODO(user): Add detailed information on how you would like others to contribute to this project

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

This project is licensed under the MIT License - see the [LICENSE](https://github.com/thegeeklab/renovate-operator/blob/main/LICENSE) file for details.
