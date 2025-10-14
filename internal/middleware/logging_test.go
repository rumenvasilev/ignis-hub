package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/matryer/is"
)

func TestLoggingMiddleware_SuccessRequest(t *testing.T) {
	is := is.New(t)
	// Capture log output
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	router := gin.New()
	router.Use(LoggingMiddleware(logger))
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "success")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("User-Agent", "TestAgent/1.0")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	is.Equal(w.Code, http.StatusOK) // status should be 200

	logOutput := buf.String()
	is.True(strings.Contains(logOutput, "HTTP request"))   // log should contain 'HTTP request'
	is.True(strings.Contains(logOutput, "GET"))            // log should contain request method
	is.True(strings.Contains(logOutput, "/test"))          // log should contain request path
	is.True(strings.Contains(logOutput, "TestAgent/1.0"))  // log should contain user agent
	is.True(strings.Contains(logOutput, `"level":"INFO"`)) // log level should be INFO for 2xx status
}

func TestLoggingMiddleware_ClientError(t *testing.T) {
	is := is.New(t)
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	router := gin.New()
	router.Use(LoggingMiddleware(logger))
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusBadRequest, "bad request")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	is.Equal(w.Code, http.StatusBadRequest) // status should be 400

	logOutput := buf.String()
	is.True(strings.Contains(logOutput, `"level":"WARN"`)) // log level should be WARN for 4xx status
	is.True(strings.Contains(logOutput, `"status":400`))   // log should contain status code 400
}

func TestLoggingMiddleware_ServerError(t *testing.T) {
	is := is.New(t)
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	router := gin.New()
	router.Use(LoggingMiddleware(logger))
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusInternalServerError, "internal error")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	is.Equal(w.Code, http.StatusInternalServerError) // status should be 500

	logOutput := buf.String()
	is.True(strings.Contains(logOutput, `"level":"ERROR"`)) // log level should be ERROR for 5xx status
	is.True(strings.Contains(logOutput, `"status":500`))    // log should contain status code 500
}

func TestLoggingMiddleware_SkipHealthCheck(t *testing.T) {
	is := is.New(t)
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	router := gin.New()
	router.Use(LoggingMiddleware(logger))
	router.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "healthy")
	})

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	is.Equal(w.Code, http.StatusOK) // status should be 200

	logOutput := buf.String()
	// Health endpoint should be skipped from logging
	is.True(!strings.Contains(logOutput, "/health")) // health check endpoint should not be logged
}

func TestLoggingMiddleware_WithError(t *testing.T) {
	is := is.New(t)
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	router := gin.New()
	router.Use(LoggingMiddleware(logger))
	router.GET("/test", func(c *gin.Context) {
		c.Error(http.ErrAbortHandler)
		c.String(http.StatusInternalServerError, "error")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	logOutput := buf.String()
	is.True(strings.Contains(logOutput, "error")) // log should contain error information
}

func TestLoggingMiddleware_DifferentMethods(t *testing.T) {
	tests := []struct {
		method string
	}{
		{"GET"},
		{"POST"},
		{"PUT"},
		{"DELETE"},
		{"PATCH"},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			is := is.New(t)
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
				Level: slog.LevelInfo,
			}))

			router := gin.New()
			router.Use(LoggingMiddleware(logger))
			router.Handle(tt.method, "/test", func(c *gin.Context) {
				c.String(http.StatusOK, "ok")
			})

			req := httptest.NewRequest(tt.method, "/test", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			logOutput := buf.String()
			is.True(strings.Contains(logOutput, tt.method)) // log should contain HTTP method
		})
	}
}

