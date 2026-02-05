package scheduler

import (
	"context"

	"github.com/thegeeklab/renovate-operator/internal/component/renovator"
	"github.com/thegeeklab/renovate-operator/internal/metadata"
	cronjob "github.com/thegeeklab/renovate-operator/internal/resource/cronjob"
	"github.com/thegeeklab/renovate-operator/internal/resource/renovate"
	"github.com/thegeeklab/renovate-operator/pkg/util/k8s"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *Reconciler) reconcileCronJob(ctx context.Context) (*ctrl.Result, error) {
	// Check if immediate renovate is requested via annotation
	if renovator.HasRenovatorOperationRenovate(r.instance.Annotations) {
		return r.handleImmediateRenovate(ctx)
	}

	job := &batchv1.CronJob{ObjectMeta: SchedulerMetadata(r.req)}

	op, err := k8s.CreateOrUpdate(ctx, r.Client, job, r.instance, func() error {
		return r.updateCronJob(job)
	})
	if err != nil {
		return &ctrl.Result{}, err
	}

	if op == controllerutil.OperationResultUpdated {
		if err := cronjob.DeleteOwnedJobs(ctx, r.Client, job); err != nil {
			return &ctrl.Result{}, err
		}
	}

	return &ctrl.Result{}, nil
}

func (r *Reconciler) handleImmediateRenovate(ctx context.Context) (*ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Check for active renovate jobs with our specific labels
	active, err := cronjob.CheckActiveJobs(ctx, r.Client, r.instance.Namespace, SchedulerName(r.req))
	if err != nil {
		return &ctrl.Result{}, err
	}

	if active {
		log.V(1).Info("Active renovate jobs found, requeuing", "delay", cronjob.RequeueDelay)

		return &ctrl.Result{RequeueAfter: cronjob.RequeueDelay}, nil
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: SchedulerName(r.req) + "-",
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

func (r *Reconciler) updateCronJob(job *batchv1.CronJob) error {
	job.Spec.Schedule = r.instance.Spec.Schedule
	job.Spec.ConcurrencyPolicy = batchv1.ForbidConcurrent
	job.Spec.Suspend = r.instance.Spec.Suspend

	r.updateJobSpec(&job.Spec.JobTemplate.Spec)

	return nil
}

func (r *Reconciler) updateJobSpec(spec *batchv1.JobSpec) {
	renovateConfigCM := metadata.GenericName(r.req, renovator.ConfigMapSuffix)
	renovateBatchesCM := metadata.GenericName(r.req, ConfigMapSuffix)

	// Apply the Spec with the "Batch Mode" option
	renovate.DefaultJobSpec(
		spec,
		r.renovate,
		renovateConfigCM,
		renovate.WithBatchMode(r.instance, renovateBatchesCM, r.batchesCount),
	)
}
