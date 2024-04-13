{
  inputs.nixpkgs.url = "github:nixos/nixpkgs/nixpkgs-unstable";

  outputs = inputs@{ flake-parts, ... }:
    flake-parts.lib.mkFlake { inherit inputs; } {
      systems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];

      perSystem = { config, pkgs, ... }:
        let
          inherit (pkgs)
            go_1_22
            just;

          name = "build-configs";
          CGO_ENABLED = "0";
        in
        {
          devShells.default = pkgs.mkShell {
            buildInputs = [ just ];
            inputsFrom = [ config.packages.default ];
          };

          packages = {
            default = pkgs.buildGo122Module {
              inherit name;
              src = ./.;
              vendorHash = "sha256-6B9O6ho4COpJy4HlkzQ0lk+ieezRO3xg9LyLHzoxYzc=";
              buildModules = [ "cmd/${name}" ];
            };

            docker = pkgs.dockerTools.buildImage {
              inherit name;
              tag = "latest";
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
