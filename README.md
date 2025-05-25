# renovate-operator

This project is a Kubernetes operator for automating [Renovate Bot](https://docs.renovatebot.com/) deployments. Renovate is a popular dependency update tool, and this operator makes it easier to manage Renovate instances in a Kubernetes environment.

> **WARNING:** This project is still in development and is not yet ready for production use.

## Getting Started

### Prerequisites

**cert-manager**

This operator requires [cert-manager](https://cert-manager.io/) to be installed in your cluster for webhook certificate management. The webhooks are used for validating and mutating the custom resources.

To install cert-manager:
```Shell
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.16.2/cert-manager.yaml
```

Wait for cert-manager to be ready:
```Shell
kubectl wait --for=condition=Available --timeout=300s deployment --all -n cert-manager
```

> **NOTE:** For production environments, please refer to the [official cert-manager documentation](https://cert-manager.io/docs/installation/) for recommended installation methods and configuration options.

### To Deploy on the cluster

**Build and push your image to the location specified by `IMG`:**

```Shell
make docker-build docker-push IMG=<some-registry>/renovate-operator:tag
```

> **NOTE:** This image ought to be published in the personal registry you specified.
> And it is required to have access to pull the image from the working environment.
> Make sure you have the proper permission to the registry if the above commands don't work.

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

## Custom Resources

### Renovator

The main custom resource that defines a Renovate instance configuration. It manages:
- Discovery of repositories via scheduled jobs
- Configuration for the Renovate bot
- Runner settings for parallel processing

### GitRepo

Represents a git repository to be processed by Renovate. These are typically created by the discovery process.

### RenovatorJob

Manages the execution of Renovate on batches of repositories. Features:
- Groups repositories for batch processing
- Tracks processing status (Pending, Running, Succeeded, Failed)
- References the Kubernetes Job that runs the actual Renovate process
- Supports parallel execution with configurable limits

The operator automatically creates RenovatorJob CRs based on:
- The runner configuration in the Renovator CR
- Available GitRepo resources
- Configured parallelism limits

Example RenovatorJob:
```yaml
apiVersion: renovate.thegeeklab.de/v1beta1
kind: RenovatorJob
metadata:
  name: renovator-batch-0
spec:
  renovatorName: my-renovator
  repositories:
    - "myorg/repo1"
    - "myorg/repo2"
  batchId: "batch-0"
  priority: 0
  jobSpec:
    template:
      spec:
        # Pod specification for running Renovate
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
