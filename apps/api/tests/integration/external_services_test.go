package integration

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/salman-frs/keystone/apps/api/internal/cache"
	"github.com/salman-frs/keystone/apps/api/internal/circuit"
	"github.com/salman-frs/keystone/apps/api/pkg/github"

	_ "github.com/mattn/go-sqlite3"
)

// ExternalServicesTestSuite tests external service integration
type ExternalServicesTestSuite struct {
	suite.Suite
	db       *sql.DB
	cache    *cache.HierarchicalCache
	detector *cache.OfflineDetector
	client   *github.Client
	server   *httptest.Server
}

// SetupSuite runs once before all tests
func (suite *ExternalServicesTestSuite) SetupSuite() {
	// Create in-memory test database
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(suite.T(), err)
	suite.db = db

	// Initialize cache
	cacheConfig := cache.DefaultCacheConfig()
	cacheConfig.L1MaxItems = 100
	hierCache, err := cache.NewHierarchicalCache(cacheConfig, db, nil)
	require.NoError(suite.T(), err)
	suite.cache = hierCache

	// Initialize offline detector
	suite.detector = cache.NewOfflineDetector(db, hierCache)

	// Start mock server
	suite.server = httptest.NewServer(suite.createMockHandler())

	// Initialize GitHub client with test server
	config := github.DefaultConfig("test-token")
	config.BaseURL = suite.server.URL
	config.CircuitBreakerConfig.FailureThreshold = 3
	config.CircuitBreakerConfig.RecoveryTimeout = 1 * time.Second
	suite.client = github.NewClient(config)
}

// TearDownSuite runs once after all tests
func (suite *ExternalServicesTestSuite) TearDownSuite() {
	suite.cache.Close()
	suite.db.Close()
	suite.server.Close()
}

// SetupTest runs before each test
func (suite *ExternalServicesTestSuite) SetupTest() {
	// Clean database tables
	suite.db.Exec("DELETE FROM cache_entries")
	suite.db.Exec("DELETE FROM external_service_status")
}

// createMockHandler creates mock HTTP handlers for testing
func (suite *ExternalServicesTestSuite) createMockHandler() http.Handler {
	mux := http.NewServeMux()

	// Mock rate limit endpoint
	mux.HandleFunc("/rate_limit", func(w http.ResponseWriter, r *http.Request) {
		response := `{
			"resources": {
				"core": {
					"limit": 5000,
					"remaining": 4950,
					"reset": %d,
					"used": 50
				}
			}
		}`
		resetTime := time.Now().Add(1 * time.Hour).Unix()
		fmt.Fprintf(w, response, resetTime)
	})

	// Mock advisories endpoint
	mux.HandleFunc("/advisories", func(w http.ResponseWriter, r *http.Request) {
		response := `[
			{
				"ghsa_id": "GHSA-test-1234",
				"summary": "Test vulnerability",
				"severity": "high",
				"cve_id": "CVE-2024-1234"
			}
		]`
		fmt.Fprint(w, response)
	})

	// Mock repository endpoint
	mux.HandleFunc("/repos/", func(w http.ResponseWriter, r *http.Request) {
		response := `{
			"name": "test-repo",
			"full_name": "owner/test-repo",
			"private": false
		}`
		fmt.Fprint(w, response)
	})

	// Mock NVD endpoint
	mux.HandleFunc("/nvd/cves", func(w http.ResponseWriter, r *http.Request) {
		response := `{
			"resultsPerPage": 1,
			"startIndex": 0,
			"totalResults": 1,
			"vulnerabilities": [
				{
					"cve": {
						"id": "CVE-2024-1234",
						"metrics": {
							"cvssMetricV31": [
								{
									"cvssData": {
										"baseScore": 7.5
									}
								}
							]
						}
					}
				}
			]
		}`
		fmt.Fprint(w, response)
	})

	return mux
}

// TestGitHubAPIRateLimit tests GitHub API rate limiting
func (suite *ExternalServicesTestSuite) TestGitHubAPIRateLimit() {
	ctx := context.Background()

	// Test rate limit retrieval
	rateLimit, err := suite.client.GetRateLimit(ctx)
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), 5000, rateLimit.Limit)
	assert.Equal(suite.T(), 4950, rateLimit.Remaining)
}

