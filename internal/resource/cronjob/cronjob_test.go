package cronjob_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	cronjob "github.com/thegeeklab/renovate-operator/internal/resource/cronjob"
)

var _ = Describe("DeleteOwnedJobs", func() {
	var (
		fakeClient client.Client
		scheme     *runtime.Scheme
		ctx        context.Context
		cronJob    *batchv1.CronJob
	)

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
		Expect(batchv1.AddToScheme(scheme)).To(Succeed())

		fakeClient = fake.NewClientBuilder().
			WithScheme(scheme).
			Build()

		ctx = context.Background()

		cronJob = &batchv1.CronJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cronjob",
				Namespace: "test-namespace",
				UID:       "cronjob-uid-123",
			},
			Spec: batchv1.CronJobSpec{
				Schedule: "* * * * *",
				JobTemplate: batchv1.JobTemplateSpec{
					Spec: batchv1.JobSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name:  "test-container",
										Image: "nginx:latest",
									},
								},
								RestartPolicy: corev1.RestartPolicyOnFailure,
							},
						},
					},
				},
			},
		}
		Expect(fakeClient.Create(ctx, cronJob)).To(Succeed())
	})

	Context("when there are no owned jobs", func() {
		It("should return no error when no jobs exist", func() {
			// Execute
			err := cronjob.DeleteOwnedJobs(ctx, fakeClient, cronJob)
			Expect(err).ToNot(HaveOccurred())

			// Verify no jobs exist
			jobList := &batchv1.JobList{}
			Expect(fakeClient.List(ctx, jobList, client.InNamespace(cronJob.Namespace))).To(Succeed())
			Expect(jobList.Items).To(BeEmpty())
		})
	})

	Context("when there are owned jobs", func() {
		var (
			ownedJob1 *batchv1.Job
			ownedJob2 *batchv1.Job
			otherJob  *batchv1.Job
		)

		BeforeEach(func() {
			// Create owned jobs by simulating how a cronjob would create them
			// This mimics the behavior of "kubectl create job --from=cronjob"

			// Helper function to create a job owned by the cronjob
			createOwnedJob := func(nameSuffix string) *batchv1.Job {
				job := &batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:      cronJob.Name + "-" + nameSuffix,
						Namespace: cronJob.Namespace,
						Labels:    map[string]string{"cronjob": cronJob.Name},
					},
					Spec: *cronJob.Spec.JobTemplate.Spec.DeepCopy(),
				}
				if err := ctrl.SetControllerReference(cronJob, job, scheme); err != nil {
					Fail("Failed to set controller reference: " + err.Error())
				}
				Expect(fakeClient.Create(ctx, job)).To(Succeed())

				return job
			}

			// Create owned jobs using the helper function
			ownedJob1 = createOwnedJob("1234567890")
			ownedJob2 = createOwnedJob("0987654321")

			// Create a job not owned by the cronjob
			otherJob = &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "other-job",
					Namespace: cronJob.Namespace,
				},
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "test-container",
									Image: "nginx:latest",
								},
							},
							RestartPolicy: corev1.RestartPolicyOnFailure,
						},
					},
				},
			}
			Expect(fakeClient.Create(ctx, otherJob)).To(Succeed())
		})

		It("should delete only the owned jobs", func() {
			// Execute
			err := cronjob.DeleteOwnedJobs(ctx, fakeClient, cronJob)
			Expect(err).ToNot(HaveOccurred())

			// Verify owned jobs are deleted
			job1 := &batchv1.Job{}
			err = fakeClient.Get(ctx, types.NamespacedName{Name: ownedJob1.Name, Namespace: ownedJob1.Namespace}, job1)
			Expect(err).To(HaveOccurred())

			job2 := &batchv1.Job{}
			err = fakeClient.Get(ctx, types.NamespacedName{Name: ownedJob2.Name, Namespace: ownedJob2.Namespace}, job2)
			Expect(err).To(HaveOccurred())

			// Verify other job still exists
			otherJobObj := &batchv1.Job{}
			err = fakeClient.Get(ctx, types.NamespacedName{Name: otherJob.Name, Namespace: otherJob.Namespace}, otherJobObj)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return error when job deletion fails", func() {
			// Create a mock client that fails on deletion
			failingClient := &failingClient{Client: fakeClient}

			// Execute
			err := cronjob.DeleteOwnedJobs(ctx, failingClient, cronJob)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to delete job"))
		})
	})

	Context("when listing jobs fails", func() {
		It("should return error when job listing fails", func() {
			// Create a mock client that fails on listing
			failingListClient := &failingListClient{Client: fakeClient}

			// Execute
			err := cronjob.DeleteOwnedJobs(ctx, failingListClient, cronJob)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to list jobs"))
		})
	})
})

// failingClient is a mock client that fails on Delete operations.
type failingClient struct {
	client.Client
}

func (c *failingClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	return errors.New("failed to delete job")
}

// failingListClient is a mock client that fails on List operations.
type failingListClient struct {
	client.Client
}

func (c *failingListClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return errors.New("failed to list jobs")
}
