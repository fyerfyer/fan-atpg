package utils

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// LogLevel represents the verbosity level of logging
type LogLevel int

const (
	ErrorLevel LogLevel = iota
	WarningLevel
	InfoLevel
	DebugLevel
	TraceLevel
)

// String returns a string representation of the log level
func (l LogLevel) String() string {
	switch l {
	case ErrorLevel:
		return "ERROR"
	case WarningLevel:
		return "WARNING"
	case InfoLevel:
		return "INFO"
	case DebugLevel:
		return "DEBUG"
	case TraceLevel:
		return "TRACE"
	default:
		return "UNKNOWN"
	}
}

// Logger represents a logging utility
type Logger struct {
	Level      LogLevel
	Output     io.Writer
	ShowTime   bool
	Prefix     string
	IndentSize int
	indent     int // Current indentation level
}

// NewLogger creates a new logger with the specified verbosity level
func NewLogger(level LogLevel) *Logger {
	return &Logger{
		Level:      level,
		Output:     os.Stdout,
		ShowTime:   true,
		IndentSize: 2,
		indent:     0,
	}
}

// NewFileLogger creates a new logger that writes to a file
func NewFileLogger(level LogLevel, filename string) (*Logger, error) {
	file, err := os.Create(filename)
	if err != nil {
		return nil, err
	}

	return &Logger{
		Level:      level,
		Output:     file,
		ShowTime:   true,
		IndentSize: 2,
		indent:     0,
	}, nil
}

// SetOutput sets the output writer
func (l *Logger) SetOutput(w io.Writer) {
	l.Output = w
}

// SetPrefix sets a prefix for all log messages
func (l *Logger) SetPrefix(prefix string) {
	l.Prefix = prefix
}

// Indent increases the indentation level
func (l *Logger) Indent() {
	l.indent++
}

// Outdent decreases the indentation level
func (l *Logger) Outdent() {
	if l.indent > 0 {
		l.indent--
	}
}

// ResetIndent resets the indentation to zero
func (l *Logger) ResetIndent() {
	l.indent = 0
}

// log logs a message at the specified level
func (l *Logger) log(level LogLevel, format string, args ...interface{}) {
	if level > l.Level {
		return
	}

	var builder strings.Builder

	// Add timestamp if enabled
	if l.ShowTime {
		builder.WriteString(time.Now().Format("15:04:05.000 "))
	}

	// Add level indicator
	builder.WriteString(fmt.Sprintf("[%s] ", level.String()))

	// Add prefix if set
	if l.Prefix != "" {
		builder.WriteString(fmt.Sprintf("%s: ", l.Prefix))
	}

	// Add indentation
	if l.indent > 0 {
		builder.WriteString(strings.Repeat(" ", l.indent*l.IndentSize))
	}

	// Add the main message
	builder.WriteString(fmt.Sprintf(format, args...))
	builder.WriteString("\n")

	fmt.Fprint(l.Output, builder.String())
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(ErrorLevel, format, args...)
}

// Warning logs a warning message
func (l *Logger) Warning(format string, args ...interface{}) {
	l.log(WarningLevel, format, args...)
}

// Info logs an informational message
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(InfoLevel, format, args...)
}

// Debug logs a debug message
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(DebugLevel, format, args...)
}

// Trace logs a trace message (highest verbosity)
func (l *Logger) Trace(format string, args ...interface{}) {
	l.log(TraceLevel, format, args...)
}

// Circuit logs information about circuit state
func (l *Logger) Circuit(format string, args ...interface{}) {
	l.log(DebugLevel, "CIRCUIT: "+format, args...)
}

// Algorithm logs information about the FAN algorithm execution
func (l *Logger) Algorithm(format string, args ...interface{}) {
	l.log(DebugLevel, "ALGORITHM: "+format, args...)
}

// Decision logs information about decision making in the algorithm
func (l *Logger) Decision(format string, args ...interface{}) {
	l.log(DebugLevel, "DECISION: "+format, args...)
}

// Backtrack logs information about backtracking
func (l *Logger) Backtrack(format string, args ...interface{}) {
	l.log(DebugLevel, "BACKTRACK: "+format, args...)
}

// Implication logs information about implication operations
func (l *Logger) Implication(format string, args ...interface{}) {
	l.log(TraceLevel, "IMPLICATION: "+format, args...)
}

// Frontier logs information about frontier updates
func (l *Logger) Frontier(format string, args ...interface{}) {
	l.log(TraceLevel, "FRONTIER: "+format, args...)
}

// DefaultLogger is the default logger instance
var DefaultLogger = NewLogger(InfoLevel)

// SetDefaultLogLevel sets the log level of the default logger
func SetDefaultLogLevel(level LogLevel) {
	DefaultLogger.Level = level
}
