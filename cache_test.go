package rustyclaw

import (
	"testing"
	"time"
)

func TestCacheMiss(t *testing.T) {
	c := NewResponseCache()
	_, ok := c.Get(12345)
	if ok {
		t.Error("expected cache miss")
	}
}

func TestCacheHit(t *testing.T) {
	c := NewResponseCache()
	resp := ChatResponse{ID: "test-1", Model: "gpt-4"}
	key := uint64(42)

	c.Put(key, resp)
	got, ok := c.Get(key)
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got.ID != "test-1" {
		t.Errorf("id: got %q, want %q", got.ID, "test-1")
	}
}

func TestCacheTTLExpiry(t *testing.T) {
	c := NewResponseCacheWithConfig(100, 50*time.Millisecond, 0)
	resp := ChatResponse{ID: "test-ttl"}
	key := uint64(99)

	c.Put(key, resp)
	_, ok := c.Get(key)
	if !ok {
		t.Fatal("expected cache hit before TTL")
	}

	time.Sleep(60 * time.Millisecond)

	_, ok = c.Get(key)
	if ok {
		t.Error("expected cache miss after TTL expiry")
	}
}

func TestCacheLRUEviction(t *testing.T) {
	c := NewResponseCacheWithConfig(3, 5*time.Minute, 0)

	// Fill cache
	c.Put(1, ChatResponse{ID: "first"})
	c.Put(2, ChatResponse{ID: "second"})
	c.Put(3, ChatResponse{ID: "third"})

	// Access first to make it recently used
	c.Get(1)

	// Add fourth — should evict key 2 (LRU)
	c.Put(4, ChatResponse{ID: "fourth"})

	if _, ok := c.Get(1); !ok {
		t.Error("key 1 should still be in cache (was accessed)")
	}
	if _, ok := c.Get(2); ok {
		t.Error("key 2 should have been evicted (LRU)")
	}
	if _, ok := c.Get(3); !ok {
		t.Error("key 3 should still be in cache")
	}
	if _, ok := c.Get(4); !ok {
		t.Error("key 4 should be in cache")
	}
}

func TestCacheDedupWindow(t *testing.T) {
	c := NewResponseCacheWithConfig(100, 5*time.Minute, 100*time.Millisecond)
	key := uint64(77)

	c.Put(key, ChatResponse{ID: "original"})

	// Try to overwrite within dedup window
	c.Put(key, ChatResponse{ID: "duplicate"})

	got, ok := c.Get(key)
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got.ID != "original" {
		t.Errorf("id: got %q, want %q (dedup should have prevented overwrite)", got.ID, "original")
	}

	// Wait for dedup window to pass
	time.Sleep(110 * time.Millisecond)

	c.Put(key, ChatResponse{ID: "updated"})
	got, ok = c.Get(key)
	if !ok {
		t.Fatal("expected cache hit after dedup window")
	}
	if got.ID != "updated" {
		t.Errorf("id: got %q, want %q (should have updated after dedup window)", got.ID, "updated")
	}
}

func TestCacheKeyDeterminism(t *testing.T) {
	msgs := []ChatMessage{
		{Role: RoleUser, Content: "Hello"},
		{Role: RoleAssistant, Content: "Hi there"},
	}

	k1 := CacheKey("gpt-4", msgs)
	k2 := CacheKey("gpt-4", msgs)
	if k1 != k2 {
		t.Errorf("same inputs should produce same key: %d != %d", k1, k2)
	}

	k3 := CacheKey("gpt-3.5-turbo", msgs)
	if k1 == k3 {
		t.Error("different models should produce different keys")
	}

	msgs2 := []ChatMessage{
		{Role: RoleUser, Content: "Different message"},
	}
	k4 := CacheKey("gpt-4", msgs2)
	if k1 == k4 {
		t.Error("different messages should produce different keys")
	}
}

func TestNewResponseCacheDefaults(t *testing.T) {
	c := NewResponseCache()
	if c.maxEntries != 100 {
		t.Errorf("maxEntries: got %d, want 100", c.maxEntries)
	}
	if c.ttl != 5*time.Minute {
		t.Errorf("ttl: got %v, want 5m", c.ttl)
	}
	if c.dedupWindow != 2*time.Second {
		t.Errorf("dedupWindow: got %v, want 2s", c.dedupWindow)
	}
}
