build:
    go build -o dist/build-configs ./cmd/build-configs/main.go

lint:
    golangci-lint run ./...

package:
    vorpal build build-configs
