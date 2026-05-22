package bench

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"testing"
	"time"
)

// sampleEvent returns a realistic JSON event payload.
func sampleEvent() map[string]any {
	return map[string]any{
		"event_id":     "evt_bench_001",
		"event_name":   "payment.completed",
		"event_version": "v1",
		"timestamp":    time.Now().UTC().Format(time.RFC3339Nano),
		"service":      "checkout-api",
		"level":        "info",
		"outcome":      "success",
		"kind":         "event",
		"schema_version": "v1",
		"duration_ms":  142,
		"http": map[string]any{
			"method": "POST",
			"path":   "/api/v1/payments",
			"status": 200,
		},
		"user": map[string]any{
			"id":    "usr_abc123",
			"email": "user@example.com",
		},
		"payment": map[string]any{
			"amount":   99.99,
			"currency": "USD",
			"provider": "stripe",
		},
	}
}

// sampleEventJSON returns a pre-serialized JSON event.
func sampleEventJSON() []byte {
	b, _ := json.Marshal(sampleEvent())
	return b
}

// BenchmarkIngestParseJSON measures single-event JSON parsing throughput.
func BenchmarkIngestParseJSON(b *testing.B) {
	data := sampleEventJSON()
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var evt map[string]any
		if err := json.Unmarshal(data, &evt); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkIngestParseJSONArray measures batch JSON array parsing.
func BenchmarkIngestParseJSONArray(b *testing.B) {
	events := make([]map[string]any, 100)
	for i := range events {
		events[i] = sampleEvent()
	}
	data, _ := json.Marshal(events)
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var batch []map[string]any
		if err := json.Unmarshal(data, &batch); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkIngestParseNDJSON measures NDJSON (newline-delimited) parsing.
func BenchmarkIngestParseNDJSON(b *testing.B) {
	ev := sampleEventJSON()
	var buf bytes.Buffer
	for i := 0; i < 100; i++ {
		buf.Write(ev)
		buf.WriteByte('\n')
	}
	data := buf.Bytes()
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		dec := json.NewDecoder(bytes.NewReader(data))
		count := 0
		for {
			var evt map[string]any
			if err := dec.Decode(&evt); err != nil {
				if err == io.EOF {
					break
				}
				b.Fatal(err)
			}
			count++
		}
	}
}

// BenchmarkIngestParseGzip measures gzip decompression + JSON parsing.
func BenchmarkIngestParseGzip(b *testing.B) {
	events := make([]map[string]any, 100)
	for i := range events {
		events[i] = sampleEvent()
	}
	jsonData, _ := json.Marshal(events)

	var gzipBuf bytes.Buffer
	gz := gzip.NewWriter(&gzipBuf)
	gz.Write(jsonData)
	gz.Close()
	compressed := gzipBuf.Bytes()

	b.SetBytes(int64(len(jsonData)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		gr, err := gzip.NewReader(bytes.NewReader(compressed))
		if err != nil {
			b.Fatal(err)
		}
		decompressed, err := io.ReadAll(gr)
		gr.Close()
		if err != nil {
			b.Fatal(err)
		}
		var batch []map[string]any
		if err := json.Unmarshal(decompressed, &batch); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkIngestParseSingleSmall measures parsing a minimal event.
func BenchmarkIngestParseSingleSmall(b *testing.B) {
	data := []byte(`{"event_name":"heartbeat","service":"test","timestamp":"2026-01-01T00:00:00Z"}`)
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var evt map[string]any
		json.Unmarshal(data, &evt)
	}
}

// BenchmarkIngestParseLarge measures parsing a large event with many attributes.
func BenchmarkIngestParseLarge(b *testing.B) {
	evt := sampleEvent()
	for j := 0; j < 50; j++ {
		evt[fmt.Sprintf("attr_%d", j)] = fmt.Sprintf("value_%d", j)
	}
	data, _ := json.Marshal(evt)
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var parsed map[string]any
		json.Unmarshal(data, &parsed)
	}
}
