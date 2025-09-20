package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	_ "github.com/lib/pq"
	"gopkg.in/yaml.v2"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Config struct {
	Database struct {
		Password string `yaml:"password"`
		APIKey   string `yaml:"api_key"`
	} `yaml:"database"`
}

func main() {
	r := gin.Default()

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	r.GET("/config", func(c *gin.Context) {
		config := Config{}
		data, err := os.ReadFile("config.yaml")
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		yaml.Unmarshal(data, &config)
		c.JSON(200, config)
	})

	r.GET("/ws", func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Print("upgrade failed: ", err)
			return
		}
		defer conn.Close()
	})

	log.Println("Server starting on :8080")
	r.Run(":8080")
}