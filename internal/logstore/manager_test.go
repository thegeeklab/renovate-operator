package logstore_test

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	"github.com/thegeeklab/renovate-operator/internal/logstore"
	"github.com/thegeeklab/renovate-operator/internal/logstore/mocks"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

var _ = Describe("Manager", func() {
	var (
		fakeClientset *fake.Clientset
		mockStore     *mocks.Store
		manager       *logstore.Manager
		ctx           context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()
		fakeClientset = fake.NewClientset()
		mockStore = mocks.NewStore(GinkgoT())
		manager = logstore.NewManager(fakeClientset, mockStore)
	})

	Describe("GetLogStream", func() {
		Context("when the job does not exist", func() {
			It("should fallback to the persistent store", func() {
				mockStore.On("GetLog", ctx, "default", "runner", "repo1", "missing-job").
					Return(io.NopCloser(strings.NewReader("archived logs")), nil)

				stream, err := manager.GetLogStream(ctx, "default", "runner", "repo1", "missing-job")
				Expect(err).NotTo(HaveOccurred())

				defer stream.Close()

				content, _ := io.ReadAll(stream)
				Expect(string(content)).To(Equal("archived logs"))
			})
		})

		Context("when the job is completed", func() {
			BeforeEach(func() {
				completedJob := &batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{Name: "completed-job", Namespace: "default"},
					Status: batchv1.JobStatus{
						Active: 0,
						Conditions: []batchv1.JobCondition{
							{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
						},
					},
				}
				Expect(fakeClientset.Tracker().Add(completedJob)).To(Succeed())
			})

			It("should skip Kubernetes logs and fallback to the persistent store", func() {
				mockStore.On("GetLog", ctx, "default", "runner", "repo1", "completed-job").
					Return(io.NopCloser(strings.NewReader("historical logs")), nil)

				stream, err := manager.GetLogStream(ctx, "default", "runner", "repo1", "completed-job")
				Expect(err).NotTo(HaveOccurred())

				defer stream.Close()

				content, _ := io.ReadAll(stream)
				Expect(string(content)).To(Equal("historical logs"))
			})
		})

		Context("when the job is actively running", func() {
			BeforeEach(func() {
				activeJob := &batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{Name: "active-job", Namespace: "default"},
					Status:     batchv1.JobStatus{Active: 1},
				}
				Expect(fakeClientset.Tracker().Add(activeJob)).To(Succeed())
			})

			It("should attempt to stream live pod logs", func() {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "active-pod",
						Namespace: "default",
						Labels:    map[string]string{"job-name": "active-job"},
					},
				}
				Expect(fakeClientset.Tracker().Add(pod)).To(Succeed())

				stream, err := manager.GetLogStream(ctx, "default", "runner", "repo1", "active-job")

				Expect(err).NotTo(HaveOccurred())
				Expect(stream).NotTo(BeNil())

				defer stream.Close()
			})

			It("should fallback to store if no pods are found for the active job", func() {
				mockStore.On("GetLog", ctx, "default", "runner", "repo1", "active-job").
					Return(io.NopCloser(strings.NewReader("fallback logs")), nil)

				stream, err := manager.GetLogStream(ctx, "default", "runner", "repo1", "active-job")
				Expect(err).NotTo(HaveOccurred())

				defer stream.Close()

				content, _ := io.ReadAll(stream)
				Expect(string(content)).To(Equal("fallback logs"))
			})
		})
	})

	Describe("ArchiveJob", func() {
		It("should return errPodNotFound if no pods match the job name", func() {
			err := manager.ArchiveJob(ctx, "default", "missing-job", "runner", "repo1", "container")
			Expect(err).To(MatchError(ContainSubstring("no pods found for job")))
		})

		It("should attempt to stream from the latest pod when multiple exist", func() {
			now := time.Now()
			olderPod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "older-pod",
					Namespace:         "default",
					Labels:            map[string]string{"job-name": "retry-job"},
					CreationTimestamp: metav1.NewTime(now.Add(-10 * time.Minute)),
				},
			}
			newerPod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "newer-pod",
					Namespace:         "default",
					Labels:            map[string]string{"job-name": "retry-job"},
					CreationTimestamp: metav1.NewTime(now),
				},
			}

			Expect(fakeClientset.Tracker().Add(olderPod)).To(Succeed())
			Expect(fakeClientset.Tracker().Add(newerPod)).To(Succeed())

			mockStore.On("SaveLog", ctx, "default", "runner", "repo1", "retry-job", mock.Anything).
				Return(nil)

			err := manager.ArchiveJob(ctx, "default", "retry-job", "runner", "repo1", "")

			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Passthrough Methods", func() {
		It("ListLogs should delegate to the store", func() {
			expectedEntry := []logstore.LogEntry{{JobName: "test-job"}}

			mockStore.On("ListLogs", ctx, "ns", "comp", "owner").Return(expectedEntry, nil)

			logs, err := manager.ListLogs(ctx, "ns", "comp", "owner")
			Expect(err).NotTo(HaveOccurred())
			Expect(logs).To(HaveLen(1))
			Expect(logs[0].JobName).To(Equal("test-job"))
		})

		It("DeleteLog should delegate to the store", func() {
			mockStore.On("DeleteLog", ctx, "ns", "comp", "owner", "job").
				Return(fmt.Errorf("simulated deletion error"))

			err := manager.DeleteLog(ctx, "ns", "comp", "owner", "job")
			Expect(err).To(MatchError("simulated deletion error"))
		})

		It("SaveLog should delegate directly to the store", func() {
			mockStore.On("SaveLog", ctx, "ns", "comp", "owner", "job", mock.Anything).
				Return(nil)

			err := manager.SaveLog(ctx, "ns", "comp", "owner", "job", strings.NewReader("data"))
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
