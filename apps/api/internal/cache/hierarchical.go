package cache

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// CacheLevel represents different cache levels
type CacheLevel int

const (
	L1Memory CacheLevel = iota // In-memory cache
	L2SQLite                   // SQLite persistent cache
	L3Actions                  // GitHub Actions cache
)

// CacheEntry represents a cached item
type CacheEntry struct {
	Key        string      `json:"key"`
	Value      interface{} `json:"value"`
	ExpiresAt  time.Time   `json:"expires_at"`
	Level      CacheLevel  `json:"level"`
	Size       int64       `json:"size"`
	AccessTime time.Time   `json:"access_time"`
	HitCount   int64       `json:"hit_count"`
}

// CacheConfig holds cache configuration
type CacheConfig struct {
	L1MaxItems     int           // Maximum items in L1 cache
	L1TTL          time.Duration // L1 cache TTL
	L2TTL          time.Duration // L2 cache TTL
	L3TTL          time.Duration // L3 cache TTL
	EvictionPolicy string        // LRU, LFU, TTL
	MaxMemoryMB    int64         // Maximum memory usage for L1
}

// DefaultCacheConfig returns default cache configuration
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		L1MaxItems:     1000,
		L1TTL:          5 * time.Minute,
		L2TTL:          1 * time.Hour,
		L3TTL:          24 * time.Hour,
		EvictionPolicy: "LRU",
		MaxMemoryMB:    100,
	}
}

// HierarchicalCache implements a multi-level caching strategy
type HierarchicalCache struct {
	config     CacheConfig
	l1Cache    map[string]*CacheEntry // In-memory cache
	l1Mutex    sync.RWMutex
	db         *sql.DB // SQLite cache
	l3Client   L3CacheClient
	metrics    *CacheMetrics
	evictChan  chan string
	stopChan   chan struct{}
	wg         sync.WaitGroup
}

// L3CacheClient interface for GitHub Actions cache
type L3CacheClient interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, data []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

// CacheMetrics tracks cache performance
type CacheMetrics struct {
	L1Hits      int64
	L1Misses    int64
	L2Hits      int64
	L2Misses    int64
	L3Hits      int64
	L3Misses    int64
	Evictions   int64
	TotalGets   int64
	TotalSets   int64
	mutex       sync.RWMutex
}

// NewHierarchicalCache creates a new hierarchical cache
func NewHierarchicalCache(config CacheConfig, db *sql.DB, l3Client L3CacheClient) (*HierarchicalCache, error) {
	cache := &HierarchicalCache{
		config:    config,
		l1Cache:   make(map[string]*CacheEntry),
		db:        db,
		l3Client:  l3Client,
		metrics:   &CacheMetrics{},
		evictChan: make(chan string, 100),
		stopChan:  make(chan struct{}),
	}

	// Initialize L2 cache table
	if err := cache.initL2Cache(); err != nil {
		return nil, fmt.Errorf("failed to initialize L2 cache: %w", err)
	}

	// Start background workers
	cache.wg.Add(2)
	go cache.evictionWorker()
	go cache.cleanupWorker()

	return cache, nil
}

// initL2Cache creates the SQLite cache table
func (h *HierarchicalCache) initL2Cache() error {
	createTableSQL := `
		CREATE TABLE IF NOT EXISTS cache_entries (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			expires_at DATETIME NOT NULL,
			size INTEGER NOT NULL,
			access_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			hit_count INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`

	_, err := h.db.Exec(createTableSQL)
	if err != nil {
		return err
	}

	// Create indexes
	indexSQL := `
		CREATE INDEX IF NOT EXISTS idx_cache_expires ON cache_entries(expires_at);
		CREATE INDEX IF NOT EXISTS idx_cache_access ON cache_entries(access_time);
	`
	_, err = h.db.Exec(indexSQL)
	return err
}

// Get retrieves a value from the cache hierarchy
func (h *HierarchicalCache) Get(ctx context.Context, key string) (interface{}, bool) {
	h.metrics.mutex.Lock()
	h.metrics.TotalGets++
	h.metrics.mutex.Unlock()

	// Try L1 cache first
	if value, found := h.getFromL1(key); found {
		h.metrics.mutex.Lock()
		h.metrics.L1Hits++
		h.metrics.mutex.Unlock()
		return value, true
	}

	h.metrics.mutex.Lock()
	h.metrics.L1Misses++
	h.metrics.mutex.Unlock()

	// Try L2 cache
	if value, found := h.getFromL2(ctx, key); found {
		h.metrics.mutex.Lock()
		h.metrics.L2Hits++
		h.metrics.mutex.Unlock()
		
		// Promote to L1
		h.setToL1(key, value, h.config.L1TTL)
		return value, true
	}

	h.metrics.mutex.Lock()
	h.metrics.L2Misses++
	h.metrics.mutex.Unlock()

	// Try L3 cache
	if value, found := h.getFromL3(ctx, key); found {
		h.metrics.mutex.Lock()
		h.metrics.L3Hits++
		h.metrics.mutex.Unlock()
		
		// Promote to L1 and L2
		h.setToL1(key, value, h.config.L1TTL)
		h.setToL2(ctx, key, value, h.config.L2TTL)
		return value, true
	}

	h.metrics.mutex.Lock()
	h.metrics.L3Misses++
	h.metrics.mutex.Unlock()

	return nil, false
}

