package cache

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

// OfflineMode represents the current offline mode state
type OfflineMode int

const (
	OnlineMode OfflineMode = iota
	LimitedMode
	OfflineMode
)

// ServiceStatus represents external service availability
type ServiceStatus struct {
	Name         string    `json:"name"`
	IsAvailable  bool      `json:"is_available"`
	LastCheck    time.Time `json:"last_check"`
	ResponseTime int64     `json:"response_time_ms"`
	ErrorCount   int       `json:"error_count"`
	LastError    string    `json:"last_error,omitempty"`
}

// OfflineDetector monitors external service availability
type OfflineDetector struct {
	services      map[string]ServiceConfig
	status        map[string]*ServiceStatus
	mode          OfflineMode
	db            *sql.DB
	cache         *HierarchicalCache
	mutex         sync.RWMutex
	stopChan      chan struct{}
	wg            sync.WaitGroup
	checkInterval time.Duration
	offlineThreshold int
}

// ServiceConfig holds service monitoring configuration
type ServiceConfig struct {
	Name     string
	URL      string
	Timeout  time.Duration
	Critical bool // If true, affects overall offline mode determination
}

// DefaultServices returns default service configurations
func DefaultServices() map[string]ServiceConfig {
	return map[string]ServiceConfig{
		"github": {
			Name:     "GitHub API",
			URL:      "https://api.github.com/rate_limit",
			Timeout:  10 * time.Second,
			Critical: true,
		},
		"nvd": {
			Name:     "NVD API",
			URL:      "https://services.nvd.nist.gov/rest/json/cves/2.0?resultsPerPage=1",
			Timeout:  15 * time.Second,
			Critical: true,
		},
		"sigstore": {
			Name:     "Sigstore Fulcio",
			URL:      "https://fulcio.sigstore.dev/api/v2/configuration",
			Timeout:  10 * time.Second,
			Critical: false,
		},
	}
}

// NewOfflineDetector creates a new offline mode detector
func NewOfflineDetector(db *sql.DB, cache *HierarchicalCache) *OfflineDetector {
	detector := &OfflineDetector{
		services:         DefaultServices(),
		status:           make(map[string]*ServiceStatus),
		mode:            OnlineMode,
		db:              db,
		cache:           cache,
		stopChan:        make(chan struct{}),
		checkInterval:   30 * time.Second,
		offlineThreshold: 3, // Consider offline after 3 consecutive failures
	}

	// Initialize service status
	for name, service := range detector.services {
		detector.status[name] = &ServiceStatus{
			Name:        service.Name,
			IsAvailable: true,
			LastCheck:   time.Now(),
		}
	}

	return detector
}

// Start begins monitoring external services
func (d *OfflineDetector) Start() {
	d.wg.Add(1)
	go d.monitorServices()
}

// Stop gracefully shuts down the detector
func (d *OfflineDetector) Stop() {
	close(d.stopChan)
	d.wg.Wait()
}

// monitorServices continuously monitors external service availability
func (d *OfflineDetector) monitorServices() {
	defer d.wg.Done()

	ticker := time.NewTicker(d.checkInterval)
	defer ticker.Stop()

	// Initial check
	d.checkAllServices()

	for {
		select {
		case <-ticker.C:
			d.checkAllServices()
		case <-d.stopChan:
			return
		}
	}
}

// checkAllServices checks all configured services
func (d *OfflineDetector) checkAllServices() {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	for name, service := range d.services {
		status := d.checkService(service)
		d.status[name] = status
		
		// Update database
		d.updateServiceStatus(status)
	}

	// Update overall mode
	d.updateMode()
}

