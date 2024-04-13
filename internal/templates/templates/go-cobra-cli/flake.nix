{
  inputs.nixpkgs.url = "github:nixos/nixpkgs/{{ .Nix.NixpkgsBranch }}";

  outputs = inputs@{ flake-parts, ... }:
    flake-parts.lib.mkFlake { inherit inputs; } {
      systems = [ ];

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

          packages.default = pkgs.{{ .Nix.BuildGoModule }} {
            inherit name;
            src = ./.;
            vendorHash = "{{ .Nix.VendorHash }}";
            buildModules = [ "cmd/${name}" ];
          };
        };
  };
}
