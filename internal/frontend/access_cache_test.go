package frontend

import (
	"fmt"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("accessCache", func() {
	var (
		cache    *accessCache
		baseTime time.Time
	)

	BeforeEach(func() {
		baseTime = time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC)
		cache = newAccessCache(60 * time.Second)
		cache.now = func() time.Time { return baseTime }
	})

	It("returns the same map after set", func() {
		repos := map[string]bool{"owner/repo": true}
		cache.set("k1", repos)

		got, ok := cache.get("k1")
		Expect(ok).To(BeTrue())
		Expect(got).To(Equal(repos))
	})

	It("returns miss after TTL elapses", func() {
		repos := map[string]bool{"owner/repo": true}
		cache.set("k1", repos)

		// Advance the clock past the TTL.
		cache.now = func() time.Time { return baseTime.Add(61 * time.Second) }

		got, ok := cache.get("k1")
		Expect(ok).To(BeFalse())
		Expect(got).To(BeNil())
	})

	It("returns hit just before TTL boundary", func() {
		repos := map[string]bool{"owner/repo": true}
		cache.set("k1", repos)

		cache.now = func() time.Time { return baseTime.Add(59 * time.Second) }

		got, ok := cache.get("k1")
		Expect(ok).To(BeTrue())
		Expect(got).To(Equal(repos))
	})

	It("removes entries on invalidate", func() {
		cache.set("k1", map[string]bool{"owner/repo": true})

		cache.invalidate("k1")

		got, ok := cache.get("k1")
		Expect(ok).To(BeFalse())
		Expect(got).To(BeNil())
	})

	It("invalidate on a missing key is a no-op", func() {
		Expect(func() { cache.invalidate("missing") }).NotTo(Panic())
	})

	It("isolates entries by key", func() {
		cache.set("k1", map[string]bool{"owner/repo1": true})

		got, ok := cache.get("k2")
		Expect(ok).To(BeFalse())
		Expect(got).To(BeNil())
	})

	It("overwrites an existing entry on set", func() {
		cache.set("k1", map[string]bool{"a/b": true})
		cache.set("k1", map[string]bool{"c/d": true})

		got, ok := cache.get("k1")
		Expect(ok).To(BeTrue())
		Expect(got).To(Equal(map[string]bool{"c/d": true}))
	})

	It("is safe under concurrent get/set/invalidate", func() {
		// Use real time here so we can exercise multiple goroutines without
		// racing on cache.now reassignment.
		concurrentCache := newAccessCache(time.Minute)

		const workers = 16

		var wg sync.WaitGroup

		wg.Add(workers)

		for i := range workers {
			go func(idx int) {
				defer GinkgoRecover()
				defer wg.Done()

				key := fmt.Sprintf("k-%d", idx%4)
				for j := range 100 {
					concurrentCache.set(key, map[string]bool{
						fmt.Sprintf("repo-%d-%d", idx, j): true,
					})

					_, _ = concurrentCache.get(key)

					if j%10 == 0 {
						concurrentCache.invalidate(key)
					}
				}
			}(i)
		}

		wg.Wait()
	})
})
