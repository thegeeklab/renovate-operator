package scheduler

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/netresearch/go-cron"
	"github.com/thegeeklab/renovate-operator/pkg/util/k8s"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	TriggerSuspended = "suspended"
	TriggerManual    = "manual"
	TriggerSchedule  = "schedule"
	TriggerWait      = "wait"
)

var ErrInvalidClientObject = errors.New("failed to convert object deep copy to client.Object")

type Schedulable interface {
	client.Object
	GetSchedule() string
	GetSuspend() bool
	GetLastScheduleTime() *metav1.Time
	SetLastScheduleTime(*metav1.Time)
	GetSuccessLimit() int
	GetFailedLimit() int
}

type Manager struct {
	client.Client
	scheme *runtime.Scheme
	clock  clock.Clock
}

type DecisionResult struct {
	ShouldRun bool
	NextRun   time.Time
	Trigger   string
}

func NewManager(c client.Client, s *runtime.Scheme, clock clock.Clock) *Manager {
	return &Manager{
		Client: c,
		scheme: s,
		clock:  clock,
	}
}

func (m *Manager) Evaluate(obj Schedulable, checkManualTrigger func(map[string]string) bool) (DecisionResult, error) {
	hasAnnotation := false
	if checkManualTrigger != nil {
		hasAnnotation = checkManualTrigger(obj.GetAnnotations())
	}

	schedule, err := cron.ParseStandard(obj.GetSchedule())
	if err != nil {
		return DecisionResult{}, fmt.Errorf("invalid schedule: %w", err)
	}

	var lastRun time.Time
	if t := obj.GetLastScheduleTime(); t != nil {
		lastRun = t.Time
	}

	nextRun := schedule.Next(lastRun)
	now := clock.Clock.Now(m.clock)
	isScheduleDue := lastRun.IsZero() || now.After(nextRun)
	isSuspended := obj.GetSuspend()

	if isSuspended && isScheduleDue && !hasAnnotation {
		return DecisionResult{ShouldRun: false, NextRun: nextRun, Trigger: TriggerSuspended}, nil
	}

	if hasAnnotation {
		return DecisionResult{ShouldRun: true, NextRun: nextRun, Trigger: TriggerManual}, nil
	}

	if isScheduleDue {
		return DecisionResult{ShouldRun: true, NextRun: nextRun, Trigger: TriggerSchedule}, nil
	}

	return DecisionResult{ShouldRun: false, NextRun: nextRun, Trigger: TriggerWait}, nil
}

func (m *Manager) EnsureJob(
	ctx context.Context, owner Schedulable, job *batchv1.Job, lockLabels map[string]string,
) (bool, error) {
	// Check local cache for active jobs
	activeJobs, err := m.getActiveJobs(ctx, owner.GetNamespace(), lockLabels)
	if err != nil {
		return false, fmt.Errorf("failed to check active jobs: %w", err)
	}

	if len(activeJobs) > 0 {
		return false, nil
	}

	if err := controllerutil.SetControllerReference(owner, job, m.scheme); err != nil {
		return false, fmt.Errorf("failed to set controller reference: %w", err)
	}

	if job.Name == "" {
		baseName := owner.GetName()

		// Extract the target base name from GenerateName if provided
		// (crucial for multiple jobs running under the same owner, like GitRepos)
		if job.GenerateName != "" {
			baseName = strings.TrimRight(job.GenerateName, "-")
			job.GenerateName = "" // Clear it so the API server doesn't use it
		}

		timeInMinutes := m.clock.Now().Unix() / int64(time.Minute.Seconds())
		suffix := fmt.Sprintf("-%d", timeInMinutes)

		jobName, err := k8s.DeterministicName(baseName, suffix)
		if err != nil {
			return false, fmt.Errorf("failed to generate deterministic job name: %w", err)
		}

		job.Name = jobName
	}

	if err := m.Create(ctx, job); err != nil {
		if api_errors.IsAlreadyExists(err) {
			return false, nil
		}

		return false, fmt.Errorf("failed to create job: %w", err)
	}

	return true, nil
}

