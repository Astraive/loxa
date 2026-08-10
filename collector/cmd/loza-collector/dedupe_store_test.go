package main

import "testing"

func TestRedisDedupeKeyHashesRawValue(t *testing.T) {
	key := redisDedupeKey("loza:", "event-id-with-user-controlled-content")
	if len(key) != len("loza:")+64 {
		t.Fatalf("redis dedupe key length = %d, want %d", len(key), len("loza:")+64)
	}
	if key == "loza:event-id-with-user-controlled-content" {
		t.Fatal("redis dedupe key must not include raw event id")
	}
}
