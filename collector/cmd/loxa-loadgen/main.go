package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	loxacollector "github.com/astraive/loxa-collector"
)

var version = loxacollector.Version

type Config struct {
	URL       string
	Events    int
	Workers   int
	BatchSize int
	APIKey    string
	BodySize  int
}

type Stats struct {
	accepted int64
	invalid  int64
	rejected int64
	errors   int64
	duration time.Duration
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "loxa-loadgen: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cfg := Config{
		URL:       "http://localhost:9308/ingest",
		Events:    10000,
		Workers:   4,
		BatchSize: 100,
		APIKey:    "",
		BodySize:  512,
	}

	fs := flag.NewFlagSet("loxa-loadgen", flag.ContinueOnError)
	url := fs.String("url", cfg.URL, "collector URL")
	events := fs.Int("events", cfg.Events, "total events to send")
	workers := fs.Int("workers", cfg.Workers, "concurrent workers")
	batchSize := fs.Int("batch", cfg.BatchSize, "batch size per request")
	apiKey := fs.String("api-key", cfg.APIKey, "API key for auth")
	bodySize := fs.Int("body-size", cfg.BodySize, "average body size in bytes")
	_ = fs.Parse(args)

	cfg.URL = *url
	cfg.Events = *events
	cfg.Workers = *workers
	cfg.BatchSize = *batchSize
	cfg.APIKey = *apiKey
	cfg.BodySize = *bodySize

	fmt.Printf("LOXA Load Generator v%s\n", version)
	fmt.Printf("Target: %s\n", cfg.URL)
	fmt.Printf("Events: %d, Workers: %d, Batch: %d\n", cfg.Events, cfg.Workers, cfg.BatchSize)
	fmt.Println()

	return runLoadTest(cfg)
}

func runLoadTest(cfg Config) error {
	client := &http.Client{Timeout: 30 * time.Second}
	stats := &Stats{}

	// Graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nShutting down...")
		cancel()
	}()

	start := time.Now()

	var wg sync.WaitGroup
	eventsPerWorker := cfg.Events / cfg.Workers

	for w := 0; w < cfg.Workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			runWorker(ctx, client, cfg, workerID, eventsPerWorker, stats)
		}(w)
	}

	// Wait for completion or cancellation
	wg.Wait()

	stats.duration = time.Since(start)

	// Print results
	fmt.Println()
	fmt.Println("=== Results ===")
	fmt.Printf("Duration:    %v\n", stats.duration)
	fmt.Printf("Accepted:    %d\n", stats.accepted)
	fmt.Printf("Invalid:     %d\n", stats.invalid)
	fmt.Printf("Rejected:    %d\n", stats.rejected)
	fmt.Printf("Errors:      %d\n", stats.errors)
	fmt.Printf("Throughput:  %.2f events/sec\n", float64(stats.accepted)/stats.duration.Seconds())

	return nil
}

func runWorker(ctx context.Context, client *http.Client, cfg Config, workerID, count int, stats *Stats) {
	rng := rand.New(rand.NewSource(int64(workerID)))
	batch := make([][]byte, 0, cfg.BatchSize)

	for i := 0; i < count && ctx.Err() == nil; i++ {
		event := generateEvent(rng, cfg.BodySize)
		batch = append(batch, event)

		if len(batch) >= cfg.BatchSize || i == count-1 {
			sendBatch(ctx, client, cfg, batch, stats)
			batch = batch[:0]
		}
	}
}

func generateEvent(rng *rand.Rand, bodySize int) []byte {
	event := map[string]any{
		"event_id":    fmt.Sprintf("evt-%d-%d", rng.Int63(), time.Now().UnixNano()),
		"event":       "loadtest.event",
		"service":     "loadtest",
		"timestamp":   time.Now().UTC().Format(time.RFC3339Nano),
		"method":      "GET",
		"path":        fmt.Sprintf("/api/test/%d", rng.Intn(1000)),
		"status_code": 200,
		"duration_ms": rng.Intn(500),
		"user_id":     fmt.Sprintf("user-%d", rng.Intn(10000)),
		"payload":     randomString(rng, bodySize),
	}
	data, _ := json.Marshal(event)
	return data
}

func randomString(rng *rand.Rand, n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rng.Intn(len(letters))]
	}
	return string(b)
}

func sendBatch(ctx context.Context, client *http.Client, cfg Config, batch [][]byte, stats *Stats) {
	if len(batch) == 0 {
		return
	}

	body, _ := json.Marshal(map[string]any{"events": batch})
	req, _ := http.NewRequestWithContext(ctx, "POST", cfg.URL, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if cfg.APIKey != "" {
		req.Header.Set("X-API-Key", cfg.APIKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		atomic.AddInt64(&stats.errors, int64(len(batch)))
		return
	}
	defer resp.Body.Close()

	var result struct {
		Accepted int `json:"accepted"`
		Invalid  int `json:"invalid"`
		Rejected int `json:"rejected"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&result)

	atomic.AddInt64(&stats.accepted, int64(result.Accepted))
	atomic.AddInt64(&stats.invalid, int64(result.Invalid))
	atomic.AddInt64(&stats.rejected, int64(result.Rejected))
}
