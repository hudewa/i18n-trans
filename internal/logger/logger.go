package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Logger handles logging to file
type Logger struct {
	logDir   string
	logFile  string
	mu       sync.Mutex
}

// New creates a new Logger
func New(logDir, logFile string) *Logger {
	return &Logger{
		logDir:  logDir,
		logFile: logFile,
	}
}

// Log writes a log message with timestamp
func (l *Logger) Log(level, message string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Create log directory if not exists
	if err := os.MkdirAll(l.logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Build log entry
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logEntry := fmt.Sprintf("[%s] [%s] %s\n", timestamp, level, message)

	// Append to file
	logPath := filepath.Join(l.logDir, l.logFile)
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(logEntry); err != nil {
		return fmt.Errorf("failed to write log: %w", err)
	}

	return nil
}

// LogInfo logs an info message
func (l *Logger) LogInfo(message string) error {
	return l.Log("INFO", message)
}

// LogError logs an error message
func (l *Logger) LogError(message string) error {
	return l.Log("ERROR", message)
}

// LogDebug logs a debug message
func (l *Logger) LogDebug(message string) error {
	return l.Log("DEBUG", message)
}
