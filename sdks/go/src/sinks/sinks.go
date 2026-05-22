package sinks

import (
	"github.com/astraive/loxa/sdks/go/src/core"
)

// StdoutSink returns a core.Sink that writes NDJSON to os.Stdout.
func StdoutSink() core.Sink { return core.StdoutSink() }

// StderrSink returns a core.Sink that writes NDJSON to os.Stderr.
func StderrSink() core.Sink { return core.StderrSink() }

// FileSink returns a core.Sink that appends NDJSON to the file at path.
func FileSink(path string) (core.Sink, error) { return core.FileSink(path) }

// RotatingFileConfig configures the rotating file sink.
type RotatingFileConfig = core.RotatingFileConfig

// RotatingFileSink returns a core.Sink that rotates log files.
func RotatingFileSink(cfg RotatingFileConfig) (core.Sink, error) { return core.RotatingFileSink(cfg) }

// MemorySinkStore holds events written to a MemorySink.
type MemorySinkStore = core.MemorySinkStore

// MemorySink returns a core.Sink and the MemorySinkStore that collects events.
func MemorySink() (core.Sink, *MemorySinkStore) { return core.MemorySink() }

// NoopSink returns a core.Sink that discards all events.
func NoopSink() core.Sink { return core.NoopSink() }
