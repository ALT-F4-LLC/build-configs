{ env }:

{
  {{ .Nix.BuildGoModule }},
  name,
  runCommand,
  zip
}:

let
  pkg = {{ .Nix.BuildGoModule }} {
    inherit name;
    inherit (env) CGO_ENABLED{{ if .PrivateModules }} GOPRIVATE{{ end }};
    ldflags = ["-s" "-w"];
    src = ../.;
    subPackages = ["cmd/${name}"];
    tags = ["lambda.norpc"];
    vendorHash = "{{ .Nix.VendorHash }}";

    preBuild = ''
      export HOME=/tmp
      ls -alh $HOME/.netrc
    '';
  };

  bootstrap = runCommand "${name}-bootstrap" {
    buildInputs = [ zip ];
  } ''
    (cd ${pkg}/bin && zip $out ${name})
  '';
in
  pkg.overrideAttrs (final: {
    passthru.bootstrap = bootstrap;
  })
