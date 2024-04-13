package config

type GoCobraCliConfig struct {
	Nix NixGoConfig `json:"nix,omitempty" yaml:"nix,omitempty"`
}

func (c GoCobraCliConfig) Render() error {
	return nil
}
