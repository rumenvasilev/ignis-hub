package middleware

import (
	"crypto/subtle"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/rumenvasilev/ignis-hub/internal/config"
)

type AuthMiddleware struct {
	config *config.Config
	logger *slog.Logger
}

func NewAuthMiddleware(cfg *config.Config, logger *slog.Logger) *AuthMiddleware {
	return &AuthMiddleware{
		config: cfg,
		logger: logger,
	}
}

// Auth middleware factory
func (a *AuthMiddleware) Auth() gin.HandlerFunc {
	if !a.config.Auth.Enabled {
		return gin.HandlerFunc(func(c *gin.Context) {
			c.Next()
		})
	}

	return gin.HandlerFunc(func(c *gin.Context) {
		// Check IP whitelist first
		if len(a.config.Auth.AllowedIPs) > 0 {
			clientIP := c.ClientIP()
			if !a.isIPAllowed(clientIP) {
				a.logger.Warn("IP not in whitelist", "client_ip", clientIP)
				c.JSON(http.StatusForbidden, gin.H{
					"error": "Access denied",
				})
				c.Abort()
				return
			}
		}

		// Perform authentication based on method
		switch a.config.Auth.Method {
		case "basic":
			if !a.basicAuth(c) {
				c.Header("WWW-Authenticate", "Basic realm=\"Terraform Registry\"")
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "Authentication required",
				})
				c.Abort()
				return
			}
		case "token":
			if !a.tokenAuth(c) {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "Invalid token",
				})
				c.Abort()
				return
			}
		case "iam":
			// For IAM authentication, you would typically validate AWS IAM credentials
			// This is a placeholder for AWS IAM integration
			if !a.iamAuth(c) {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "IAM authentication failed",
				})
				c.Abort()
				return
			}
		default:
			a.logger.Error("Unknown authentication method configured")
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Authentication method misconfigured",
			})
			c.Abort()
			return
		}

		c.Next()
	})
}

// Basic authentication
func (a *AuthMiddleware) basicAuth(c *gin.Context) bool {
	username, password, hasAuth := c.Request.BasicAuth()
	if !hasAuth {
		return false
	}

	// Use constant-time comparison to prevent timing attacks
	usernameMatch := subtle.ConstantTimeCompare([]byte(username), []byte(a.config.Auth.Username)) == 1
	passwordMatch := subtle.ConstantTimeCompare([]byte(password), []byte(a.config.Auth.Password)) == 1

	return usernameMatch && passwordMatch
}

// Token authentication
func (a *AuthMiddleware) tokenAuth(c *gin.Context) bool {
	token := a.extractToken(c)
	if token == "" {
		return false
	}

	// Use constant-time comparison to prevent timing attacks
	return subtle.ConstantTimeCompare([]byte(token), []byte(a.config.Auth.Token)) == 1
}

// IAM authentication (placeholder)
func (a *AuthMiddleware) iamAuth(c *gin.Context) bool {
	// This is a placeholder for AWS IAM authentication
	// You would typically:
	// 1. Extract AWS signature headers
	// 2. Validate the signature against AWS IAM
	// 3. Check if the IAM principal has required permissions

	authHeader := c.GetHeader("Authorization")
	if !strings.HasPrefix(authHeader, "AWS4-HMAC-SHA256") {
		return false
	}

	// For now, just return true if the header looks like AWS signature
	// In a real implementation, you'd validate the signature
	a.logger.Warn("IAM authentication is not fully implemented - allowing all AWS-signed requests")
	return true
}

// Extract token from various headers
func (a *AuthMiddleware) extractToken(c *gin.Context) string {
	// Try Authorization header with Bearer token
	authHeader := c.GetHeader("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}

	// Try X-API-Token header
	if token := c.GetHeader("X-API-Token"); token != "" {
		return token
	}

	// Try query parameter (not recommended for production)
	if token := c.Query("token"); token != "" {
		return token
	}

	return ""
}

// Check if IP is in the allowed list
func (a *AuthMiddleware) isIPAllowed(clientIP string) bool {
	for _, allowedIP := range a.config.Auth.AllowedIPs {
		// Support both single IPs and CIDR ranges
		if strings.Contains(allowedIP, "/") {
			// CIDR range
			_, cidr, err := net.ParseCIDR(allowedIP)
			if err != nil {
				a.logger.Error("Invalid CIDR in allowed IPs", "error", err, "cidr", allowedIP)
				continue
			}
			if cidr.Contains(net.ParseIP(clientIP)) {
				return true
			}
		} else {
			// Single IP
			if clientIP == allowedIP {
				return true
			}
		}
	}
	return false
}
