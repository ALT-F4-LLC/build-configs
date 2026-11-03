build:
    go build -o dist/build-configs ./cmd/build-configs/main.go

package:
    vorpal build build-configs
