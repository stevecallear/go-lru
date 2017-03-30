package lru_test

import (
	"fmt"
	"log"
	"sync"
	"testing"
	"time"

	lru "github.com/stevecallear/go-lru"
)

func ExampleGetOrAdd() {
	c := lru.NewCache(lru.Options{
		Capacity: 1000,
		Policy:   lru.NewFixedExpirationPolicy(),
	})

	r := lru.GetOrAdd{
		Key: "key",
		TTL: 1 * time.Minute,
		Create: func() interface{} {
			return "value"
		},
	}

	if err := c.GetOrAdd(&r); err != nil {
		log.Fatalln(err)
	}

	fmt.Println(r.Result)
}

func TestCache(t *testing.T) {
	tests := []struct {
		capacity    int
		items       []lru.Item
		operations  int
		invocations int
		evictions   int
	}{
		{
			capacity: 1,
			items: []lru.Item{
				{Key: "key", Value: "value"},
			},
			operations:  10,
			invocations: 1,
			evictions:   0,
		},
		{
			capacity: 1,
			items: []lru.Item{
				{Key: "key_1", Value: 1},
				{Key: "key_2", Value: 2},
			},
			operations:  10,
			invocations: 10,
			evictions:   9,
		},
		{
			capacity: 2,
			items: []lru.Item{
				{Key: "key_1", Value: 1},
				{Key: "key_2", Value: 2},
			},
			operations:  10,
			invocations: 2,
			evictions:   0,
		},
	}

	for tn, tt := range tests {
		invocations := 0
		evictions := 0

		c := lru.NewCache(lru.Options{
			Capacity: tt.capacity,
		})

		c.ItemEvicted = func(*lru.Item) {
			evictions++
		}

		for idx := 0; idx < tt.operations; idx++ {
			item := tt.items[idx%len(tt.items)]
			req := lru.GetOrAdd{
				Key: item.Key,
				Create: func() interface{} {
					invocations++
					return item.Value
				},
			}

			if err := c.GetOrAdd(&req); err != nil {
				t.Errorf("GetOrAdd(%d); got %v, expected nil", tn, err)
			}
			if req.Result != item.Value {
				t.Errorf("GetOrAdd(%d); got %v, expected %v", tn, req.Result, item.Value)
			}
		}

		if invocations != tt.invocations {
			t.Errorf("GetOrAdd(%d); got %d func invocations, expected %d", tn, invocations, tt.invocations)
		}
		if evictions != tt.evictions {
			t.Errorf("GetOrAdd(%d); got %d evictions, expected %d", tn, evictions, tt.evictions)
		}
	}
}

func TestCacheWithExpiration(t *testing.T) {
	now := time.Now().UTC()
	invocations := 0

	c := lru.NewCache(lru.Options{
		Policy: lru.NewFixedExpirationPolicy(),
	})

	req := lru.GetOrAdd{
		Key: "key",
		TTL: 1 * time.Minute,
		Create: func() interface{} {
			invocations++
			return "value"
		},
	}

	fixTime(now, func() {
		c.GetOrAdd(&req)
	})
	if invocations != 1 {
		t.Errorf("GetOrAdd(); got %d, expected 1", invocations)
	}

	fixTime(now.Add(30*time.Second), func() {
		c.GetOrAdd(&req)
	})
	if invocations != 1 {
		t.Errorf("GetOrAdd(); got %d, expected 1", invocations)
	}

	fixTime(now.Add(90*time.Second), func() {
		c.GetOrAdd(&req)
	})
	if invocations != 2 {
		t.Errorf("GetOrAdd(); got %d, expected 2", invocations)
	}

	fixTime(now.Add(120*time.Second), func() {
		c.GetOrAdd(&req)
	})
	if invocations != 2 {
		t.Errorf("GetOrAdd(); got %d, expected 2", invocations)
	}
}

