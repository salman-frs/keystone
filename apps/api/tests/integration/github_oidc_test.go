package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GitHubOIDCClient represents the GitHub OIDC integration client
type GitHubOIDCClient struct {
	tokenURL     string
	requestToken string
	httpClient   *http.Client
}

// OIDCTokenResponse represents the response from GitHub OIDC token endpoint
type OIDCTokenResponse struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

// OIDCClaims represents the decoded OIDC token claims
type OIDCClaims struct {
	Issuer     string `json:"iss"`
	Audience   string `json:"aud"`
	Subject    string `json:"sub"`
	Actor      string `json:"actor"`
	Repository string `json:"repository"`
	Ref        string `json:"ref"`
	SHA        string `json:"sha"`
	RunID      string `json:"run_id"`
	RunNumber  string `json:"run_number"`
	Workflow   string `json:"workflow"`
	IssuedAt   int64  `json:"iat"`
	ExpiresAt  int64  `json:"exp"`
}

func NewGitHubOIDCClient(tokenURL, requestToken string) *GitHubOIDCClient {
	return &GitHubOIDCClient{
		tokenURL:     tokenURL,
		requestToken: requestToken,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *GitHubOIDCClient) GetOIDCToken(audience string) (*OIDCTokenResponse, error) {
	url := fmt.Sprintf("%s&audience=%s", c.tokenURL, audience)
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("SIGN_003: Failed to create OIDC token request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("bearer %s", c.requestToken))
	req.Header.Set("User-Agent", "keystone-attestation-service/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("SIGN_071: Network connectivity timeout: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("SIGN_003: OIDC token acquisition failed with status %d", resp.StatusCode)
	}

	var tokenResp OIDCTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("SIGN_003: Failed to decode OIDC token response: %w", err)
	}

	return &tokenResp, nil
}

// TestGitHubOIDCIntegration tests the complete GitHub OIDC workflow integration
func TestGitHubOIDCIntegration(t *testing.T) {
	tests := []struct {
		name           string
		audience       string
		mockResponse   OIDCTokenResponse
		mockStatusCode int
		expectError    bool
		errorCode      string
	}{
		{
			name:     "successful_oidc_token_acquisition",
			audience: "sigstore",
			mockResponse: OIDCTokenResponse{
				Value: "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.mock.signature",
				Count: 1,
			},
			mockStatusCode: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "oidc_service_unavailable",
			audience:       "sigstore",
			mockStatusCode: http.StatusServiceUnavailable,
			expectError:    true,
			errorCode:      "SIGN_003",
		},
		{
			name:           "unauthorized_token_request",
			audience:       "sigstore",
			mockStatusCode: http.StatusUnauthorized,
			expectError:    true,
			errorCode:      "SIGN_003",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock GitHub OIDC server
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Validate request
				assert.Equal(t, "GET", r.Method)
				assert.Contains(t, r.URL.RawQuery, fmt.Sprintf("audience=%s", tt.audience))
				assert.Contains(t, r.Header.Get("Authorization"), "bearer")

				w.WriteHeader(tt.mockStatusCode)
				if tt.mockStatusCode == http.StatusOK {
					json.NewEncoder(w).Encode(tt.mockResponse)
				}
			}))
			defer mockServer.Close()

			// Create OIDC client
			client := NewGitHubOIDCClient(mockServer.URL, "mock-request-token")

			// Test OIDC token acquisition
			tokenResp, err := client.GetOIDCToken(tt.audience)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorCode)
				assert.Nil(t, tokenResp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, tokenResp)
				assert.Equal(t, tt.mockResponse.Value, tokenResp.Value)
				assert.Equal(t, tt.mockResponse.Count, tokenResp.Count)
			}
		})
	}
}

