# build-configs

[![License: Apache 2.0](https://img.shields.io/github/license/ALT-F4-LLC/build-configs)](./LICENSE)
[![Build Status](https://img.shields.io/github/actions/workflow/status/ALT-F4-LLC/build-configs/.github/workflows/flake.yaml)](https://github.com/ALT-F4-LLC/build-configs/actions)

build-configs is an easy-to-use, standardised configuration generator built to
ease development overhead when bootstrapping and updating configuration in
ALT-F4 projects.

It was built primarily to solve problems within ALT-F4, but was chosen to be an
open source project in order to demonstrate how the issue of type-standard
configuration updates in a polyrepo environment could be implemented.

## Installation

Installing build-configs can be done via Nix or by Go binary install:

```shell
$ nix profile install 'github:ALT-F4-LLC/build-configs#default' # with nix
$ go install github.com/ALT-F4-LLC/build-configs@latest # with go
```

## Examples

Some example configurations for our template types exist in the
[`examples`](./examples/) directory of the repo.

## Contributing

While this is an internal project at ALT-F4, we still welcome contributions from
the community in case you can spot an improvement or a suggestion!

Feel free to raise PRs and issues against this repository, but also understand
that as this _is_ an internal piece of tooling, some opinionations in templates
and/or logic will be present and we may be stubborn with them!

## License

build-configs is licensed under the [Apache License version 2.0](./LICENSE).
