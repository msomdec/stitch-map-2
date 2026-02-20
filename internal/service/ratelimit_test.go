package service_test

import (
	"testing"

	"github.com/msomdec/stitch-map-2/internal/service"
)

func TestTokenBucket_AllowsUpToCapacity(t *testing.T) {
	tb := service.NewTokenBucket(1, 3) // rate=1/s, capacity=3

	// Should allow 3 requests immediately (full bucket).
	for i := 0; i < 3; i++ {
		if !tb.Allow("test-key") {
			t.Fatalf("request %d should be allowed (bucket not yet empty)", i+1)
		}
	}

	// 4th request should be denied (bucket empty).
	if tb.Allow("test-key") {
		t.Fatal("4th request should be denied (bucket empty)")
	}
}

func TestTokenBucket_DifferentKeysAreIndependent(t *testing.T) {
	tb := service.NewTokenBucket(1, 1) // capacity=1

	if !tb.Allow("ip-a") {
		t.Fatal("ip-a first request should be allowed")
	}
	if tb.Allow("ip-a") {
		t.Fatal("ip-a second request should be denied")
	}

	// ip-b has its own bucket.
	if !tb.Allow("ip-b") {
		t.Fatal("ip-b first request should be allowed (independent bucket)")
	}
}

func TestTokenBucket_NewKeyStartsFull(t *testing.T) {
	tb := service.NewTokenBucket(10, 5)

	for i := 0; i < 5; i++ {
		if !tb.Allow("new-key") {
			t.Fatalf("new key request %d should be allowed (starts full)", i+1)
		}
	}
	if tb.Allow("new-key") {
		t.Fatal("6th request should be denied")
	}
}

func TestTokenBucket_ZeroRateNeverRefills(t *testing.T) {
	tb := service.NewTokenBucket(0, 2) // never refills

	if !tb.Allow("k") {
		t.Fatal("first request should be allowed")
	}
	if !tb.Allow("k") {
		t.Fatal("second request should be allowed")
	}
	if tb.Allow("k") {
		t.Fatal("third request should be denied (no refill)")
	}
}