func (m *Manager) CompleteRun(
	ctx context.Context, obj Schedulable, cleanupManualTrigger func(map[string]string) map[string]string,
) error {
	key := client.ObjectKeyFromObject(obj)
	if err := m.Get(ctx, key, obj); err != nil {
		return fmt.Errorf("failed to refresh object for status update: %w", err)
	}

	if cleanupManualTrigger != nil {
		deepCopy := obj.DeepCopyObject()

		patchObj, ok := deepCopy.(client.Object)
		if !ok {
			return ErrInvalidClientObject
		}

		patchBase := client.MergeFrom(patchObj)

		newAnnotations := cleanupManualTrigger(obj.GetAnnotations())
		obj.SetAnnotations(newAnnotations)

		if err := m.Patch(ctx, obj, patchBase); err != nil {
			return fmt.Errorf("failed to remove annotation: %w", err)
		}
	}

	deepCopy := obj.DeepCopyObject()

	statusPatchObj, ok := deepCopy.(client.Object)
	if !ok {
		return ErrInvalidClientObject
	}

	statusPatchBase := client.MergeFrom(statusPatchObj)

	now := metav1.NewTime(m.clock.Now())
	obj.SetLastScheduleTime(&now)

	if err := m.Status().Patch(ctx, obj, statusPatchBase); err != nil {
		return fmt.Errorf("failed to patch status: %w", err)
	}

	return nil
}

func (m *Manager) getActiveJobs(
	ctx context.Context, namespace string, matchLabels map[string]string,
) ([]batchv1.Job, error) {
	var jobList batchv1.JobList
	if err := m.List(ctx, &jobList, client.InNamespace(namespace), client.MatchingLabels(matchLabels)); err != nil {
		return nil, err
	}

	var active []batchv1.Job

	for _, job := range jobList.Items {
		isFinished := isJobFinished(&job)
		if !isFinished {
			active = append(active, job)
		}
	}

	return active, nil
}

func (m *Manager) PruneJobs(
	ctx context.Context, ns string, labels map[string]string, successLimit, failedLimit int,
) error {
	var (
		jobList        batchv1.JobList
		successfulJobs []batchv1.Job
		failedJobs     []batchv1.Job
	)

	if err := m.List(ctx, &jobList, client.InNamespace(ns), client.MatchingLabels(labels)); err != nil {
		return err
	}

	for _, job := range jobList.Items {
		if isJobFinished(&job) {
			if job.Status.Succeeded > 0 {
				successfulJobs = append(successfulJobs, job)
			} else if job.Status.Failed > 0 {
				failedJobs = append(failedJobs, job)
			}
		}
	}

	if err := m.deleteOldJobs(ctx, successfulJobs, successLimit); err != nil {
		return err
	}

	if err := m.deleteOldJobs(ctx, failedJobs, failedLimit); err != nil {
		return err
	}

	return nil
}

func (m *Manager) deleteOldJobs(ctx context.Context, jobs []batchv1.Job, limit int) error {
	if limit == 0 {
		return nil
	}

	if len(jobs) <= limit {
		return nil
	}

	sort.Slice(jobs, func(i, j int) bool {
		if jobs[i].Status.CompletionTime == nil {
			return false
		}

		if jobs[j].Status.CompletionTime == nil {
			return true
		}

		return jobs[i].Status.CompletionTime.Before(jobs[j].Status.CompletionTime)
	})

	deleteCount := len(jobs) - limit
	policy := metav1.DeletePropagationBackground

	for i := 0; i < deleteCount; i++ {
		if err := m.Delete(ctx, &jobs[i], &client.DeleteOptions{PropagationPolicy: &policy}); err != nil {
			if client.IgnoreNotFound(err) != nil {
				return err
			}
		}
	}

	return nil
}

func isJobFinished(job *batchv1.Job) bool {
	for _, c := range job.Status.Conditions {
		if (c.Type == batchv1.JobComplete || c.Type == batchv1.JobFailed) && c.Status == corev1.ConditionTrue {
			return true
		}
	}

	return false
}
