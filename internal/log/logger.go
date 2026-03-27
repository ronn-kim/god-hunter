package log

import (
	"fmt"
	"log"
	"os"
	"time"
)

// Logger provides structured logging with levels
type Logger struct {
	silent bool
	level  Level
}

// Level represents logging level
type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

var (
	logger *log.Logger
)

func init() {
	logger = log.New(os.Stderr, "", log.LstdFlags)
}

// NewLogger creates a new logger instance
func NewLogger(silent bool) *Logger {
	return &Logger{
		silent: silent,
		level:  INFO,
	}
}

// Debug logs debug-level messages
func (l *Logger) Debug(format string, args ...interface{}) {
	if l.silent || l.level > DEBUG {
		return
	}
	l.log("[DEBUG]", format, args...)
}

// Info logs info-level messages
func (l *Logger) Info(format string, args ...interface{}) {
	if l.silent || l.level > INFO {
		return
	}
	l.log("[+]", format, args...)
}

// Warn logs warning-level messages
func (l *Logger) Warn(format string, args ...interface{}) {
	if l.silent || l.level > WARN {
		return
	}
	l.log("[!]", format, args...)
}

// Error logs error-level messages (always shown unless silent)
func (l *Logger) Error(format string, args ...interface{}) {
	if l.silent {
		return
	}
	l.log("[ERROR]", format, args...)
}

func (l *Logger) log(prefix string, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format("15:04:05")
	fmt.Fprintf(os.Stderr, "%s %s %s\n", timestamp, prefix, msg)
}

// WithLevel returns a logger with a specific level
func (l *Logger) WithLevel(level Level) *Logger {
	l.level = level
	return l
}