// checkService checks a single service
func (d *OfflineDetector) checkService(service ServiceConfig) *ServiceStatus {
	start := time.Now()
	status := &ServiceStatus{
		Name:      service.Name,
		LastCheck: start,
	}

	ctx, cancel := context.WithTimeout(context.Background(), service.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", service.URL, nil)
	if err != nil {
		status.IsAvailable = false
		status.LastError = fmt.Sprintf("Failed to create request: %v", err)
		return status
	}

	client := &http.Client{
		Timeout: service.Timeout,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: 5 * time.Second,
			}).DialContext,
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		status.IsAvailable = false
		status.LastError = fmt.Sprintf("Request failed: %v", err)
		status.ErrorCount = d.getErrorCount(service.Name) + 1
	} else {
		resp.Body.Close()
		status.IsAvailable = resp.StatusCode < 500
		status.ResponseTime = time.Since(start).Milliseconds()
		
		if !status.IsAvailable {
			status.LastError = fmt.Sprintf("HTTP %d", resp.StatusCode)
			status.ErrorCount = d.getErrorCount(service.Name) + 1
		} else {
			status.ErrorCount = 0 // Reset on success
		}
	}

	return status
}

// getErrorCount retrieves the current error count for a service
func (d *OfflineDetector) getErrorCount(serviceName string) int {
	if status, exists := d.status[serviceName]; exists {
		return status.ErrorCount
	}
	return 0
}

