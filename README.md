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

#### Vorpal SDK Floor

The generated `cmd/vorpal/main.go` calls `config.SYSTEMS`, which the Vorpal Go
SDK (`github.com/ALT-F4-LLC/vorpal/sdk/go`) exports from
`v0.0.0-20260915234803-77ac6ad903ae`. A repository on an older SDK fails to
compile the generated program, so bump the SDK in `go.mod` when adopting
`go-lambda`.

#### OpenAPI Client

`openapi.enable` adds a `build-client` recipe that generates a Go client from
`api/<openapi.filename>` with `openapi-generator-cli`. Three options place and
run it:

```yaml
parameters:
  openapi:
    enable: true
    filename: ai.json
    output: internal/client_ai   # default pkg/client; the recipe deletes it first
    packageName: client_ai       # default client
    generateInCI: true           # default false
```

`output` must be a relative path of letters, digits, `_`, `.`, `/` and `-` with
no `..`, and `packageName` a Go identifier; anything else fails generation.
With `generateInCI`, `just check` depends on `build-client` and the CI `build`
and `build-services` jobs run `just build-client` right after checkout, before
the GitHub App token is written, then `just build`. Use it
when the generated client is not committed. The generator needs a JDK, so add
one to `vorpal.tools`.

#### Lint and Deploy Options

- `lint.disable` lists golangci-lint linters to turn off, such as `errcheck`.
- A `deploy` entry that omits `roleArn` gets
  `arn:aws:iam::<account>:role/altf4llc-gha-<name>-deploy-<environment>`, built
  from the entry's own `environment` and `account`. Earlier versions built it
  from the placeholder environment `unspecified`, which named no real role.
  An entry with no `roleArn` and no `if` therefore now tries a real role on
  every same-repository pull request, so set `if` on every deploy entry. The
  `if` is not a security boundary against a pull request that edits the
  workflow: restrict the role's trust policy to the GitHub environment and
  limit that environment to `main`.
- With `privateModules` set, the `check` job runs `just generate` and
  `just build-client` (when configured) first, writes the GitHub App token to
  `~/.netrc`, runs `just lint`, and removes the token, because lint type-checks
  packages that import a private module. `just lint` runs the linter alone, so
  no generator runs with the token on disk. `just check` still runs the
  generators and the linter locally.
- With `privateModules` set, `cmd/vorpal/main.go` also gives each lambda and
  service build what the Vorpal sandbox needs to fetch the module over HTTPS:
  `GOPRIVATE`, and on Linux targets the git CA bundle
  `/etc/ssl/certs/ca-certificates.crt`. When the build host's `~/.netrc` has a
  `github.com` entry, the program passes its login and password as secrets and
  writes a `.netrc` into the sandbox workspace. The CI jobs write that file
  before `just build`, and a local `vorpal build` reads the developer's own
  `~/.netrc` the same way. Without the file the build runs with the other
  settings only. A config with no lambdas and no services gets none of this.
- `deploy[].if` is joined to the fork guard with `&&`. Use
  `github.event_name == 'push' && github.ref == 'refs/heads/main'` to deploy
  only from pushes to `main`. `deploy[].githubEnvironment` runs the job in that
  GitHub environment.

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
`vorpal.tools`. With `privateModules` set, `project.Go` receives a builder that
already carries the sandbox environment and credential settings. Its
`WithEnvironments` call replaces the environment list and drops them, so avoid
it in the hook. `WithSecrets` and `WithSourceScript` merge safely.

If you would rather hand-write `cmd/vorpal/main.go`, list it in `.bcignore`
and the generator leaves it alone:

```text
cmd/vorpal/main.go
```

### `go-library`

`go-library` scaffolds the Vorpal development environment and CI for a Go
library. A library produces no binary, so `cmd/vorpal/main.go` builds only the
development environment, and CI runs a `check` job and then a `test` job. The
`justfile` has two recipes:

- `check` runs `golangci-lint run ./...`.
- `test` runs `go test -count=1 ./...`, so Go's test cache never reuses a
  result.

The development environment carries `golangci-lint`, `govulncheck`, `gitleaks`
and `rumdl`. Set `vorpal.tools` to replace that list with other Vorpal artifact
aliases:

```yaml
parameters:
  vorpal:
    tools:
      - golangci-lint:2.11.4
      - just:1.45.0
```

The linter configuration is the one every Go template shares. Silence a finding
for one repository with `lint.exclude` rather than editing `.golangci.yaml`.
That shared configuration skips missing doc-comment findings, which also
applies to the exported API of a library.

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
