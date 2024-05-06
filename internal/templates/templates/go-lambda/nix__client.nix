{ runCommand, openapi-generator-cli }:
runCommand "client"
{
  src = ../api;
  buildInputs = [ openapi-generator-cli ];
} ''
  mkdir -p $out

  openapi-generator-cli generate \
    -i $src/openapi.yaml \
    -g go \
    -o $out \
    -p packageName="client" \
    -p packageVersion="0.1.0"

  rm -rf \
    $out/.gitignore \
    $out/.openapi-generator \
    $out/.openapi-generator-ignore \
    $out/.travis.yml \
    $out/README.md \
    $out/api \
    $out/docs \
    $out/git_push.sh \
    $out/test
'';
