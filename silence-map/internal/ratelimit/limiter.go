package ratelimit

import (
	"sync"
	"time"
)

type Limiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	buckets map[string]*bucket
}

type bucket struct {
	count     int
	resetAt   time.Time
	updatedAt time.Time
}

func New(limit int, window time.Duration) *Limiter {
	return &Limiter{
		limit:   limit,
		window:  window,
		buckets: make(map[string]*bucket),
	}
}

func (l *Limiter) Allow(key string) bool {
	if key == "" {
		key = "anonymous"
	}

	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.buckets) > 4096 {
		for k, b := range l.buckets {
			if now.Sub(b.updatedAt) > 2*l.window {
				delete(l.buckets, k)
			}
		}
	}

	b, ok := l.buckets[key]
	if !ok || now.After(b.resetAt) {
		l.buckets[key] = &bucket{
			count:     1,
			resetAt:   now.Add(l.window),
			updatedAt: now,
		}
		return true
	}

	b.updatedAt = now
	if b.count >= l.limit {
		return false
	}
	b.count++
	return true
}
