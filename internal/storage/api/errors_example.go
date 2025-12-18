package api

// Example usage of typed errors
//
// You can now check for specific error types using errors.Is():
//
// import "errors"
//
// // Check if a provider binary was not found
// metadata, err := storage.GetProviderBinary(ctx, namespace, typeName, version, os, arch)
// if errors.Is(err, storage.ErrProviderBinaryNotFound) {
//     // Handle provider binary not found specifically
//     return nil, fmt.Errorf("provider binary does not exist: %w", err)
// }
//
// // Check if a module was not found
// moduleMetadata, err := storage.GetModuleVersions(ctx, namespace, name, system)
// if errors.Is(err, storage.ErrModuleNotFound) {
//     // Handle module not found specifically
//     return nil, fmt.Errorf("module does not exist: %w", err)
// }
//
// // Check if a module version was not found
// downloadURL, err := storage.GetModuleDownloadURL(ctx, namespace, name, system, version)
// if errors.Is(err, storage.ErrModuleVersionNotFound) {
//     // Handle module version not found specifically
//     return nil, fmt.Errorf("specific module version does not exist: %w", err)
// }
//
// // Check if a provider was not found
// providerMetadata, err := storage.GetProviderVersions(ctx, namespace, typeName)
// if errors.Is(err, storage.ErrProviderNotFound) {
//     // Handle provider not found specifically
//     return nil, fmt.Errorf("provider does not exist: %w", err)
// }
