{
  inputs = {
    flake-parts = {
      inputs.nixpkgs-lib.follows = "nixpkgs";
      url = "github:hercules-ci/flake-parts";
    };

    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
  };

  outputs = inputs:
    inputs.flake-parts.lib.mkFlake { inherit inputs; } {
      systems = [ "x86_64-linux" ];

      perSystem = { pkgs, lib, config, ... }:
        let
          inherit (lib.importTOML (inputs.self + "/Cargo.toml")) package;
        in
        {
          packages = {
            default = pkgs.rustPlatform.buildRustPackage {
              inherit (package) version;

              cargoLock.lockFile = (inputs.self + "/Cargo.lock");
              pname = package.name;
              src = inputs.self;
            };
          };

          devShells.default = pkgs.mkShell {
            packages = with pkgs; [
              cargo
              rustc
              rustfmt
            ];
          };

          apps = {
            default = {
              program = "${config.packages.default}/bin/build-configs";
              type = "app";
            };
          };

          formatter = pkgs.nixpkgs-fmt;
        };
    };
}
