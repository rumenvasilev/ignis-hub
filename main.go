package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/rumenvasilev/ignis-hub/internal/config"
	"github.com/rumenvasilev/ignis-hub/internal/handlers"
	"github.com/rumenvasilev/ignis-hub/internal/middleware"
	"github.com/rumenvasilev/ignis-hub/internal/storage"
)

var Version = "dev"

func main() {
	// Parse command-line flags
	provider := flag.String("provider", "aws", "Cloud provider to use (aws or gcp)")
	flag.Parse()

	// Validate provider flag
	if *provider != "aws" && *provider != "gcp" {
		slog.Error("Invalid provider specified", "provider", *provider, "valid_options", "aws, gcp")
		os.Exit(1)
	}

	// Load configuration
	cfg, err := config.Load(*provider)
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Setup logger
	logger := setupLogger(cfg)
	logger.Info("Starting Terraform Registry Server", "provider", *provider)

	// Initialize storage based on provider
	storageProvider, err := storage.NewStorage(cfg, logger)
	if err != nil {
		logger.Error("Failed to initialize storage", "error", err)
		os.Exit(1)
	}
	logger.Info("Initialized storage", "provider", *provider)

	// Test storage connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := storageProvider.HealthCheck(ctx); err != nil {
		logger.Error("Storage health check failed", "error", err, "provider", *provider)
		os.Exit(1)
	}
	logger.Info("Storage connection verified", "provider", *provider)

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
	setupRoutes(router, storageProvider, cfg, logger)

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
		logger.Info("Starting HTTP server",
			"host", cfg.Server.Host,
			"port", cfg.Server.Port)

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Failed to start server", "error", err)
			os.Exit(1)
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
		logger.Error("Server forced to shutdown", "error", err)
	} else {
		logger.Info("Server shutdown completed")
	}
}

func setupLogger(cfg *config.Config) *slog.Logger {
	// Parse log level
	var level slog.Level
	switch cfg.Log.Level {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	// Create handler based on format
	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: level,
	}

	if cfg.Log.Format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}

func setupMiddleware(router *gin.Engine, cfg *config.Config, logger *slog.Logger) {
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

func setupRoutes(router *gin.Engine, storage storage.Storage, cfg *config.Config, logger *slog.Logger) {
	// Initialize handlers
	registryHandlers := handlers.NewRegistryHandlers(storage, cfg, logger)

	// Register routes
	// Service discovery (Terraform registry protocol)
	router.GET("/.well-known/terraform.json", registryHandlers.ServiceDiscovery)

	// Root endpoint
	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"name":    "Terraform Registry API",
			"version": Version,
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
