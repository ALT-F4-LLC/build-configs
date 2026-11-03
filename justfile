build:
    go build -o dist/build-configs ./cmd/build-configs/main.go

check: lint build

lint:
    golangci-lint run ./...

package:
    vorpal build build-configs