func TestCacheParallel(t *testing.T) {
	c := lru.NewCache(lru.Options{
		Capacity: 100,
	})

	wg := new(sync.WaitGroup)

	for r := 0; r < 100; r++ {
		wg.Add(1)
		go func(c *lru.Cache) {
			defer wg.Done()

			for o := 0; o < 100; o++ {
				exp := fmt.Sprintf("value:%d", o)

				req := lru.GetOrAdd{
					Key: fmt.Sprintf("key:%d", o),
					Create: func() interface{} {
						return exp
					},
				}

				if err := c.GetOrAdd(&req); err != nil {
					t.Errorf("GetOrAdd(); got %v, expected nil", err)
				}
				if req.Result != exp {
					t.Errorf("GetOrAdd(); got %s, expected %s", req.Result, exp)
				}
			}
		}(c)
	}

	wg.Wait()
}

func TestNoExpirationPolicy(t *testing.T) {
	now := time.Now()

	tests := []struct {
		expire time.Time
		access time.Time
		err    bool
		exp    time.Time
	}{
		{
			expire: now,
			access: now.Add(1 * time.Minute),
			err:    false,
			exp:    now,
		},
		{
			expire: now.Add(1 * time.Minute),
			access: now,
			err:    false,
			exp:    now.Add(1 * time.Minute),
		},
	}

	for tn, tt := range tests {
		fixTime(tt.access, func() {
			i := lru.Item{Expires: tt.expire}
			p := lru.NewNoExpirationPolicy()

			err := p.Apply(&i)

			if err != nil && !tt.err {
				t.Errorf("Apply(%d); got %v, expected nil", tn, err)
			}
			if err == nil && tt.err {
				t.Errorf("Apply(%d); got nil, expected an error", tn)
			}
			if i.Expires != tt.expire {
				t.Errorf("Apply(%d); got %v, expected %v", tn, i.Expires, tt.expire)
			}
		})
	}
}

func TestFixedExpirationPolicy(t *testing.T) {
	now := time.Now()

	tests := []struct {
		expire time.Time
		access time.Time
		err    bool
		exp    time.Time
	}{
		{
			expire: now,
			access: now.Add(1 * time.Minute),
			err:    true,
			exp:    now,
		},
		{
			expire: now,
			access: now,
			err:    true,
			exp:    now,
		},
		{
			expire: now.Add(1 * time.Minute),
			access: now,
			err:    false,
			exp:    now.Add(1 * time.Minute),
		},
	}

	for tn, tt := range tests {
		fixTime(tt.access, func() {
			i := lru.Item{Expires: tt.expire}
			p := lru.NewFixedExpirationPolicy()

			err := p.Apply(&i)

			if err != nil && !tt.err {
				t.Errorf("Apply(%d); got %v, expected nil", tn, err)
			}
			if err == nil && tt.err {
				t.Errorf("Apply(%d); got nil, expected an error", tn)
			}
			if i.Expires != tt.expire {
				t.Errorf("Apply(%d); got %v, expected %v", tn, i.Expires, tt.expire)
			}
		})
	}
}

func TestSlidingExpirationPolicy(t *testing.T) {
	now := time.Now()

	tests := []struct {
		ttl    time.Duration
		expire time.Time
		access time.Time
		err    bool
		exp    time.Time
	}{
		{
			ttl:    2 * time.Minute,
			expire: now,
			access: now.Add(1 * time.Minute),
			err:    true,
			exp:    now,
		},
		{
			ttl:    2 * time.Minute,
			expire: now,
			access: now,
			err:    true,
			exp:    now,
		},
		{
			ttl:    2 * time.Minute,
			expire: now.Add(1 * time.Minute),
			access: now,
			err:    false,
			exp:    now.Add(2 * time.Minute),
		},
	}

	for tn, tt := range tests {
		fixTime(tt.access, func() {
			i := lru.Item{Expires: tt.expire}
			p := lru.NewSlidingExpirationPolicy(tt.ttl)

			err := p.Apply(&i)

			if err != nil && !tt.err {
				t.Errorf("Apply(%d); got %v, expected nil", tn, err)
			}
			if err == nil && tt.err {
				t.Errorf("Apply(%d); got nil, expected an error", tn)
			}
			if i.Expires != tt.exp {
				t.Errorf("Apply(%d); got %v, expected %v", tn, i.Expires, tt.exp)
			}
		})
	}
}

func fixTime(t time.Time, fn func()) {
	pfn := lru.UTCNow
	lru.UTCNow = func() time.Time {
		return t
	}

	fn()
	lru.UTCNow = pfn
}
