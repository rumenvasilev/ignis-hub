package middleware

import (
	"encoding/base64"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/matryer/is"
	"github.com/rumenvasilev/ignis-hub/internal/config"
)

func init() {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)
}

func TestNewAuthMiddleware(t *testing.T) {
	is := is.New(t)
	cfg := &config.Config{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	authMW := NewAuthMiddleware(cfg, logger)

	is.True(authMW != nil)           // authMW should not be nil
	is.True(authMW.config == cfg)    // config should be set correctly
	is.True(authMW.logger == logger) // logger should be set correctly
}

func TestAuth_Disabled(t *testing.T) {
	is := is.New(t)
	cfg := &config.Config{
		Auth: config.AuthConfig{
			Enabled: false,
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	authMW := NewAuthMiddleware(cfg, logger)

	// Create test router
	router := gin.New()
	router.Use(authMW.Auth())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "success")
	})

	// Test request
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	is.Equal(w.Code, http.StatusOK)      // status should be 200
	is.Equal(w.Body.String(), "success") // response body should be success
}

func TestAuth_BasicAuth_Success(t *testing.T) {
	is := is.New(t)
	cfg := &config.Config{
		Auth: config.AuthConfig{
			Enabled:  true,
			Method:   "basic",
			Username: "admin",
			Password: "secret",
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	authMW := NewAuthMiddleware(cfg, logger)

	router := gin.New()
	router.Use(authMW.Auth())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "success")
	})

	// Create request with valid basic auth
	req := httptest.NewRequest("GET", "/test", nil)
	auth := base64.StdEncoding.EncodeToString([]byte("admin:secret"))
	req.Header.Set("Authorization", "Basic "+auth)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	is.Equal(w.Code, http.StatusOK) // status should be 200
}

func TestAuth_BasicAuth_InvalidCredentials(t *testing.T) {
	cfg := &config.Config{
		Auth: config.AuthConfig{
			Enabled:  true,
			Method:   "basic",
			Username: "admin",
			Password: "secret",
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	authMW := NewAuthMiddleware(cfg, logger)

	router := gin.New()
	router.Use(authMW.Auth())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "success")
	})

	tests := []struct {
		name     string
		username string
		password string
	}{
		{"wrong username", "wrong", "secret"},
		{"wrong password", "admin", "wrong"},
		{"both wrong", "wrong", "wrong"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			is := is.New(t)
			req := httptest.NewRequest("GET", "/test", nil)
			auth := base64.StdEncoding.EncodeToString([]byte(tt.username + ":" + tt.password))
			req.Header.Set("Authorization", "Basic "+auth)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			is.Equal(w.Code, http.StatusUnauthorized) // status should be 401
		})
	}
}

func TestAuth_BasicAuth_MissingCredentials(t *testing.T) {
	is := is.New(t)
	cfg := &config.Config{
		Auth: config.AuthConfig{
			Enabled:  true,
			Method:   "basic",
			Username: "admin",
			Password: "secret",
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	authMW := NewAuthMiddleware(cfg, logger)

	router := gin.New()
	router.Use(authMW.Auth())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "success")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	is.Equal(w.Code, http.StatusUnauthorized)                                          // status should be 401
	is.Equal(w.Header().Get("WWW-Authenticate"), "Basic realm=\"Terraform Registry\"") // WWW-Authenticate header should be set
}

func TestAuth_TokenAuth_Success(t *testing.T) {
	cfg := &config.Config{
		Auth: config.AuthConfig{
			Enabled: true,
			Method:  "token",
			Token:   "my-secret-token",
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	authMW := NewAuthMiddleware(cfg, logger)

	router := gin.New()
	router.Use(authMW.Auth())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "success")
	})

	tests := []struct {
		name   string
		header string
		value  string
	}{
		{"bearer token", "Authorization", "Bearer my-secret-token"},
		{"x-api-token", "X-API-Token", "my-secret-token"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			is := is.New(t)
			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set(tt.header, tt.value)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			is.Equal(w.Code, http.StatusOK) // status should be 200
		})
	}
}

func TestAuth_TokenAuth_QueryParameter(t *testing.T) {
	is := is.New(t)
	cfg := &config.Config{
		Auth: config.AuthConfig{
			Enabled: true,
			Method:  "token",
			Token:   "my-secret-token",
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	authMW := NewAuthMiddleware(cfg, logger)

	router := gin.New()
	router.Use(authMW.Auth())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "success")
	})

	req := httptest.NewRequest("GET", "/test?token=my-secret-token", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	is.Equal(w.Code, http.StatusOK) // status should be 200
}

func TestAuth_TokenAuth_InvalidToken(t *testing.T) {
	cfg := &config.Config{
		Auth: config.AuthConfig{
			Enabled: true,
			Method:  "token",
			Token:   "my-secret-token",
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	authMW := NewAuthMiddleware(cfg, logger)

	router := gin.New()
	router.Use(authMW.Auth())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "success")
	})

	tests := []struct {
		name   string
		header string
		value  string
	}{
		{"wrong bearer token", "Authorization", "Bearer wrong-token"},
		{"wrong x-api-token", "X-API-Token", "wrong-token"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			is := is.New(t)
			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set(tt.header, tt.value)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			is.Equal(w.Code, http.StatusUnauthorized) // status should be 401
		})
	}
}

