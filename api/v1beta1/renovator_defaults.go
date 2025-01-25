package v1beta1

func (r *Renovator) Default() {
	if r.Spec.Logging.Level == "" {
		r.Spec.Logging.Level = "info"
	}

	if r.Spec.Worker.Strategy == "" {
		r.Spec.Worker.Strategy = "none"
	}

	if r.Spec.Worker.Instances == 0 {
		r.Spec.Worker.Instances = 1
	}

	if r.Spec.Discovery.Schedule == "" {
		r.Spec.Discovery.Schedule = "0 */2 * * *"
	}

	if r.Spec.Image == "" {
		r.Spec.Image = "docker.io/thegeeklab/renovate-operator:latest"
	}

	if r.Spec.ImagePullPolicy == "" {
		r.Spec.ImagePullPolicy = "IfNotPresent"
	}

	if r.Spec.Renovate.Image == "" {
		r.Spec.Renovate.Image = "ghcr.io/renovatebot/renovate"
	}

	if r.Spec.Renovate.ImagePullPolicy == "" {
		r.Spec.Renovate.ImagePullPolicy = "IfNotPresent"
	}
}
