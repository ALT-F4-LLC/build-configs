{
  inputs.nixpkgs.url = "github:nixos/nixpkgs/nixpkgs-unstable";

  outputs = inputs @ {flake-parts, ...}:
    flake-parts.lib.mkFlake {inherit inputs;} {
      systems = ["x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin"];

      perSystem = {
        config,
        pkgs,
        ...
      }: let
        inherit (pkgs) just;
        name = "build-configs";
        version = "0.1.0";
        CGO_ENABLED = "0";
      in {
        devShells.default = pkgs.mkShell {
          buildInputs = [just];
          inputsFrom = [config.packages.default];
        };

        packages = {
          default = pkgs.buildGoModule {
            inherit CGO_ENABLED name version;
            src = ./.;
            subPackages = ["cmd/${name}"];
            vendorHash = "sha256-6B9O6ho4COpJy4HlkzQ0lk+ieezRO3xg9LyLHzoxYzc=";
          };
        };
      };
    };
}
