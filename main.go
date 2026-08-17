package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

func main() {
	ginEngine := gin.Default()

	ginEngine.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})

	err := ginEngine.Run()
	if err != nil {
		log.Fatalf("failed to run server: %v", err)
	}
}
