// Package controller provides shared helpers for the operator's
// reconcilers, including condition management, status patching and event
// recording.
package controller

import (
	"context"
	"errors"
	"fmt"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ErrRenovateConfigNotFound is returned when no RenovateConfig can be
// resolved for a managed resource.
var ErrRenovateConfigNotFound = errors.New("RenovateConfig not found")

// StatusObject is the contract that every API kind reconciled by this
// operator must satisfy in order to participate in the shared status helpers.
type StatusObject interface {
	client.Object

	SetCondition(conditionType string, status metav1.ConditionStatus, reason, message string)
	RemoveCondition(conditionType string)
}

// MarkReady marks the object as Ready=True and clears the Reconciling
// condition. It only mutates the in-memory object; callers must persist the
// status with PatchStatus.
func MarkReady(obj StatusObject, reason, message string) {
	obj.SetCondition(renovatev1beta1.ConditionReady, metav1.ConditionTrue, reason, message)
	obj.RemoveCondition(renovatev1beta1.ConditionReconciling)
}

// MarkNotReady marks the object as Ready=False with the given reason and
// message. It does not flip Reconciling, because the object may either be
// retrying (transient error) or stuck (terminal error) — that signal belongs
// to the controller via the returned ctrl.Result.
func MarkNotReady(obj StatusObject, reason, message string) {
	obj.SetCondition(renovatev1beta1.ConditionReady, metav1.ConditionFalse, reason, message)
}

// MarkReconciling marks the object as Reconciling=True. It is intended to be
// used at the start of a reconciliation that is expected to take more than a
// single pass, so that consumers can distinguish "work in progress" from
// "ready" or "failed".
func MarkReconciling(obj StatusObject, reason, message string) {
	obj.SetCondition(renovatev1beta1.ConditionReconciling, metav1.ConditionTrue, reason, message)
}

// PatchStatus persists the in-memory status of obj using a merge patch
// computed against original. NotFound errors are ignored because the resource
// may have been deleted concurrently. Conflict errors are returned so the
// caller can requeue.
func PatchStatus(ctx context.Context, c client.Client, original, obj client.Object) error {
	if obj == nil || original == nil {
		return nil
	}

	patch := client.MergeFrom(original)
	if err := c.Status().Patch(ctx, obj, patch); err != nil {
		if api_errors.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf("patch status: %w", err)
	}

	return nil
}

// Eventf records an event on obj using the new events API. It is a no-op when
// recorder is nil so that controllers do not need to guard every call site.
func Eventf(
	recorder events.EventRecorder,
	obj runtime.Object,
	event, reason, action, note string,
	args ...any,
) {
	if recorder == nil || obj == nil {
		return
	}

	recorder.Eventf(obj, nil, event, reason, action, note, args...)
}

// HandleReconcileResult normalizes the (*ctrl.Result, error) pair returned by
// component reconcilers into the (ctrl.Result, error) signature expected by
// controller-runtime. A nil res is treated as the zero value while preserving
// any RequeueAfter the component requested, even on success.
func HandleReconcileResult(res *ctrl.Result, err error) (ctrl.Result, error) {
	if res == nil {
		return ctrl.Result{}, err
	}

	return *res, err
}

// Outcome captures the result of a reconciliation pass. The Terminal flag
// signals that the reconciler has already chosen a final Ready condition.
type Outcome struct {
	Result   *ctrl.Result
	Err      error
	Terminal bool
}

// FinalizeStatusOptions describes how to translate an Outcome into a Ready
// condition and a Kubernetes Event. Each controller provides its own
// human-readable success message and event reasons.
type FinalizeStatusOptions struct {
	// SuccessMessage is the message used for both the Ready=True condition
	// and the Normal event when reconciliation succeeds.
	SuccessMessage string
	// EventAction is the action label written into emitted events.
	// It defaults to renovatev1beta1.EventActionReconciling when empty.
	EventAction string
}

// FinalizeStatus applies the Ready condition, emits an event for the outcome
// and patches the status subresource. It is the single point where status is
// written from a controller, which avoids racing concurrent helper functions.
func FinalizeStatus(
	ctx context.Context,
	c client.Client,
	recorder events.EventRecorder,
	original client.Object,
	obj StatusObject,
	outcome Outcome,
	opts FinalizeStatusOptions,
) {
	action := opts.EventAction
	if action == "" {
		action = renovatev1beta1.EventActionReconciling
	}

	switch {
	case outcome.Err != nil:
		MarkNotReady(obj, renovatev1beta1.ReasonReconcileError, outcome.Err.Error())
		Eventf(
			recorder, obj,
			renovatev1beta1.EventTypeWarning,
			renovatev1beta1.ReasonReconcileError,
			action,
			"%s", outcome.Err.Error(),
		)
	case !outcome.Terminal:
		MarkReady(obj, renovatev1beta1.ReasonReconcileSuccess, opts.SuccessMessage)
		Eventf(
			recorder, obj,
			renovatev1beta1.EventTypeNormal,
			renovatev1beta1.ReasonReconciled,
			action,
			"%s", opts.SuccessMessage,
		)
	}

	if err := PatchStatus(ctx, c, original, obj); err != nil {
		ctrl.LoggerFrom(ctx).Error(err, "failed to patch status")
	}
}
