package storage

import (
	"logstream/pkg/models"
	"sync"
	"time"
)

// MemoryStore provides fast in-memory log storage with custom indexing
type MemoryStore struct {
	logs          []models.LogEntry
	indexByLevel  map[string][]int // level -> array of log indices
	indexByTime   *TimeIndex
	mu            sync.RWMutex
	maxLogs       int
}

// TimeIndex provides fast time-range queries
type TimeIndex struct {
	buckets map[int64][]int // timestamp bucket (minute) -> log indices
	mu      sync.RWMutex
}

// NewMemoryStore creates a new in-memory store
func NewMemoryStore(maxLogs int) *MemoryStore {
	return &MemoryStore{
		logs:         make([]models.LogEntry, 0, maxLogs),
		indexByLevel: make(map[string][]int),
		indexByTime: &TimeIndex{
			buckets: make(map[int64][]int),
		},
		maxLogs: maxLogs,
	}
}

// Store adds a log entry with automatic indexing
func (ms *MemoryStore) Store(entry models.LogEntry) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	// Add to main storage
	idx := len(ms.logs)
	ms.logs = append(ms.logs, entry)

	// Index by level
	ms.indexByLevel[entry.Level] = append(ms.indexByLevel[entry.Level], idx)

	// Index by time (bucket by minute for fast range queries)
	timeBucket := entry.Timestamp.Unix() / 60
	ms.indexByTime.mu.Lock()
	ms.indexByTime.buckets[timeBucket] = append(ms.indexByTime.buckets[timeBucket], idx)
	ms.indexByTime.mu.Unlock()

	// Evict old logs if we exceed max capacity
	if len(ms.logs) > ms.maxLogs {
		ms.evictOldest()
	}
}

// GetByLevel returns all logs of a specific level (fast indexed lookup)
func (ms *MemoryStore) GetByLevel(level string) []models.LogEntry {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	indices := ms.indexByLevel[level]
	result := make([]models.LogEntry, 0, len(indices))
	for _, idx := range indices {
		if idx < len(ms.logs) {
			result = append(result, ms.logs[idx])
		}
	}
	return result
}

// GetByTimeRange returns logs within a time range (fast indexed lookup)
func (ms *MemoryStore) GetByTimeRange(start, end time.Time) []models.LogEntry {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	startBucket := start.Unix() / 60
	endBucket := end.Unix() / 60

	result := make([]models.LogEntry, 0)
	ms.indexByTime.mu.RLock()
	defer ms.indexByTime.mu.RUnlock()

	// Iterate through relevant time buckets
	for bucket := startBucket; bucket <= endBucket; bucket++ {
		if indices, exists := ms.indexByTime.buckets[bucket]; exists {
			for _, idx := range indices {
				if idx < len(ms.logs) {
					log := ms.logs[idx]
					if !log.Timestamp.Before(start) && !log.Timestamp.After(end) {
						result = append(result, log)
					}
				}
			}
		}
	}
	return result
}

// GetRecent returns the N most recent logs
func (ms *MemoryStore) GetRecent(n int) []models.LogEntry {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	start := len(ms.logs) - n
	if start < 0 {
		start = 0
	}
	return ms.logs[start:]
}

// Count returns total number of logs stored
func (ms *MemoryStore) Count() int {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	return len(ms.logs)
}

// evictOldest removes the oldest 20% of logs when capacity is exceeded
func (ms *MemoryStore) evictOldest() {
	evictCount := ms.maxLogs / 5 // Remove 20%
	ms.logs = ms.logs[evictCount:]

	// Rebuild indices after eviction
	ms.rebuildIndices()
}

// rebuildIndices reconstructs all indices after eviction
func (ms *MemoryStore) rebuildIndices() {
	ms.indexByLevel = make(map[string][]int)
	ms.indexByTime.mu.Lock()
	ms.indexByTime.buckets = make(map[int64][]int)
	ms.indexByTime.mu.Unlock()

	for idx, log := range ms.logs {
		ms.indexByLevel[log.Level] = append(ms.indexByLevel[log.Level], idx)
		
		timeBucket := log.Timestamp.Unix() / 60
		ms.indexByTime.mu.Lock()
		ms.indexByTime.buckets[timeBucket] = append(ms.indexByTime.buckets[timeBucket], idx)
		ms.indexByTime.mu.Unlock()
	}
}