// Set stores a value in the cache hierarchy
func (h *HierarchicalCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	h.metrics.mutex.Lock()
	h.metrics.TotalSets++
	h.metrics.mutex.Unlock()

	// Set in all levels
	h.setToL1(key, value, ttl)
	
	if err := h.setToL2(ctx, key, value, ttl); err != nil {
		return fmt.Errorf("failed to set L2 cache: %w", err)
	}

	if err := h.setToL3(ctx, key, value, ttl); err != nil {
		// L3 failures are not critical
		fmt.Printf("Warning: failed to set L3 cache: %v\n", err)
	}

	return nil
}

// getFromL1 retrieves from L1 cache
func (h *HierarchicalCache) getFromL1(key string) (interface{}, bool) {
	h.l1Mutex.RLock()
	defer h.l1Mutex.RUnlock()

	entry, exists := h.l1Cache[key]
	if !exists {
		return nil, false
	}

	// Check expiration
	if time.Now().After(entry.ExpiresAt) {
		// Schedule for deletion
		select {
		case h.evictChan <- key:
		default:
		}
		return nil, false
	}

	// Update access statistics
	entry.AccessTime = time.Now()
	entry.HitCount++

	return entry.Value, true
}

// setToL1 stores in L1 cache
func (h *HierarchicalCache) setToL1(key string, value interface{}, ttl time.Duration) {
	h.l1Mutex.Lock()
	defer h.l1Mutex.Unlock()

	// Check if we need to evict
	if len(h.l1Cache) >= h.config.L1MaxItems {
		h.evictFromL1()
	}

	entry := &CacheEntry{
		Key:        key,
		Value:      value,
		ExpiresAt:  time.Now().Add(ttl),
		Level:      L1Memory,
		AccessTime: time.Now(),
		HitCount:   0,
	}

	h.l1Cache[key] = entry
}

// evictFromL1 removes entries based on eviction policy
func (h *HierarchicalCache) evictFromL1() {
	if len(h.l1Cache) == 0 {
		return
	}

	var keyToEvict string
	switch h.config.EvictionPolicy {
	case "LRU":
		oldestTime := time.Now()
		for key, entry := range h.l1Cache {
			if entry.AccessTime.Before(oldestTime) {
				oldestTime = entry.AccessTime
				keyToEvict = key
			}
		}
	case "LFU":
		lowestHits := int64(^uint64(0) >> 1) // Max int64
		for key, entry := range h.l1Cache {
			if entry.HitCount < lowestHits {
				lowestHits = entry.HitCount
				keyToEvict = key
			}
		}
	default: // TTL
		earliestExpiry := time.Now().Add(24 * time.Hour)
		for key, entry := range h.l1Cache {
			if entry.ExpiresAt.Before(earliestExpiry) {
				earliestExpiry = entry.ExpiresAt
				keyToEvict = key
			}
		}
	}

	if keyToEvict != "" {
		delete(h.l1Cache, keyToEvict)
		h.metrics.mutex.Lock()
		h.metrics.Evictions++
		h.metrics.mutex.Unlock()
	}
}

// getFromL2 retrieves from SQLite cache
func (h *HierarchicalCache) getFromL2(ctx context.Context, key string) (interface{}, bool) {
	query := `
		SELECT value FROM cache_entries 
		WHERE key = ? AND expires_at > datetime('now')
	`

	var valueJSON string
	err := h.db.QueryRowContext(ctx, query, key).Scan(&valueJSON)
	if err != nil {
		return nil, false
	}

	// Update access statistics
	updateSQL := `
		UPDATE cache_entries 
		SET access_time = datetime('now'), hit_count = hit_count + 1 
		WHERE key = ?
	`
	h.db.ExecContext(ctx, updateSQL, key)

	var value interface{}
	if err := json.Unmarshal([]byte(valueJSON), &value); err != nil {
		return nil, false
	}

	return value, true
}

