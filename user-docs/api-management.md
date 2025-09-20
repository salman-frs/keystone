# API Management Guide

## Overview

This guide covers advanced API management features including rate limiting, circuit breaker patterns, and monitoring for the Keystone DevSecOps platform. These features ensure reliable external service integration and graceful degradation during service outages.

## Rate Limiting Strategy

### GitHub API Rate Limits

**Primary Limits:**
- **REST API:** 5,000 requests per hour (authenticated)
- **GraphQL API:** 5,000 points per hour (variable consumption)
- **Secondary Limit:** 100 content creation requests per minute
- **Search API:** 30 requests per minute

**Configuration:**
```bash
# Environment variables for rate limiting
export GITHUB_API_RATE_LIMIT_THRESHOLD=1000  # 20% buffer (1000 of 5000)
export GITHUB_API_BACKOFF_BASE=2             # 2 second base backoff
export GITHUB_API_MAX_BACKOFF=60             # Maximum 60 seconds
export GITHUB_API_CIRCUIT_TIMEOUT=300       # 5 minutes circuit timeout
```

**Monitoring Rate Limits:**
```bash
# Check current rate limit status
curl -H "Authorization: token $GITHUB_TOKEN" \
  "https://api.github.com/rate_limit" | jq '.resources.core'
```

### Adaptive Rate Limiting

The system implements adaptive rate limiting that automatically adjusts request frequency based on:
- Current rate limit consumption
- Response time trends
- Error rate patterns
- Time until rate limit reset

**Implementation Example:**
```go
// apps/api/pkg/github/client.go
func (c *Client) shouldBackoff() (bool, time.Duration) {
    if c.lastRateLimit.Remaining <= c.config.RateLimitThreshold {
        factor := float64(c.config.RateLimitThreshold - c.lastRateLimit.Remaining)
        backoffDuration := time.Duration(math.Pow(2, factor/100)) * c.config.BackoffBase
        
        if backoffDuration > c.config.MaxBackoff {
            backoffDuration = c.config.MaxBackoff
        }
        return true, backoffDuration
    }
    return false, 0
}
```

## Circuit Breaker Pattern

### Configuration

Circuit breaker states and thresholds:
- **CLOSED:** Normal operation, requests flow through
- **OPEN:** Service unavailable, requests fail fast
- **HALF_OPEN:** Testing service recovery, limited requests

**Default Settings:**
```go
circuit.Config{
    FailureThreshold:   10,               // Open after 10 failures
    RecoveryTimeout:    5 * time.Minute,  // Wait 5 minutes before retry
    SuccessThreshold:   3,                // Close after 3 successes
    RequestTimeout:     30 * time.Second, // Individual request timeout
    MaxConcurrentCalls: 5,                // Max concurrent in half-open
}
```

### Usage Example

```go
ctx := context.Background()
breaker := circuit.New(circuit.DefaultConfig())

err := breaker.Call(ctx, func() error {
    _, err := http.Get("https://api.github.com/rate_limit")
    return err
})

if err == circuit.ErrCircuitOpen {
    log.Println("Circuit breaker open, using cached data")
    return getCachedData()
}
```

### Monitoring Circuit Breaker

**Check Circuit State:**
```go
stats := circuitBreaker.Stats()
switch stats.State {
case circuit.StateClosed:
    log.Println("Circuit is healthy")
case circuit.StateOpen:
    log.Printf("Circuit is open, %d failures", stats.FailureCount)
case circuit.StateHalfOpen:
    log.Printf("Circuit testing recovery, %d active calls", stats.ActiveCalls)
}
```

## Request Queue Management

### Priority-Based Processing

The queue system processes requests based on priority levels:
1. **Critical:** Security vulnerabilities, immediate processing
2. **High:** Important updates, processed within 5 minutes
3. **Normal:** Regular operations, processed within 15 minutes
4. **Low:** Background tasks, processed when capacity allows

**Queue Configuration:**
```go
queueConfig := github.QueueConfig{
    Workers:       5,               // Number of worker goroutines
    MaxRetries:    3,               // Retry attempts for failed requests
    RetryDelay:    5 * time.Second, // Delay between retries
    BatchSize:     10,              // Requests per batch
    BatchInterval: 1 * time.Second, // Batch processing interval
    QueueSize:     1000,            // Maximum queued requests
}
```

**Usage Example:**
```go
queue := github.NewQueue(client, queueConfig)
queue.Start()

// Enqueue critical vulnerability check
result := queue.Enqueue(ctx, "cve-check-123", github.PriorityCritical, func(ctx context.Context) error {
    return processVulnerability(ctx, "CVE-2024-1234")
})

select {
case err := <-result:
    if err != nil {
        log.Printf("Request failed: %v", err)
    }
case <-ctx.Done():
    log.Println("Request cancelled")
}
```

### Batch Processing

Batch processing optimizes API usage by grouping related requests:

