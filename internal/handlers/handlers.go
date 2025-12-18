package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/rumenvasilev/ignis-hub/internal/config"
	"github.com/rumenvasilev/ignis-hub/internal/models"
	"github.com/rumenvasilev/ignis-hub/internal/storage/api"
)

type RegistryHandlers struct {
	storage api.Storage
	config  *config.Config
	logger  *slog.Logger
}

func NewRegistryHandlers(storage api.Storage, cfg *config.Config, logger *slog.Logger) *RegistryHandlers {
	return &RegistryHandlers{
		storage: storage,
		config:  cfg,
		logger:  logger,
	}
}

// GetWellKnown handles /.well-known/terraform.json
func (h *RegistryHandlers) GetWellKnown(c *gin.Context) {
	baseURL := h.config.Server.BaseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}

	response := models.WellKnownResponse{
		ModulesV1:   baseURL + "v1/modules/",
		ProvidersV1: baseURL + "v1/providers/",
	}

	h.logger.Info("Served well-known response", "endpoint", "well-known")
	c.JSON(http.StatusOK, response)
}

// ListModuleVersions handles GET /v1/modules/{namespace}/{name}/{system}/versions
func (h *RegistryHandlers) ListModuleVersions(c *gin.Context) {
	var params models.ModuleParams
	if err := c.ShouldBindUri(&params); err != nil {
		h.logger.Error("Failed to bind module parameters", "error", err)
		c.JSON(http.StatusBadRequest, models.NotFoundResponse{
			Errors: []string{"Invalid module parameters"},
		})
		return
	}

	h.logger.Info("Listing module versions",
		"namespace", params.Namespace,
		"name", params.Name,
		"system", params.System)

	metadata, err := h.storage.GetVersions(c.Request.Context(), api.ResourceIdentifier{
		Type:      api.ResourceTypeModule,
		Namespace: params.Namespace,
		Name:      params.Name,
		System:    params.System,
	})
	if err != nil {
		h.logger.Error("Failed to get module versions", "error", err)
		c.JSON(http.StatusNotFound, models.NotFoundResponse{
			Errors: []string{api.ErrModuleNotFound.Error()},
		})
		return
	}

	response := models.ListModuleVersionsResponse{
		Modules: []models.VersionsModule{
			{
				Versions: metadata.Module.Versions,
			},
		},
	}

	c.JSON(http.StatusOK, response)
}

// GetModuleVersion handles GET /v1/modules/{namespace}/{name}/{system}/{version}/download
func (h *RegistryHandlers) GetModuleVersion(c *gin.Context) {
	var params models.ModuleParams
	if err := c.ShouldBindUri(&params); err != nil {
		h.logger.Error("Failed to bind module parameters", "error", err)
		c.JSON(http.StatusBadRequest, models.NotFoundResponse{
			Errors: []string{"Invalid module parameters"},
		})
		return
	}

	h.logger.Info("Getting module download URL",
		"namespace", params.Namespace,
		"name", params.Name,
		"system", params.System,
		"version", params.Version)

	downloadInfo, err := h.storage.GetDownloadInfo(c.Request.Context(), api.ResourceIdentifier{
		Type:      api.ResourceTypeModule,
		Namespace: params.Namespace,
		Name:      params.Name,
		System:    params.System,
		Version:   params.Version,
	})
	if err != nil {
		h.logger.Error("Failed to get module download URL", "error", err)
		c.JSON(http.StatusNotFound, models.NotFoundResponse{
			Errors: []string{api.ErrModuleVersionNotFound.Error()},
		})
		return
	}

	// Set the X-Terraform-Get header and return 204 No Content
	c.Header("X-Terraform-Get", downloadInfo.URL)
	c.Status(http.StatusNoContent)
}

// ListProviderVersions handles GET /v1/providers/{namespace}/{type}/versions
func (h *RegistryHandlers) ListProviderVersions(c *gin.Context) {
	var params models.ProviderParams
	if err := c.ShouldBindUri(&params); err != nil {
		h.logger.Error("Failed to bind provider parameters", "error", err)
		c.JSON(http.StatusBadRequest, models.NotFoundResponse{
			Errors: []string{"Invalid provider parameters"},
		})
		return
	}

	h.logger.Info("Listing provider versions",
		"namespace", params.Namespace,
		"type", params.Type)

	versionsResp, err := h.storage.GetVersions(c.Request.Context(), api.ResourceIdentifier{
		Type:      api.ResourceTypeProvider,
		Namespace: params.Namespace,
		Name:      params.Type,
	})
	if err != nil {
		h.logger.Error("Failed to get provider versions", "error", err)
		c.JSON(http.StatusNotFound, models.NotFoundResponse{
			Errors: []string{api.ErrProviderNotFound.Error()},
		})
		return
	}

	response := models.ListProviderVersionsResponse{
		Versions: versionsResp.Provider.Versions,
	}

	c.JSON(http.StatusOK, response)
}

// GetProviderVersion handles GET /v1/providers/{namespace}/{type}/{version}/download/{os}/{arch}
func (h *RegistryHandlers) GetProviderVersion(c *gin.Context) {
	var params models.ProviderParams
	if err := c.ShouldBindUri(&params); err != nil {
		h.logger.Error("Failed to bind provider parameters", "error", err)
		c.JSON(http.StatusBadRequest, models.NotFoundResponse{
			Errors: []string{"Invalid provider parameters"},
		})
		return
	}

	h.logger.Info("Getting provider binary information",
		"namespace", params.Namespace,
		"type", params.Type,
		"version", params.Version,
		"os", params.OS,
		"arch", params.Arch)

	downloadInfo, err := h.storage.GetDownloadInfo(c.Request.Context(), api.ResourceIdentifier{
		Type:      api.ResourceTypeProvider,
		Namespace: params.Namespace,
		Name:      params.Type,
		Version:   params.Version,
		OS:        params.OS,
		Arch:      params.Arch,
	})
	if err != nil {
		h.logger.Error("Failed to get provider binary", "error", err)
		c.JSON(http.StatusNotFound, models.NotFoundResponse{
			Errors: []string{api.ErrProviderBinaryNotFound.Error()},
		})
		return
	}

	binaryMetadata := downloadInfo.ProviderBinary
	response := models.GetProviderVersionResponse{
		Arch:                binaryMetadata.Arch,
		DownloadURL:         binaryMetadata.DownloadURL,
		Filename:            binaryMetadata.Filename,
		OS:                  binaryMetadata.OS,
		Protocols:           binaryMetadata.Protocols,
		Shasum:              binaryMetadata.Shasum,
		ShasumsURL:          binaryMetadata.ShasumsURL,
		ShasumsSignatureURL: binaryMetadata.ShasumsSignatureURL,
		SigningKeys:         binaryMetadata.SigningKeys,
	}

	c.JSON(http.StatusOK, response)
}

// HealthCheck endpoint for service health monitoring
func (h *RegistryHandlers) HealthCheck(c *gin.Context) {
	// Check S3 storage health
	if err := h.storage.HealthCheck(c.Request.Context()); err != nil {
		h.logger.Error("Health check failed", "error", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "unhealthy",
			"error":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
	})
}

// Service discovery endpoint for Terraform provider registry protocol
func (h *RegistryHandlers) ServiceDiscovery(c *gin.Context) {
	// Extract hostname from request
	hostname := c.Request.Host

	discovery := map[string]interface{}{
		"providers.v1": fmt.Sprintf("http://%s/v1/providers/", hostname),
		"modules.v1":   fmt.Sprintf("http://%s/v1/modules/", hostname),
	}

	c.JSON(http.StatusOK, discovery)
}
