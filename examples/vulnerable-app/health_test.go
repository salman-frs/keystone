package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestHealthEndpoint(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)
	
	// Create a new router instance
	router := gin.New()
	router.GET("/health", healthHandler)

	// Create a test request
	req, err := http.NewRequest("GET", "/health", nil)
	if err != nil {
		t.Fatalf("Could not create request: %v", err)
	}

	// Create a response recorder
	rr := httptest.NewRecorder()

	// Measure response time
	start := time.Now()
	router.ServeHTTP(rr, req)
	duration := time.Since(start)

	// Check status code
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, rr.Code)
	}

	// Check response time (should be under 200ms)
	if duration > 200*time.Millisecond {
		t.Errorf("Health check took too long: %v (expected < 200ms)", duration)
	}

	// Check content type
	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json" && contentType != "application/json; charset=utf-8" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	// Parse JSON response
	var response HealthResponse
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Could not parse JSON response: %v", err)
	}

	// Validate response structure
	if response.Status != "healthy" {
		t.Errorf("Expected status 'healthy', got '%s'", response.Status)
	}

	if response.Version == "" {
		t.Error("Expected version to be set")
	}

	if response.Uptime == "" {
		t.Error("Expected uptime to be set")
	}

	if response.Timestamp.IsZero() {
		t.Error("Expected timestamp to be set")
	}

	// Validate metadata
	if response.Metadata["go_version"] == "" {
		t.Error("Expected go_version in metadata")
	}

	if response.Metadata["arch"] == "" {
		t.Error("Expected arch in metadata")
	}

	if response.Metadata["os"] == "" {
		t.Error("Expected os in metadata")
	}
}

func TestVersionEndpoint(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)
	
	// Create a new router instance
	router := gin.New()
	router.GET("/version", versionHandler)

	// Create a test request
	req, err := http.NewRequest("GET", "/version", nil)
	if err != nil {
		t.Fatalf("Could not create request: %v", err)
	}

	// Create a response recorder
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	// Check status code
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, rr.Code)
	}

	// Parse JSON response
	var response VersionResponse
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Could not parse JSON response: %v", err)
	}

	// Validate response structure
	if response.Application != "vulnerable-demo-app" {
		t.Errorf("Expected application 'vulnerable-demo-app', got '%s'", response.Application)
	}

	if response.Version == "" {
		t.Error("Expected version to be set")
	}

	if response.GoVersion == "" {
		t.Error("Expected go_version to be set")
	}

	// Validate dependencies are listed
	expectedDeps := []string{"gin-gonic/gin", "gorilla/websocket", "lib/pq", "gopkg.in/yaml.v2"}
	for _, dep := range expectedDeps {
		if _, exists := response.Dependencies[dep]; !exists {
			t.Errorf("Expected dependency '%s' to be listed", dep)
		}
	}
}

func TestPingEndpoint(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)
	
	// Create a new router instance
	router := gin.New()
	router.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	// Create a test request
	req, err := http.NewRequest("GET", "/ping", nil)
	if err != nil {
		t.Fatalf("Could not create request: %v", err)
	}

	// Create a response recorder
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	// Check status code
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, rr.Code)
	}

	// Parse JSON response
	var response map[string]string
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Could not parse JSON response: %v", err)
	}

	// Validate response
	if response["message"] != "pong" {
		t.Errorf("Expected message 'pong', got '%s'", response["message"])
	}
}

func BenchmarkHealthEndpoint(b *testing.B) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)
	
	// Create a new router instance
	router := gin.New()
	router.GET("/health", healthHandler)

	// Create a test request
	req, err := http.NewRequest("GET", "/health", nil)
	if err != nil {
		b.Fatalf("Could not create request: %v", err)
	}

	// Run benchmark
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			b.Errorf("Expected status code %d, got %d", http.StatusOK, rr.Code)
		}
	}
}