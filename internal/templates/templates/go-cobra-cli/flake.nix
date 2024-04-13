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

          name = "{{ .Name }}";
          CGO_ENABLED = "0";
        in
        {
          devShells.default = pkgs.mkShell {
            buildInputs = [ just ];
            inputsFrom = [ config.packages.default ];
          };

          packages = {
            default = pkgs.{{ .Nix.BuildGoModule }} {
              inherit name;
              src = ./.;
              vendorHash = "{{ .Nix.VendorHash }}";
              buildModules = [ "cmd/${name}" ];
            };

            docker = pkgs.dockerTools.buildImage {
              inherit name;
              tag = version;
              config = {
                Entrypoint = [ "${config.packages.default}/bin/${name}" ];
                Env = [
                  "SSL_CERT_FILE=${pkgs.cacert}/etc/ssl/certs/ca-bundle.crt"
                ];
              };
            };
          };
        };
  };
}
