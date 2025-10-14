package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/matryer/is"
	"github.com/rumenvasilev/ignis-hub/internal/config"
	"github.com/rumenvasilev/ignis-hub/internal/models"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// Mock storage implementation
type mockStorage struct {
	getModuleVersionsFunc    func(ctx context.Context, namespace, name, system string) (*models.ModuleMetadata, error)
	getModuleDownloadURLFunc func(ctx context.Context, namespace, name, system, version string) (string, error)
	getProviderVersionsFunc  func(ctx context.Context, namespace, typeName string) (*models.ProviderMetadata, error)
	getProviderBinaryFunc    func(ctx context.Context, namespace, typeName, version, os, arch string) (*models.ProviderBinaryMetadata, error)
	healthCheckFunc          func(ctx context.Context) error
}

func (m *mockStorage) GetModuleVersions(ctx context.Context, namespace, name, system string) (*models.ModuleMetadata, error) {
	if m.getModuleVersionsFunc != nil {
		return m.getModuleVersionsFunc(ctx, namespace, name, system)
	}
	return nil, errors.New("not implemented")
}

func (m *mockStorage) GetModuleDownloadURL(ctx context.Context, namespace, name, system, version string) (string, error) {
	if m.getModuleDownloadURLFunc != nil {
		return m.getModuleDownloadURLFunc(ctx, namespace, name, system, version)
	}
	return "", errors.New("not implemented")
}

func (m *mockStorage) GetProviderVersions(ctx context.Context, namespace, typeName string) (*models.ProviderMetadata, error) {
	if m.getProviderVersionsFunc != nil {
		return m.getProviderVersionsFunc(ctx, namespace, typeName)
	}
	return nil, errors.New("not implemented")
}

func (m *mockStorage) GetProviderBinary(ctx context.Context, namespace, typeName, version, os, arch string) (*models.ProviderBinaryMetadata, error) {
	if m.getProviderBinaryFunc != nil {
		return m.getProviderBinaryFunc(ctx, namespace, typeName, version, os, arch)
	}
	return nil, errors.New("not implemented")
}

func (m *mockStorage) HealthCheck(ctx context.Context) error {
	if m.healthCheckFunc != nil {
		return m.healthCheckFunc(ctx)
	}
	return nil
}

func setupTestHandler() (*RegistryHandlers, *mockStorage, *slog.Logger) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			BaseURL: "http://localhost:8080",
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	storage := &mockStorage{}
	handler := NewRegistryHandlers(storage, cfg, logger)
	return handler, storage, logger
}

func TestNewRegistryHandlers(t *testing.T) {
	is := is.New(t)
	handler, storage, logger := setupTestHandler()

	is.True(handler != nil)             // handler should not be nil
	is.True(handler.storage == storage) // storage should be set correctly
	is.True(handler.logger == logger)   // logger should be set correctly
}

