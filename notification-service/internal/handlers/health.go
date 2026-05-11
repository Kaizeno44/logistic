package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"service": "Notification Service",
		"status":  "Running smooth on Docker! RabbitMQ Consumer is active.",
	})
}
