{
  inputs = {
    dream2nix = {
      inputs.nixpkgs.follows = "nixpkgs";
      url = "github:nix-community/dream2nix";
    };

    flake-parts = {
      inputs.nixpkgs-lib.follows = "nixpkgs";
      url = "github:hercules-ci/flake-parts";
    };

    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
  };

  outputs = inputs:
    inputs.flake-parts.lib.mkFlake { inherit inputs; } {
      imports = [ inputs.dream2nix.flakeModuleBeta ];
      systems = [ "x86_64-linux" ];

      perSystem = { pkgs, config, ... }: {
        dream2nix.inputs.self.source = inputs.self;
        formatter = pkgs.nixpkgs-fmt;
      };
    };
}
