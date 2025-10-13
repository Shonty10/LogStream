package models

import "time"

// LogEntry represents a single log message
type LogEntry struct {
	ID        string                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	Level     string                 `json:"level"` // INFO, WARNING, ERROR, CRITICAL
	Message   string                 `json:"message"`
	Service   string                 `json:"service"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// LogLevel constants
const (
	LevelInfo     = "INFO"
	LevelWarning  = "WARNING"
	LevelError    = "ERROR"
	LevelCritical = "CRITICAL"
)