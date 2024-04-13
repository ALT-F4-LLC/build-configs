build profile='default':
    nix build \
        --json \
        --no-link \
        --print-build-logs \
        --sandbox \
        --sandbox-paths "{{ netrc }}" \
        '.#{{ profile }}'

build-docker: (build 'docker')

check:
    nix flake check
