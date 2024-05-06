{
  inputs.nixpkgs.url = "github:nixos/nixpkgs/{{ .Nix.NixpkgsBranch }}";

  outputs = inputs@{ flake-parts, ... }:
    flake-parts.lib.mkFlake { inherit inputs; } {
      systems = [ {{range .Nix.Systems}}"{{.}}" {{end}}];

      perSystem = { config, pkgs, ... }:
        let
          inherit (import ./nix) env lambda{{ if .OpenAPI.Enable }} client{{ end }};
          inherit (pkgs) go-migrate golangci-lint just zip;
        in
        {
          devShells.default = pkgs.mkShell {
            inherit (env) CGO_ENABLED{{ if .PrivateModules }} GOPRIVATE{{ end }};
            nativeBuildInputs = [ go-migrate golangci-lint just zip ];
            inputsFrom = [
              {{- range .Lambdas }}
              config.packages.{{.}}
              {{- end }}
              {{- if .OpenAPI.Enable }}
              config.packages.client
              {{- end }}
            ];
          };

          packages = {
            {{- range $fn := .Lambdas }}
            {{ $fn }} = pkgs.callPackage lambda { name = "{{ $fn }}"; };
            {{- end }}
            {{- if .OpenAPI.Enable }}
            client = pkgs.callPackage client {};
            {{- end }}
          };
        };
  };
}
