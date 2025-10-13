package alerting

import (
	"fmt"
	"logstream/pkg/models"
	"sync"
	"time"
)

// AlertRule defines conditions that trigger an alert
type AlertRule struct {
	Name      string
	Level     string        // Log level to monitor (ERROR, CRITICAL)
	Threshold int           // Number of occurrences
	Window    time.Duration // Time window to check
	Pattern   string        // Optional: keyword to match in message
}

// Alert represents a triggered alert
type Alert struct {
	RuleName  string
	Message   string
	Count     int
	Timestamp time.Time
}

// AlertManager monitors logs and triggers alerts
type AlertManager struct {
	rules         []AlertRule
	alertChannel  chan Alert
	recentLogs    []logEntry
	mu            sync.Mutex
	alertCallback func(Alert)
}

// logEntry stores minimal info for alert checking
type logEntry struct {
	timestamp time.Time
	level     string
	message   string
}

// NewAlertManager creates a new alert manager
func NewAlertManager(callback func(Alert)) *AlertManager {
	return &AlertManager{
		rules:         make([]AlertRule, 0),
		alertChannel:  make(chan Alert, 100),
		recentLogs:    make([]logEntry, 0, 1000),
		alertCallback: callback,
	}
}

// AddRule adds a new alert rule
func (am *AlertManager) AddRule(rule AlertRule) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.rules = append(am.rules, rule)
}

// Start begins monitoring for alerts
func (am *AlertManager) Start() {
	go am.processAlerts()
}

// ProcessLog checks a new log against all rules (called by ingestor)
func (am *AlertManager) ProcessLog(log models.LogEntry) {
	am.mu.Lock()
	defer am.mu.Unlock()

	// Add to recent logs for window-based checking
	am.recentLogs = append(am.recentLogs, logEntry{
		timestamp: log.Timestamp,
		level:     log.Level,
		message:   log.Message,
	})

	// Clean old logs outside the largest window
	maxWindow := am.getMaxWindow()
	cutoff := time.Now().Add(-maxWindow)
	am.recentLogs = am.cleanOldLogs(am.recentLogs, cutoff)

	// Check each rule
	for _, rule := range am.rules {
		if am.shouldTriggerAlert(rule) {
			alert := Alert{
				RuleName:  rule.Name,
				Message:   fmt.Sprintf("Alert: %s triggered! %d %s logs in last %v", rule.Name, rule.Threshold, rule.Level, rule.Window),
				Count:     rule.Threshold,
				Timestamp: time.Now(),
			}

			// Non-blocking send to alert channel
			select {
			case am.alertChannel <- alert:
			default:
				// Channel full, skip this alert
			}
		}
	}
}

// shouldTriggerAlert checks if a rule's conditions are met
func (am *AlertManager) shouldTriggerAlert(rule AlertRule) bool {
	now := time.Now()
	windowStart := now.Add(-rule.Window)

	count := 0
	for _, log := range am.recentLogs {
		// Check if log is within time window
		if log.timestamp.After(windowStart) {
			// Check level match
			if log.level == rule.Level {
				// Check pattern match if specified
				if rule.Pattern == "" || containsPattern(log.message, rule.Pattern) {
					count++
				}
			}
		}
	}

	return count >= rule.Threshold
}

// processAlerts handles triggered alerts
func (am *AlertManager) processAlerts() {
	for alert := range am.alertChannel {
		if am.alertCallback != nil {
			// Call the callback within 500ms target
			go am.alertCallback(alert)
		}
	}
}

// getMaxWindow returns the largest time window from all rules
func (am *AlertManager) getMaxWindow() time.Duration {
	max := time.Minute
	for _, rule := range am.rules {
		if rule.Window > max {
			max = rule.Window
		}
	}
	return max
}

// cleanOldLogs removes logs older than cutoff
func (am *AlertManager) cleanOldLogs(logs []logEntry, cutoff time.Time) []logEntry {
	result := make([]logEntry, 0, len(logs))
	for _, log := range logs {
		if log.timestamp.After(cutoff) {
			result = append(result, log)
		}
	}
	return result
}

// containsPattern checks if message contains pattern (simple substring match)
func containsPattern(message, pattern string) bool {
	// Simple implementation - could be enhanced with regex
	return len(pattern) == 0 || len(message) >= len(pattern) && contains(message, pattern)
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Stop closes the alert channel
func (am *AlertManager) Stop() {
	close(am.alertChannel)
}
