package scheduler

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakeclock "k8s.io/utils/clock/testing"
)

// MockSchedulable for testing.
type MockSchedulable struct {
	metav1.TypeMeta // Add this
	metav1.ObjectMeta
	Schedule         string
	Suspend          bool
	LastScheduleTime *metav1.Time
}

func (m *MockSchedulable) GetSchedule() string                { return m.Schedule }
func (m *MockSchedulable) GetSuspend() bool                   { return m.Suspend }
func (m *MockSchedulable) GetLastScheduleTime() *metav1.Time  { return m.LastScheduleTime }
func (m *MockSchedulable) SetLastScheduleTime(t *metav1.Time) { m.LastScheduleTime = t }
func (m *MockSchedulable) GetSuccessLimit() int               { return 3 }
func (m *MockSchedulable) GetFailedLimit() int                { return 1 }
func (m *MockSchedulable) DeepCopyObject() runtime.Object {
	return &MockSchedulable{
		TypeMeta:         m.TypeMeta,
		ObjectMeta:       m.ObjectMeta,
		Schedule:         m.Schedule,
		Suspend:          m.Suspend,
		LastScheduleTime: m.LastScheduleTime,
	}
}

var _ = Describe("Scheduler Manager", func() {
	var (
		mgr       *Manager
		fakeClock *fakeclock.FakeClock
		obj       *MockSchedulable
		now       time.Time
	)

	BeforeEach(func() {
		now = time.Date(2026, 2, 25, 12, 0, 0, 0, time.UTC)
		fakeClock = fakeclock.NewFakeClock(now)
		mgr = NewManager(nil, nil, fakeClock)

		obj = &MockSchedulable{
			ObjectMeta: metav1.ObjectMeta{Name: "test-task"},
			Schedule:   "*/5 * * * *", // Every 5 minutes
			Suspend:    false,
		}
	})

	Describe("Evaluate", func() {
		Context("when the job has never run before", func() {
			It("should trigger the job immediately", func() {
				res, err := mgr.Evaluate(obj, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(res.ShouldRun).To(BeTrue())
				Expect(res.Trigger).To(Equal(TriggerSchedule))
			})
		})

		Context("when the schedule is not yet due", func() {
			It("should not trigger", func() {
				// Set last run to exactly "now"
				lastRun := metav1.NewTime(now)
				obj.LastScheduleTime = &lastRun

				res, err := mgr.Evaluate(obj, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(res.ShouldRun).To(BeFalse())
				Expect(res.Trigger).To(Equal(TriggerWait))
				// Next run should be 5 mins from now
				Expect(res.NextRun.Unix()).To(Equal(now.Add(5 * time.Minute).Unix()))
			})
		})

		Context("when the schedule is due", func() {
			It("should trigger the job", func() {
				// Last run was 10 minutes ago
				lastRun := metav1.NewTime(now.Add(-10 * time.Minute))
				obj.LastScheduleTime = &lastRun

				res, err := mgr.Evaluate(obj, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(res.ShouldRun).To(BeTrue())
				Expect(res.Trigger).To(Equal(TriggerSchedule))
			})
		})

		Context("when the job is suspended", func() {
			BeforeEach(func() {
				obj.Suspend = true
			})

			It("should not trigger even if due", func() {
				res, err := mgr.Evaluate(obj, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(res.ShouldRun).To(BeFalse())
				Expect(res.Trigger).To(Equal(TriggerSuspended))
			})

			It("should still trigger if a manual override is present", func() {
				manualTrigger := func(ann map[string]string) bool { return true }
				res, err := mgr.Evaluate(obj, manualTrigger)
				Expect(err).NotTo(HaveOccurred())
				Expect(res.ShouldRun).To(BeTrue())
				Expect(res.Trigger).To(Equal(TriggerManual))
			})
		})

		Context("with an invalid cron expression", func() {
			It("should return an error", func() {
				obj.Schedule = "invalid-cron"
				_, err := mgr.Evaluate(obj, nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid schedule"))
			})
		})
	})
})
