{
  inputs.nixpkgs.url = "github:nixos/nixpkgs/{{ .Nix.NixpkgsBranch }}";

  outputs = inputs@{ flake-parts, ... }:
    flake-parts.lib.mkFlake { inherit inputs; } {
      systems = [ {{range .Nix.Systems}}"{{.}}" {{end}}];

      perSystem = { config, pkgs, ... }:
        let
          inherit (pkgs)
            {{ .Nix.GoPackage }}
            just;

          lambdas = {
            {{- range $fn := .Lambdas }}
            {{ $fn }} = pkgs.callPackage ./nix/lambda.nix { name = "{{ $fn }}"; };
            {{- end }}
          };
        in
        {
          devShells.default = pkgs.mkShell {
            {{ if .PrivateModules -}}
            GOPRIVATE = "{{ .PrivateModules }}";
            {{- end }}
            buildInputs = [ just ];
            inputsFrom = [
              {{- range .Lambdas }}
              config.packages.{{.}}
              {{- end }}
            ];
          };

          packages = { } // lambdas;
        };
  };
}
