package discovery

import (
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Discovery struct {
	Name           string
	Namespace      string
	FilePath       string
	KubeconfigPath string

	Client client.Client
	Scheme *runtime.Scheme

	kubeconfig *rest.Config
}

const (
	EnvRenovatorInstanceName      = "RENOVATOR_INSTANCE_NAME"
	EnvRenovatorInstanceNamespace = "RENOVATOR_INSTANCE_NAMESPACE"
	EnvRenovateOutputFile         = "RENOVATE_OUTPUT_FILE"
	EnvKubeconfig                 = "KUBECONFIG"
)

var (
	ErrDiscoveryClient  = fmt.Errorf("failed to create discovery client")
	ErrEnvVarNotDefined = fmt.Errorf("environment variable not defined")
)

func New(scheme *runtime.Scheme) (*Discovery, error) {
	d := &Discovery{
		Scheme: scheme,
	}

	var err error
	if d.Name, err = parseEnv(EnvRenovatorInstanceName); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDiscoveryClient, err)
	}

	if d.Namespace, err = parseEnv(EnvRenovatorInstanceNamespace); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDiscoveryClient, err)
	}

	if d.FilePath, err = parseEnv(EnvRenovateOutputFile); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDiscoveryClient, err)
	}

	d.KubeconfigPath, _ = parseEnv(EnvKubeconfig)

	if err := d.getKubeconfig(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDiscoveryClient, err)
	}

	if err := d.getKubernetesClient(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDiscoveryClient, err)
	}

	return d, nil
}

func parseEnv(envVariable string) (string, error) {
	if value, isSet := os.LookupEnv(envVariable); isSet {
		return value, nil
	}

	return "", fmt.Errorf("%w: %s", ErrEnvVarNotDefined, envVariable)
}

func (d *Discovery) getKubeconfig() error {
	kc, err := rest.InClusterConfig()
	if err != nil {
		if d.KubeconfigPath == "" {
			return err
		}

		kc, err = clientcmd.BuildConfigFromFlags("", d.KubeconfigPath)
		if err != nil {
			return err
		}
	}

	d.kubeconfig = kc

	return nil
}

func (d *Discovery) getKubernetesClient() error {
	cl, err := client.New(d.kubeconfig, client.Options{Scheme: d.Scheme})
	if err != nil {
		return err
	}

	d.Client = cl

	return nil
}
