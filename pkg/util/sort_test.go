package util

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("SortItems", func() {
	type testItem struct {
		Name string
		Date time.Time
	}

	nameFn := func(i testItem) string { return i.Name }
	dateFn := func(i testItem) time.Time { return i.Date }

	var items []testItem

	BeforeEach(func() {
		items = []testItem{
			{Name: "charlie", Date: time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)},
			{Name: "alpha", Date: time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC)},
			{Name: "bravo", Date: time.Date(2024, 2, 20, 0, 0, 0, 0, time.UTC)},
		}
	})

	Describe("sorting by name", func() {
		It("should sort in ascending order", func() {
			SortItems(items, SortByName, SortAsc, nameFn, dateFn)
			Expect(items).To(Equal([]testItem{
				{Name: "alpha", Date: time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC)},
				{Name: "bravo", Date: time.Date(2024, 2, 20, 0, 0, 0, 0, time.UTC)},
				{Name: "charlie", Date: time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)},
			}))
		})

		It("should sort in descending order", func() {
			SortItems(items, SortByName, SortDesc, nameFn, dateFn)
			Expect(items).To(Equal([]testItem{
				{Name: "charlie", Date: time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)},
				{Name: "bravo", Date: time.Date(2024, 2, 20, 0, 0, 0, 0, time.UTC)},
				{Name: "alpha", Date: time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC)},
			}))
		})
	})

	Describe("sorting by date", func() {
		It("should sort in ascending order", func() {
			SortItems(items, SortByDate, SortAsc, nameFn, dateFn)
			Expect(items).To(Equal([]testItem{
				{Name: "alpha", Date: time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC)},
				{Name: "bravo", Date: time.Date(2024, 2, 20, 0, 0, 0, 0, time.UTC)},
				{Name: "charlie", Date: time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)},
			}))
		})

		It("should sort in descending order", func() {
			SortItems(items, SortByDate, SortDesc, nameFn, dateFn)
			Expect(items).To(Equal([]testItem{
				{Name: "charlie", Date: time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)},
				{Name: "bravo", Date: time.Date(2024, 2, 20, 0, 0, 0, 0, time.UTC)},
				{Name: "alpha", Date: time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC)},
			}))
		})
	})

	Describe("empty slice", func() {
		It("should handle empty slice without error", func() {
			var empty []testItem

			Expect(func() {
				SortItems(empty, SortByName, SortAsc, nameFn, dateFn)
			}).NotTo(Panic())
			Expect(empty).To(BeEmpty())
		})
	})

	Describe("default sort by name", func() {
		It("should sort by name when sortBy is empty", func() {
			SortItems(items, "", SortAsc, nameFn, dateFn)
			Expect(items).To(Equal([]testItem{
				{Name: "alpha", Date: time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC)},
				{Name: "bravo", Date: time.Date(2024, 2, 20, 0, 0, 0, 0, time.UTC)},
				{Name: "charlie", Date: time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)},
			}))
		})
	})
})
