rec {
  env = {
    CGO_ENABLED = 0;
    {{- if .PrivateModules }}
    GOPRIVATE = "{{ .PrivateModules }}";
    {{- end }}
  };

  lambda = import ./lambda.nix { inherit env; };

  {{- if .OpenAPI.Enable }}
  client = import ./client.nix;
  {{- end }}
}
