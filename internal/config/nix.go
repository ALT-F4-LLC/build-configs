package config

type NixConfig struct {
	Cachix        NixCachixConfig `json:"cachix,omitempty" yaml:"cachix,omitempty"`
	NixpkgsBranch string          `json:"nixpkgsBranch,omitempty" yaml:"nixpkgsBranch,omitempty"`
	Systems       []string        `json:"systems,omitempty" yaml:"systems,omitempty"`
}

func NewNixConfig() NixConfig {
	return NixConfig{
		Cachix: NixCachixConfig{
			BinaryCache: "altf4llc",
		},

		NixpkgsBranch: "nixpkgs-unstable",
		Systems:       []string{"x86_64-linux", "aarch64-linux", "x86_64-darwin", "aarch64-darwin"},
	}
}

type NixCachixConfig struct {
	BinaryCache string `json:"binaryCache,omitempty" yaml:"binaryCache,omitempty"`
}

type NixGoConfig struct {
	NixConfig
	GoPackage     string `json:"goPackage,omitempty" yaml:"goPackage,omitempty"`
	BuildGoModule string `json:"buildGoModule,omitempty" yaml:"buildGoModule,omitempty"`
	VendorHash    string `json:"vendorHash,omitempty" yaml:"vendorHash,omitempty"`
}
