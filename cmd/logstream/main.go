package main

import (
	"encoding/json"
	"fmt"
	"log"
	"logstream/internal/alerting"
	"logstream/internal/ingestion"
	"logstream/internal/storage"
	"logstream/pkg/models"
	"math/rand"
	"net/http"
	"time"

	"github.com/google/uuid"
)

var (
	ingestor *ingestion.Ingestor
	store    *storage.MemoryStore
)

func main() {
	fmt.Println("🚀 Starting LogStream - High-Performance Log Ingestion Engine")

	// Initialize components
	store = storage.NewMemoryStore(100000) // Store up to 100k logs

	alertMgr := alerting.NewAlertManager(handleAlert)

	// Add some default alert rules
	alertMgr.AddRule(alerting.AlertRule{
		Name:      "High Error Rate",
		Level:     models.LevelError,
		Threshold: 10,
		Window:    1 * time.Minute,
	})

	alertMgr.AddRule(alerting.AlertRule{
		Name:      "Critical Errors",
		Level:     models.LevelCritical,
		Threshold: 3,
		Window:    30 * time.Second,
	})

	alertMgr.Start()

	// Create ingestor with 20 workers and 10k buffer
	ingestor = ingestion.NewIngestor(store, alertMgr, 20, 10000)
	ingestor.Start()

	// Setup HTTP API
	http.HandleFunc("/ingest", handleIngest)
	http.HandleFunc("/logs", handleGetLogs)
	http.HandleFunc("/logs/recent", handleGetRecent)
	http.HandleFunc("/stats", handleStats)
	http.HandleFunc("/simulate", handleSimulate)
	http.HandleFunc("/", handleRoot)

	fmt.Println("✅ LogStream is running on http://localhost:8080")
	fmt.Println("📊 API Endpoints:")
	fmt.Println("   POST /ingest        - Ingest a log entry")
	fmt.Println("   GET  /logs          - Get logs by level or time range")
	fmt.Println("   GET  /logs/recent   - Get recent logs")
	fmt.Println("   GET  /stats         - Get ingestion statistics")
	fmt.Println("   POST /simulate      - Simulate high-volume log traffic")
	fmt.Println()

	log.Fatal(http.ListenAndServe(":8080", nil))
}

// handleIngest receives and processes a single log entry
func handleIngest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var entry models.LogEntry
	if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Set timestamp if not provided
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	// Generate ID if not provided
	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}

	// Ingest the log
	if !ingestor.Ingest(entry) {
		http.Error(w, "Ingestion queue full", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "accepted",
		"id":     entry.ID,
	})
}

// handleGetLogs queries logs by level or time range
func handleGetLogs(w http.ResponseWriter, r *http.Request) {
	level := r.URL.Query().Get("level")

	var logs []models.LogEntry

	if level != "" {
		logs = store.GetByLevel(level)
	} else {
		// Get logs from last hour by default
		end := time.Now()
		start := end.Add(-1 * time.Hour)
		logs = store.GetByTimeRange(start, end)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"count": len(logs),
		"logs":  logs,
	})
}

// handleGetRecent returns the most recent N logs
func handleGetRecent(w http.ResponseWriter, r *http.Request) {
	logs := store.GetRecent(100) // Last 100 logs

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"count": len(logs),
		"logs":  logs,
	})
}

// handleStats returns ingestion statistics
func handleStats(w http.ResponseWriter, r *http.Request) {
	stats := ingestor.GetStats()

	elapsed := time.Since(stats.StartTime).Seconds()
	avgThroughput := float64(stats.TotalProcessed) / elapsed

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"total_processed": stats.TotalProcessed,
		"total_dropped":   stats.TotalDropped,
		"uptime_seconds":  int(elapsed),
		"avg_throughput":  int(avgThroughput),
		"logs_in_storage": store.Count(),
	})
}

// handleSimulate generates high-volume test traffic
func handleSimulate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	go simulateTraffic(10000) // Generate 10k logs

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "simulation started",
		"logs":   "10000",
	})
}

// simulateTraffic generates realistic log traffic
func simulateTraffic(count int) {
	fmt.Printf("🔥 Simulating %d logs...\n", count)

	services := []string{"auth-service", "payment-service", "user-service", "api-gateway", "database"}
	levels := []string{models.LevelInfo, models.LevelWarning, models.LevelError, models.LevelCritical}
	messages := []string{
		"Request processed successfully",
		"Connection timeout",
		"Database query failed",
		"Invalid authentication token",
		"Service unavailable",
		"Rate limit exceeded",
		"Memory usage high",
	}

	startTime := time.Now()

	for i := 0; i < count; i++ {
		entry := models.LogEntry{
			ID:        uuid.New().String(),
			Timestamp: time.Now(),
			Level:     levels[rand.Intn(len(levels))],
			Message:   messages[rand.Intn(len(messages))],
			Service:   services[rand.Intn(len(services))],
			Metadata: map[string]interface{}{
				"user_id":    rand.Intn(1000),
				"request_id": uuid.New().String(),
			},
		}

		ingestor.Ingest(entry)

		// Simulate realistic timing
		if i%100 == 0 {
			time.Sleep(1 * time.Millisecond)
		}
	}

	elapsed := time.Since(startTime)
	throughput := float64(count) / elapsed.Seconds()

	fmt.Printf("✅ Simulation complete! Ingested %d logs in %.2fs (%.0f logs/sec)\n", count, elapsed.Seconds(), throughput)
}

// handleAlert is called when an alert is triggered
func handleAlert(alert alerting.Alert) {
	fmt.Printf("🚨 ALERT: %s - %s\n", alert.RuleName, alert.Message)
}

// handleRoot shows a welcome message
func handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	html := `
	<!DOCTYPE html>
	<html>
	<head>
		<title>LogStream</title>
		<style>
			body { font-family: monospace; max-width: 800px; margin: 50px auto; padding: 20px; }
			h1 { color: #2563eb; }
			code { background: #f3f4f6; padding: 2px 6px; border-radius: 3px; }
			.endpoint { margin: 10px 0; }
		</style>
	</head>
	<body>
		<h1>🚀 LogStream - High-Performance Log Ingestion Engine</h1>
		<p>A concurrent log processing system built with Go</p>
		
		<h2>API Endpoints:</h2>
		<div class="endpoint"><strong>POST /ingest</strong> - Ingest a log entry</div>
		<div class="endpoint"><strong>GET /logs?level=ERROR</strong> - Get logs by level</div>
		<div class="endpoint"><strong>GET /logs/recent</strong> - Get 100 most recent logs</div>
		<div class="endpoint"><strong>GET /stats</strong> - Get ingestion statistics</div>
		<div class="endpoint"><strong>POST /simulate</strong> - Simulate 10k logs</div>
		
		<h2>Quick Test:</h2>
		<p>Try: <code>curl -X POST http://localhost:8080/simulate</code></p>
	</body>
	</html>
	`

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}
