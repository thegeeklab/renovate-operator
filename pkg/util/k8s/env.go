package k8s

import corev1 "k8s.io/api/core/v1"

// EnvVarExists reports whether an environment variable with the given name is present.
func EnvVarExists(env []corev1.EnvVar, name string) bool {
	for _, e := range env {
		if e.Name == name {
			return true
		}
	}

	return false
}
