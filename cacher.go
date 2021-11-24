package memo

import (
	"sync"
	"time"

	logging "github.com/ipfs/go-log"
)

var log = logging.Logger("cacher")

func NewCacher() *Cacher {
	return &Cacher{
		cache: make(map[string]*cacheEntry),
	}
}

type Cacher struct {
	lk    sync.Mutex
	cache map[string]*cacheEntry
}

type cacheEntry struct {
	lk           sync.Mutex
	lastComputed time.Time
	val          interface{}
	err          error
	working      bool
}

// Get returns a value from the cache if found.
// If the cached value is older than the ttl, it is still returned, but a new
// value is computed async to be cached upon completion.
// If no value is found, Get blocks until the value function completes.
// Concurrent calls will not trigger multiple calls to the value function.
func (c *Cacher) Get(key string, ttl time.Duration, vf func() (interface{}, error)) (interface{}, error) {
	c.lk.Lock()
	ec, ok := c.cache[key]
	if !ok || ec.val == nil {
		if !ok {
			ec = &cacheEntry{}
			c.cache[key] = ec
		}

		// take the new cache entry lock while holding the cache lock, this is
		// safe since we just created it
		ec.lk.Lock()
		defer ec.lk.Unlock()

		c.lk.Unlock()
		val, err := vf()
		if err != nil {
			ec.err = err
			return nil, err
		}

		ec.val = val
		ec.lastComputed = time.Now()

		return val, nil
	}

	// drop cache lock before attempting to take cache entry lock to avoid
	// potential deadlock where we hold the cache lock, and another goroutine
	// holds the cache entry lock in the above '!ok' branch.
	c.lk.Unlock()
	ec.lk.Lock()
	defer ec.lk.Unlock()

	// if the cached value is still valid, just return it and move on
	if time.Since(ec.lastComputed) < ttl {
		return ec.val, nil
	}

	// In the case that the first call to be cached failed, we end up in a
	// tricky position. Other goroutines may be waiting on the first call to
	// complete, and when it does with an error, they will proceed into the
	// locked section here. We need to detect that case and do our best to
	// compute the value again (under the lock), and hope it doesnt error this
	// time.
	if ec.err != nil {
		log.Errorf("initial cache computation for %q encountered an error, retrying now: %s", key, ec.err)
		val, err := vf()
		if err != nil {
			return nil, err
		}

		// okay, we succeeded. clear the error, set the value, and continue on with life
		ec.err = nil
		ec.val = val
		ec.lastComputed = time.Now()
		return ec.val, nil
	}

	// otherwise, we need to kick off a recomputation of the value
	if !ec.working {
		ec.working = true
		go func() {
			val, err := vf()
			if err != nil {
				log.Errorf("endpoint cache %q failed to recompute: %s", key, err)
			}

			ec.lk.Lock()
			ec.working = false
			if err == nil {
				ec.val = val
				ec.lastComputed = time.Now()
			}
			ec.lk.Unlock()

		}()
	}

	// and return the old value while we wait...
	return ec.val, nil
}
