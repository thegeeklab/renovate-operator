package metrics

import (
	"sync"
)

type CardinalityGuard struct {
	maxCardinality int
	keys           sync.Map
}

func NewCardinalityGuard(maxCardinality int) *CardinalityGuard {
	return &CardinalityGuard{
		maxCardinality: maxCardinality,
	}
}

func (c *CardinalityGuard) Allow(key string) bool {
	if _, ok := c.keys.Load(key); ok {
		return true
	}

	currentCount := c.countCount()
	if currentCount >= c.maxCardinality {
		return false
	}

	c.keys.Store(key, struct{}{})

	return true
}

func (c *CardinalityGuard) Remove(key string) {
	c.keys.Delete(key)
}

func (c *CardinalityGuard) countCount() int {
	count := 0

	c.keys.Range(func(_, _ any) bool {
		count++

		return true
	})

	return count
}
