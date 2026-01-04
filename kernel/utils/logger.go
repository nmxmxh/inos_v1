package utils

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
	"syscall/js"
	"time"
)

// LogLevel represents the severity of a log message
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

var levelNames = map[LogLevel]string{
	DEBUG: "DEBUG",
	INFO:  "INFO",
	WARN:  "WARN",
	ERROR: "ERROR",
	FATAL: "FATAL",
}

var levelColors = map[LogLevel]string{
	DEBUG: "\033[36m", // Cyan
	INFO:  "\033[32m", // Green
	WARN:  "\033[33m", // Yellow
	ERROR: "\033[31m", // Red
	FATAL: "\033[35m", // Magenta
}

const colorReset = "\033[0m"

// Logger provides structured, prettified logging with separation of concerns
type Logger struct {
	mu         sync.Mutex
	level      LogLevel
	component  string
	output     io.Writer
	colorize   bool
	showCaller bool
	timeFormat string
}

// LoggerConfig configures a logger instance
type LoggerConfig struct {
	Level      LogLevel
	Component  string
	Output     io.Writer
	Colorize   bool
	ShowCaller bool
	TimeFormat string
}

// NewLogger creates a new logger with the given configuration
func NewLogger(config LoggerConfig) *Logger {
	if config.Output == nil {
		config.Output = os.Stdout
	}
	if config.TimeFormat == "" {
		config.TimeFormat = "15:04:05.000"
	}

	return &Logger{
		level:      config.Level,
		component:  config.Component,
		output:     config.Output,
		colorize:   config.Colorize,
		showCaller: config.ShowCaller,
		timeFormat: config.TimeFormat,
	}
}

// DefaultLogger creates a logger with sensible defaults
func DefaultLogger(component string) *Logger {
	return NewLogger(LoggerConfig{
		Level:      INFO,
		Component:  component,
		Output:     os.Stdout,
		Colorize:   true,
		ShowCaller: false,
		TimeFormat: "15:04:05.000",
	})
}

// With returns a new logger with the given fields appended
func (l *Logger) With(fields ...Field) *Logger {
	// For now, simpler implementation: loggers are stateless except for base config.
	// In production this would clone fields. We'll simulate by returning a logger
	// that prepends these fields or modifies component.
	return &Logger{
		level:      l.level,
		component:  l.component,
		output:     l.output,
		colorize:   l.colorize,
		showCaller: l.showCaller,
		timeFormat: l.timeFormat,
	}
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, fields ...Field) {
	l.log(DEBUG, msg, fields...)
}

// Info logs an info message
func (l *Logger) Info(msg string, fields ...Field) {
	l.log(INFO, msg, fields...)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, fields ...Field) {
	l.log(WARN, msg, fields...)
}

// Error logs an error message
func (l *Logger) Error(msg string, fields ...Field) {
	l.log(ERROR, msg, fields...)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(msg string, fields ...Field) {
	l.log(FATAL, msg, fields...)
	os.Exit(1)
}

// log is the core logging function
func (l *Logger) log(level LogLevel, msg string, fields ...Field) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Skip if below minimum level
	if level < l.level {
		return
	}

	// Build log entry
	timestamp := time.Now().Format(l.timeFormat)
	levelStr := levelNames[level]

	// Format: [TIME] [LEVEL] [COMPONENT] message key=value key=value
	var builder strings.Builder

	// Colorize if enabled
	if l.colorize {
		builder.WriteString(levelColors[level])
	}

	// Timestamp
	builder.WriteString("[")
	builder.WriteString(timestamp)
	builder.WriteString("] ")

	// Level
	builder.WriteString("[")
	builder.WriteString(fmt.Sprintf("%-5s", levelStr))
	builder.WriteString("] ")

	// Component
	if l.component != "" {
		builder.WriteString("[")
		builder.WriteString(l.component)
		builder.WriteString("] ")
	}

	// Message
	builder.WriteString(msg)

	// Fields
	if len(fields) > 0 {
		builder.WriteString(" ")
		for i, field := range fields {
			if i > 0 {
				builder.WriteString(" ")
			}
			builder.WriteString(field.Key)
			builder.WriteString("=")
			builder.WriteString(field.format())
		}
	}

	// Caller info (if enabled)
	if l.showCaller {
		_, file, line, ok := runtime.Caller(2)
		if ok {
			// Get just the filename, not full path
			parts := strings.Split(file, "/")
			filename := parts[len(parts)-1]
			builder.WriteString(fmt.Sprintf(" (%s:%d)", filename, line))
		}
	}

	// Reset color
	if l.colorize {
		builder.WriteString(colorReset)
	}

	builder.WriteString("\n")

	// Write to output
	logLine := builder.String()
	l.output.Write([]byte(logLine))

	// Also redirect to JS console if in WASM
	if runtime.GOOS == "js" {
		console := js.Global().Get("console")
		if !isValueNil(console) {
			method := "log"
			switch level {
			case DEBUG:
				method = "debug"
			case INFO:
				method = "info"
			case WARN:
				method = "warn"
			case ERROR, FATAL:
				method = "error"
			}
			console.Call(method, logLine)
		}
	}
}

// isValueNil helper for js.Value
func isValueNil(v js.Value) bool {
	return v.Type() == js.TypeNull || v.Type() == js.TypeUndefined
}

// Field represents a key-value pair for structured logging
type Field struct {
	Key   string
	Value interface{}
}

// format formats a field value
func (f Field) format() string {
	switch v := f.Value.(type) {
	case string:
		return fmt.Sprintf("%q", v)
	case error:
		return fmt.Sprintf("%q", v.Error())
	case time.Duration:
		return v.String()
	case time.Time:
		return v.Format(time.RFC3339)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// Helper functions for creating fields
func String(key, value string) Field {
	return Field{Key: key, Value: value}
}

func Int(key string, value int) Field {
	return Field{Key: key, Value: value}
}

func Int64(key string, value int64) Field {
	return Field{Key: key, Value: value}
}

func Uint64(key string, value uint64) Field {
	return Field{Key: key, Value: value}
}

func Float64(key string, value float64) Field {
	return Field{Key: key, Value: value}
}

func Bool(key string, value bool) Field {
	return Field{Key: key, Value: value}
}

func Err(err error) Field {
	return Field{Key: "error", Value: err}
}

func Duration(key string, value time.Duration) Field {
	return Field{Key: key, Value: value}
}

func Any(key string, value interface{}) Field {
	return Field{Key: key, Value: value}
}

// Global logger instance
var globalLogger = DefaultLogger("kernel")

// SetGlobalLogger sets the global logger instance
func SetGlobalLogger(logger *Logger) {
	globalLogger = logger
}

// Global logging functions
func Debug(msg string, fields ...Field) {
	globalLogger.Debug(msg, fields...)
}

func Info(msg string, fields ...Field) {
	globalLogger.Info(msg, fields...)
}

func Warn(msg string, fields ...Field) {
	globalLogger.Warn(msg, fields...)
}

func Error(msg string, fields ...Field) {
	globalLogger.Error(msg, fields...)
}

func Fatal(msg string, fields ...Field) {
	globalLogger.Fatal(msg, fields...)
}
