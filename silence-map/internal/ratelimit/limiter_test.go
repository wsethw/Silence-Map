package ratelimit

import (
	"testing"
	"time"
)

func TestLimiterAllowsUntilLimit(t *testing.T) {
	limiter := New(2, time.Minute)

	if !limiter.Allow("user") {
		t.Fatal("first request denied")
	}
	if !limiter.Allow("user") {
		t.Fatal("second request denied")
	}
	if limiter.Allow("user") {
		t.Fatal("third request allowed, want denied")
	}
}
