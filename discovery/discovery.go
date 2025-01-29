package discovery

import (
	"fmt"
	"os"
)

type Config struct {
	Name           string
	Namespace      string
	FilePath       string
	KubeConfigPath string
}

const (
	EnvRenovatorInstanceName      = "RENOVATOR_INSTANCE_NAME"
	EnvRenovatorInstanceNamespace = "RENOVATOR_INSTANCE_NAMESPACE"
	EnvRenovateOutputFile         = "RENOVATE_OUTPUT_FILE"
	EnvKubeConfig                 = "KUBECONFIG"
)

func LoadConfig() (*Config, error) {
	cfg := &Config{}

	var err error
	if cfg.Name, err = setEnvVariable(EnvRenovatorInstanceName); err != nil {
		return cfg, err
	}

	if cfg.Namespace, err = setEnvVariable(EnvRenovatorInstanceNamespace); err != nil {
		return cfg, err
	}

	if cfg.FilePath, err = setEnvVariable(EnvRenovateOutputFile); err != nil {
		return cfg, err
	}

	cfg.KubeConfigPath, _ = setEnvVariable(EnvKubeConfig)

	return cfg, nil
}

func setEnvVariable(envVariable string) (string, error) {
	if value, isSet := os.LookupEnv(envVariable); isSet {
		return value, nil
	}

	return "", fmt.Errorf("environment variable %s is not defined", envVariable)
}
