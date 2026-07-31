package mirror

import (
	"sync"
	"time"

	"github.com/Scalewithus-India/linux-mirror-cashing-server/internal/store"
)

type headEntry struct {
	head store.ObjectHead
	exp  time.Time // zero = no expiry (packages)
}

// headCache avoids a HeadObject RTT on hot package/metadata hits.
type headCache struct {
	mu      sync.RWMutex
	entries map[string]headEntry
	max     int
}

func newHeadCache(max int) *headCache {
	if max < 1024 {
		max = 1024
	}
	return &headCache{entries: make(map[string]headEntry, 1024), max: max}
}

func (c *headCache) Get(key string) *store.ObjectHead {
	c.mu.RLock()
	e, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok {
		return nil
	}
	if !e.exp.IsZero() && time.Now().After(e.exp) {
		c.mu.Lock()
		delete(c.entries, key)
		c.mu.Unlock()
		return nil
	}
	h := e.head
	return &h
}

func (c *headCache) Put(key string, head *store.ObjectHead, ttl time.Duration) {
	if head == nil {
		return
	}
	e := headEntry{head: *head}
	if ttl > 0 {
		e.exp = time.Now().Add(ttl)
	}
	c.mu.Lock()
	if len(c.entries) >= c.max {
		// Drop ~1% of entries when full (cheap pressure relief).
		n := c.max / 100
		if n < 64 {
			n = 64
		}
		for k := range c.entries {
			delete(c.entries, k)
			n--
			if n <= 0 {
				break
			}
		}
	}
	c.entries[key] = e
	c.mu.Unlock()
}

func (c *headCache) Delete(key string) {
	c.mu.Lock()
	delete(c.entries, key)
	c.mu.Unlock()
}

func (c *headCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}
