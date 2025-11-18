package util

import (
	"errors"
	"fmt"
	"os"
)

var ErrEnvVarNotDefined = errors.New("environment variable not defined")

func ParseEnv(envVariable string) (string, error) {
	if value, isSet := os.LookupEnv(envVariable); isSet {
		return value, nil
	}

	return "", fmt.Errorf("%w: %s", ErrEnvVarNotDefined, envVariable)
}