// TestGitHubAPICircuitBreaker tests circuit breaker functionality
func (suite *ExternalServicesTestSuite) TestGitHubAPICircuitBreaker() {
	ctx := context.Background()

	// Configure server to return errors
	suite.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	// Make multiple requests to trigger circuit breaker
	for i := 0; i < 5; i++ {
		_, err := suite.client.GetRateLimit(ctx)
		assert.Error(suite.T(), err)
	}

	// Check circuit breaker state
	stats := suite.client.Stats()
	assert.Equal(suite.T(), circuit.StateOpen, stats.CircuitBreakerState)

	// Wait for recovery timeout
	time.Sleep(1100 * time.Millisecond)

	// Restore normal server behavior
	suite.server.Config.Handler = suite.createMockHandler()

	// Circuit should allow requests again
	_, err := suite.client.GetRateLimit(ctx)
	assert.NoError(suite.T(), err)
}

// TestHierarchicalCache tests multi-level caching
func (suite *ExternalServicesTestSuite) TestHierarchicalCache() {
	ctx := context.Background()

	// Test cache miss
	value, found := suite.cache.Get(ctx, "test-key")
	assert.False(suite.T(), found)
	assert.Nil(suite.T(), value)

	// Test cache set and get
	testData := map[string]interface{}{
		"key":   "value",
		"count": 42,
	}

	err := suite.cache.Set(ctx, "test-key", testData, 1*time.Hour)
	require.NoError(suite.T(), err)

	// Test L1 cache hit
	value, found = suite.cache.Get(ctx, "test-key")
	assert.True(suite.T(), found)
	assert.Equal(suite.T(), testData, value)

	// Clear L1 cache to test L2
	suite.cache.Delete(ctx, "test-key")
	
	// Should still be in L2
	value, found = suite.cache.Get(ctx, "test-key")
	assert.True(suite.T(), found)

	// Test cache statistics
	stats := suite.cache.Stats()
	assert.Greater(suite.T(), stats.Metrics.TotalGets, int64(0))
	assert.Greater(suite.T(), stats.Metrics.TotalSets, int64(0))
}

// TestOfflineDetection tests offline mode detection
func (suite *ExternalServicesTestSuite) TestOfflineDetection() {
	// Start with online mode
	suite.detector.Start()
	defer suite.detector.Stop()

	// Wait for initial check
	time.Sleep(100 * time.Millisecond)
	assert.True(suite.T(), suite.detector.IsOnline())

	// Simulate service failure
	suite.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	// Force service check
	time.Sleep(500 * time.Millisecond)

	// Check service status
	services := suite.detector.GetServiceStatus()
	assert.NotEmpty(suite.T(), services)

	// Restore service
	suite.server.Config.Handler = suite.createMockHandler()
}

// TestOfflineModeManager tests offline mode operations
func (suite *ExternalServicesTestSuite) TestOfflineModeManager() {
	ctx := context.Background()

	manager := cache.NewOfflineModeManager(suite.detector, suite.cache, suite.db)

	// Seed local database
	vulnerabilities := []map[string]interface{}{
		{
			"cve_id":      "CVE-2024-1234",
			"severity":    "HIGH",
			"description": "Test vulnerability",
			"cvss_score":  7.5,
		},
	}

	err := manager.SeedLocalDatabase(ctx, vulnerabilities)
	require.NoError(suite.T(), err)

	// Test vulnerability data retrieval
	data, err := manager.GetVulnerabilityData(ctx, "CVE-2024-1234")
	require.NoError(suite.T(), err)
	assert.NotNil(suite.T(), data)

	// Test offline capabilities
	capabilities := manager.GetOfflineCapabilities()
	assert.Equal(suite.T(), 1, capabilities["local_vulnerabilities"])
}

// TestRateLimitHandling tests rate limit scenarios
func (suite *ExternalServicesTestSuite) TestRateLimitHandling() {
	ctx := context.Background()

	// Configure server to return rate limit headers
	suite.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Limit", "5000")
		w.Header().Set("X-RateLimit-Remaining", "10") // Low remaining
		w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(1*time.Hour).Unix()))
		w.Header().Set("X-RateLimit-Used", "4990")

		if r.URL.Path == "/rate_limit" {
			response := `{
				"resources": {
					"core": {
						"limit": 5000,
						"remaining": 10,
						"reset": %d,
						"used": 4990
					}
				}
			}`
			resetTime := time.Now().Add(1 * time.Hour).Unix()
			fmt.Fprintf(w, response, resetTime)
		}
	})

	// Get rate limit
	rateLimit, err := suite.client.GetRateLimit(ctx)
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), 10, rateLimit.Remaining)

	// Subsequent requests should be rate limited
	start := time.Now()
	_, err = suite.client.GetRateLimit(ctx)
	duration := time.Since(start)

	// Should have been delayed due to rate limiting
	assert.Greater(suite.T(), duration, 1*time.Second)
}