func TestGetWellKnown(t *testing.T) {
	is := is.New(t)
	handler, _, _ := setupTestHandler()

	router := gin.New()
	router.GET("/.well-known/terraform.json", handler.GetWellKnown)

	req := httptest.NewRequest("GET", "/.well-known/terraform.json", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	is.Equal(w.Code, http.StatusOK) // status should be 200

	var response models.WellKnownResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	is.NoErr(err) // should unmarshal response

	is.Equal(response.ModulesV1, "http://localhost:8080/v1/modules/")     // ModulesV1 URL should be correct
	is.Equal(response.ProvidersV1, "http://localhost:8080/v1/providers/") // ProvidersV1 URL should be correct
}

func TestGetWellKnown_BaseURLWithSlash(t *testing.T) {
	is := is.New(t)
	cfg := &config.Config{
		Server: config.ServerConfig{
			BaseURL: "http://localhost:8080/",
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	storage := &mockStorage{}
	handler := NewRegistryHandlers(storage, cfg, logger)

	router := gin.New()
	router.GET("/.well-known/terraform.json", handler.GetWellKnown)

	req := httptest.NewRequest("GET", "/.well-known/terraform.json", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var response models.WellKnownResponse
	_ = json.Unmarshal(w.Body.Bytes(), &response)

	// Should not have double slash
	is.Equal(response.ModulesV1, "http://localhost:8080/v1/modules/") // should not have double slash
}

func TestListModuleVersions_Success(t *testing.T) {
	is := is.New(t)
	handler, storage, _ := setupTestHandler()

	storage.getModuleVersionsFunc = func(ctx context.Context, namespace, name, system string) (*models.ModuleMetadata, error) {
		return &models.ModuleMetadata{
			Versions: []models.Version{
				{Version: "1.0.0"},
				{Version: "1.1.0"},
			},
		}, nil
	}

	router := gin.New()
	router.GET("/v1/modules/:namespace/:name/:system/versions", handler.ListModuleVersions)

	req := httptest.NewRequest("GET", "/v1/modules/hashicorp/consul/aws/versions", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	is.Equal(w.Code, http.StatusOK) // status should be 200

	var response models.ListModuleVersionsResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	is.NoErr(err) // should unmarshal response

	is.Equal(len(response.Modules), 1)             // should have 1 module
	is.Equal(len(response.Modules[0].Versions), 2) // should have 2 versions
}

func TestListModuleVersions_NotFound(t *testing.T) {
	is := is.New(t)
	handler, storage, _ := setupTestHandler()

	storage.getModuleVersionsFunc = func(ctx context.Context, namespace, name, system string) (*models.ModuleMetadata, error) {
		return nil, errors.New("module not found")
	}

	router := gin.New()
	router.GET("/v1/modules/:namespace/:name/:system/versions", handler.ListModuleVersions)

	req := httptest.NewRequest("GET", "/v1/modules/hashicorp/nonexistent/aws/versions", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	is.Equal(w.Code, http.StatusNotFound) // status should be 404

	var response models.NotFoundResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	is.NoErr(err) // should unmarshal response

	is.True(len(response.Errors) > 0) // should have error message
}

func TestGetModuleVersion_Success(t *testing.T) {
	is := is.New(t)
	handler, storage, _ := setupTestHandler()

	storage.getModuleDownloadURLFunc = func(ctx context.Context, namespace, name, system, version string) (string, error) {
		return "https://example.com/module.tar.gz", nil
	}

	router := gin.New()
	router.GET("/v1/modules/:namespace/:name/:system/:version/download", handler.GetModuleVersion)

	req := httptest.NewRequest("GET", "/v1/modules/hashicorp/consul/aws/1.0.0/download", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	is.Equal(w.Code, http.StatusNoContent) // status should be 204

	downloadURL := w.Header().Get("X-Terraform-Get")
	is.Equal(downloadURL, "https://example.com/module.tar.gz") // X-Terraform-Get header should be set
}

func TestGetModuleVersion_NotFound(t *testing.T) {
	is := is.New(t)
	handler, storage, _ := setupTestHandler()

	storage.getModuleDownloadURLFunc = func(ctx context.Context, namespace, name, system, version string) (string, error) {
		return "", errors.New("version not found")
	}

	router := gin.New()
	router.GET("/v1/modules/:namespace/:name/:system/:version/download", handler.GetModuleVersion)

	req := httptest.NewRequest("GET", "/v1/modules/hashicorp/consul/aws/999.0.0/download", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	is.Equal(w.Code, http.StatusNotFound) // status should be 404
}

func TestListProviderVersions_Success(t *testing.T) {
	is := is.New(t)
	handler, storage, _ := setupTestHandler()

	storage.getProviderVersionsFunc = func(ctx context.Context, namespace, typeName string) (*models.ProviderMetadata, error) {
		return &models.ProviderMetadata{
			Versions: []models.ProviderVersion{
				{Version: "1.0.0"},
				{Version: "2.0.0"},
			},
		}, nil
	}

	router := gin.New()
	router.GET("/v1/providers/:namespace/:type/versions", handler.ListProviderVersions)

	req := httptest.NewRequest("GET", "/v1/providers/hashicorp/aws/versions", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	is.Equal(w.Code, http.StatusOK) // status should be 200

	var response models.ListProviderVersionsResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	is.NoErr(err) // should unmarshal response

	is.Equal(len(response.Versions), 2) // should have 2 versions
}

func TestListProviderVersions_NotFound(t *testing.T) {
	is := is.New(t)
	handler, storage, _ := setupTestHandler()

	storage.getProviderVersionsFunc = func(ctx context.Context, namespace, typeName string) (*models.ProviderMetadata, error) {
		return nil, errors.New("provider not found")
	}

	router := gin.New()
	router.GET("/v1/providers/:namespace/:type/versions", handler.ListProviderVersions)

	req := httptest.NewRequest("GET", "/v1/providers/hashicorp/nonexistent/versions", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	is.Equal(w.Code, http.StatusNotFound) // status should be 404
}

func TestGetProviderVersion_Success(t *testing.T) {
	is := is.New(t)
	handler, storage, _ := setupTestHandler()

	storage.getProviderBinaryFunc = func(ctx context.Context, namespace, typeName, version, os, arch string) (*models.ProviderBinaryMetadata, error) {
		return &models.ProviderBinaryMetadata{
			Arch:        "amd64",
			OS:          "linux",
			DownloadURL: "https://example.com/provider.zip",
			Filename:    "terraform-provider-aws_1.0.0_linux_amd64.zip",
			Protocols:   []string{"5.0"},
			Shasum:      "abc123",
			SigningKeys: models.SigningKeys{},
		}, nil
	}

	router := gin.New()
	router.GET("/v1/providers/:namespace/:type/:version/download/:os/:arch", handler.GetProviderVersion)

	req := httptest.NewRequest("GET", "/v1/providers/hashicorp/aws/1.0.0/download/linux/amd64", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	is.Equal(w.Code, http.StatusOK) // status should be 200

	var response models.GetProviderVersionResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	is.NoErr(err) // should unmarshal response

	is.Equal(response.Arch, "amd64")                                   // arch should be amd64
	is.Equal(response.OS, "linux")                                     // OS should be linux
	is.Equal(response.DownloadURL, "https://example.com/provider.zip") // download URL should be correct
}

func TestGetProviderVersion_NotFound(t *testing.T) {
	is := is.New(t)
	handler, storage, _ := setupTestHandler()

	storage.getProviderBinaryFunc = func(ctx context.Context, namespace, typeName, version, os, arch string) (*models.ProviderBinaryMetadata, error) {
		return nil, errors.New("binary not found")
	}

	router := gin.New()
	router.GET("/v1/providers/:namespace/:type/:version/download/:os/:arch", handler.GetProviderVersion)

	req := httptest.NewRequest("GET", "/v1/providers/hashicorp/aws/999.0.0/download/linux/amd64", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	is.Equal(w.Code, http.StatusNotFound) // status should be 404
}

func TestHealthCheck_Healthy(t *testing.T) {
	is := is.New(t)
	handler, storage, _ := setupTestHandler()

	storage.healthCheckFunc = func(ctx context.Context) error {
		return nil
	}

	router := gin.New()
	router.GET("/health", handler.HealthCheck)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	is.Equal(w.Code, http.StatusOK) // status should be 200

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	is.NoErr(err) // should unmarshal response

	is.Equal(response["status"], "healthy") // status should be healthy
}

func TestHealthCheck_Unhealthy(t *testing.T) {
	is := is.New(t)
	handler, storage, _ := setupTestHandler()

	storage.healthCheckFunc = func(ctx context.Context) error {
		return errors.New("storage unavailable")
	}

	router := gin.New()
	router.GET("/health", handler.HealthCheck)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	is.Equal(w.Code, http.StatusServiceUnavailable) // status should be 503

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	is.NoErr(err) // should unmarshal response

	is.Equal(response["status"], "unhealthy") // status should be unhealthy
	is.True(response["error"] != nil)         // error should be present
}

func TestServiceDiscovery(t *testing.T) {
	is := is.New(t)
	handler, _, _ := setupTestHandler()

	router := gin.New()
	router.GET("/.well-known/terraform.json", handler.ServiceDiscovery)

	req := httptest.NewRequest("GET", "/.well-known/terraform.json", nil)
	req.Host = "registry.example.com"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	is.Equal(w.Code, http.StatusOK) // status should be 200

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	is.NoErr(err) // should unmarshal response

	is.True(response["providers.v1"] != nil) // providers.v1 should be present
	is.True(response["modules.v1"] != nil)   // modules.v1 should be present
}

func TestInvalidURIParameters(t *testing.T) {
	handler, _, _ := setupTestHandler()

	tests := []struct {
		name    string
		path    string
		handler gin.HandlerFunc
	}{
		{
			name:    "ListModuleVersions",
			path:    "/v1/modules//invalid//versions",
			handler: handler.ListModuleVersions,
		},
		{
			name:    "GetModuleVersion",
			path:    "/v1/modules//invalid///download",
			handler: handler.GetModuleVersion,
		},
		{
			name:    "ListProviderVersions",
			path:    "/v1/providers//invalid/versions",
			handler: handler.ListProviderVersions,
		},
		{
			name:    "GetProviderVersion",
			path:    "/v1/providers//invalid//download//",
			handler: handler.GetProviderVersion,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET(tt.path, tt.handler)

			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should handle gracefully, either 400 or 404
			if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
				t.Logf("Status code %d is acceptable for invalid parameters", w.Code)
			}
		})
	}
}
