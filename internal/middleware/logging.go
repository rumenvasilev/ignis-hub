package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// LoggingMiddleware creates a gin middleware for structured logging
func LoggingMiddleware(logger *slog.Logger) gin.HandlerFunc {
	return gin.LoggerWithConfig(gin.LoggerConfig{
		Formatter: func(param gin.LogFormatterParams) string {
			// Prepare log attributes
			attrs := []any{
				"timestamp", param.TimeStamp.Format(time.RFC3339),
				"status", param.StatusCode,
				"latency", param.Latency,
				"client_ip", param.ClientIP,
				"method", param.Method,
				"path", param.Path,
				"user_agent", param.Request.UserAgent(),
				"body_size", param.BodySize,
			}

			// Add error if present
			if param.ErrorMessage != "" {
				attrs = append(attrs, "error", param.ErrorMessage)
			}

			// Log based on status code
			if param.StatusCode >= 500 {
				logger.Error("HTTP request", attrs...)
			} else if param.StatusCode >= 400 {
				logger.Warn("HTTP request", attrs...)
			} else {
				logger.Info("HTTP request", attrs...)
			}

			// Return empty string as we've already logged
			return ""
		},
		SkipPaths: []string{"/health"}, // Skip health check logs to reduce noise
	})
}

// RequestIDMiddleware adds a request ID to each request
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			// Generate a simple request ID (in production, use UUID)
			requestID = generateRequestID()
		}

		c.Header("X-Request-ID", requestID)
		c.Set("request_id", requestID)
		c.Next()
	}
}

// Simple request ID generator (use proper UUID in production)
func generateRequestID() string {
	return time.Now().Format("20060102150405") + "-" + "req"
}

// CORSMiddleware handles CORS headers for browser clients
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization, X-Terraform-Version, X-API-Token")
		c.Header("Access-Control-Expose-Headers", "X-Terraform-Get, X-Request-ID")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
