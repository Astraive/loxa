package pool

import "sync"

const (
	defaultBufSize = 2 * 1024  // 2 KB
	maxBufSize     = 64 * 1024 // 64 KB — buffers larger than this are discarded
)

// BytePool is a sync.Pool of []byte buffers.
var BytePool = &sync.Pool{
	New: func() any {
		buf := make([]byte, 0, defaultBufSize)
		return &buf
	},
}

// Get returns a zeroed buffer from the pool.
func Get() []byte {
	bp := BytePool.Get().(*[]byte)
	buf := (*bp)[:0]
	return buf
}

// Put returns a buffer to the pool. Buffers larger than maxBufSize are dropped
// to prevent long-lived large allocations from staying in the pool.
func Put(buf []byte) {
	if cap(buf) > maxBufSize {
		return
	}
	buf = buf[:0]
	BytePool.Put(&buf)
}
