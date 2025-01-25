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
}
