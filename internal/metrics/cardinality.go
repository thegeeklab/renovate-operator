package metrics

import (
	"sync"
	"sync/atomic"
)

type CardinalityGuard struct {
	maxCardinality int64
	size           atomic.Int64
	keys           sync.Map
	mu             sync.Mutex
}

func NewCardinalityGuard(maxCardinality int) *CardinalityGuard {
	return &CardinalityGuard{
		maxCardinality: int64(maxCardinality),
	}
}

func (c *CardinalityGuard) Allow(key string) bool {
	if _, ok := c.keys.Load(key); ok {
		return true
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.keys.Load(key); ok {
		return true
	}

	if c.size.Load() >= c.maxCardinality {
		return false
	}

	c.keys.Store(key, struct{}{})
	c.size.Add(1)

	return true
}

func (c *CardinalityGuard) Remove(key string) {
	if _, loaded := c.keys.LoadAndDelete(key); loaded {
		c.size.Add(-1)
	}
}
