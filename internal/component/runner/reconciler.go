package runner

import (
	"context"
	"errors"
	"fmt"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/frontend"
	"github.com/thegeeklab/renovate-operator/internal/scheduler"
	"github.com/thegeeklab/renovate-operator/pkg/util/reconciler"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	"k8s.io/utils/clock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var ErrMaxRepoCount = errors.New("max repo count reached")

type Reconciler struct {
	client.Client
	scheme        *runtime.Scheme
	scheduler     *scheduler.Manager
	broker        *frontend.SSEBroker
	eventRecorder events.EventRecorder
	req           ctrl.Request
	instance      *renovatev1beta1.Runner
	renovate      *renovatev1beta1.RenovateConfig
}

type JobData struct {
	Repositories []string `json:"repositories"`
}

func NewReconciler(
	c client.Client,
	scheme *runtime.Scheme,
	broker *frontend.SSEBroker,
	recorder events.EventRecorder,
	instance *renovatev1beta1.Runner,
	renovate *renovatev1beta1.RenovateConfig,
) (*Reconciler, error) {
	return &Reconciler{
		Client:        c,
		scheme:        scheme,
		scheduler:     scheduler.NewManager(c, scheme, clock.RealClock{}),
		broker:        broker,
		eventRecorder: recorder,
		req:           ctrl.Request{NamespacedName: client.ObjectKey{Namespace: instance.Namespace, Name: instance.Name}},
		instance:      instance,
		renovate:      renovate,
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

	conditionType := renovatev1beta1.ConditionReady
	reason := renovatev1beta1.ReasonReconcileSuccess
	message := "Runner reconciled successfully"
	eventType := renovatev1beta1.EventTypeNormal

	if reconcileErr != nil {
		conditionType = renovatev1beta1.ConditionReconcileError
		reason = renovatev1beta1.ReasonReconcileError
		message = reconcileErr.Error()
		eventType = renovatev1beta1.EventTypeWarning
	}

	r.instance.SetCondition(conditionType, "True", reason, message)
	r.eventRecorder.Eventf(r.instance, nil, eventType, reason, reason, "%s", message)

	if err := r.Client.Status().Update(ctx, r.instance); err != nil {
		return &ctrl.Result{}, fmt.Errorf("failed to update status: %w", err)
	}

	if r.broker != nil {
		r.broker.Broadcast("job-updated", "refresh")
	}

	return results.ToResult(), reconcileErr
}
