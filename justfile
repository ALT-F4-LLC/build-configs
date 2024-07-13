build:
    go build -o build-configs ./cmd/build-configs/main.go

check:
    nix flake check

package profile='default':
    nix build \
        --json \
        --no-link \
        --print-build-logs \
        '.#{{ profile }}'

package-docker: (package 'docker')
