_default:
    just --list

check:
    nix flake check

docs:
	terraform-docs markdown table \
		--output-file README.md \
		--output-mode inject .

init:
    terraform init

package:
    nix build --json --no-link --print-build-logs .

validate:
    terraform validate
