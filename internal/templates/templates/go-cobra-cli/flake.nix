{
  inputs.nixpkgs.url = "github:nixos/nixpkgs/{{ .Nix.NixpkgsBranch }}";

  outputs = inputs @ {flake-parts, ...}:
    flake-parts.lib.mkFlake {inherit inputs;} {
      systems = [{{range .Nix.Systems}}"{{.}}" {{end}}];

      perSystem = {
        config,
        pkgs,
        ...
      }: let
        inherit (pkgs) just;
        name = "{{ .Name }}";
        version = "{{ .Version }}";
        {{ if .CgoEnabled }}CGO_ENABLED = "1";{{ else }}CGO_ENABLED = "0";{{ end }}
      in {
        devShells.default = pkgs.mkShell {
          buildInputs = [just];
          inputsFrom = [config.packages.default];
        };

        packages = {
          default = pkgs.{{ .Nix.BuildGoModule }} {
            inherit CGO_ENABLED name version;
            src = ./.;
            subPackages = ["cmd/${name}"];
            vendorHash = "{{ .Nix.VendorHash }}";
          };
        };
      };
    };
}