```yaml
# GitHub Actions workflow example
- name: Process Vulnerabilities in Batches
  run: |
    # Process critical vulnerabilities immediately
    jq -r '.vulnerabilities[] | select(.severity=="CRITICAL") | .id' results.json | \
      head -10 | xargs -I {} ./scripts/process-vulnerability.sh {} critical

    # Batch process high-priority vulnerabilities
    jq -r '.vulnerabilities[] | select(.severity=="HIGH") | .id' results.json | \
      head -20 | xargs -P 3 -I {} ./scripts/process-vulnerability.sh {} high
```

## Caching Strategy

### Multi-Level Cache Architecture

The hierarchical cache provides three levels of data storage:

**L1 Cache (In-Memory):**
- Capacity: 1,000 items (configurable)
- TTL: 5 minutes
- Eviction: LRU policy
- Best for: Frequently accessed data

**L2 Cache (SQLite):**
- Capacity: 100,000 items
- TTL: 1 hour
- Persistence: Survives application restarts
- Best for: Recent scan results, API responses

**L3 Cache (GitHub Actions):**
- Capacity: 10 GB per repository
- TTL: 24 hours
- Persistence: Across workflow runs
- Best for: Large datasets, vulnerability databases

### Cache Key Strategies

**Vulnerability Data:**
```
cve:{cve_id}                    # Individual CVE details
repo:{owner}/{name}:vulns       # Repository vulnerability scan
scan:{scan_id}:results          # Complete scan results
advisory:{ghsa_id}              # GitHub security advisory
```

**API Responses:**
```
github:rate_limit               # Rate limit status
github:advisories:{page}        # Paginated advisories
nvd:cve:{cve_id}               # NVD vulnerability data
```

**Cache TTL Guidelines:**
```go
var cacheTTL = map[string]time.Duration{
    "rate_limit":        30 * time.Second,  // Frequent updates
    "repository_meta":   5 * time.Minute,   // Relatively stable
    "security_advisory": 1 * time.Hour,     // Updated periodically
    "cve_details":       24 * time.Hour,    // Rarely change
    "scan_results":      7 * 24 * time.Hour, // Persistent for analysis
}
```

## Offline Mode Management

### Automatic Fallback Strategy

The system automatically detects service availability and switches between modes:

1. **Online Mode:** All external services available
2. **Limited Mode:** Some services unavailable, degraded functionality
3. **Offline Mode:** External services unavailable, local-only operation

**Service Health Monitoring:**
```go
services := map[string]ServiceConfig{
    "github": {
        URL:      "https://api.github.com/rate_limit",
        Timeout:  10 * time.Second,
        Critical: true,
    },
    "nvd": {
        URL:      "https://services.nvd.nist.gov/rest/json/cves/2.0?resultsPerPage=1",
        Timeout:  15 * time.Second,
        Critical: true,
    },
    "sigstore": {
        URL:      "https://fulcio.sigstore.dev/api/v2/configuration",
        Timeout:  10 * time.Second,
        Critical: false,
    },
}
```

### Local Data Sources

**Offline Scanning Capabilities:**
- **Trivy Database:** Local vulnerability database updated weekly
- **Grype Database:** Alternative vulnerability source
- **NIST CVE Feeds:** JSON feeds for offline CVE lookup
- **GitHub Advisories Cache:** Cached security advisories

**Database Seeding:**
```bash
#!/bin/bash
# Script: seed-offline-data.sh

# Download NVD CVE feeds
curl -o nvd-recent.json.gz \
  "https://nvd.nist.gov/feeds/json/cve/1.1/nvdcve-1.1-recent.json.gz"
gunzip nvd-recent.json.gz

# Update Trivy database
trivy image --download-db-only

# Update Grype database
grype db update

# Seed local database
go run ./cmd/seed/main.go --source nvd-recent.json --cache-duration 168h
```

## Monitoring and Alerting

### Key Metrics

**API Performance Metrics:**
- Request latency (p50, p95, p99)
- Success rate percentage
- Rate limit consumption
- Circuit breaker state changes

**Cache Performance Metrics:**
- Hit ratio per cache level
- Eviction rate
- Memory usage
- Storage size

**System Health Metrics:**
- Service availability percentage
- Offline mode frequency
- Error rate by service
- Recovery time after outages

### Prometheus Integration

**Example Metrics Collection:**
```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'keystone-api'
    static_configs:
      - targets: ['localhost:8080']
    scrape_interval: 30s
    metrics_path: /metrics
```

**Custom Metrics:**
```go
// Define custom metrics
var (
    apiRequestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "keystone_api_request_duration_seconds",
            Help: "Duration of API requests",
        },
        []string{"service", "endpoint", "status"},
    )

    cacheHitRatio = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "keystone_cache_hit_ratio",
            Help: "Cache hit ratio by level",
        },
        []string{"level"},
    )

    circuitBreakerState = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "keystone_circuit_breaker_state",
            Help: "Circuit breaker state (0=closed, 1=open, 2=half-open)",
        },
        []string{"service"},
    )
)
```

