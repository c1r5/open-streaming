package cache

import (
	"errors"
	"log"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

type entry[V any] struct {
	val       V
	expiresAt time.Time
}

func newEntry[V any](val V, ttl time.Duration) entry[V] {
	return entry[V]{val: val, expiresAt: time.Now().Add(ttl)}
}

type Cache[V any] struct {
	sf    singleflight.Group
	store sync.Map
	ttl   time.Duration
}

func NewCache[V any](ttl time.Duration) *Cache[V] {
	c := &Cache[V]{ttl: ttl}
	go c.cleanup()
	return c
}

func (c *Cache[V]) cleanup() {
	log.Printf("Cache cleanup started with TTL: %s", c.ttl)
	empty := true
	c.store.Range(func(k, v any) bool {
		empty = false
		return false // stop after first item
	})
	if empty {
		log.Println("Cache is empty, skipping cleanup")
		return
	}
	ticker := time.NewTicker(c.ttl)
	defer ticker.Stop()
	for range ticker.C {
		c.store.Range(func(k, v any) bool {
			if time.Now().After(v.(entry[V]).expiresAt) {
				c.store.Delete(k)
			}
			return true
		})
	}
	log.Println("Cache cleanup stopped")
}

func (c *Cache[V]) Set(key string, val V) error {
	c.store.Store(key, newEntry(val, c.ttl))
	return nil
}

func (c *Cache[V]) Retrieve(key string, fetch func() (V, error)) (V, error) {
	if v, ok := c.store.Load(key); ok {
		return v.(entry[V]).val, nil
	}

	v, err, _ := c.sf.Do(key, func() (any, error) {
		if v, ok := c.store.Load(key); ok {
			return v.(entry[V]).val, nil
		}

		val, err := fetch()
		if err != nil {
			return val, err
		}

		c.store.Store(key, newEntry(val, c.ttl))
		return val, nil
	})

	if err != nil {
		var zero V
		return zero, err
	}
	return v.(V), nil
}

func (c *Cache[V]) Get(key string) (V, error) {
	if v, ok := c.store.Load(key); ok {
		return v.(entry[V]).val, nil
	}

	var zero V
	return zero, errors.New("cache: key not found")
}
