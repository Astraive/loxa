package core

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// IDGenerator generates unique string IDs for events.
type IDGenerator interface {
	NewID() string
}

// uuidV7Gen generates monotonic UUIDv7 IDs.
// UUIDv7 layout (128 bits):
//
//	[48-bit unix_ts_ms][4-bit ver=7][12-bit seq][2-bit var=10][62-bit random]
type uuidV7Gen struct {
	mu     sync.Mutex
	lastMS int64
	seq    uint16
}

var globalIDGen = &uuidV7Gen{}

// NewUUIDv7 generates a new UUIDv7 string using the global generator.
// IDs are monotonically increasing within the same millisecond.
func NewUUIDv7() string {
	return globalIDGen.newID()
}

// NewID implements IDGenerator.
func (g *uuidV7Gen) NewID() string { return g.newID() }

func (g *uuidV7Gen) newID() string {
	g.mu.Lock()

	now := time.Now().UnixMilli()
	if now == g.lastMS {
		g.seq++
		if g.seq > 0x0FFF {
			// Sequence overflow: wait for next millisecond.
			for now == g.lastMS {
				g.mu.Unlock()
				time.Sleep(time.Microsecond * 100)
				g.mu.Lock()
				now = time.Now().UnixMilli()
			}
			g.seq = 0
		}
	} else {
		g.lastMS = now
		g.seq = 0
	}

	ms := now
	seq := g.seq
	g.mu.Unlock()

	var b [16]byte

	// Bytes 0–5: 48-bit timestamp (big-endian)
	b[0] = byte(ms >> 40)
	b[1] = byte(ms >> 32)
	b[2] = byte(ms >> 24)
	b[3] = byte(ms >> 16)
	b[4] = byte(ms >> 8)
	b[5] = byte(ms)

	// Bytes 6–7: version 7 + 12-bit sequence
	b[6] = 0x70 | byte(seq>>8)&0x0F
	b[7] = byte(seq)

	// Bytes 8–15: variant bits + random
	_, _ = rand.Read(b[8:])
	b[8] = (b[8] & 0x3F) | 0x80 // variant 10xx

	return formatUUID(b)
}

func formatUUID(b [16]byte) string {
	var buf [36]byte
	hex.Encode(buf[0:8], b[0:4])
	buf[8] = '-'
	hex.Encode(buf[9:13], b[4:6])
	buf[13] = '-'
	hex.Encode(buf[14:18], b[6:8])
	buf[18] = '-'
	hex.Encode(buf[19:23], b[8:10])
	buf[23] = '-'
	hex.Encode(buf[24:36], b[10:16])
	return string(buf[:])
}