func TestAuth_TokenAuth_MissingToken(t *testing.T) {
	is := is.New(t)
	cfg := &config.Config{
		Auth: config.AuthConfig{
			Enabled: true,
			Method:  "token",
			Token:   "my-secret-token",
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	authMW := NewAuthMiddleware(cfg, logger)

	router := gin.New()
	router.Use(authMW.Auth())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "success")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	is.Equal(w.Code, http.StatusUnauthorized) // status should be 401
}

func TestAuth_IAMAuth_Success(t *testing.T) {
	is := is.New(t)
	cfg := &config.Config{
		Auth: config.AuthConfig{
			Enabled: true,
			Method:  "iam",
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	authMW := NewAuthMiddleware(cfg, logger)

	router := gin.New()
	router.Use(authMW.Auth())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "success")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=...")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	is.Equal(w.Code, http.StatusOK) // status should be 200
}

func TestAuth_IAMAuth_MissingHeader(t *testing.T) {
	is := is.New(t)
	cfg := &config.Config{
		Auth: config.AuthConfig{
			Enabled: true,
			Method:  "iam",
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	authMW := NewAuthMiddleware(cfg, logger)

	router := gin.New()
	router.Use(authMW.Auth())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "success")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	is.Equal(w.Code, http.StatusUnauthorized) // status should be 401
}

func TestAuth_UnknownMethod(t *testing.T) {
	is := is.New(t)
	cfg := &config.Config{
		Auth: config.AuthConfig{
			Enabled: true,
			Method:  "unknown",
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	authMW := NewAuthMiddleware(cfg, logger)

	router := gin.New()
	router.Use(authMW.Auth())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "success")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	is.Equal(w.Code, http.StatusInternalServerError) // status should be 500
}

func TestAuth_IPWhitelist_SingleIP_Allowed(t *testing.T) {
	is := is.New(t)
	cfg := &config.Config{
		Auth: config.AuthConfig{
			Enabled:    true,
			Method:     "token",
			Token:      "test-token",
			AllowedIPs: []string{"127.0.0.1"},
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	authMW := NewAuthMiddleware(cfg, logger)

	router := gin.New()
	router.Use(authMW.Auth())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "success")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Token", "test-token")
	req.RemoteAddr = "127.0.0.1:12345"

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	is.Equal(w.Code, http.StatusOK) // status should be 200
}

func TestAuth_IPWhitelist_SingleIP_Denied(t *testing.T) {
	is := is.New(t)
	cfg := &config.Config{
		Auth: config.AuthConfig{
			Enabled:    true,
			Method:     "token",
			Token:      "test-token",
			AllowedIPs: []string{"127.0.0.1"},
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	authMW := NewAuthMiddleware(cfg, logger)

	router := gin.New()
	router.Use(authMW.Auth())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "success")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Token", "test-token")
	req.RemoteAddr = "192.168.1.1:12345"

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	is.Equal(w.Code, http.StatusForbidden) // status should be 403
}

func TestAuth_IPWhitelist_CIDR_Allowed(t *testing.T) {
	is := is.New(t)
	cfg := &config.Config{
		Auth: config.AuthConfig{
			Enabled:    true,
			Method:     "token",
			Token:      "test-token",
			AllowedIPs: []string{"192.168.1.0/24"},
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	authMW := NewAuthMiddleware(cfg, logger)

	router := gin.New()
	router.Use(authMW.Auth())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "success")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Token", "test-token")
	req.RemoteAddr = "192.168.1.50:12345"

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	is.Equal(w.Code, http.StatusOK) // status should be 200
}

func TestAuth_IPWhitelist_CIDR_Denied(t *testing.T) {
	is := is.New(t)
	cfg := &config.Config{
		Auth: config.AuthConfig{
			Enabled:    true,
			Method:     "token",
			Token:      "test-token",
			AllowedIPs: []string{"192.168.1.0/24"},
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	authMW := NewAuthMiddleware(cfg, logger)

	router := gin.New()
	router.Use(authMW.Auth())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "success")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Token", "test-token")
	req.RemoteAddr = "192.168.2.50:12345"

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	is.Equal(w.Code, http.StatusForbidden) // status should be 403
}

func TestIsIPAllowed_InvalidCIDR(t *testing.T) {
	is := is.New(t)
	cfg := &config.Config{
		Auth: config.AuthConfig{
			AllowedIPs: []string{"invalid-cidr/24", "127.0.0.1"},
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	authMW := NewAuthMiddleware(cfg, logger)

	// Should fall through to next IP and allow 127.0.0.1
	is.True(authMW.isIPAllowed("127.0.0.1")) // 127.0.0.1 should be allowed

	// Should not be allowed
	is.True(!authMW.isIPAllowed("192.168.1.1")) // 192.168.1.1 should be denied
}

func TestExtractToken(t *testing.T) {
	cfg := &config.Config{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	authMW := NewAuthMiddleware(cfg, logger)

	tests := []struct {
		name          string
		setupRequest  func(*http.Request)
		expectedToken string
	}{
		{
			name: "bearer token",
			setupRequest: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer test-token-123")
			},
			expectedToken: "test-token-123",
		},
		{
			name: "x-api-token header",
			setupRequest: func(req *http.Request) {
				req.Header.Set("X-API-Token", "test-token-456")
			},
			expectedToken: "test-token-456",
		},
		{
			name: "query parameter",
			setupRequest: func(req *http.Request) {
				// Query is already in the URL
			},
			expectedToken: "test-token-789",
		},
		{
			name: "no token",
			setupRequest: func(req *http.Request) {
				// Don't set anything
			},
			expectedToken: "",
		},
		{
			name: "bearer priority over x-api-token",
			setupRequest: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer priority-token")
				req.Header.Set("X-API-Token", "secondary-token")
			},
			expectedToken: "priority-token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			is := is.New(t)
			url := "/test"
			if tt.name == "query parameter" {
				url = "/test?token=test-token-789"
			}

			req := httptest.NewRequest("GET", url, nil)
			tt.setupRequest(req)

			// Create a gin context
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = req

			token := authMW.extractToken(c)
			is.Equal(token, tt.expectedToken) // token should match expected value
		})
	}
}
