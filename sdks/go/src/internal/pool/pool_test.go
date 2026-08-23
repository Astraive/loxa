package pool

import "testing"

func TestGetReturnsReusableEmptyBuffer(t *testing.T) {
	buf := Get()
	if len(buf) != 0 {
		t.Fatalf("Get length = %d, want 0", len(buf))
	}
	buf = append(buf, "payload"...)
	Put(buf)
	reused := Get()
	if len(reused) != 0 {
		t.Fatalf("reused length = %d, want 0", len(reused))
	}
	Put(reused)
}

func TestPutDiscardsOversizedBuffer(t *testing.T) {
	Put(make([]byte, maxBufSize+1))
	buf := Get()
	if cap(buf) > maxBufSize {
		t.Fatalf("Get returned oversized capacity %d", cap(buf))
	}
	Put(buf)
}
