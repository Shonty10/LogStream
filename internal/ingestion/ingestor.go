package ingestion

import (
	"logstream/internal/alerting"
	"logstream/internal/storage"
	"logstream/pkg/models"
	"sync"
	"sync/atomic"
	"time"
)

// Ingestor handles concurrent log ingestion
type Ingestor struct {
	store        *storage.MemoryStore
	alertManager *alerting.AlertManager
	logChannel   chan models.LogEntry
	workerCount  int
	wg           sync.WaitGroup
	stats        *Stats
	shutdown     chan struct{}
}

// Stats tracks ingestion performance
type Stats struct {
	TotalProcessed uint64
	TotalDropped   uint64
	StartTime      time.Time
}

// NewIngestor creates a new log ingestor
func NewIngestor(store *storage.MemoryStore, alertMgr *alerting.AlertManager, workerCount int, bufferSize int) *Ingestor {
	return &Ingestor{
		store:        store,
		alertManager: alertMgr,
		logChannel:   make(chan models.LogEntry, bufferSize),
		workerCount:  workerCount,
		stats: &Stats{
			StartTime: time.Now(),
		},
		shutdown: make(chan struct{}),
	}
}

// Start begins the ingestion workers
func (ing *Ingestor) Start() {
	for i := 0; i < ing.workerCount; i++ {
		ing.wg.Add(1)
		go ing.worker(i)
	}

	// Start stats reporter
	go ing.reportStats()
}

// Ingest adds a log entry to the processing queue (non-blocking)
func (ing *Ingestor) Ingest(entry models.LogEntry) bool {
	select {
	case ing.logChannel <- entry:
		return true
	default:
		// Channel full, drop log and increment counter
		atomic.AddUint64(&ing.stats.TotalDropped, 1)
		return false
	}
}

// worker processes logs from the channel
func (ing *Ingestor) worker(id int) {
	defer ing.wg.Done()

	for {
		select {
		case log := <-ing.logChannel:
			// Store the log (fast in-memory operation)
			ing.store.Store(log)

			// Process for alerts (async, non-blocking)
			if ing.alertManager != nil {
				ing.alertManager.ProcessLog(log)
			}

			// Update stats
			atomic.AddUint64(&ing.stats.TotalProcessed, 1)

		case <-ing.shutdown:
			return
		}
	}
}

// reportStats prints throughput statistics every 10 seconds
func (ing *Ingestor) reportStats() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	lastCount := uint64(0)
	lastTime := time.Now()

	for {
		select {
		case <-ticker.C:
			currentCount := atomic.LoadUint64(&ing.stats.TotalProcessed)
			currentTime := time.Now()

			elapsed := currentTime.Sub(lastTime).Seconds()
			processed := currentCount - lastCount

			throughput := float64(processed) / elapsed

			// Print stats
			dropped := atomic.LoadUint64(&ing.stats.TotalDropped)
			totalTime := currentTime.Sub(ing.stats.StartTime).Seconds()
			avgThroughput := float64(currentCount) / totalTime

			println("========== LogStream Stats ==========")
			println("Current Throughput:", int(throughput), "logs/sec")
			println("Average Throughput:", int(avgThroughput), "logs/sec")
			println("Total Processed:", currentCount)
			println("Total Dropped:", dropped)
			println("Logs in Store:", ing.store.Count())
			println("=====================================")

			lastCount = currentCount
			lastTime = currentTime

		case <-ing.shutdown:
			return
		}
	}
}

// Stop gracefully shuts down the ingestor
func (ing *Ingestor) Stop() {
	close(ing.shutdown)
	close(ing.logChannel)
	ing.wg.Wait()
}

// GetStats returns current ingestion statistics
func (ing *Ingestor) GetStats() Stats {
	return Stats{
		TotalProcessed: atomic.LoadUint64(&ing.stats.TotalProcessed),
		TotalDropped:   atomic.LoadUint64(&ing.stats.TotalDropped),
		StartTime:      ing.stats.StartTime,
	}
}
