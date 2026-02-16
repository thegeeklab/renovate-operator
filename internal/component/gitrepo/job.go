package gitrepo

import (
	"context"

	"github.com/thegeeklab/renovate-operator/internal/component/renovator"
	"github.com/thegeeklab/renovate-operator/internal/component/runner"
	"github.com/thegeeklab/renovate-operator/internal/metadata"
	cronjob "github.com/thegeeklab/renovate-operator/internal/resource/cronjob"
	"github.com/thegeeklab/renovate-operator/internal/resource/renovate"
	"github.com/thegeeklab/renovate-operator/pkg/util/k8s"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *Reconciler) reconcileBatchJob(ctx context.Context) (*ctrl.Result, error) {
	log := logf.FromContext(ctx)

	if !renovator.HasRenovatorOperationRenovate(r.instance.Annotations) {
		return &ctrl.Result{}, nil
	}

	// Check for active renovate jobs with our specific labels
	active, err := cronjob.CheckActiveJobs(ctx, r.Client, r.instance.Namespace, runner.RunnerName(r.req))
	if err != nil {
		return &ctrl.Result{}, err
	}

	if active {
		log.V(1).Info("Active renovate jobs found, requeuing", "delay", cronjob.RequeueDelay)

		return &ctrl.Result{RequeueAfter: cronjob.RequeueDelay}, nil
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: runner.RunnerName(r.req) + "-",
			Namespace:    r.instance.Namespace,
		},
		Spec: batchv1.JobSpec{},
	}

	r.updateJobSpec(&job.Spec)

	if _, err := k8s.CreateOrUpdate(ctx, r.Client, job, r.instance, nil); err != nil {
		return &ctrl.Result{}, err
	}

	// Remove renovate annotation
	r.instance.Annotations = renovator.RemoveRenovatorOperation(r.instance.Annotations)
	if err := r.Update(ctx, r.instance); err != nil {
		return &ctrl.Result{}, err
	}

	return &ctrl.Result{}, nil
}

func (r *Reconciler) updateJobSpec(spec *batchv1.JobSpec) {
	renovateConfigCM := metadata.GenericName(r.req, renovator.ConfigMapSuffix)

	if spec == nil {
		spec = &batchv1.JobSpec{}
	}

	renovate.DefaultJobSpec(
		spec,
		r.renovate,
		renovateConfigCM,
		renovate.WithSingleRepoMode(r.instance.Name),
	)
}