// TestAPIFailureScenarios tests various API failure conditions
func (suite *ExternalServicesTestSuite) TestAPIFailureScenarios() {
	ctx := context.Background()

	testCases := []struct {
		name       string
		statusCode int
		expectErr  bool
	}{
		{"Success", http.StatusOK, false},
		{"Not Found", http.StatusNotFound, true},
		{"Forbidden", http.StatusForbidden, true},
		{"Server Error", http.StatusInternalServerError, true},
		{"Bad Gateway", http.StatusBadGateway, true},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Configure server response
			suite.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
				if tc.statusCode == http.StatusOK && r.URL.Path == "/rate_limit" {
					response := `{
						"resources": {
							"core": {
								"limit": 5000,
								"remaining": 4950,
								"reset": %d,
								"used": 50
							}
						}
					}`
					resetTime := time.Now().Add(1 * time.Hour).Unix()
					fmt.Fprintf(w, response, resetTime)
				}
			})

			_, err := suite.client.GetRateLimit(ctx)
			if tc.expectErr {
				assert.Error(suite.T(), err)
			} else {
				assert.NoError(suite.T(), err)
			}
		})
	}
}

// TestCachePerformance tests cache performance characteristics
func (suite *ExternalServicesTestSuite) TestCachePerformance() {
	ctx := context.Background()

	// Measure cache set performance
	start := time.Now()
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("perf-test-%d", i)
		data := map[string]interface{}{"value": i}
		suite.cache.Set(ctx, key, data, 1*time.Hour)
	}
	setDuration := time.Since(start)

	// Measure cache get performance
	start = time.Now()
	hitCount := 0
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("perf-test-%d", i)
		if _, found := suite.cache.Get(ctx, key); found {
			hitCount++
		}
	}
	getDuration := time.Since(start)

	// Performance assertions
	assert.Less(suite.T(), setDuration, 5*time.Second, "Cache set operations too slow")
	assert.Less(suite.T(), getDuration, 1*time.Second, "Cache get operations too slow")
	assert.Equal(suite.T(), 1000, hitCount, "Not all items found in cache")

	// Check cache statistics
	stats := suite.cache.Stats()
	assert.Greater(suite.T(), stats.HitRatio, 0.9, "Cache hit ratio too low")
}

// TestConcurrentOperations tests concurrent cache and API operations
func (suite *ExternalServicesTestSuite) TestConcurrentOperations() {
	ctx := context.Background()
	const numGoroutines = 10
	const operationsPerGoroutine = 100

	// Channel to collect errors
	errChan := make(chan error, numGoroutines)

	// Run concurrent operations
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { errChan <- nil }()

			for j := 0; j < operationsPerGoroutine; j++ {
				key := fmt.Sprintf("concurrent-%d-%d", id, j)
				data := map[string]interface{}{"goroutine": id, "operation": j}

				// Set data
				if err := suite.cache.Set(ctx, key, data, 1*time.Hour); err != nil {
					errChan <- fmt.Errorf("set failed: %w", err)
					return
				}

				// Get data
				if _, found := suite.cache.Get(ctx, key); !found {
					errChan <- fmt.Errorf("get failed for key %s", key)
					return
				}

				// Make API call
				if _, err := suite.client.GetRateLimit(ctx); err != nil {
					// API errors are acceptable in concurrent tests
				}
			}
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		err := <-errChan
		assert.NoError(suite.T(), err)
	}
}

// TestIntegrationWithMockExternalServices runs the full test suite
func TestExternalServicesIntegration(t *testing.T) {
	// Skip integration tests in short mode
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	// Skip if database driver not available
	if os.Getenv("SKIP_INTEGRATION_TESTS") == "true" {
		t.Skip("Integration tests disabled")
	}

	suite.Run(t, new(ExternalServicesTestSuite))
}

// Benchmark tests for performance validation
func BenchmarkCacheOperations(b *testing.B) {
	db, _ := sql.Open("sqlite3", ":memory:")
	defer db.Close()

	config := cache.DefaultCacheConfig()
	hierCache, _ := cache.NewHierarchicalCache(config, db, nil)
	defer hierCache.Close()

	ctx := context.Background()

	b.Run("Set", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("bench-set-%d", i)
			data := map[string]interface{}{"value": i}
			hierCache.Set(ctx, key, data, 1*time.Hour)
		}
	})

	// Seed data for get benchmark
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("bench-get-%d", i)
		data := map[string]interface{}{"value": i}
		hierCache.Set(ctx, key, data, 1*time.Hour)
	}

	b.Run("Get", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("bench-get-%d", i%1000)
			hierCache.Get(ctx, key)
		}
	})
}