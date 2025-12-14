package v1beta1

import corev1 "k8s.io/api/core/v1"

func (r *RenovateConfig) Default() {
	if r.Spec.Logging == nil {
		r.Spec.Logging = &LoggingSpec{}
	}

	if r.Spec.Logging.Level == "" {
		r.Spec.Logging.Level = LogLevel_INFO
	}

	if r.Spec.Image == "" {
		r.Spec.Image = RenovateContainerImage
	}

	if r.Spec.ImagePullPolicy == "" {
		r.Spec.ImagePullPolicy = corev1.PullIfNotPresent
	}
}
