package util

import (
	"fmt"
	"os"
)

var ErrEnvVarNotDefined = fmt.Errorf("environment variable not defined")

func ParseEnv(envVariable string) (string, error) {
	if value, isSet := os.LookupEnv(envVariable); isSet {
		return value, nil
	}

	return "", fmt.Errorf("%w: %s", ErrEnvVarNotDefined, envVariable)
}
