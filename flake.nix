{
  inputs = {
    flake-utils.url = "github:numtide/flake-utils";
    naersk.url = "github:nix-community/naersk";
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
  };

  outputs = { self, flake-utils, naersk, nixpkgs }:
    let
      supportedSystems =
        [ "x86_64-linux" "x86_64-darwin" "aarch64-linux" "aarch64-darwin" ];
      forAllSystems = nixpkgs.lib.genAttrs supportedSystems;
      pkgs = forAllSystems (system: nixpkgs.legacyPackages.${system});
      naersk' = forAllSystems (system: pkgs.${system}.callPackage naersk { });
    in {
      packages = forAllSystems
        (system: { default = naersk'.${system}.buildPackage { src = ./.; }; });

      devShells = forAllSystems (system: {
        default = pkgs.${system}.mkShellNoCC {
          RUST_SRC_PATH = pkgs.${system}.rustPlatform.rustLibSrc;
          buildInputs = with pkgs.${system}; [ rustfmt ];
          nativeBuildInputs = with pkgs.${system}; [ rustc cargo gcc libiconv ];
        };
      });

      apps = forAllSystems (system: {
        default = {
          program = "${self.packages.${system}.default}/bin/build-configs";
          type = "app";
        };
      });
    };
}