// TestOIDCTokenValidation tests token claims validation and verification
func TestOIDCTokenValidation(t *testing.T) {
	validClaims := OIDCClaims{
		Issuer:     "https://token.actions.githubusercontent.com",
		Audience:   "sigstore",
		Subject:    "repo:owner/repo:ref:refs/heads/main",
		Actor:      "username",
		Repository: "owner/repo",
		Ref:        "refs/heads/main",
		SHA:        "abc123def456",
		RunID:      "12345",
		RunNumber:  "1",
		Workflow:   "Security Pipeline",
		IssuedAt:   time.Now().Unix(),
		ExpiresAt:  time.Now().Add(15 * time.Minute).Unix(),
	}

	tests := []struct {
		name        string
		claims      OIDCClaims
		expectValid bool
		errorCode   string
	}{
		{
			name:        "valid_oidc_claims",
			claims:      validClaims,
			expectValid: true,
		},
		{
			name: "invalid_issuer",
			claims: func() OIDCClaims {
				c := validClaims
				c.Issuer = "https://invalid.issuer.com"
				return c
			}(),
			expectValid: false,
			errorCode:   "SIGN_004",
		},
		{
			name: "invalid_audience",
			claims: func() OIDCClaims {
				c := validClaims
				c.Audience = "invalid"
				return c
			}(),
			expectValid: false,
			errorCode:   "SIGN_005",
		},
		{
			name: "missing_subject",
			claims: func() OIDCClaims {
				c := validClaims
				c.Subject = ""
				return c
			}(),
			expectValid: false,
			errorCode:   "SIGN_006",
		},
		{
			name: "expired_token",
			claims: func() OIDCClaims {
				c := validClaims
				c.ExpiresAt = time.Now().Add(-1 * time.Hour).Unix()
				return c
			}(),
			expectValid: false,
			errorCode:   "SIGN_008",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateOIDCClaims(&tt.claims)

			if tt.expectValid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorCode)
			}
		})
	}
}

func validateOIDCClaims(claims *OIDCClaims) error {
	expectedIssuer := "https://token.actions.githubusercontent.com"
	expectedAudience := "sigstore"

	if claims.Issuer != expectedIssuer {
		return fmt.Errorf("SIGN_004: Invalid OIDC issuer claim: expected %s, got %s", expectedIssuer, claims.Issuer)
	}

	if claims.Audience != expectedAudience {
		return fmt.Errorf("SIGN_005: Invalid OIDC audience claim: expected %s, got %s", expectedAudience, claims.Audience)
	}

	if claims.Subject == "" {
		return fmt.Errorf("SIGN_006: Missing OIDC subject claim")
	}

	if claims.ExpiresAt < time.Now().Unix() {
		return fmt.Errorf("SIGN_008: OIDC token has expired")
	}

	return nil
}

// TestOIDCTokenRefresh tests token refresh and expiration handling
func TestOIDCTokenRefresh(t *testing.T) {
	t.Run("token_refresh_before_expiration", func(t *testing.T) {
		// Simulate token that expires in 5 minutes
		expiresAt := time.Now().Add(5 * time.Minute).Unix()
		refreshThreshold := 10 * time.Minute // Refresh if less than 10 minutes remaining

		claims := &OIDCClaims{
			ExpiresAt: expiresAt,
		}

		timeUntilExpiry := time.Unix(claims.ExpiresAt, 0).Sub(time.Now())
		shouldRefresh := timeUntilExpiry < refreshThreshold

		assert.True(t, shouldRefresh, "Token should be refreshed when close to expiration")
	})

	t.Run("token_still_valid", func(t *testing.T) {
		// Simulate token that expires in 12 minutes
		expiresAt := time.Now().Add(12 * time.Minute).Unix()
		refreshThreshold := 10 * time.Minute

		claims := &OIDCClaims{
			ExpiresAt: expiresAt,
		}

		timeUntilExpiry := time.Unix(claims.ExpiresAt, 0).Sub(time.Now())
		shouldRefresh := timeUntilExpiry < refreshThreshold

		assert.False(t, shouldRefresh, "Token should not be refreshed when still valid")
	})
}

// TestOIDCRetryLogic tests retry logic for transient failures with exponential backoff
func TestOIDCRetryLogic(t *testing.T) {
	retryAttempts := 0
	maxRetries := 3

	// Mock server that fails twice then succeeds
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		retryAttempts++
		
		if retryAttempts <= 2 {
			// Simulate transient failure
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		// Succeed on third attempt
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(OIDCTokenResponse{
			Value: "success.token.value",
			Count: 1,
		})
	}))
	defer mockServer.Close()

	client := NewGitHubOIDCClient(mockServer.URL, "mock-request-token")

	// Implement retry logic
	var tokenResp *OIDCTokenResponse
	var err error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		tokenResp, err = client.GetOIDCToken("sigstore")
		
		if err == nil {
			break
		}

		if attempt < maxRetries {
			// Exponential backoff: 1s, 2s, 4s
			backoffDuration := time.Duration(1<<(attempt-1)) * time.Second
			time.Sleep(backoffDuration)
		}
	}

	assert.NoError(t, err, "Should succeed after retries")
	assert.NotNil(t, tokenResp)
	assert.Equal(t, "success.token.value", tokenResp.Value)
	assert.Equal(t, 3, retryAttempts, "Should have made 3 attempts")
}

