package config

type NixConfig struct {
	Cachix        NixCachixConfig `json:"cachix,omitempty" yaml:"cachix,omitempty"`
	NixpkgsBranch string          `json:"nixpkgsBranch,omitempty" yaml:"nixpkgsBranch,omitempty"`
}

func NewNixConfig() NixConfig {
	return NixConfig{
		NixpkgsBranch: "nixpkgs-unstable",
		Cachix: NixCachixConfig{
			BinaryCache: "altf4llc",
		},
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
