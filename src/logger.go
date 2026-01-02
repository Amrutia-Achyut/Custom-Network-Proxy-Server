package main

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// LogEntry represents a single log entry
type LogEntry struct {
	Timestamp      time.Time
	ClientIP       string
	ClientPort     int
	DestinationHost string
	DestinationPort int
	Method         string
	RequestTarget  string
	Action         string // ALLOWED or BLOCKED
	UpstreamStatus int
	BytesUpstream  int64
	BytesDownstream int64
	BlockedRule    string // Rule that caused block, if any
}

// Logger provides thread-safe logging
type Logger struct {
	file       *os.File
	mu         sync.Mutex
	maxSizeMB  int
	currentSize int64
	filePath   string
}

// NewLogger creates a new logger instance
func NewLogger(filePath string, maxSizeMB int) (*Logger, error) {
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	// Get current file size
	info, err := file.Stat()
	var size int64
	if err == nil {
		size = info.Size()
	}

	return &Logger{
		file:       file,
		maxSizeMB:  maxSizeMB,
		currentSize: size,
		filePath:   filePath,
	}, nil
}

// Log writes a log entry
func (l *Logger) Log(entry LogEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Check if rotation is needed
	maxSizeBytes := int64(l.maxSizeMB) * 1024 * 1024
	if l.currentSize >= maxSizeBytes {
		l.rotate()
	}

	// Format log line
	line := l.formatLogEntry(entry)
	
	// Write to file
	fmt.Fprintln(l.file, line)
	l.file.Sync() // Ensure immediate write
	
	// Update size
	l.currentSize += int64(len(line) + 1) // +1 for newline
}

// formatLogEntry formats a log entry as a single line
func (l *Logger) formatLogEntry(entry LogEntry) string {
	timestamp := entry.Timestamp.UTC().Format(time.RFC3339)
	clientAddr := fmt.Sprintf("%s:%d", entry.ClientIP, entry.ClientPort)
	destAddr := fmt.Sprintf("%s:%d", entry.DestinationHost, entry.DestinationPort)
	requestLine := fmt.Sprintf("%s %s HTTP/1.1", entry.Method, entry.RequestTarget)

	var statusCode string
	if entry.UpstreamStatus > 0 {
		statusCode = fmt.Sprintf("%d", entry.UpstreamStatus)
	} else {
		statusCode = "-"
	}

	line := fmt.Sprintf("%s %s -> %s \"%s\" %s %s %d %d",
		timestamp,
		clientAddr,
		destAddr,
		requestLine,
		entry.Action,
		statusCode,
		entry.BytesUpstream,
		entry.BytesDownstream,
	)

	if entry.BlockedRule != "" {
		line += fmt.Sprintf(" [BLOCKED: %s]", entry.BlockedRule)
	}

	return line
}

// rotate closes the current log file and opens a new one
func (l *Logger) rotate() {
	l.file.Close()
	
	// Rename old file with timestamp
	timestamp := time.Now().Format("20060102-150405")
	oldPath := fmt.Sprintf("%s.%s", l.filePath, timestamp)
	os.Rename(l.filePath, oldPath)
	
	// Open new file
	file, err := os.OpenFile(l.filePath, os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		l.file = file
		l.currentSize = 0
	}
}

// Close closes the log file
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.file.Close()
}

