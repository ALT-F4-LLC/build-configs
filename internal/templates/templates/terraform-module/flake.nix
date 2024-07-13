{
  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-24.05";

  outputs = inputs @ {
    flake-parts,
    nixpkgs,
    ...
  }:
    flake-parts.lib.mkFlake {inherit inputs;} {
      systems = ["x86_64-linux" "aarch64-linux" "aarch64-darwin" "x86_64-darwin"];

      perSystem = {
        config,
        pkgs,
        system,
        ...
      }: let
        inherit (pkgs) just mkShell terraform-docs;
        terraform = pkgs.terraform.withPlugins (ps: [
          {{- range $p := .Providers }}
          ps.{{ $p }}
          {{- end }}
        ]);
      in {
        _module.args.pkgs = import nixpkgs {
          inherit system;
          config.allowUnfree = true;
        };

        devShells = {
          default = mkShell {
            inputsFrom = [config.packages.default];
            nativeBuildInputs = [
              just
              terraform-docs
            ];
          };
        };

        packages = {
          default =
            pkgs.runCommand "default"
            {
              src = ./.;
            } ''
              mkdir -p $out
              cp -R $src/*.tf $out
              ${terraform}/bin/terraform -chdir="$out" init
              ${terraform}/bin/terraform -chdir="$out" validate
            '';
        };
      };
    };
}
