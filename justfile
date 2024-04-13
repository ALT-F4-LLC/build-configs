build profile='default':
    nix build \
        --json \
        --no-link \
        --print-build-logs \'.#{{ profile }}'

build-docker: (build 'docker')

check:
    nix flake check
