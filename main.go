package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

func newRouter() *gin.Engine {
	ginEngine := gin.Default()

	ginEngine.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})

	return ginEngine
}

func main() {
	err := newRouter().Run()
	if err != nil {
		log.Fatalf("failed to run server: %v", err)
	}
}
