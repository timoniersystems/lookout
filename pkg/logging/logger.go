// Package logging provides structured logging with different severity levels.
package logging

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"
)

// LogLevel represents the severity level of a log message.
type LogLevel int

const (
	// DebugLevel is for detailed debugging information
	DebugLevel LogLevel = iota
	// InfoLevel is for general informational messages
	InfoLevel
	// WarnLevel is for warning messages
	WarnLevel
	// ErrorLevel is for error messages
	ErrorLevel
)

// String returns the string representation of a log level.
func (l LogLevel) String() string {
	switch l {
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Logger provides structured logging functionality.
type Logger struct {
	level      LogLevel
	output     io.Writer
	timeFormat string
}

// NewLogger creates a new logger with the specified minimum log level.
func NewLogger(level LogLevel) *Logger {
	return &Logger{
		level:      level,
		output:     os.Stdout,
		timeFormat: "2006-01-02 15:04:05",
	}
}

// NewLoggerWithOutput creates a new logger with custom output writer.
func NewLoggerWithOutput(level LogLevel, output io.Writer) *Logger {
	return &Logger{
		level:      level,
		output:     output,
		timeFormat: "2006-01-02 15:04:05",
	}
}

// SetLevel sets the minimum log level.
func (l *Logger) SetLevel(level LogLevel) {
	l.level = level
}

// SetOutput sets the output writer.
func (l *Logger) SetOutput(output io.Writer) {
	l.output = output
}

// logMessage writes a formatted log message if the level is appropriate.
func (l *Logger) logMessage(level LogLevel, format string, args ...interface{}) {
	if level < l.level {
		return
	}

	timestamp := time.Now().Format(l.timeFormat)
	levelStr := level.String()
	message := fmt.Sprintf(format, args...)

	logLine := fmt.Sprintf("[%s] %s: %s\n", timestamp, levelStr, message)
	fmt.Fprint(l.output, logLine)
}

// Debug logs a debug message.
func (l *Logger) Debug(format string, args ...interface{}) {
	l.logMessage(DebugLevel, format, args...)
}

// Info logs an informational message.
func (l *Logger) Info(format string, args ...interface{}) {
	l.logMessage(InfoLevel, format, args...)
}

// Warn logs a warning message.
func (l *Logger) Warn(format string, args ...interface{}) {
	l.logMessage(WarnLevel, format, args...)
}

// Error logs an error message.
func (l *Logger) Error(format string, args ...interface{}) {
	l.logMessage(ErrorLevel, format, args...)
}

// Global logger instance
var defaultLogger = NewLogger(InfoLevel)

// SetGlobalLevel sets the log level for the default logger.
func SetGlobalLevel(level LogLevel) {
	defaultLogger.SetLevel(level)
}

// SetGlobalOutput sets the output writer for the default logger.
func SetGlobalOutput(output io.Writer) {
	defaultLogger.SetOutput(output)
}

// Debug logs a debug message using the default logger.
func Debug(format string, args ...interface{}) {
	defaultLogger.Debug(format, args...)
}

// Info logs an informational message using the default logger.
func Info(format string, args ...interface{}) {
	defaultLogger.Info(format, args...)
}

// Warn logs a warning message using the default logger.
func Warn(format string, args ...interface{}) {
	defaultLogger.Warn(format, args...)
}

// Error logs an error message using the default logger.
func Error(format string, args ...interface{}) {
	defaultLogger.Error(format, args...)
}

// ParseLevel parses a log level string into a LogLevel.
func ParseLevel(level string) LogLevel {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return DebugLevel
	case "INFO":
		return InfoLevel
	case "WARN", "WARNING":
		return WarnLevel
	case "ERROR":
		return ErrorLevel
	default:
		log.Printf("Unknown log level %s, defaulting to INFO", level)
		return InfoLevel
	}
}

// InitFromEnv initializes the global logger from environment variables.
// Reads LOG_LEVEL env var (defaults to INFO if not set).
func InitFromEnv() {
	levelStr := os.Getenv("LOG_LEVEL")
	if levelStr == "" {
		levelStr = "INFO"
	}

	level := ParseLevel(levelStr)
	SetGlobalLevel(level)

	Info("Logger initialized with level: %s", level.String())
}