// setToL2 stores in SQLite cache
func (h *HierarchicalCache) setToL2(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	valueJSON, err := json.Marshal(value)
	if err != nil {
		return err
	}

	insertSQL := `
		INSERT OR REPLACE INTO cache_entries (key, value, expires_at, size)
		VALUES (?, ?, ?, ?)
	`

	expiresAt := time.Now().Add(ttl)
	size := int64(len(valueJSON))

	_, err = h.db.ExecContext(ctx, insertSQL, key, string(valueJSON), expiresAt, size)
	return err
}

// getFromL3 retrieves from GitHub Actions cache
func (h *HierarchicalCache) getFromL3(ctx context.Context, key string) (interface{}, bool) {
	if h.l3Client == nil {
		return nil, false
	}

	data, err := h.l3Client.Get(ctx, key)
	if err != nil {
		return nil, false
	}

	var value interface{}
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, false
	}

	return value, true
}

// setToL3 stores in GitHub Actions cache
func (h *HierarchicalCache) setToL3(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if h.l3Client == nil {
		return nil // L3 cache not available
	}

	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return h.l3Client.Set(ctx, key, data, ttl)
}

// Delete removes a key from all cache levels
func (h *HierarchicalCache) Delete(ctx context.Context, key string) error {
	// Delete from L1
	h.l1Mutex.Lock()
	delete(h.l1Cache, key)
	h.l1Mutex.Unlock()

	// Delete from L2
	deleteSQL := `DELETE FROM cache_entries WHERE key = ?`
	h.db.ExecContext(ctx, deleteSQL, key)

	// Delete from L3
	if h.l3Client != nil {
		h.l3Client.Delete(ctx, key)
	}

	return nil
}

// evictionWorker handles background eviction
func (h *HierarchicalCache) evictionWorker() {
	defer h.wg.Done()

	for {
		select {
		case key := <-h.evictChan:
			h.l1Mutex.Lock()
			delete(h.l1Cache, key)
			h.l1Mutex.Unlock()
		case <-h.stopChan:
			return
		}
	}
}

// cleanupWorker handles periodic cleanup
func (h *HierarchicalCache) cleanupWorker() {
	defer h.wg.Done()

	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			h.cleanup()
		case <-h.stopChan:
			return
		}
	}
}

// cleanup removes expired entries
func (h *HierarchicalCache) cleanup() {
	// Clean L1 cache
	h.l1Mutex.Lock()
	now := time.Now()
	for key, entry := range h.l1Cache {
		if now.After(entry.ExpiresAt) {
			delete(h.l1Cache, key)
		}
	}
	h.l1Mutex.Unlock()

	// Clean L2 cache
	cleanupSQL := `DELETE FROM cache_entries WHERE expires_at < datetime('now')`
	h.db.Exec(cleanupSQL)
}

// Stats returns cache statistics
type Stats struct {
	L1Size    int           `json:"l1_size"`
	L2Size    int           `json:"l2_size"`
	Metrics   *CacheMetrics `json:"metrics"`
	HitRatio  float64       `json:"hit_ratio"`
	L1Ratio   float64       `json:"l1_ratio"`
	L2Ratio   float64       `json:"l2_ratio"`
	L3Ratio   float64       `json:"l3_ratio"`
}

// Stats returns current cache statistics
func (h *HierarchicalCache) Stats() *Stats {
	h.metrics.mutex.RLock()
	defer h.metrics.mutex.RUnlock()

	h.l1Mutex.RLock()
	l1Size := len(h.l1Cache)
	h.l1Mutex.RUnlock()

	var l2Size int
	h.db.QueryRow("SELECT COUNT(*) FROM cache_entries WHERE expires_at > datetime('now')").Scan(&l2Size)

	totalHits := h.metrics.L1Hits + h.metrics.L2Hits + h.metrics.L3Hits
	totalRequests := h.metrics.TotalGets

	stats := &Stats{
		L1Size:  l1Size,
		L2Size:  l2Size,
		Metrics: h.metrics,
	}

	if totalRequests > 0 {
		stats.HitRatio = float64(totalHits) / float64(totalRequests)
		stats.L1Ratio = float64(h.metrics.L1Hits) / float64(totalRequests)
		stats.L2Ratio = float64(h.metrics.L2Hits) / float64(totalRequests)
		stats.L3Ratio = float64(h.metrics.L3Hits) / float64(totalRequests)
	}

	return stats
}

// Close gracefully shuts down the cache
func (h *HierarchicalCache) Close() error {
	close(h.stopChan)
	h.wg.Wait()
	close(h.evictChan)
	return nil
}