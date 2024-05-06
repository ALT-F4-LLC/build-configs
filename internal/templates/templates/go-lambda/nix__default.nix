{
  env = {
    CGO_ENABLED = 0;
    {{- if .PrivateModules }}
    GOPRIVATE = "{{ .PrivateModules }}";
    {{- end }}
  };

  lambda = import ./lambda.nix;

  {{- if .OpenAPI.Enable }}
  client = import ./client.nix;
  {{- end }}
}
