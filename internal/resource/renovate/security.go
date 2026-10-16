package renovate

import (
	corev1 "k8s.io/api/core/v1"
)

// ApplySecurityHardening hardens the given pod spec against privileged
// execution. Unless allowPrivileged is true, hostPath volumes are removed and
// privileged/privilege-escalation are forced off on every container (init and
// app). It reports whether a user-provided privileged setting or hostPath
// volume was overridden. Call this after all containers and volumes have been
// assembled.
func ApplySecurityHardening(podSpec *corev1.PodSpec, allowPrivileged bool) bool {
	if allowPrivileged {
		return false
	}

	restricted := false

	filtered := filterHostPathVolumes(podSpec.Volumes)
	if len(filtered) != len(podSpec.Volumes) {
		restricted = true
	}

	podSpec.Volumes = filtered

	for i := range podSpec.InitContainers {
		securityContext, changed := hardenedSecurityContext(podSpec.InitContainers[i].SecurityContext)
		if changed {
			restricted = true
		}

		podSpec.InitContainers[i].SecurityContext = securityContext
	}

	for i := range podSpec.Containers {
		securityContext, changed := hardenedSecurityContext(podSpec.Containers[i].SecurityContext)
		if changed {
			restricted = true
		}

		podSpec.Containers[i].SecurityContext = securityContext
	}

	return restricted
}

// hardenedSecurityContext returns the effective container security context with
// privileged and privilege escalation forced off, preserving any other
// user-provided settings. It reports whether a user-requested privileged or
// privilege-escalation flag was overridden.
func hardenedSecurityContext(user *corev1.SecurityContext) (*corev1.SecurityContext, bool) {
	securityContext := &corev1.SecurityContext{}
	if user != nil {
		securityContext = user.DeepCopy()
	}

	changed := (securityContext.Privileged != nil && *securityContext.Privileged) ||
		(securityContext.AllowPrivilegeEscalation != nil && *securityContext.AllowPrivilegeEscalation)

	securityContext.Privileged = new(false)
	securityContext.AllowPrivilegeEscalation = new(false)

	return securityContext, changed
}

// filterHostPathVolumes removes hostPath volumes from the given volumes.
func filterHostPathVolumes(volumes []corev1.Volume) []corev1.Volume {
	filtered := make([]corev1.Volume, 0, len(volumes))

	for _, volume := range volumes {
		if volume.HostPath != nil {
			continue
		}

		filtered = append(filtered, volume)
	}

	return filtered
}
