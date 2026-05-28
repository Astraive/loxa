package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const fallbackVersion = "0.2.5"

func loadVersion() string {
	candidates := []string{
		"loxa-collector.metadata.yaml",
		"../loxa-collector.metadata.yaml",
		"../../loxa-collector.metadata.yaml",
		"../../../loxa-collector.metadata.yaml",
	}
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "version:") {
				value := strings.TrimSpace(strings.TrimPrefix(trimmed, "version:"))
				value = strings.Trim(value, "\"'")
				if value != "" {
					return value
				}
			}
		}
	}
	return fallbackVersion
}

type Result struct {
	StatusCode int
	Duration   time.Duration
	Error      string
	Bytes      int
}

type TestResult struct {
	Name        string
	Concurrency int
	Duration    time.Duration
	TotalReqs   int64
	SuccessReqs int64
	ErrorReqs   int64
	StatusCodes map[int]int64
	AvgLatency  time.Duration
	P50Latency  time.Duration
	P95Latency  time.Duration
	P99Latency  time.Duration
	MaxLatency  time.Duration
	MinLatency  time.Duration
	RPS         float64
	Errors      []string
}

func (tr *TestResult) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n=== %s ===\n", tr.Name))
	sb.WriteString(fmt.Sprintf("Concurrency: %d\n", tr.Concurrency))
	sb.WriteString(fmt.Sprintf("Duration:    %v\n", tr.Duration))
	sb.WriteString(fmt.Sprintf("Total Reqs:  %d\n", tr.TotalReqs))
	sb.WriteString(fmt.Sprintf("Success:     %d\n", tr.SuccessReqs))
	sb.WriteString(fmt.Sprintf("Errors:      %d\n", tr.ErrorReqs))
	sb.WriteString(fmt.Sprintf("RPS:         %.2f\n", tr.RPS))
	sb.WriteString(fmt.Sprintf("Avg Latency: %v\n", tr.AvgLatency))
	sb.WriteString(fmt.Sprintf("P50 Latency: %v\n", tr.P50Latency))
	sb.WriteString(fmt.Sprintf("P95 Latency: %v\n", tr.P95Latency))
	sb.WriteString(fmt.Sprintf("P99 Latency: %v\n", tr.P99Latency))
	sb.WriteString(fmt.Sprintf("Max Latency: %v\n", tr.MaxLatency))
	sb.WriteString(fmt.Sprintf("Min Latency: %v\n", tr.MinLatency))
	sb.WriteString("Status Codes:\n")
	for code, count := range tr.StatusCodes {
		sb.WriteString(fmt.Sprintf("  %d: %d\n", code, count))
	}
	if len(tr.Errors) > 0 {
		sb.WriteString("Sample Errors:\n")
		for i, e := range tr.Errors {
			if i >= 5 {
				break
			}
			sb.WriteString(fmt.Sprintf("  - %s\n", e))
		}
	}
	return sb.String()
}

func runLoadTest(name string, concurrency int, duration time.Duration, reqFunc func() Result) *TestResult {
	results := make(chan Result, concurrency*100)
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	var wg sync.WaitGroup
	var totalReqs int64

	// Start workers
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					r := reqFunc()
					atomic.AddInt64(&totalReqs, 1)
					select {
					case results <- r:
					default:
						// Drop if channel full
					}
				}
			}
		}()
	}

	// Collect results
	var collected []Result
	done := make(chan struct{})
	go func() {
		for r := range results {
			collected = append(collected, r)
		}
		close(done)
	}()

	wg.Wait()
	close(results)
	<-done

	// Calculate stats
	tr := &TestResult{
		Name:        name,
		Concurrency: concurrency,
		Duration:    duration,
		StatusCodes: make(map[int]int64),
		MinLatency:  time.Hour,
	}

	latencies := make([]time.Duration, 0, len(collected))
	for _, r := range collected {
		tr.TotalReqs++
		if r.Error != "" {
			tr.ErrorReqs++
			if len(tr.Errors) < 10 {
				tr.Errors = append(tr.Errors, r.Error)
			}
		} else if r.StatusCode >= 200 && r.StatusCode < 300 {
			tr.SuccessReqs++
		} else {
			tr.ErrorReqs++
		}
		tr.StatusCodes[r.StatusCode]++
		latencies = append(latencies, r.Duration)
		if r.Duration > tr.MaxLatency {
			tr.MaxLatency = r.Duration
		}
		if r.Duration < tr.MinLatency && r.Duration > 0 {
			tr.MinLatency = r.Duration
		}
	}

	if len(latencies) > 0 {
		sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
		var total time.Duration
		for _, l := range latencies {
			total += l
		}
		tr.AvgLatency = total / time.Duration(len(latencies))
		tr.P50Latency = latencies[len(latencies)*50/100]
		tr.P95Latency = latencies[len(latencies)*95/100]
		tr.P99Latency = latencies[len(latencies)*99/100]
		tr.RPS = float64(tr.TotalReqs) / duration.Seconds()
	}

	return tr
}

