# build-configs

[![License: Apache 2.0](https://img.shields.io/github/license/ALT-F4-LLC/build-configs)](./LICENSE)
[![Build Status](https://img.shields.io/github/actions/workflow/status/ALT-F4-LLC/build-configs/.github/workflows/ci.yaml)](https://github.com/ALT-F4-LLC/build-configs/actions)

build-configs is an easy-to-use, standardised configuration generator built to
ease development overhead when bootstrapping and updating configuration in
ALT-F4 projects.

It was built primarily to solve problems within ALT-F4, but was chosen to be an
open source project in order to demonstrate how the issue of type-standard
configuration updates in a polyrepo environment could be implemented.

## Installation

Installing build-configs can be done via Vorpal or by Go binary install:

```shell
# with vorpal, from a clone of this repository
vorpal build build-configs
# with go
go install github.com/ALT-F4-LLC/build-configs/cmd/build-configs@latest
```

The Vorpal build places the binary at `bin/build-configs` under the path that
`vorpal build --path build-configs` prints.

## Usage

To generate configuration with build-configs, you can use the `generate`
subcommand in a directory with a `build-configs.yaml` or `build-configs.json`
file present. See [Examples](#examples) for help with this.

```shell
build-configs generate
# when built with vorpal
"$(vorpal build --path build-configs)/bin/build-configs" generate
```

It is also possible to configure build-configs to leave otherwise templated be,
similarly to the `.gitignore` file in a Git repository. The file used is called
`.bcignore`, and takes a direct list of paths (globs are currently unsupported)
to skip templating:

```gitignore
# ignore the justfile, we customize it further.
justfile
```

## Development

The Vorpal dev shell provides Go and golangci-lint. Enter it with direnv, then
lint and build:

```shell
direnv allow
just lint
just build
```

## Templates

### `go-cobra-cli`

`go-cobra-cli` scaffolds a Cobra CLI tool with Go that can be used to build out
internal tooling. This is the template this CLI uses.

### `go-lambda`

`go-lambda` scaffolds an lambda setup in Go.

To use it, you need to run the generator and then run `go mod init` and
`go mod tidy`, then update your `vendorHash`.

#### Services

`services` lists Go binaries built from `cmd/<name>` that are not lambdas, such
as a long-running MCP server:

```yaml
parameters:
  lambdas: [api]
  services: [mcp]
```

A service is added to `cmd/vorpal/main.go` with build directory `cmd/<name>`,
`-ldflags='-s -w'` and `CGO_ENABLED=0`, as a lambda is, but without
`-tags='lambda.norpc'`. CI builds it in a `build-services` job with `just build`
only: there is no package, upload or deploy step, and the `*-all` recipes in the
`justfile` leave it out. The template does not scaffold `cmd/<name>/main.go`.
A name listed in both `lambdas` and `services` is an error. `lambdas` defaults
to `[<name>]`, so set `lambdas: []` for a services-only project. That project
has no lambda `build` job, and `deploy` together with `services` and an empty
`lambdas` is an error.

#### `vorpal.extension`

`vorpal.extension` is the full import path of a package in your own repository.
When set, the generated `cmd/vorpal/main.go` imports it as `project`, calls
`project.Artifacts` once after the development environment, and passes every
lambda and service builder through `project.Go` before it builds:

```yaml
parameters:
  vorpal:
    extension: github.com/example/platform/internal/vorpal
```

The generated `Vorpal.toml` lists only `cmd/vorpal`, `go.mod`, `go.sum` and
`internal/vorpal` under `[source] includes`, so keep the extension package, and
any repository package it imports, under `internal/vorpal`. This has not been
confirmed with a Vorpal build.

The package must export both hooks:

```go
// Artifacts registers extra Vorpal artifacts for the repository.
func Artifacts(ctx *config.ConfigContext) error

// Go adjusts the builder for the lambda or service called name, and returns
// the builder to build. Return b unchanged to leave it alone.
func Go(ctx *config.ConfigContext, name string, b *language.Go) (*language.Go, error)
```

`config` is `github.com/ALT-F4-LLC/vorpal/sdk/go/pkg/config` and `language` is
`github.com/ALT-F4-LLC/vorpal/sdk/go/pkg/artifact/language`. With the extension
unset (the default), `cmd/vorpal/main.go` has no project import and builds each
artifact in one chained call. Setting `vorpal.extension` does not change
`vorpal.tools`.

If you would rather hand-write `cmd/vorpal/main.go`, list it in `.bcignore`
and the generator leaves it alone:

```
cmd/vorpal/main.go
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
