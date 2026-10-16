package runner

import (
	"context"
	"errors"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/frontend"
	"github.com/thegeeklab/renovate-operator/internal/logreader"
	"github.com/thegeeklab/renovate-operator/internal/metrics"
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
	scheme          *runtime.Scheme
	scheduler       *scheduler.Manager
	broker          *frontend.SSEBroker
	req             ctrl.Request
	instance        *renovatev1beta1.Runner
	renovate        *renovatev1beta1.RenovateConfig
	metrics         metrics.Recorder
	logReader       logreader.Reader
	eventRecorder   events.EventRecorder
	allowPrivileged bool
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
	metricsRecorder metrics.Recorder,
	logReader logreader.Reader,
	eventRecorder events.EventRecorder,
	allowPrivileged bool,
) (*Reconciler, error) {
	return &Reconciler{
		Client:          c,
		scheme:          scheme,
		scheduler:       scheduler.NewManager(c, scheme, clock.RealClock{}),
		broker:          broker,
		req:             ctrl.Request{NamespacedName: client.ObjectKey{Namespace: instance.Namespace, Name: instance.Name}},
		instance:        instance,
		renovate:        renovate,
		metrics:         metricsRecorder,
		logReader:       logReader,
		eventRecorder:   eventRecorder,
		allowPrivileged: allowPrivileged,
	}, nil
}

func (r *Reconciler) Reconcile(ctx context.Context) (*ctrl.Result, error) {
	results := &reconciler.Results{}

	reconcileFuncs := []func(context.Context) (*ctrl.Result, error){
		r.reconcileMetrics,
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

	if r.broker != nil && r.broker.ClientCount() > 0 {
		r.broker.Broadcast("job-updated", "refresh")
	}

	return results.ToResult(), reconcileErr
}