func sampleEvent(id int) map[string]any {
	return map[string]any{
		"event_id":      fmt.Sprintf("evt_stress_%06d", id),
		"event":         "checkout.completed",
		"event_version": "v1",
		"timestamp":     time.Now().UTC().Format(time.RFC3339Nano),
		"service":       "stress-test-api",
		"level":         "info",
		"outcome":       "success",
		"kind":          "event",
		"schema_version": "v1",
		"duration_ms":   rand.Intn(500) + 10,
		"http": map[string]any{
			"method": "POST",
			"path":   "/api/checkout",
			"status": 200,
		},
		"user": map[string]any{
			"id":    fmt.Sprintf("usr_%d", rand.Intn(10000)),
			"email": fmt.Sprintf("user%d@example.com", rand.Intn(10000)),
		},
		"payment": map[string]any{
			"amount":   float64(rand.Intn(10000)) / 100.0,
			"currency": "USD",
			"provider": "stripe",
		},
	}
}

func main() {
	// Limit OS threads on Windows to avoid thread exhaustion
	runtime.GOMAXPROCS(32)
	collectorURL := "http://127.0.0.1:9308"
	cortexURL := "http://127.0.0.1:9312"
	// Use connection pooling to avoid Windows thread exhaustion
	transport := &http.Transport{
		MaxIdleConns:        200,
		MaxIdleConnsPerHost: 200,
		MaxConnsPerHost:     200,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false,
	}
	client := &http.Client{Timeout: 30 * time.Second, Transport: transport}

	var allResults []*TestResult
	jsonResults := make([]map[string]any, 0)

	recordResult := func(tr *TestResult) {
		allResults = append(allResults, tr)
		fmt.Print(tr)
		jr := map[string]any{
			"name":        tr.Name,
			"concurrency": tr.Concurrency,
			"duration_s":  tr.Duration.Seconds(),
			"total_reqs":  tr.TotalReqs,
			"success":     tr.SuccessReqs,
			"errors":      tr.ErrorReqs,
			"rps":         tr.RPS,
			"avg_ms":      float64(tr.AvgLatency.Microseconds()) / 1000.0,
			"p50_ms":      float64(tr.P50Latency.Microseconds()) / 1000.0,
			"p95_ms":      float64(tr.P95Latency.Microseconds()) / 1000.0,
			"p99_ms":      float64(tr.P99Latency.Microseconds()) / 1000.0,
			"max_ms":      float64(tr.MaxLatency.Microseconds()) / 1000.0,
			"status_codes": tr.StatusCodes,
		}
		jsonResults = append(jsonResults, jr)
	}

	fmt.Println("========================================")
	fmt.Printf("LOXA v%s STRESS TEST SUITE\n", loadVersion())
	fmt.Printf("Collector: %s\n", collectorURL)
	fmt.Printf("Cortex:    %s\n", cortexURL)
	fmt.Printf("Started:   %s\n", time.Now().Format(time.RFC3339))
	fmt.Println("========================================")

	// === PHASE 1: WARMUP ===
	fmt.Println("\n--- PHASE 1: WARMUP ---")

	// Warmup: Health endpoints
	recordResult(runLoadTest("Warmup: Collector /health", 2, 5*time.Second, func() Result {
		start := time.Now()
		resp, err := client.Get(collectorURL + "/health")
		if err != nil {
			return Result{Duration: time.Since(start), Error: err.Error()}
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return Result{StatusCode: resp.StatusCode, Duration: time.Since(start), Bytes: len(body)}
	}))

	recordResult(runLoadTest("Warmup: Cortex /healthz", 2, 5*time.Second, func() Result {
		start := time.Now()
		resp, err := client.Get(cortexURL + "/healthz")
		if err != nil {
			return Result{Duration: time.Since(start), Error: err.Error()}
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return Result{StatusCode: resp.StatusCode, Duration: time.Since(start), Bytes: len(body)}
	}))

	// === PHASE 2: LIGHT STRESS (10 concurrent, 30s) ===
	fmt.Println("\n--- PHASE 2: LIGHT STRESS (10 concurrent, 30s) ---")

	recordResult(runLoadTest("Collector /health [10c/30s]", 10, 30*time.Second, func() Result {
		start := time.Now()
		resp, err := client.Get(collectorURL + "/health")
		if err != nil {
			return Result{Duration: time.Since(start), Error: err.Error()}
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return Result{StatusCode: resp.StatusCode, Duration: time.Since(start), Bytes: len(body)}
	}))

	recordResult(runLoadTest("Collector /status [10c/30s]", 10, 30*time.Second, func() Result {
		start := time.Now()
		resp, err := client.Get(collectorURL + "/status")
		if err != nil {
			return Result{Duration: time.Since(start), Error: err.Error()}
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return Result{StatusCode: resp.StatusCode, Duration: time.Since(start), Bytes: len(body)}
	}))

	recordResult(runLoadTest("Collector /version [10c/30s]", 10, 30*time.Second, func() Result {
		start := time.Now()
		resp, err := client.Get(collectorURL + "/version")
		if err != nil {
			return Result{Duration: time.Since(start), Error: err.Error()}
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return Result{StatusCode: resp.StatusCode, Duration: time.Since(start), Bytes: len(body)}
	}))

	recordResult(runLoadTest("Cortex /healthz [10c/30s]", 10, 30*time.Second, func() Result {
		start := time.Now()
		resp, err := client.Get(cortexURL + "/healthz")
		if err != nil {
			return Result{Duration: time.Since(start), Error: err.Error()}
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return Result{StatusCode: resp.StatusCode, Duration: time.Since(start), Bytes: len(body)}
	}))

	// === PHASE 3: INGEST STRESS ===
	fmt.Println("\n--- PHASE 3: INGEST STRESS ---")

	// Single event ingest - light
	recordResult(runLoadTest("Collector /ingest single [10c/30s]", 10, 30*time.Second, func() Result {
		evt := sampleEvent(rand.Intn(1000000))
		data, _ := json.Marshal(evt)
		start := time.Now()
		resp, err := client.Post(collectorURL+"/ingest", "application/x-ndjson", bytes.NewReader(data))
		if err != nil {
			return Result{Duration: time.Since(start), Error: err.Error()}
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return Result{StatusCode: resp.StatusCode, Duration: time.Since(start), Bytes: len(body)}
	}))

	// Batch NDJSON ingest - 10 events per request
	recordResult(runLoadTest("Collector /ingest batch-10 [10c/30s]", 10, 30*time.Second, func() Result {
		var buf bytes.Buffer
		for i := 0; i < 10; i++ {
			evt := sampleEvent(rand.Intn(1000000))
			data, _ := json.Marshal(evt)
			buf.Write(data)
			buf.WriteByte('\n')
		}
		start := time.Now()
		resp, err := client.Post(collectorURL+"/ingest", "application/x-ndjson", &buf)
		if err != nil {
			return Result{Duration: time.Since(start), Error: err.Error()}
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return Result{StatusCode: resp.StatusCode, Duration: time.Since(start), Bytes: len(body)}
	}))

	// Batch NDJSON ingest - 100 events per request
	recordResult(runLoadTest("Collector /ingest batch-100 [10c/30s]", 10, 30*time.Second, func() Result {
		var buf bytes.Buffer
		for i := 0; i < 100; i++ {
			evt := sampleEvent(rand.Intn(1000000))
			data, _ := json.Marshal(evt)
			buf.Write(data)
			buf.WriteByte('\n')
		}
		start := time.Now()
		resp, err := client.Post(collectorURL+"/ingest", "application/x-ndjson", &buf)
		if err != nil {
			return Result{Duration: time.Since(start), Error: err.Error()}
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return Result{StatusCode: resp.StatusCode, Duration: time.Since(start), Bytes: len(body)}
	}))

	// === PHASE 4: MEDIUM STRESS (25 concurrent) ===
	fmt.Println("\n--- PHASE 4: MEDIUM STRESS (25 concurrent, 30s) ---")

	recordResult(runLoadTest("Collector /health [25c/30s]", 25, 30*time.Second, func() Result {
		start := time.Now()
		resp, err := client.Get(collectorURL + "/health")
		if err != nil {
			return Result{Duration: time.Since(start), Error: err.Error()}
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return Result{StatusCode: resp.StatusCode, Duration: time.Since(start), Bytes: len(body)}
	}))

	recordResult(runLoadTest("Collector /ingest batch-10 [25c/30s]", 25, 30*time.Second, func() Result {
		var buf bytes.Buffer
		for i := 0; i < 10; i++ {
			evt := sampleEvent(rand.Intn(1000000))
			data, _ := json.Marshal(evt)
			buf.Write(data)
			buf.WriteByte('\n')
		}
		start := time.Now()
		resp, err := client.Post(collectorURL+"/ingest", "application/x-ndjson", &buf)
		if err != nil {
			return Result{Duration: time.Since(start), Error: err.Error()}
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return Result{StatusCode: resp.StatusCode, Duration: time.Since(start), Bytes: len(body)}
	}))

	recordResult(runLoadTest("Cortex /healthz [25c/30s]", 25, 30*time.Second, func() Result {
		start := time.Now()
		resp, err := client.Get(cortexURL + "/healthz")
		if err != nil {
			return Result{Duration: time.Since(start), Error: err.Error()}
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return Result{StatusCode: resp.StatusCode, Duration: time.Since(start), Bytes: len(body)}
	}))

	// === PHASE 5: HIGH STRESS (30 concurrent, 60s) ===
	fmt.Println("\n--- PHASE 5: HIGH STRESS (30 concurrent, 60s) ---")

	recordResult(runLoadTest("Collector /health [30c/60s]", 30, 60*time.Second, func() Result {
		start := time.Now()
		resp, err := client.Get(collectorURL + "/health")
		if err != nil {
			return Result{Duration: time.Since(start), Error: err.Error()}
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return Result{StatusCode: resp.StatusCode, Duration: time.Since(start), Bytes: len(body)}
	}))

	recordResult(runLoadTest("Collector /ingest batch-10 [30c/60s]", 30, 60*time.Second, func() Result {
		var buf bytes.Buffer
		for i := 0; i < 10; i++ {
			evt := sampleEvent(rand.Intn(1000000))
			data, _ := json.Marshal(evt)
			buf.Write(data)
			buf.WriteByte('\n')
		}
		start := time.Now()
		resp, err := client.Post(collectorURL+"/ingest", "application/x-ndjson", &buf)
		if err != nil {
			return Result{Duration: time.Since(start), Error: err.Error()}
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return Result{StatusCode: resp.StatusCode, Duration: time.Since(start), Bytes: len(body)}
	}))

	recordResult(runLoadTest("Collector /ingest batch-100 [30c/60s]", 30, 60*time.Second, func() Result {
		var buf bytes.Buffer
		for i := 0; i < 100; i++ {
			evt := sampleEvent(rand.Intn(1000000))
			data, _ := json.Marshal(evt)
			buf.Write(data)
			buf.WriteByte('\n')
		}
		start := time.Now()
		resp, err := client.Post(collectorURL+"/ingest", "application/x-ndjson", &buf)
		if err != nil {
			return Result{Duration: time.Since(start), Error: err.Error()}
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return Result{StatusCode: resp.StatusCode, Duration: time.Since(start), Bytes: len(body)}
	}))

	// === PHASE 6: LARGE PAYLOAD TESTS ===
	fmt.Println("\n--- PHASE 6: LARGE PAYLOAD TESTS ---")

	// 500 events per request (large batch)
	recordResult(runLoadTest("Collector /ingest batch-500 [5c/15s]", 5, 15*time.Second, func() Result {
		var buf bytes.Buffer
		for i := 0; i < 500; i++ {
			evt := sampleEvent(rand.Intn(1000000))
			data, _ := json.Marshal(evt)
			buf.Write(data)
			buf.WriteByte('\n')
		}
		start := time.Now()
		resp, err := client.Post(collectorURL+"/ingest", "application/x-ndjson", &buf)
		if err != nil {
			return Result{Duration: time.Since(start), Error: err.Error()}
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return Result{StatusCode: resp.StatusCode, Duration: time.Since(start), Bytes: len(body)}
	}))

	// 1000 events per request (max batch)
	recordResult(runLoadTest("Collector /ingest batch-1000 [5c/15s]", 5, 15*time.Second, func() Result {
		var buf bytes.Buffer
		for i := 0; i < 1000; i++ {
			evt := sampleEvent(rand.Intn(1000000))
			data, _ := json.Marshal(evt)
			buf.Write(data)
			buf.WriteByte('\n')
		}
		start := time.Now()
		resp, err := client.Post(collectorURL+"/ingest", "application/x-ndjson", &buf)
		if err != nil {
			return Result{Duration: time.Since(start), Error: err.Error()}
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return Result{StatusCode: resp.StatusCode, Duration: time.Since(start), Bytes: len(body)}
	}))

	// Large event with many attributes (stress validation)
	recordResult(runLoadTest("Collector /ingest large-event [5c/15s]", 5, 15*time.Second, func() Result {
		evt := sampleEvent(rand.Intn(1000000))
		// Add many nested attributes
		for i := 0; i < 50; i++ {
			evt[fmt.Sprintf("custom_attr_%d", i)] = fmt.Sprintf("value_%d_%s", i, strings.Repeat("x", 200))
		}
		data, _ := json.Marshal(evt)
		start := time.Now()
		resp, err := client.Post(collectorURL+"/ingest", "application/x-ndjson", bytes.NewReader(data))
		if err != nil {
			return Result{Duration: time.Since(start), Error: err.Error()}
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return Result{StatusCode: resp.StatusCode, Duration: time.Since(start), Bytes: len(body)}
	}))

	// === PHASE 7: QUERY PRESSURE ===
	fmt.Println("\n--- PHASE 7: QUERY PRESSURE ---")

	// Query endpoint stress (after ingesting data)
	recordResult(runLoadTest("Collector /query [10c/30s]", 10, 30*time.Second, func() Result {
		queryBody := map[string]any{
			"sql":    "SELECT count(*) FROM events WHERE service = 'stress-test-api' LIMIT 10",
			"format": "json",
		}
		data, _ := json.Marshal(queryBody)
		start := time.Now()
		resp, err := client.Post(collectorURL+"/query", "application/json", bytes.NewReader(data))
		if err != nil {
			return Result{Duration: time.Since(start), Error: err.Error()}
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return Result{StatusCode: resp.StatusCode, Duration: time.Since(start), Bytes: len(body)}
	}))

	// LQL query stress
	recordResult(runLoadTest("Collector /lql/query [10c/30s]", 10, 30*time.Second, func() Result {
		queryBody := map[string]any{
			"query":  "events | where service == 'stress-test-api' | limit 10",
			"format": "json",
		}
		data, _ := json.Marshal(queryBody)
		start := time.Now()
		resp, err := client.Post(collectorURL+"/lql/query", "application/json", bytes.NewReader(data))
		if err != nil {
			return Result{Duration: time.Since(start), Error: err.Error()}
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return Result{StatusCode: resp.StatusCode, Duration: time.Since(start), Bytes: len(body)}
	}))

	// Cortex graph query stress
	recordResult(runLoadTest("Cortex /graph/service [10c/30s]", 10, 30*time.Second, func() Result {
		start := time.Now()
		resp, err := client.Get(cortexURL + "/graph/service/stress-test-api")
		if err != nil {
			return Result{Duration: time.Since(start), Error: err.Error()}
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return Result{StatusCode: resp.StatusCode, Duration: time.Since(start), Bytes: len(body)}
	}))

	// === PHASE 8: ERROR PRESSURE ===
	fmt.Println("\n--- PHASE 8: ERROR PRESSURE ---")

	// Invalid JSON
	recordResult(runLoadTest("Collector /ingest invalid-JSON [5c/15s]", 5, 15*time.Second, func() Result {
		start := time.Now()
		resp, err := client.Post(collectorURL+"/ingest", "application/json", bytes.NewReader([]byte("{invalid json")))
		if err != nil {
			return Result{Duration: time.Since(start), Error: err.Error()}
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return Result{StatusCode: resp.StatusCode, Duration: time.Since(start), Bytes: len(body)}
	}))

	// Empty body
	recordResult(runLoadTest("Collector /ingest empty-body [5c/15s]", 5, 15*time.Second, func() Result {
		start := time.Now()
		resp, err := client.Post(collectorURL+"/ingest", "application/json", bytes.NewReader([]byte("")))
		if err != nil {
			return Result{Duration: time.Since(start), Error: err.Error()}
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return Result{StatusCode: resp.StatusCode, Duration: time.Since(start), Bytes: len(body)}
	}))

	// Wrong method
	recordResult(runLoadTest("Collector GET /ingest [5c/15s]", 5, 15*time.Second, func() Result {
		start := time.Now()
		resp, err := client.Get(collectorURL + "/ingest")
		if err != nil {
			return Result{Duration: time.Since(start), Error: err.Error()}
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return Result{StatusCode: resp.StatusCode, Duration: time.Since(start), Bytes: len(body)}
	}))

	// === PHASE 9: BURST TRAFFIC ===
	fmt.Println("\n--- PHASE 9: BURST TRAFFIC (40 concurrent, 15s) ---")

	recordResult(runLoadTest("Collector /health BURST [40c/15s]", 40, 15*time.Second, func() Result {
		start := time.Now()
		resp, err := client.Get(collectorURL + "/health")
		if err != nil {
			return Result{Duration: time.Since(start), Error: err.Error()}
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return Result{StatusCode: resp.StatusCode, Duration: time.Since(start), Bytes: len(body)}
	}))

	recordResult(runLoadTest("Collector /ingest BURST batch-10 [40c/15s]", 40, 15*time.Second, func() Result {
		var buf bytes.Buffer
		for i := 0; i < 10; i++ {
			evt := sampleEvent(rand.Intn(1000000))
			data, _ := json.Marshal(evt)
			buf.Write(data)
			buf.WriteByte('\n')
		}
		start := time.Now()
		resp, err := client.Post(collectorURL+"/ingest", "application/x-ndjson", &buf)
		if err != nil {
			return Result{Duration: time.Since(start), Error: err.Error()}
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return Result{StatusCode: resp.StatusCode, Duration: time.Since(start), Bytes: len(body)}
	}))

	// === PHASE 10: MIXED WORKLOAD ===
	fmt.Println("\n--- PHASE 10: MIXED WORKLOAD (30c, 30s, mixed ops) ---")

	recordResult(runLoadTest("Mixed: ingest+query+health [30c/30s]", 30, 30*time.Second, func() Result {
		r := rand.Intn(100)
		var start time.Time
		var resp *http.Response
		var err error

		switch {
		case r < 60: // 60% ingest
			var buf bytes.Buffer
			for i := 0; i < 10; i++ {
				evt := sampleEvent(rand.Intn(1000000))
				data, _ := json.Marshal(evt)
				buf.Write(data)
				buf.WriteByte('\n')
			}
			start = time.Now()
			resp, err = client.Post(collectorURL+"/ingest", "application/x-ndjson", &buf)
		case r < 80: // 20% query
			queryBody := map[string]any{
				"sql":    "SELECT count(*) FROM events LIMIT 1",
				"format": "json",
			}
			data, _ := json.Marshal(queryBody)
			start = time.Now()
			resp, err = client.Post(collectorURL+"/query", "application/json", bytes.NewReader(data))
		default: // 20% health/status
			endpoints := []string{"/health", "/status", "/version"}
			ep := endpoints[rand.Intn(len(endpoints))]
			start = time.Now()
			resp, err = client.Get(collectorURL + ep)
		}

		if err != nil {
			return Result{Duration: time.Since(start), Error: err.Error()}
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return Result{StatusCode: resp.StatusCode, Duration: time.Since(start), Bytes: len(body)}
	}))

	// === PHASE 11: SUSTAINED LOAD ===
	fmt.Println("\n--- PHASE 11: SUSTAINED LOAD (20c, 120s, ingest) ---")

	recordResult(runLoadTest("Sustained: Collector /ingest [20c/120s]", 20, 120*time.Second, func() Result {
		var buf bytes.Buffer
		for i := 0; i < 10; i++ {
			evt := sampleEvent(rand.Intn(1000000))
			data, _ := json.Marshal(evt)
			buf.Write(data)
			buf.WriteByte('\n')
		}
		start := time.Now()
		resp, err := client.Post(collectorURL+"/ingest", "application/x-ndjson", &buf)
		if err != nil {
			return Result{Duration: time.Since(start), Error: err.Error()}
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return Result{StatusCode: resp.StatusCode, Duration: time.Since(start), Bytes: len(body)}
	}))

	// Final health check
	fmt.Println("\n--- FINAL HEALTH CHECK ---")
	resp, err := client.Get(collectorURL + "/health")
	if err != nil {
		fmt.Printf("Collector health check FAILED: %v\n", err)
	} else {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		fmt.Printf("Collector /health: %d - %s\n", resp.StatusCode, string(body))
	}

	resp, err = client.Get(collectorURL + "/status")
	if err != nil {
		fmt.Printf("Collector status check FAILED: %v\n", err)
	} else {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		fmt.Printf("Collector /status: %d - %s\n", resp.StatusCode, string(body))
	}

	resp, err = client.Get(cortexURL + "/healthz")
	if err != nil {
		fmt.Printf("Cortex health check FAILED: %v\n", err)
	} else {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		fmt.Printf("Cortex /healthz: %d - %s\n", resp.StatusCode, string(body))
	}

	// === SUMMARY ===
	fmt.Println("\n========================================")
	fmt.Println("STRESS TEST SUMMARY")
	fmt.Println("========================================")
	var totalReqs, totalErrors int64
	var totalRPS float64
	for _, tr := range allResults {
		totalReqs += tr.TotalReqs
		totalErrors += tr.ErrorReqs
		totalRPS += tr.RPS
		status := "PASS"
		if tr.ErrorReqs > 0 && float64(tr.ErrorReqs)/float64(tr.TotalReqs) > 0.01 {
			status = "FAIL"
		}
		fmt.Printf("%-55s RPS: %8.1f  Errors: %6d/%6d  P95: %v  [%s]\n",
			tr.Name, tr.RPS, tr.ErrorReqs, tr.TotalReqs, tr.P95Latency, status)
	}
	fmt.Printf("\nTotal Requests: %d\n", totalReqs)
	fmt.Printf("Total Errors:   %d\n", totalErrors)
	fmt.Printf("Error Rate:     %.4f%%\n", float64(totalErrors)/float64(totalReqs)*100)

	// Write JSON results
	jsonData, _ := json.MarshalIndent(map[string]any{
		"date":        time.Now().Format(time.RFC3339),
		"target":      "localhost",
		"collector":   collectorURL,
		"cortex":      cortexURL,
		"total_reqs":  totalReqs,
		"total_errors": totalErrors,
		"error_rate":  float64(totalErrors) / float64(totalReqs) * 100,
		"tests":       jsonResults,
	}, "", "  ")
	os.WriteFile("/e/astraive/loxa/docs/stress-test-results.json", jsonData, 0644)
	fmt.Println("\nJSON results written to docs/stress-test-results.json")
}
