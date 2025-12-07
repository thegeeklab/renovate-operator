package v1beta1

import corev1 "k8s.io/api/core/v1"

func (r *Renovator) Default() {
	if r.Spec.Logging.Level == "" {
		r.Spec.Logging.Level = LogLevel_INFO
	}

	if r.Spec.Runner.Strategy == "" {
		r.Spec.Runner.Strategy = "none"
	}

	if r.Spec.Runner.Instances == 0 {
		r.Spec.Runner.Instances = 1
	}

	if r.Spec.Discovery.Schedule == "" {
		r.Spec.Discovery.Schedule = "0 */2 * * *"
	}

	if r.Spec.Image == "" {
		r.Spec.Image = OperatorContainerImage
	}

	if r.Spec.ImagePullPolicy == "" {
		r.Spec.ImagePullPolicy = corev1.PullIfNotPresent
	}

	if r.Spec.Renovate.Image == "" {
		r.Spec.Renovate.Image = RenovateContainerImage
	}

	if r.Spec.Renovate.ImagePullPolicy == "" {
		r.Spec.Renovate.ImagePullPolicy = corev1.PullIfNotPresent
	}
}
