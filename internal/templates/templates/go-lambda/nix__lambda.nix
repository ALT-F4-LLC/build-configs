{ env }:

{
  {{ .Nix.BuildGoModule }},
  name,
}:

{{ .Nix.BuildGoModule }} {
  inherit name;
  inherit (env) CGO_ENABLED{{ if .PrivateModules }} GOPRIVATE{{ end }};
  ldflags = ["-s" "-w"];
  src = ../.;
  subPackages = ["cmd/${name}"];
  tags = ["lambda.norpc"];
  vendorHash = "";

  preBuild = ''
    export HOME=/tmp
    ls -alh $HOME/.netrc
  '';
}
