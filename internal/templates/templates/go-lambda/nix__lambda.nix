{
  {{ .Nix.BuildGoModule }},
  name,
}:

{{ .Nix.BuildGoModule }} {
  inherit name;
  ldflags = ["-s" "-w"];
  src = ../.;
  subPackages = ["cmd/${name}"];
  tags = ["lambda.norpc"];
  vendorHash = "";

  {{ if .PrivateModules -}}
  GOPRIVATE = "{{ .PrivateModules }}";
  {{- end }}

  preBuild = ''
    export HOME=/tmp
    ls -alh $HOME/.netrc
  '';
}
