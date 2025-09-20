package main

import (
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	_ "github.com/lib/pq"
	"gopkg.in/yaml.v2"
)

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Vulnerable: allows any origin
		},
	}
	appVersion = "1.0.0"
	startTime  = time.Now()
	logger     = slog.New(slog.NewJSONHandler(os.Stdout, nil))
)

type Config struct {
	Database struct {
		Password string `yaml:"password"`
		APIKey   string `yaml:"api_key"`
	} `yaml:"database"`
}

type HealthResponse struct {
	Status    string            `json:"status"`
	Version   string            `json:"version"`
	Uptime    string            `json:"uptime"`
	Timestamp time.Time         `json:"timestamp"`
	Metadata  map[string]string `json:"metadata"`
}

type VersionResponse struct {
	Application string            `json:"application"`
	Version     string            `json:"version"`
	GoVersion   string            `json:"go_version"`
	BuildTime   time.Time         `json:"build_time"`
	Dependencies map[string]string `json:"dependencies"`
}

func main() {
	logger.Info("Starting vulnerable demo application", "version", appVersion, "port", 8080)

	r := gin.Default()

	// Health check endpoint - AC requirement
	r.GET("/health", healthHandler)

	// Version endpoint showing dependencies
	r.GET("/version", versionHandler)

	// Legacy ping endpoint
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	// Vulnerable config endpoint - exposes sensitive data
	r.GET("/config", func(c *gin.Context) {
		config := Config{}
		data, err := os.ReadFile("config.yaml")
		if err != nil {
			logger.Error("Failed to read config", "error", err)
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		yaml.Unmarshal(data, &config) // Vulnerable: no error checking
		c.JSON(200, config)
	})

	// Vulnerable WebSocket endpoint
	r.GET("/ws", func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			logger.Error("WebSocket upgrade failed", "error", err)
			return
		}
		defer conn.Close()
		
		// Echo any message (vulnerable to various attacks)
		for {
			msgType, msg, err := conn.ReadMessage()
			if err != nil {
				break
			}
			conn.WriteMessage(msgType, msg)
		}
	})

	logger.Info("Server starting", "port", 8080)
	r.Run(":8080")
}

func healthHandler(c *gin.Context) {
	uptime := time.Since(startTime)
	
	response := HealthResponse{
		Status:    "healthy",
		Version:   appVersion,
		Uptime:    uptime.String(),
		Timestamp: time.Now(),
		Metadata: map[string]string{
			"go_version": runtime.Version(),
			"arch":       runtime.GOARCH,
			"os":         runtime.GOOS,
		},
	}
	
	c.Header("Content-Type", "application/json")
	c.JSON(http.StatusOK, response)
}

func versionHandler(c *gin.Context) {
	response := VersionResponse{
		Application: "vulnerable-demo-app",
		Version:     appVersion,
		GoVersion:   runtime.Version(),
		BuildTime:   startTime,
		Dependencies: map[string]string{
			"gin-gonic/gin":      "v1.7.4",
			"gorilla/websocket":  "v1.4.2",
			"lib/pq":             "v1.10.2",
			"gopkg.in/yaml.v2":   "v2.4.0",
		},
	}
	
	c.Header("Content-Type", "application/json")
	c.JSON(http.StatusOK, response)
}