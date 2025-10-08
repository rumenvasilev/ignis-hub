package models

// Well-known endpoint response
type WellKnownResponse struct {
	ModulesV1   string `json:"modules.v1"`
	ProvidersV1 string `json:"providers.v1"`
}

// Module-related structures
type Version struct {
	Version string `json:"version"`
}

type VersionsModule struct {
	Versions []Version `json:"versions"`
}

type ListModuleVersionsResponse struct {
	Modules []VersionsModule `json:"modules"`
}

// Provider-related structures
type Platform struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

type ProviderVersion struct {
	Version   string     `json:"version"`
	Protocols []string   `json:"protocols"`
	Platforms []Platform `json:"platforms"`
}

type ListProviderVersionsResponse struct {
	Versions []ProviderVersion `json:"versions"`
}

type GPGPublicKey struct {
	KeyID      string `json:"key_id"`
	ASCIIArmor string `json:"ascii_armor"`
}

type SigningKeys struct {
	GPGPublicKeys []GPGPublicKey `json:"gpg_public_keys"`
}

type GetProviderVersionResponse struct {
	Arch                string      `json:"arch"`
	DownloadURL         string      `json:"download_url"`
	Filename            string      `json:"filename"`
	OS                  string      `json:"os"`
	Protocols           []string    `json:"protocols"`
	Shasum              string      `json:"shasum"`
	ShasumsURL          string      `json:"shasums_url"`
	ShasumsSignatureURL string      `json:"shasums_signature_url"`
	SigningKeys         SigningKeys `json:"signing_keys,omitempty"`
}

// Error response
type NotFoundResponse struct {
	Errors []string `json:"errors"`
}

// Path parameters and request structures
type ModuleParams struct {
	Namespace string `uri:"namespace" binding:"required"`
	Name      string `uri:"name" binding:"required"`
	System    string `uri:"system" binding:"required"`
	Version   string `uri:"version"`
}

type ProviderParams struct {
	Namespace string `uri:"namespace" binding:"required"`
	Type      string `uri:"type" binding:"required"`
	Version   string `uri:"version"`
	OS        string `uri:"os"`
	Arch      string `uri:"arch"`
}

// S3 metadata structures for internal use
type ModuleMetadata struct {
	Namespace string    `json:"namespace"`
	Name      string    `json:"name"`
	System    string    `json:"system"`
	Versions  []Version `json:"versions"`
}

type ProviderMetadata struct {
	Namespace string            `json:"namespace"`
	Type      string            `json:"type"`
	Versions  []ProviderVersion `json:"versions"`
}

type ProviderBinaryMetadata struct {
	Namespace           string      `json:"namespace"`
	Type                string      `json:"type"`
	Version             string      `json:"version"`
	OS                  string      `json:"os"`
	Arch                string      `json:"arch"`
	Filename            string      `json:"filename"`
	DownloadURL         string      `json:"download_url"`
	Shasum              string      `json:"shasum"`
	ShasumsURL          string      `json:"shasums_url"`
	ShasumsSignatureURL string      `json:"shasums_signature_url"`
	Protocols           []string    `json:"protocols"`
	SigningKeys         SigningKeys `json:"signing_keys,omitempty"`
}
