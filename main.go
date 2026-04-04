package main

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

func status(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}

func main() {
	r := gin.Default()

	r.GET("/status", status)

	err := r.Run()

	if err != nil {
		os.Exit(1)
	}
}
