{ runCommand, openapi-generator-cli }:
runCommand "client"
{
  src = ../api;
  buildInputs = [ openapi-generator-cli ];
} ''
  mkdir -p $out

  openapi-generator-cli generate \
    -i $src/{{ .OpenAPI.Filename }} \
    -g go \
    -o $out \
    -p packageName="client"

  rm -rf \
    $out/.gitignore \
    $out/.openapi-generator \
    $out/.openapi-generator-ignore \
    $out/.travis.yml \
    $out/README.md \
    $out/api \
    $out/docs \
    $out/git_push.sh
''
