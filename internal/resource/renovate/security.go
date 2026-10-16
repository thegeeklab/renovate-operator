package renovate

import (
	corev1 "k8s.io/api/core/v1"
)

// hardenedSecurityContext returns the effective container security context.
// Unless privileged pods are explicitly allowed, privileged and privilege
// escalation are forced off, while any other user-provided security settings
// are preserved.
func hardenedSecurityContext(user *corev1.SecurityContext, allowPrivileged bool) *corev1.SecurityContext {
	if allowPrivileged {
		return user
	}

	securityContext := &corev1.SecurityContext{}
	if user != nil {
		securityContext = user.DeepCopy()
	}

	securityContext.Privileged = new(false)
	securityContext.AllowPrivilegeEscalation = new(false)

	return securityContext
}

// filterHostPathVolumes removes hostPath volumes from the given volumes unless
// privileged pods are explicitly allowed.
func filterHostPathVolumes(volumes []corev1.Volume, allowPrivileged bool) []corev1.Volume {
	if allowPrivileged {
		return volumes
	}

	filtered := make([]corev1.Volume, 0, len(volumes))
	for _, volume := range volumes {
		if volume.HostPath != nil {
			continue
		}

		filtered = append(filtered, volume)
	}

	return filtered
}
