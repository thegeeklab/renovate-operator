package runner

import (
	"context"
	"errors"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/frontend"
	"github.com/thegeeklab/renovate-operator/internal/scheduler"
	"github.com/thegeeklab/renovate-operator/pkg/util/reconciler"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/clock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var ErrMaxRepoCount = errors.New("max repo count reached")

type Reconciler struct {
	client.Client
	scheme    *runtime.Scheme
	scheduler *scheduler.Manager
	broker    *frontend.SSEBroker
	req       ctrl.Request
	instance  *renovatev1beta1.Runner
	renovate  *renovatev1beta1.RenovateConfig
}

type JobData struct {
	Repositories []string `json:"repositories"`
}

func NewReconciler(
	c client.Client,
	scheme *runtime.Scheme,
	broker *frontend.SSEBroker,
	instance *renovatev1beta1.Runner,
	renovate *renovatev1beta1.RenovateConfig,
) (*Reconciler, error) {
	return &Reconciler{
		Client:    c,
		scheme:    scheme,
		scheduler: scheduler.NewManager(c, scheme, clock.RealClock{}),
		broker:    broker,
		req:       ctrl.Request{NamespacedName: client.ObjectKey{Namespace: instance.Namespace, Name: instance.Name}},
		instance:  instance,
		renovate:  renovate,
	}, nil
}

func (r *Reconciler) Reconcile(ctx context.Context) (*ctrl.Result, error) {
	results := &reconciler.Results{}

	reconcileFuncs := []func(context.Context) (*ctrl.Result, error){
		r.reconcileJob,
	}

	var reconcileErr error

	for _, reconcileFunc := range reconcileFuncs {
		result, err := reconcileFunc(ctx)
		if err != nil {
			reconcileErr = err

			break
		}

		results.Collect(result)
	}

	if statusErr := r.recordStatus(ctx, reconcileErr); statusErr != nil {
		ctrl.LoggerFrom(ctx).Error(statusErr, "failed to update status")
	}

	if r.broker != nil {
		r.broker.Broadcast("job-updated", "refresh")
	}

	return results.ToResult(), reconcileErr
}

func (r *Reconciler) recordStatus(ctx context.Context, reconcileErr error) error {
	if reconcileErr != nil {
		r.instance.SetCondition(
			renovatev1beta1.ConditionReconciling,
			metav1.ConditionTrue,
			renovatev1beta1.ReasonReconcileError,
			reconcileErr.Error(),
		)
		r.instance.SetCondition(
			renovatev1beta1.ConditionReady,
			metav1.ConditionFalse,
			renovatev1beta1.ReasonReconcileError,
			reconcileErr.Error(),
		)
	} else {
		r.instance.SetCondition(
			renovatev1beta1.ConditionReady,
			metav1.ConditionTrue,
			renovatev1beta1.ReasonReconcileSuccess,
			"Runner reconciled successfully",
		)
		r.instance.RemoveCondition(renovatev1beta1.ConditionReconciling)
	}

	return r.Client.Status().Update(ctx, r.instance)
}
