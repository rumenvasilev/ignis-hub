package api

import "errors"

// Sentinel errors for storage operations.
var (
	ErrModuleNotFound         = errors.New("module not found")
	ErrModuleVersionNotFound  = errors.New("module version not found")
	ErrProviderNotFound       = errors.New("provider not found")
	ErrProviderBinaryNotFound = errors.New("provider binary not found")
)
