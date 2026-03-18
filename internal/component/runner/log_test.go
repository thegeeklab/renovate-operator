package runner

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/thegeeklab/renovate-operator/internal/webhook/v1beta1"

	"github.com/stretchr/testify/mock"
	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/logstore"
	logstorte_mocks "github.com/thegeeklab/renovate-operator/internal/logstore/mocks"
	"github.com/thegeeklab/renovate-operator/internal/scheduler"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kubernetesfake "k8s.io/client-go/kubernetes/fake"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	fakeclock "k8s.io/utils/clock/testing"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("reconcileLogs", func() {
	var (
		fakeClient    client.Client
		fakeClientset *kubernetesfake.Clientset
		mockStore     *logstorte_mocks.Store
		reconciler    *Reconciler
		instance      *renovatev1beta1.Runner
		ctx           context.Context
		scheme        *runtime.Scheme
		now           time.Time
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme = runtime.NewScheme()
		Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
		Expect(renovatev1beta1.AddToScheme(scheme)).To(Succeed())

		instance = &renovatev1beta1.Runner{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-runner",
				Namespace: "default",
				Labels: map[string]string{
					renovatev1beta1.LabelRenovator: "renovator-id",
				},
			},
			Spec: renovatev1beta1.RunnerSpec{
				JobSpec: renovatev1beta1.JobSpec{
					Schedule: "*/5 * * * *",
				},
			},
		}

		rr := &RunnerCustomDefaulter{}
		Expect(rr.Default(ctx, instance)).To(Succeed())

		renovate := &renovatev1beta1.RenovateConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-renovate",
				Namespace: "default",
			},
		}
		rd := &RenovateConfigCustomDefaulter{}
		Expect(rd.Default(ctx, renovate)).To(Succeed())

		now = time.Date(2026, 2, 27, 15, 0, 0, 0, time.UTC)
		fakeClock := fakeclock.NewFakeClock(now)

		fakeClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(instance, renovate).
			WithStatusSubresource(instance).
			Build()

		fakeClientset = kubernetesfake.NewClientset()
		mockStore = logstorte_mocks.NewStore(GinkgoT())

		reconciler = &Reconciler{
			Client:     fakeClient,
			scheme:     scheme,
			scheduler:  scheduler.NewManager(fakeClient, scheme, fakeClock),
			logManager: logstore.NewManager(fakeClientset, mockStore),
			req: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: instance.Namespace,
					Name:      instance.Name,
				},
			},
			instance: instance,
			renovate: renovate,
		}
	})

	createJobAndPod := func(name string, finished, collected bool) *batchv1.Job {
		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "default",
				Labels: map[string]string{
					renovatev1beta1.LabelAppName:      renovatev1beta1.OperatorName,
					renovatev1beta1.LabelAppInstance:  instance.Name,
					renovatev1beta1.LabelAppComponent: renovatev1beta1.ComponentRunner,
					renovatev1beta1.LabelAppManagedBy: renovatev1beta1.OperatorManagedBy,
					renovatev1beta1.LabelRenovator:    "renovator-id",
				},
				Annotations: make(map[string]string),
			},
		}
		if collected {
			job.Annotations[renovatev1beta1.LabelLogsCollected] = "true"
		}

		if finished {
			job.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}}
		} else {
			job.Status.Active = 1
		}

		Expect(fakeClient.Create(ctx, job)).To(Succeed())

		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:              name + "-pod",
				Namespace:         "default",
				Labels:            map[string]string{"job-name": name},
				CreationTimestamp: metav1.NewTime(now),
			},
		}
		Expect(fakeClientset.Tracker().Add(pod)).To(Succeed())

		return job
	}

	It("should skip jobs that are still running or already collected", func() {
		createJobAndPod("running-job", false, false)
		createJobAndPod("collected-job", true, true)

		result, err := reconciler.reconcileLogs(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(&ctrl.Result{}))
	})

	It("should archive logs and patch annotation for finished jobs", func() {
		createJobAndPod("finished-job", true, false)

		mockStore.On("SaveLog", ctx, "default", "runner", "test-runner", "finished-job", mock.Anything).
			Return(nil)

		result, err := reconciler.reconcileLogs(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(&ctrl.Result{}))

		updatedJob := &batchv1.Job{}
		jobKey := types.NamespacedName{Name: "finished-job", Namespace: "default"}
		Expect(fakeClient.Get(ctx, jobKey, updatedJob)).To(Succeed())
		Expect(updatedJob.Annotations[renovatev1beta1.LabelLogsCollected]).To(Equal("true"))
	})

	It("should requeue when hitting maxLogsPerReconcile", func() {
		for i := 0; i < 6; i++ {
			jobName := fmt.Sprintf("job-%d", i)
			createJobAndPod(jobName, true, false)

			if i < 5 {
				mockStore.On("SaveLog", ctx, "default", "runner", "test-runner", jobName, mock.Anything).
					Return(nil)
			}
		}

		result, err := reconciler.reconcileLogs(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(BeZero())

		collectedCount := 0
		jobList := &batchv1.JobList{}
		Expect(fakeClient.List(ctx, jobList, client.InNamespace("default"))).To(Succeed())

		for _, job := range jobList.Items {
			if job.Annotations[renovatev1beta1.LabelLogsCollected] == "true" {
				collectedCount++
			}
		}

		Expect(collectedCount).To(Equal(5))
	})

	It("should handle archive errors gracefully without patching the job", func() {
		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "missing-pod-job",
				Namespace: "default",
				Labels: map[string]string{
					renovatev1beta1.LabelAppName:      renovatev1beta1.OperatorName,
					renovatev1beta1.LabelAppInstance:  instance.Name,
					renovatev1beta1.LabelAppComponent: renovatev1beta1.ComponentRunner,
					renovatev1beta1.LabelRenovator:    "renovator-id",
				},
			},
			Status: batchv1.JobStatus{
				Conditions: []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}},
			},
		}
		Expect(fakeClient.Create(ctx, job)).To(Succeed())

		result, err := reconciler.reconcileLogs(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(&ctrl.Result{}))

		updatedJob := &batchv1.Job{}
		jobKey := types.NamespacedName{Name: "missing-pod-job", Namespace: "default"}
		Expect(fakeClient.Get(ctx, jobKey, updatedJob)).To(Succeed())
		Expect(updatedJob.Annotations[renovatev1beta1.LabelLogsCollected]).To(BeEmpty())
	})
})
