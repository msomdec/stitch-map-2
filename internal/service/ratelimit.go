package service

import (
	"sync"
	"time"
)

// TokenBucket is a simple in-memory per-key rate limiter using the token bucket algorithm.
// It is safe for concurrent use.
type TokenBucket struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	rate     float64 // tokens added per second
	capacity float64 // maximum tokens
}

type bucket struct {
	tokens float64
	last   time.Time
}

// NewTokenBucket creates a rate limiter that allows up to capacity tokens per key,
// refilling at the given rate (tokens per second).
func NewTokenBucket(rate, capacity float64) *TokenBucket {
	return &TokenBucket{
		buckets:  make(map[string]*bucket),
		rate:     rate,
		capacity: capacity,
	}
}

// Allow reports whether the given key is allowed to proceed under the rate limit.
// Each call consumes one token. Returns false if the bucket is empty.
func (tb *TokenBucket) Allow(key string) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	b, ok := tb.buckets[key]
	if !ok {
		b = &bucket{tokens: tb.capacity, last: time.Now()}
		tb.buckets[key] = b
	}

	now := time.Now()
	elapsed := now.Sub(b.last).Seconds()
	b.tokens = min(b.tokens+elapsed*tb.rate, tb.capacity)
	b.last = now

	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}
