package storage

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/matryer/is"
	"github.com/rumenvasilev/ignis-hub/internal/config"
)

func TestNewStorage_InvalidProvider(t *testing.T) {
	is := is.New(t)
	ctx := context.Background()
	cfg := &config.Config{
		Provider: "azure", // Invalid provider
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	storage, err := New(ctx, cfg, logger)
	is.True(err != nil)                                      // NewStorage should fail for invalid provider
	is.True(storage == nil)                                  // storage should be nil for invalid provider
	is.Equal(err.Error(), "unknown storage provider: azure") // error message should match
}

func TestNewStorage_EmptyProvider(t *testing.T) {
	is := is.New(t)
	ctx := context.Background()
	cfg := &config.Config{
		Provider: "",
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	storage, err := New(ctx, cfg, logger)
	is.True(err != nil)     // NewStorage should fail for empty provider
	is.True(storage == nil) // storage should be nil for empty provider
}
