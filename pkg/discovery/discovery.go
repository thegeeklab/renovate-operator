package discovery

import (
	"errors"
	"fmt"

	"github.com/thegeeklab/renovate-operator/pkg/util"
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

	KubeClient client.Client
	Scheme     *runtime.Scheme

	kubeconfig *rest.Config
}

const (
	EnvDiscoveryInstanceName      = "DISCOVERY_INSTANCE_NAME"
	EnvDiscoveryInstanceNamespace = "DISCOVERY_INSTANCE_NAMESPACE"
	EnvRenovateOutputFile         = "RENOVATE_OUTPUT_FILE"
	EnvKubeconfig                 = "KUBECONFIG"
)

var ErrDiscoveryClient = errors.New("failed to create discovery client")

func New(scheme *runtime.Scheme) (*Discovery, error) {
	d := &Discovery{
		Scheme: scheme,
	}

	var err error
	if d.Name, err = util.ParseEnv(EnvDiscoveryInstanceName); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDiscoveryClient, err)
	}

	if d.Namespace, err = util.ParseEnv(EnvDiscoveryInstanceNamespace); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDiscoveryClient, err)
	}

	if d.FilePath, err = util.ParseEnv(EnvRenovateOutputFile); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDiscoveryClient, err)
	}

	d.KubeconfigPath, _ = util.ParseEnv(EnvKubeconfig)

	if err := d.getKubeconfig(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDiscoveryClient, err)
	}

	if err := d.getKubernetesClient(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDiscoveryClient, err)
	}

	return d, nil
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

	d.KubeClient = cl

	return nil
}
