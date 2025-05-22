package memcacheiface

import "github.com/bradfitz/gomemcache/memcache"

// Client defines the interface for Memcache client operations needed by rate limiters.
// This allows for mocking the Memcache client in unit tests.
type Client interface {
	Get(key string) (*memcache.Item, error)
	Set(item *memcache.Item) error
	Add(item *memcache.Item) error
	Increment(key string, delta uint64) (newValue uint64, err error)
	// Add other methods like Decrement, Delete if they become necessary.
}