### Alerting Rules

**Critical Alerts:**
```yaml
# alerts.yml
groups:
  - name: keystone_critical
    rules:
      - alert: ExternalServiceDown
        expr: keystone_service_availability < 0.95
        for: 5m
        annotations:
          summary: "External service {{ $labels.service }} is experiencing issues"

      - alert: HighRateLimitConsumption
        expr: keystone_rate_limit_usage > 0.8
        for: 2m
        annotations:
          summary: "Rate limit consumption above 80% for {{ $labels.service }}"

      - alert: CircuitBreakerOpen
        expr: keystone_circuit_breaker_state == 1
        for: 1m
        annotations:
          summary: "Circuit breaker open for {{ $labels.service }}"
```

### Dashboard Configuration

**Grafana Dashboard Panels:**

1. **API Health Overview**
   - Service availability status
   - Request success rate
   - Average response time

2. **Rate Limiting Status**
   - Current rate limit usage
   - Remaining requests
   - Time until reset

3. **Cache Performance**
   - Hit ratio by cache level
   - Cache size and memory usage
   - Eviction rate trends

4. **Circuit Breaker Status**
   - Current state by service
   - Failure count trends
   - Recovery patterns

## Troubleshooting Common Issues

### High Rate Limit Consumption

**Symptoms:**
- Requests taking longer to complete
- Rate limit warnings in logs
- Circuit breaker opening frequently

**Solutions:**
1. **Enable Request Batching:**
   ```bash
   export GITHUB_API_BATCH_SIZE=10
   export GITHUB_API_BATCH_INTERVAL=5s
   ```

2. **Increase Cache TTL:**
   ```bash
   export CACHE_TTL_MULTIPLIER=2  # Double all TTL values
   ```

3. **Implement Request Prioritization:**
   ```go
   // Use higher priority for critical requests
   queue.Enqueue(ctx, id, github.PriorityCritical, requestFunc)
   ```

### Circuit Breaker Stuck Open

**Symptoms:**
- All requests failing with circuit breaker error
- Service appears healthy but circuit won't close

**Solutions:**
1. **Check Service Health:**
   ```bash
   curl -I https://api.github.com/rate_limit
   ```

2. **Reset Circuit Breaker:**
   ```go
   circuitBreaker.Reset()
   ```

3. **Adjust Thresholds:**
   ```go
   config.FailureThreshold = 20  // More lenient
   config.RecoveryTimeout = 1 * time.Minute  // Faster recovery
   ```

### Cache Performance Issues

**Symptoms:**
- Low cache hit ratio
- High memory usage
- Slow response times

**Solutions:**
1. **Analyze Cache Statistics:**
   ```bash
   curl localhost:8080/api/cache/stats | jq '.'
   ```

2. **Optimize Cache Keys:**
   ```go
   // Use consistent, predictable keys
   key := fmt.Sprintf("cve:%s:summary", cveID)
   ```

3. **Adjust Cache Size:**
   ```bash
   export CACHE_L1_MAX_ITEMS=2000
   export CACHE_MAX_MEMORY_MB=200
   ```

### Offline Mode Not Triggering

**Symptoms:**
- Services unavailable but system stays online
- No fallback to cached data

**Solutions:**
1. **Check Service Configuration:**
   ```bash
   # Verify critical services are marked correctly
   grep -r "Critical.*true" apps/api/
   ```

2. **Lower Failure Threshold:**
   ```bash
   export OFFLINE_FAILURE_THRESHOLD=2  # Trigger faster
   ```

3. **Test Offline Detection:**
   ```go
   // Force offline mode for testing
   detector.SetMode(OfflineMode)
   ```

## Best Practices

### API Client Implementation

1. **Always Use Circuit Breakers:** Protect against cascading failures
2. **Implement Exponential Backoff:** Reduce load on recovering services
3. **Cache Aggressively:** Minimize external API calls
4. **Monitor Rate Limits:** Stay within service quotas
5. **Handle Failures Gracefully:** Provide meaningful fallbacks

### Cache Management

1. **Use Appropriate TTL:** Balance freshness with performance
2. **Implement Cache Warming:** Pre-populate frequently used data
3. **Monitor Hit Ratios:** Optimize cache keys and sizes
4. **Clean Up Expired Entries:** Prevent memory leaks
5. **Test Cache Invalidation:** Ensure data consistency

### Monitoring and Alerting

1. **Define SLIs/SLOs:** Set measurable reliability targets
2. **Alert on Leading Indicators:** Catch issues before they impact users
3. **Use Runbooks:** Document response procedures
4. **Regular Health Checks:** Proactive monitoring
5. **Capacity Planning:** Monitor trends and plan for growth

### Development Workflow

1. **Test with Realistic Data:** Use production-like datasets
2. **Simulate Failures:** Test circuit breakers and fallbacks
3. **Load Testing:** Validate performance under load
4. **Monitor Deployments:** Watch metrics during releases
5. **Document Changes:** Keep API documentation current