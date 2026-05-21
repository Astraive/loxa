package loadgen

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"time"
)

func GenerateEvent(rng *rand.Rand, bodySize int) ([]byte, error) {
	event := map[string]any{
		"event_id":    fmt.Sprintf("evt-%d-%d", rng.Int63(), time.Now().UnixNano()),
		"event":       "loadtest.event",
		"service":     "loadtest",
		"timestamp":   time.Now().UTC().Format(time.RFC3339Nano),
		"status_code": 200,
		"duration_ms": rng.Intn(500),
		"payload":     randomString(rng, bodySize),
	}
	data, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func randomString(rng *rand.Rand, n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	buf := make([]byte, n)
	for i := range buf {
		buf[i] = letters[rng.Intn(len(letters))]
	}
	return string(buf)
}