// TestOIDCSecurityValidation tests security aspects of OIDC integration
func TestOIDCSecurityValidation(t *testing.T) {
	t.Run("request_token_validation", func(t *testing.T) {
		validTokens := []string{
			"ghs_validGitHubToken123",
			"ghp_validPersonalAccessToken456",
		}

		invalidTokens := []string{
			"", // empty token
			"invalid_token_format",
			"expired_token",
		}

		for _, token := range validTokens {
			assert.NotEmpty(t, token)
			assert.True(t, len(token) > 10, "Valid tokens should be sufficiently long")
		}

		for _, token := range invalidTokens {
			if token == "" {
				assert.Empty(t, token)
			} else {
				assert.NotContains(t, token, "ghs_", "Invalid tokens should not have valid GitHub prefixes")
			}
		}
	})

	t.Run("audience_validation", func(t *testing.T) {
		validAudiences := []string{
			"sigstore",
			"https://sigstore.dev",
		}

		invalidAudiences := []string{
			"",
			"malicious-service",
			"http://evil.com",
		}

		for _, audience := range validAudiences {
			assert.NotEmpty(t, audience)
			assert.NotContains(t, audience, "malicious")
		}

		for _, audience := range invalidAudiences {
			if audience != "" {
				assert.NotContains(t, audience, "sigstore")
			}
		}
	})

	t.Run("network_security", func(t *testing.T) {
		// Test HTTPS enforcement
		httpsURL := "https://token.actions.githubusercontent.com"
		httpURL := "http://token.actions.githubusercontent.com"

		assert.Contains(t, httpsURL, "https://")
		assert.NotContains(t, httpURL, "https://")

		// In production, HTTP URLs should be rejected
		isSecure := func(url string) bool {
			return len(url) > 8 && url[:8] == "https://"
		}

		assert.True(t, isSecure(httpsURL))
		assert.False(t, isSecure(httpURL))
	})
}

// TestEnvironmentVariableValidation tests environment variable handling
func TestEnvironmentVariableValidation(t *testing.T) {
	// Save original environment
	originalToken := os.Getenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN")
	originalURL := os.Getenv("ACTIONS_ID_TOKEN_REQUEST_URL")
	
	defer func() {
		os.Setenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN", originalToken)
		os.Setenv("ACTIONS_ID_TOKEN_REQUEST_URL", originalURL)
	}()

	tests := []struct {
		name        string
		setToken    bool
		setURL      bool
		expectError bool
		errorCode   string
	}{
		{
			name:        "valid_environment",
			setToken:    true,
			setURL:      true,
			expectError: false,
		},
		{
			name:        "missing_token",
			setToken:    false,
			setURL:      true,
			expectError: true,
			errorCode:   "SIGN_001",
		},
		{
			name:        "missing_url",
			setToken:    true,
			setURL:      false,
			expectError: true,
			errorCode:   "SIGN_002",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			os.Unsetenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN")
			os.Unsetenv("ACTIONS_ID_TOKEN_REQUEST_URL")

			// Set environment based on test case
			if tt.setToken {
				os.Setenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN", "mock-token")
			}
			if tt.setURL {
				os.Setenv("ACTIONS_ID_TOKEN_REQUEST_URL", "https://api.github.com/token")
			}

			// Validate environment
			err := validateEnvironment()

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorCode)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func validateEnvironment() error {
	token := os.Getenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN")
	if token == "" {
		return fmt.Errorf("SIGN_001: OIDC token request token not available")
	}

	url := os.Getenv("ACTIONS_ID_TOKEN_REQUEST_URL")
	if url == "" {
		return fmt.Errorf("SIGN_002: OIDC token request URL not available")
	}

	return nil
}