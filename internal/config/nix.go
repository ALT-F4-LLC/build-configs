package config

type NixConfig struct {
	NixpkgsBranch string `json:"nixpkgsBranch,omitempty" yaml:"nixpkgsBranch,omitempty"`
}

type NixGoConfig struct {
	NixConfig
	GoPackage     string `json:"goPackage,omitempty" yaml:"goPackage,omitempty"`
	BuildGoModule string `json:"buildGoModule,omitempty" yaml:"buildGoModule,omitempty"`
	VendorHash    string `json:"vendorHash,omitempty" yaml:"vendorHash,omitempty"`
}