// updateServiceStatus updates service status in database
func (d *OfflineDetector) updateServiceStatus(status *ServiceStatus) {
	insertSQL := `
		INSERT OR REPLACE INTO external_service_status 
		(service_name, is_available, last_check, response_time_ms, failure_count, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	d.db.Exec(insertSQL,
		status.Name,
		status.IsAvailable,
		status.LastCheck,
		status.ResponseTime,
		status.ErrorCount,
		time.Now(),
	)
}

// updateMode determines the current operational mode
func (d *OfflineDetector) updateMode() {
	criticalServicesDown := 0
	totalCriticalServices := 0

	for name, service := range d.services {
		if service.Critical {
			totalCriticalServices++
			if status := d.status[name]; status.ErrorCount >= d.offlineThreshold {
				criticalServicesDown++
			}
		}
	}

	previousMode := d.mode

	switch {
	case criticalServicesDown == 0:
		d.mode = OnlineMode
	case criticalServicesDown < totalCriticalServices:
		d.mode = LimitedMode
	default:
		d.mode = OfflineMode
	}

	if d.mode != previousMode {
		log.Printf("Mode changed from %v to %v", previousMode, d.mode)
	}
}

// GetMode returns the current operational mode
func (d *OfflineDetector) GetMode() OfflineMode {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	return d.mode
}

// IsOnline returns true if in online mode
func (d *OfflineDetector) IsOnline() bool {
	return d.GetMode() == OnlineMode
}

// IsOffline returns true if in offline mode
func (d *OfflineDetector) IsOffline() bool {
	return d.GetMode() == OfflineMode
}

// GetServiceStatus returns status for all services
func (d *OfflineDetector) GetServiceStatus() map[string]*ServiceStatus {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	result := make(map[string]*ServiceStatus)
	for name, status := range d.status {
		// Create a copy to avoid race conditions
		result[name] = &ServiceStatus{
			Name:         status.Name,
			IsAvailable:  status.IsAvailable,
			LastCheck:    status.LastCheck,
			ResponseTime: status.ResponseTime,
			ErrorCount:   status.ErrorCount,
			LastError:    status.LastError,
		}
	}

	return result
}

// OfflineModeManager handles offline mode operations
type OfflineModeManager struct {
	detector *OfflineDetector
	cache    *HierarchicalCache
	db       *sql.DB
}

// NewOfflineModeManager creates a new offline mode manager
func NewOfflineModeManager(detector *OfflineDetector, cache *HierarchicalCache, db *sql.DB) *OfflineModeManager {
	return &OfflineModeManager{
		detector: detector,
		cache:    cache,
		db:       db,
	}
}

// GetVulnerabilityData retrieves vulnerability data with fallback strategy
func (o *OfflineModeManager) GetVulnerabilityData(ctx context.Context, cveID string) (interface{}, error) {
	// Try cache first (all modes)
	if data, found := o.cache.Get(ctx, fmt.Sprintf("cve:%s", cveID)); found {
		return data, nil
	}

	mode := o.detector.GetMode()

	switch mode {
	case OnlineMode:
		// Fetch from live APIs
		return o.fetchFromLiveAPI(ctx, cveID)

	case LimitedMode:
		// Try local databases first, then limited API calls
		if data, err := o.fetchFromLocalDB(ctx, cveID); err == nil {
			return data, nil
		}
		return o.fetchFromLiveAPI(ctx, cveID)

	case OfflineMode:
		// Only use local databases and cache
		return o.fetchFromLocalDB(ctx, cveID)
	}

	return nil, fmt.Errorf("no vulnerability data available for %s", cveID)
}

// fetchFromLiveAPI fetches data from external APIs
func (o *OfflineModeManager) fetchFromLiveAPI(ctx context.Context, cveID string) (interface{}, error) {
	// This would integrate with actual API clients
	// For now, return a placeholder
	data := map[string]interface{}{
		"cve_id":      cveID,
		"source":      "live_api",
		"fetched_at":  time.Now(),
		"description": fmt.Sprintf("Live API data for %s", cveID),
	}

	// Cache the result
	o.cache.Set(ctx, fmt.Sprintf("cve:%s", cveID), data, 1*time.Hour)

	return data, nil
}

// fetchFromLocalDB fetches data from local vulnerability database
func (o *OfflineModeManager) fetchFromLocalDB(ctx context.Context, cveID string) (interface{}, error) {
	query := `
		SELECT raw_data FROM vulnerability_cache 
		WHERE cve_id = ? AND (cache_expires_at > datetime('now') OR source = 'local')
		ORDER BY updated_at DESC LIMIT 1
	`

	var rawData string
	err := o.db.QueryRowContext(ctx, query, cveID).Scan(&rawData)
	if err != nil {
		return nil, fmt.Errorf("vulnerability not found in local database: %w", err)
	}

	var data interface{}
	if err := json.Unmarshal([]byte(rawData), &data); err != nil {
		return nil, fmt.Errorf("failed to parse cached data: %w", err)
	}

	return data, nil
}

// SeedLocalDatabase seeds local database with vulnerability data
func (o *OfflineModeManager) SeedLocalDatabase(ctx context.Context, vulnerabilities []map[string]interface{}) error {
	tx, err := o.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	insertSQL := `
		INSERT OR REPLACE INTO vulnerability_cache 
		(cve_id, severity, description, cvss_score, source, raw_data, cache_expires_at)
		VALUES (?, ?, ?, ?, 'local', ?, datetime('now', '+1 year'))
	`

	stmt, err := tx.PrepareContext(ctx, insertSQL)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, vuln := range vulnerabilities {
		cveID, _ := vuln["cve_id"].(string)
		severity, _ := vuln["severity"].(string)
		description, _ := vuln["description"].(string)
		cvssScore, _ := vuln["cvss_score"].(float64)

		rawData, err := json.Marshal(vuln)
		if err != nil {
			continue // Skip malformed entries
		}

		_, err = stmt.ExecContext(ctx, cveID, severity, description, cvssScore, string(rawData))
		if err != nil {
			log.Printf("Failed to insert vulnerability %s: %v", cveID, err)
		}
	}

	return tx.Commit()
}

// GetOfflineCapabilities returns information about offline capabilities
func (o *OfflineModeManager) GetOfflineCapabilities() map[string]interface{} {
	var localVulnCount int
	o.db.QueryRow("SELECT COUNT(*) FROM vulnerability_cache WHERE source = 'local'").Scan(&localVulnCount)

	var cachedVulnCount int
	o.db.QueryRow("SELECT COUNT(*) FROM vulnerability_cache WHERE cache_expires_at > datetime('now')").Scan(&cachedVulnCount)

	mode := o.detector.GetMode()
	services := o.detector.GetServiceStatus()

	return map[string]interface{}{
		"mode":                    mode,
		"local_vulnerabilities":   localVulnCount,
		"cached_vulnerabilities":  cachedVulnCount,
		"service_status":          services,
		"offline_scanning":        true, // Trivy/Grype work offline
		"policy_evaluation":       true, // OPA works offline
		"vulnerability_correlation": localVulnCount > 0,
	}
}