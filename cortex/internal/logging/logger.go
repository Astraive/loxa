package logging

import (
	"io"
	"os"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Logger wraps zerolog.Logger with additional context
type Logger struct {
	logger zerolog.Logger
}

// New creates a new logger with the specified configuration
func New(level, format string) *Logger {
	// Set log level
	logLevel := parseLevel(level)
	zerolog.SetGlobalLevel(logLevel)

	// Configure output format
	var output io.Writer = os.Stdout
	if format == "console" {
		output = zerolog.ConsoleWriter{Out: os.Stdout}
	}

	// Create logger
	logger := zerolog.New(output).With().Timestamp().Logger()

	return &Logger{logger: logger}
}

// parseLevel converts string level to zerolog.Level
func parseLevel(level string) zerolog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return zerolog.DebugLevel
	case "info":
		return zerolog.InfoLevel
	case "warn":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	default:
		return zerolog.InfoLevel
	}
}

// Debug logs a debug message
func (l *Logger) Debug(msg string) {
	l.logger.Debug().Msg(msg)
}

// Debugf logs a formatted debug message
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.logger.Debug().Msgf(format, args...)
}

// Info logs an info message
func (l *Logger) Info(msg string) {
	l.logger.Info().Msg(msg)
}

// Infof logs a formatted info message
func (l *Logger) Infof(format string, args ...interface{}) {
	l.logger.Info().Msgf(format, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string) {
	l.logger.Warn().Msg(msg)
}

// Warnf logs a formatted warning message
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.logger.Warn().Msgf(format, args...)
}

// Error logs an error message
func (l *Logger) Error(msg string) {
	l.logger.Error().Msg(msg)
}

// Errorf logs a formatted error message
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.logger.Error().Msgf(format, args...)
}

// ErrorWithErr logs an error message with an error object
func (l *Logger) ErrorWithErr(err error, msg string) {
	l.logger.Error().Err(err).Msg(msg)
}

// WithField returns a logger with an additional field
func (l *Logger) WithField(key string, value interface{}) *Logger {
	return &Logger{logger: l.logger.With().Interface(key, value).Logger()}
}

// WithFields returns a logger with additional fields
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	ctx := l.logger.With()
	for k, v := range fields {
		ctx = ctx.Interface(k, v)
	}
	return &Logger{logger: ctx.Logger()}
}

// LogAuthFailure logs an authentication failure event
func (l *Logger) LogAuthFailure(apiKeyName, remoteAddr, reason string) {
	l.logger.Warn().
		Str("event_type", "auth_failure").
		Str("api_key_name", apiKeyName).
		Str("remote_addr", remoteAddr).
		Str("reason", reason).
		Msg("Authentication failed")
}

// LogAuthzFailure logs an authorization failure event
func (l *Logger) LogAuthzFailure(apiKeyName, requestedPath, reason string) {
	l.logger.Warn().
		Str("event_type", "authz_failure").
		Str("api_key_name", apiKeyName).
		Str("requested_path", requestedPath).
		Str("reason", reason).
		Msg("Authorization failed")
}

// LogRateLimitExceeded logs a rate limit exceeded event
func (l *Logger) LogRateLimitExceeded(apiKeyName, remoteAddr string) {
	l.logger.Warn().
		Str("event_type", "rate_limit_exceeded").
		Str("api_key_name", apiKeyName).
		Str("remote_addr", remoteAddr).
		Msg("Rate limit exceeded")
}

// GetZerologLogger returns the underlying zerolog.Logger for advanced usage
func (l *Logger) GetZerologLogger() zerolog.Logger {
	return l.logger
}

// Global returns the global logger instance
func Global() *Logger {
	return &Logger{logger: log.Logger}
}
