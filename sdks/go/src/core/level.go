package core

import "strings"

// Level represents the severity level of a log event.
type Level uint8

const (
	LevelDebug Level = iota
	LevelInfo
	LevelNotice
	LevelWarn
	LevelError
	LevelFatal
)

// String returns the lower-case string representation of the level.
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "debug"
	case LevelInfo:
		return "info"
	case LevelNotice:
		return "notice"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	case LevelFatal:
		return "fatal"
	default:
		return "info"
	}
}

// ParseLevel parses a string into a Level. Defaults to LevelInfo on unknown input.
func ParseLevel(s string) Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "notice":
		return LevelNotice
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	case "fatal":
		return LevelFatal
	default:
		return LevelInfo
	}
}

// Parse parses a string into a Level. Defaults to LevelInfo on unknown input.
func Parse(s string) Level {
	return ParseLevel(s)
}
