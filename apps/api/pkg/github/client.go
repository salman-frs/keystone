package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/salman-frs/keystone/apps/api/internal/circuit"
)

// RateLimit represents GitHub API rate limit information
type RateLimit struct {
	Limit     int       `json:"limit"`
	Remaining int       `json:"remaining"`
	Reset     time.Time `json:"reset"`
	Used      int       `json:"used"`
}

// RateLimitResponse represents the GitHub rate limit API response
type RateLimitResponse struct {
	Resources struct {
		Core RateLimit `json:"core"`
	} `json:"resources"`
}

// Config holds the GitHub client configuration
type Config struct {
	Token                string
	BaseURL              string
	RateLimitThreshold   int           // Stop at this many remaining requests (80% buffer)
	BackoffBase          time.Duration // Base time for exponential backoff
	MaxBackoff           time.Duration // Maximum backoff time
	CircuitBreakerConfig circuit.Config
}

// DefaultConfig returns a default GitHub client configuration
func DefaultConfig(token string) Config {
	return Config{
		Token:              token,
		BaseURL:            "https://api.github.com",
		RateLimitThreshold: 1000, // 20% of 5000 requests/hour
		BackoffBase:        2 * time.Second,
		MaxBackoff:         60 * time.Second,
		CircuitBreakerConfig: circuit.Config{
			FailureThreshold:   5,
			RecoveryTimeout:    5 * time.Minute,
			SuccessThreshold:   3,
			RequestTimeout:     30 * time.Second,
			MaxConcurrentCalls: 10,
		},
	}
}

// Client provides GitHub API access with rate limiting and circuit breaker
type Client struct {
	config        Config
	httpClient    *http.Client
	circuitBreaker *circuit.Breaker
	lastRateLimit *RateLimit
}

// NewClient creates a new GitHub client
func NewClient(config Config) *Client {
	return &Client{
		config:         config,
		httpClient:     &http.Client{Timeout: 30 * time.Second},
		circuitBreaker: circuit.New(config.CircuitBreakerConfig),
	}
}

// GetRateLimit fetches current rate limit status
func (c *Client) GetRateLimit(ctx context.Context) (*RateLimit, error) {
	var rateLimit *RateLimit
	
	err := c.circuitBreaker.Call(ctx, func() error {
		url := fmt.Sprintf("%s/rate_limit", c.config.BaseURL)
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return err
		}

		req.Header.Set("Authorization", "token "+c.config.Token)
		req.Header.Set("Accept", "application/vnd.github.v3+json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("rate limit API returned status %d", resp.StatusCode)
		}

		var rateLimitResp RateLimitResponse
		if err := json.NewDecoder(resp.Body).Decode(&rateLimitResp); err != nil {
			return err
		}

		rateLimit = &rateLimitResp.Resources.Core
		c.lastRateLimit = rateLimit
		return nil
	})

	return rateLimit, err
}

// shouldBackoff checks if we should back off based on rate limiting
func (c *Client) shouldBackoff() (bool, time.Duration) {
	if c.lastRateLimit == nil {
		return false, 0
	}

	// Check if we're approaching the rate limit threshold
	if c.lastRateLimit.Remaining <= c.config.RateLimitThreshold {
		// Calculate exponential backoff
		factor := float64(c.config.RateLimitThreshold - c.lastRateLimit.Remaining)
		backoffDuration := time.Duration(math.Pow(2, factor/100)) * c.config.BackoffBase
		
		if backoffDuration > c.config.MaxBackoff {
			backoffDuration = c.config.MaxBackoff
		}

		return true, backoffDuration
	}

	return false, 0
}

// makeRequest executes an HTTP request with rate limiting and circuit breaker protection
func (c *Client) makeRequest(ctx context.Context, method, url string, body io.Reader) (*http.Response, error) {
	var resp *http.Response
	
	err := c.circuitBreaker.Call(ctx, func() error {
		// Check rate limit before making request
		if shouldBackoff, backoffDuration := c.shouldBackoff(); shouldBackoff {
			select {
			case <-time.After(backoffDuration):
				// Continue after backoff
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		req, err := http.NewRequestWithContext(ctx, method, url, body)
		if err != nil {
			return err
		}

		req.Header.Set("Authorization", "token "+c.config.Token)
		req.Header.Set("Accept", "application/vnd.github.v3+json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err = c.httpClient.Do(req)
		if err != nil {
			return err
		}

		// Update rate limit from response headers
		c.updateRateLimitFromHeaders(resp.Header)

		// Handle rate limit exceeded
		if resp.StatusCode == http.StatusForbidden {
			if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
				if seconds, err := strconv.Atoi(retryAfter); err == nil {
					select {
					case <-time.After(time.Duration(seconds) * time.Second):
						// Continue after retry delay
					case <-ctx.Done():
						return ctx.Err()
					}
				}
			}
			return fmt.Errorf("rate limit exceeded")
		}

		if resp.StatusCode >= 500 {
			return fmt.Errorf("server error: %d", resp.StatusCode)
		}

		return nil
	})

	return resp, err
}

// updateRateLimitFromHeaders updates rate limit info from response headers
func (c *Client) updateRateLimitFromHeaders(headers http.Header) {
	limitStr := headers.Get("X-RateLimit-Limit")
	remainingStr := headers.Get("X-RateLimit-Remaining")
	resetStr := headers.Get("X-RateLimit-Reset")
	usedStr := headers.Get("X-RateLimit-Used")

	if limitStr == "" || remainingStr == "" || resetStr == "" {
		return
	}

	limit, _ := strconv.Atoi(limitStr)
	remaining, _ := strconv.Atoi(remainingStr)
	resetUnix, _ := strconv.ParseInt(resetStr, 10, 64)
	used, _ := strconv.Atoi(usedStr)

	c.lastRateLimit = &RateLimit{
		Limit:     limit,
		Remaining: remaining,
		Reset:     time.Unix(resetUnix, 0),
		Used:      used,
	}
}

// GetSecurityAdvisories fetches security advisories from GitHub
func (c *Client) GetSecurityAdvisories(ctx context.Context, perPage int) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("%s/advisories?per_page=%d", c.config.BaseURL, perPage)
	
	resp, err := c.makeRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("advisories API returned status %d", resp.StatusCode)
	}

	var advisories []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&advisories); err != nil {
		return nil, err
	}

	return advisories, nil
}

// GetRepositoryAdvisories fetches security advisories for a specific repository
func (c *Client) GetRepositoryAdvisories(ctx context.Context, owner, repo string) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/security-advisories", c.config.BaseURL, owner, repo)
	
	resp, err := c.makeRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("repository advisories API returned status %d", resp.StatusCode)
	}

	var advisories []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&advisories); err != nil {
		return nil, err
	}

	return advisories, nil
}

// GetRepository fetches repository information
func (c *Client) GetRepository(ctx context.Context, owner, repo string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/repos/%s/%s", c.config.BaseURL, owner, repo)
	
	resp, err := c.makeRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("repository API returned status %d", resp.StatusCode)
	}

	var repository map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&repository); err != nil {
		return nil, err
	}

	return repository, nil
}

// Stats returns client statistics including circuit breaker state
type Stats struct {
	CircuitBreakerState circuit.State
	LastRateLimit       *RateLimit
	CircuitBreakerStats circuit.Stats
}

// Stats returns current client statistics
func (c *Client) Stats() Stats {
	return Stats{
		CircuitBreakerState: c.circuitBreaker.State(),
		LastRateLimit:       c.lastRateLimit,
		CircuitBreakerStats: c.circuitBreaker.Stats(),
	}
}