{
  description = "Build configuration generation CLI tool.";

  inputs = {
    advisory-db = {
      url = "github:rustsec/advisory-db";
      flake = false;
    };

    crane = {
      url = "github:ipetkov/crane";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    flake-utils.url = "github:numtide/flake-utils";

    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
  };

  outputs = { self, nixpkgs, crane, flake-utils, advisory-db, ... }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };

        inherit (pkgs) lib;

        craneLib = crane.lib.${system};

        src = craneLib.cleanCargoSource ./.;

        buildInputs = [ ]
          ++ lib.optionals pkgs.stdenv.isDarwin [ pkgs.libiconv ];

        cargoArtifacts = craneLib.buildDepsOnly { inherit src buildInputs; };

        build-configs =
          craneLib.buildPackage { inherit cargoArtifacts src buildInputs; };
      in {
        checks = {
          inherit build-configs;

          build-configs-clippy = craneLib.cargoClippy {
            inherit cargoArtifacts src buildInputs;

            cargoClippyExtraArgs = "--all-targets -- --deny warnings";
          };

          build-configs-doc =
            craneLib.cargoDoc { inherit cargoArtifacts src buildInputs; };

          build-configs-fmt = craneLib.cargoFmt { inherit src; };

          build-configs-audit =
            craneLib.cargoAudit { inherit src advisory-db; };

          build-configs-nextest = craneLib.cargoNextest {
            inherit cargoArtifacts src buildInputs;
            partitions = 1;
            partitionType = "count";
          };
        } // lib.optionalAttrs (system == "x86_64-linux") {
          build-configs-coverage =
            craneLib.cargoTarpaulin { inherit cargoArtifacts src; };
        };

        packages.default = build-configs;

        apps.default = flake-utils.lib.mkApp { drv = build-configs; };

        devShells.default = pkgs.mkShell {
          inputsFrom = builtins.attrValues self.checks;

          nativeBuildInputs = with pkgs; [ cargo rustc ];
        };
      });
}