func TestRequestIDMiddleware_NewID(t *testing.T) {
	is := is.New(t)
	router := gin.New()
	router.Use(RequestIDMiddleware())
	router.GET("/test", func(c *gin.Context) {
		requestID, exists := c.Get("request_id")
		is.True(exists)          // request_id should be set in context
		is.True(requestID != "") // request_id should not be empty
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Check response header
	requestID := w.Header().Get("X-Request-ID")
	is.True(requestID != "")                      // X-Request-ID header should be set
	is.True(strings.HasSuffix(requestID, "-req")) // generated request ID should have expected format
}

func TestRequestIDMiddleware_ExistingID(t *testing.T) {
	is := is.New(t)
	router := gin.New()
	router.Use(RequestIDMiddleware())
	router.GET("/test", func(c *gin.Context) {
		requestID, exists := c.Get("request_id")
		is.True(exists)                            // request_id should be set in context
		is.Equal(requestID, "existing-request-id") // request_id should match header value
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", "existing-request-id")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Check response header
	requestID := w.Header().Get("X-Request-ID")
	is.Equal(requestID, "existing-request-id") // X-Request-ID header should be preserved
}

func TestRequestIDMiddleware_PropagatesID(t *testing.T) {
	is := is.New(t)
	router := gin.New()
	router.Use(RequestIDMiddleware())

	var capturedID string
	router.GET("/test", func(c *gin.Context) {
		requestID, _ := c.Get("request_id")
		capturedID = requestID.(string)
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", "test-id-123")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	is.Equal(capturedID, "test-id-123") // request ID should be propagated to handler
}

func TestGenerateRequestID(t *testing.T) {
	is := is.New(t)
	id1 := generateRequestID()
	id2 := generateRequestID()

	is.True(id1 != "")                      // generated request ID should not be empty
	is.True(strings.HasSuffix(id1, "-req")) // generated request ID should end with '-req'
	// IDs generated at different times should be different
	// (though this could theoretically fail if called in same second)
	if id1 == id2 {
		t.Log("Warning: Generated IDs are the same (may be expected if called in same second)")
	}
}

func TestCORSMiddleware_Headers(t *testing.T) {
	is := is.New(t)
	router := gin.New()
	router.Use(CORSMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Check CORS headers
	headers := w.Header()

	is.Equal(headers.Get("Access-Control-Allow-Origin"), "*")                               // Access-Control-Allow-Origin should be '*'
	is.True(strings.Contains(headers.Get("Access-Control-Allow-Methods"), "GET"))           // Access-Control-Allow-Methods should contain GET
	is.True(strings.Contains(headers.Get("Access-Control-Allow-Headers"), "Authorization")) // Access-Control-Allow-Headers should contain Authorization
	is.True(strings.Contains(headers.Get("Access-Control-Expose-Headers"), "X-Request-ID")) // Access-Control-Expose-Headers should contain X-Request-ID
}

func TestCORSMiddleware_OPTIONSRequest(t *testing.T) {
	is := is.New(t)
	router := gin.New()
	router.Use(CORSMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// OPTIONS should return 204 No Content
	is.Equal(w.Code, http.StatusNoContent) // OPTIONS request should return 204

	// CORS headers should still be present
	is.Equal(w.Header().Get("Access-Control-Allow-Origin"), "*") // CORS headers should be set for OPTIONS request
}

func TestCORSMiddleware_PreflightRequest(t *testing.T) {
	is := is.New(t)
	router := gin.New()
	router.Use(CORSMiddleware())
	router.POST("/api/data", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("OPTIONS", "/api/data", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	is.Equal(w.Code, http.StatusNoContent) // preflight request should return 204

	// Verify all required CORS headers are present
	headers := w.Header()
	is.True(headers.Get("Access-Control-Allow-Origin") != "")  // Access-Control-Allow-Origin should be set
	is.True(headers.Get("Access-Control-Allow-Methods") != "") // Access-Control-Allow-Methods should be set
	is.True(headers.Get("Access-Control-Allow-Headers") != "") // Access-Control-Allow-Headers should be set
}

func TestCORSMiddleware_AllowedHeaders(t *testing.T) {
	is := is.New(t)
	router := gin.New()
	router.Use(CORSMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	allowHeaders := w.Header().Get("Access-Control-Allow-Headers")

	requiredHeaders := []string{
		"Origin",
		"Content-Type",
		"Authorization",
		"X-Terraform-Version",
		"X-API-Token",
	}

	for _, header := range requiredHeaders {
		is.True(strings.Contains(allowHeaders, header)) // Access-Control-Allow-Headers should contain required header
	}
}

func TestCORSMiddleware_ExposedHeaders(t *testing.T) {
	is := is.New(t)
	router := gin.New()
	router.Use(CORSMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	exposeHeaders := w.Header().Get("Access-Control-Expose-Headers")

	requiredHeaders := []string{
		"X-Terraform-Get",
		"X-Request-ID",
	}

	for _, header := range requiredHeaders {
		is.True(strings.Contains(exposeHeaders, header)) // Access-Control-Expose-Headers should contain required header
	}
}

func TestCORSMiddleware_NonOPTIONSPasses(t *testing.T) {
	is := is.New(t)
	router := gin.New()
	router.Use(CORSMiddleware())

	handlerCalled := false
	router.POST("/test", func(c *gin.Context) {
		handlerCalled = true
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("POST", "/test", strings.NewReader("data"))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	is.True(handlerCalled)          // handler should be called for non-OPTIONS requests
	is.Equal(w.Code, http.StatusOK) // non-OPTIONS request should return handler status
}

func TestMiddlewareChaining(t *testing.T) {
	is := is.New(t)
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	router := gin.New()
	router.Use(RequestIDMiddleware())
	router.Use(CORSMiddleware())
	router.Use(LoggingMiddleware(logger))

	router.GET("/test", func(c *gin.Context) {
		requestID, exists := c.Get("request_id")
		is.True(exists)          // request_id should be available from previous middleware
		is.True(requestID != "") // request_id should not be empty
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify all middleware executed
	is.True(w.Header().Get("X-Request-ID") != "")                // RequestIDMiddleware should have set X-Request-ID
	is.True(w.Header().Get("Access-Control-Allow-Origin") != "") // CORSMiddleware should have set CORS headers

	logOutput := buf.String()
	is.True(strings.Contains(logOutput, "HTTP request")) // LoggingMiddleware should have logged the request
}
