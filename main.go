package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"github.com/rumenvasilev/ignis-hub/internal/config"
	"github.com/rumenvasilev/ignis-hub/internal/handlers"
	"github.com/rumenvasilev/ignis-hub/internal/middleware"
	"github.com/rumenvasilev/ignis-hub/internal/storage"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logrus.WithError(err).Fatal("Failed to load configuration")
	}

	// Setup logger
	logger := setupLogger(cfg)
	logger.Info("Starting Terraform Registry Server")

	// Initialize S3 storage
	s3Storage, err := storage.NewS3Storage(cfg, logger)
	if err != nil {
		logger.WithError(err).Fatal("Failed to initialize S3 storage")
	}

	// Test storage connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s3Storage.HealthCheck(ctx); err != nil {
		logger.WithError(err).Fatal("S3 storage health check failed")
	}
	logger.Info("S3 storage connection verified")

	// Setup Gin
	if cfg.Log.Level == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create router
	router := gin.New()

	// Setup middleware
	setupMiddleware(router, cfg, logger)

	// Setup routes
	setupRoutes(router, s3Storage, cfg, logger)

	// Create HTTP server
	srv := &http.Server{
		Addr:           fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:        router,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1 MB
	}

	// Start server in a goroutine
	go func() {
		logger.WithFields(logrus.Fields{
			"host": cfg.Server.Host,
			"port": cfg.Server.Port,
		}).Info("Starting HTTP server")

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.WithError(err).Fatal("Failed to start server")
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.WithError(err).Error("Server forced to shutdown")
	} else {
		logger.Info("Server shutdown completed")
	}
}

func setupLogger(cfg *config.Config) *logrus.Logger {
	logger := logrus.New()

	// Set log level
	level, err := logrus.ParseLevel(cfg.Log.Level)
	if err != nil {
		level = logrus.InfoLevel
	}
	logger.SetLevel(level)

	// Set log format
	if cfg.Log.Format == "json" {
		logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: time.RFC3339,
		})
	} else {
		logger.SetFormatter(&logrus.TextFormatter{
			TimestampFormat: time.RFC3339,
			FullTimestamp:   true,
		})
	}

	return logger
}

func setupMiddleware(router *gin.Engine, cfg *config.Config, logger *logrus.Logger) {
	// Recovery middleware
	router.Use(gin.Recovery())

	// Request ID middleware
	router.Use(middleware.RequestIDMiddleware())

	// CORS middleware
	router.Use(middleware.CORSMiddleware())

	// Logging middleware
	router.Use(middleware.LoggingMiddleware(logger))

	// Authentication middleware
	authMiddleware := middleware.NewAuthMiddleware(cfg, logger)
	router.Use(authMiddleware.Auth())
}

func setupRoutes(router *gin.Engine, storage *storage.S3Storage, cfg *config.Config, logger *logrus.Logger) {
	// Initialize handlers
	registryHandlers := handlers.NewRegistryHandlers(storage, cfg, logger)

	// Register routes
	// Service discovery (Terraform registry protocol)
	router.GET("/.well-known/terraform.json", registryHandlers.ServiceDiscovery)

	// Root endpoint
	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"name":    "Terraform Registry API",
			"version": "1.0.0",
			"status":  "running",
			"endpoints": gin.H{
				"well_known": "/.well-known/terraform.json",
				"modules":    "/v1/modules/",
				"providers":  "/v1/providers/",
				"health":     "/health",
			},
		})
	})

	// Health endpoint
	router.GET("/health", registryHandlers.HealthCheck)

	// Module endpoints
	moduleGroup := router.Group("/v1/modules")
	{
		moduleGroup.GET("/:namespace/:name/:system/versions", registryHandlers.ListModuleVersions)
		moduleGroup.GET("/:namespace/:name/:system/:version/download", registryHandlers.GetModuleVersion)
	}

	// Provider endpoints
	providerGroup := router.Group("/v1/providers")
	{
		providerGroup.GET("/:namespace/:type/versions", registryHandlers.ListProviderVersions)
		providerGroup.GET("/:namespace/:type/:version/download/:os/:arch", registryHandlers.GetProviderVersion)
	}
}
