// Package logger provides structured JSON logging for the APIPulse backend.
//
// All log output is JSON-formatted and safe for ingestion by log aggregation
// systems (e.g. Loki, Datadog, Cloud Logging).
//
// SECURITY CONTRACT:
//   - Never log request or response bodies
//   - Never log authentication tokens, API keys, or secrets
//   - Never log PII or customer-identifiable data
//   - The request_id field uses a generated UUID, never a customer identifier
package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

// Level represents a log severity level.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "debug"
	case LevelInfo:
		return "info"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	default:
		return "unknown"
	}
}

// Logger is a structured JSON logger.
type Logger struct {
	out     io.Writer
	level   Level
	service string
}

// New creates a new Logger writing to os.Stdout.
func New(service, levelStr string) *Logger {
	return &Logger{
		out:     os.Stdout,
		service: service,
		level:   parseLevel(levelStr),
	}
}

// entry is the JSON log record structure.
type entry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Service   string `json:"service"`
	Component string `json:"component,omitempty"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
	Dependency string `json:"dependency,omitempty"`
	Region    string `json:"region,omitempty"`
	Error     string `json:"error,omitempty"`
}

// With returns a contextual sub-logger with fixed fields.
func (l *Logger) With(component string) *ContextLogger {
	return &ContextLogger{parent: l, component: component}
}

func (l *Logger) write(level Level, msg string, fields map[string]string) {
	if level < l.level {
		return
	}
	e := entry{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Level:     level.String(),
		Service:   l.service,
		Message:   msg,
	}
	if v, ok := fields["component"]; ok {
		e.Component = v
	}
	if v, ok := fields["request_id"]; ok {
		e.RequestID = v
	}
	if v, ok := fields["dependency"]; ok {
		e.Dependency = v
	}
	if v, ok := fields["region"]; ok {
		e.Region = v
	}
	if v, ok := fields["error"]; ok {
		e.Error = v
	}
	b, err := json.Marshal(e)
	if err != nil {
		fmt.Fprintf(l.out, `{"level":"error","message":"failed to marshal log entry"}`+"\n")
		return
	}
	fmt.Fprintf(l.out, "%s\n", b)
}

// Info logs at INFO level.
func (l *Logger) Info(msg string) { l.write(LevelInfo, msg, nil) }

// Error logs at ERROR level with an error value.
func (l *Logger) Error(msg string, err error) {
	fields := map[string]string{}
	if err != nil {
		fields["error"] = err.Error()
	}
	l.write(LevelError, msg, fields)
}

// Debug logs at DEBUG level.
func (l *Logger) Debug(msg string) { l.write(LevelDebug, msg, nil) }

// Warn logs at WARN level.
func (l *Logger) Warn(msg string) { l.write(LevelWarn, msg, nil) }

func parseLevel(s string) Level {
	switch s {
	case "debug":
		return LevelDebug
	case "warn":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

// ContextLogger is a logger with fixed component context.
type ContextLogger struct {
	parent    *Logger
	component string
	requestID string
	dependency string
	region    string
}

func (c *ContextLogger) fields() map[string]string {
	f := map[string]string{"component": c.component}
	if c.requestID != "" {
		f["request_id"] = c.requestID
	}
	if c.dependency != "" {
		f["dependency"] = c.dependency
	}
	if c.region != "" {
		f["region"] = c.region
	}
	return f
}

// WithRequestID returns a new ContextLogger with a request ID.
func (c *ContextLogger) WithRequestID(id string) *ContextLogger {
	nc := *c
	nc.requestID = id
	return &nc
}

// Info logs at INFO level with context.
func (c *ContextLogger) Info(msg string) { c.parent.write(LevelInfo, msg, c.fields()) }

// Error logs at ERROR level with context.
func (c *ContextLogger) Error(msg string, err error) {
	f := c.fields()
	if err != nil {
		f["error"] = err.Error()
	}
	c.parent.write(LevelError, msg, f)
}

// Warn logs at WARN level with context.
func (c *ContextLogger) Warn(msg string) { c.parent.write(LevelWarn, msg, c.fields()) }

// Debug logs at DEBUG level with context.
func (c *ContextLogger) Debug(msg string) { c.parent.write(LevelDebug, msg, c.fields()) }
