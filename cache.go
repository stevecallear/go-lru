package lru

import (
	"container/list"
	"errors"
	"sync"
	"time"
)

// UTCNow returns the current UTC time
var UTCNow = func() time.Time {
	return time.Now().UTC()
}

// Options represents a set of LRU cache options
type Options struct {
	Capacity int
	Policy   ExpirationPolicy
}

// NewCache returns a new LRU cache
func NewCache(o Options) *Cache {
	var cap int
	if o.Capacity > 0 {
		cap = o.Capacity
	} else {
		cap = 100
	}

	var pol ExpirationPolicy
	if o.Policy != nil {
		pol = o.Policy
	} else {
		pol = NewNoExpirationPolicy()
	}

	return &Cache{
		ItemEvicted: func(*Item) {},
		cap:         cap,
		policy:      pol,
		items:       map[string]*list.Element{},
		lru:         list.New(),
		mu:          &sync.Mutex{},
	}
}

// Cache represents an LRU memory cache
type Cache struct {
	ItemEvicted func(*Item)
	cap         int
	policy      ExpirationPolicy
	items       map[string]*list.Element
	lru         *list.List
	mu          *sync.Mutex
}

// GetOrAdd returns the cached item with the request key if it exists.
// If the key does not exist then the create func is invoked and the result cached.
func (c *Cache) GetOrAdd(r *GetOrAdd) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var el *list.Element
	var i *Item

	if el, ok := c.items[r.Key]; ok {
		i = el.Value.(*Item)

		if err := c.policy.Apply(i); err == nil {
			c.lru.MoveToBack(el)

			r.Result = i.Value
			return nil
		}

		// item has expired
		c.lru.Remove(el)
	}

	if len(c.items) >= c.cap {
		el = c.lru.Front()
		i = el.Value.(*Item)

		c.lru.Remove(el)
		delete(c.items, i.Key)

		c.ItemEvicted(i)
	}

	i = &Item{
		Key:     r.Key,
		Value:   r.Create(),
		Expires: UTCNow().Add(r.TTL),
	}

	el = c.lru.PushBack(i)
	c.items[r.Key] = el

	r.Result = i.Value
	return nil
}

// GetOrAdd represents a cache GetOrAdd request
type GetOrAdd struct {
	Key    string
	TTL    time.Duration
	Create func() interface{}
	Result interface{}
}

// Item represents a cached value
type Item struct {
	Key     string
	Value   interface{}
	Expires time.Time
}

// ExpirationPolicy represents a cache item expiration policy
type ExpirationPolicy interface {
	Apply(*Item) error
}

// NewNoExpirationPolicy returns a new NoExpirationPolicy
func NewNoExpirationPolicy() *NoExpirationPolicy {
	return new(NoExpirationPolicy)
}

// NoExpirationPolicy represents a non-expiring expiration policy
type NoExpirationPolicy struct {
}

// Apply is a no-op as the policy does not allow items to expire
func (p *NoExpirationPolicy) Apply(i *Item) error {
	return nil
}

// NewFixedExpirationPolicy returns a new FixedExpirationPolicy
func NewFixedExpirationPolicy() *FixedExpirationPolicy {
	return new(FixedExpirationPolicy)
}

// FixedExpirationPolicy represents a fixed expiration policy
type FixedExpirationPolicy struct {
}

// Apply returns an error if the item has expired. The item expiry will not be updated.
func (p *FixedExpirationPolicy) Apply(i *Item) error {
	now := UTCNow()

	if i.Expires.Before(now) || i.Expires.Equal(now) {
		return errors.New("item has expired")
	}

	return nil
}

// NewSlidingExpirationPolicy returns a new SlidingExpirationPolicy with
// the specified TTL
func NewSlidingExpirationPolicy(ttl time.Duration) *SlidingExpirationPolicy {
	return &SlidingExpirationPolicy{ttl: ttl}
}

// SlidingExpirationPolicy represents a sliding expiration policy
type SlidingExpirationPolicy struct {
	ttl time.Duration
}

// Apply resets the TTL for the specified item. An error will be returned if
// the item has expired and cannot be refreshed.
func (p *SlidingExpirationPolicy) Apply(i *Item) error {
	now := UTCNow()

	if i.Expires.Before(now) || i.Expires.Equal(now) {
		return errors.New("item has expired")
	}

	i.Expires = now.Add(p.ttl)
	return nil
}
