package controller

import (
	"context"
	"errors"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var ErrRenovateConfigNotFound = errors.New("RenovateConfig not found")

type ConditionSetter interface {
	SetCondition(conditionType string, status metav1.ConditionStatus, reason, message string)
	client.Object
}

func RecordError(
	ctx context.Context,
	c client.StatusClient,
	obj ConditionSetter,
	recorder events.EventRecorder,
	reason string,
	err error,
) {
	obj.SetCondition(
		renovatev1beta1.ConditionReconciling,
		metav1.ConditionTrue,
		reason,
		err.Error(),
	)

	obj.SetCondition(
		renovatev1beta1.ConditionReady,
		metav1.ConditionFalse,
		reason,
		err.Error(),
	)

	if updateErr := c.Status().Update(ctx, obj); updateErr != nil {
		ctrl.LoggerFrom(ctx).Error(updateErr, "failed to update status")

		return
	}

	if recorder != nil {
		recorder.Eventf(
			obj, nil,
			renovatev1beta1.EventTypeWarning,
			reason,
			renovatev1beta1.EventActionReconciling,
			"%s", err.Error(),
		)
	}
}

func RecordEvent(recorder events.EventRecorder, obj client.Object, event, reason, action, message string) {
	if recorder != nil {
		recorder.Eventf(obj, nil, event, reason, action, "%s", message)
	}
}

func HandleReconcileResult(res *ctrl.Result, err error) (ctrl.Result, error) {
	if err != nil {
		if res != nil {
			return *res, err
		}

		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}
