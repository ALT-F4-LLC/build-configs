build:
    go build -o dist/build-configs ./cmd/build-configs/main.go

doc-validate:
    rumdl check --no-cache --deny-config-warnings .

install:
    go install ./cmd/build-configs

lint:
    golangci-lint run ./...

package:
    vorpal build build-configs

secret-scan:
    gitleaks git --no-banner --redact --exit-code 1 --log-opts=HEAD .

test:
    go test ./...

vuln-scan:
    govulncheck ./...
