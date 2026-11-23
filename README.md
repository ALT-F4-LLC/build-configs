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

Every Go template's `justfile` carries the recipes this repository's own
`justfile` defines, by name:

- `doc-validate` runs `rumdl check --no-cache --deny-config-warnings .`.
- `lint` runs `golangci-lint run ./...`.
- `secret-scan` runs `gitleaks` over the history up to `HEAD`.
- `test` runs `go test -count=1 ./...`, so Go's test cache never reuses a
  result.
- `vuln-scan` runs `govulncheck ./...`.
- `build` runs the language's own build tool, and `package` runs Vorpal. In
  `go-cobra-cli`, `build` runs `go build` on the build directory into
  `dist/<name>`, `package` runs `vorpal build --path <name>`, and `install`
  runs `go install` on the build directory. In `go-library`, `build` is a
  `go build ./...` compile check, `package` runs `vorpal build --path` on its
  development environment, and there is no `install`. In
  `go-lambda`, `build` and `package` take a `profile` argument, and there is
  no `install`: `build` runs `go build` on `cmd/<profile>` into
  `dist/<profile>/<profile>`, with `-tags lambda.norpc` for lambdas only,
  `package` builds the Vorpal artifact and zips it as `bootstrap`, and
  `package-all` packages every lambda. There is no native packaging recipe.
  `go-service` is described under [`go-service`](#go-service).
  `go-cobra-cli`, `go-lambda` and `go-service` CI run the Vorpal package
  recipes and never `build`, which stays a local recipe; `go-library` CI runs
  neither, because every CI step already builds the development environment
  that its `package` builds. Add `dist/` to the project's
  `.gitignore`. Vorpal does not read `.gitignore`, so the `go-lambda` Vorpal
  program excludes the top-level `dist/` directory from every lambda and
  service source itself, and files an earlier `build` or `package` wrote there
  leave the artifact digests unchanged. See [Vorpal source](#vorpal-source). With
  `privateModules` set, the `go-cobra-cli` justfile exports `GOPRIVATE` to
  every recipe, and its `cmd/vorpal/main.go` sets `GOPRIVATE` on the
  development environment and gives the CLI build the same sandbox settings
  and `~/.netrc` credential as a `go-lambda` lambda build, described under
  [Lint and Deploy Options](#lint-and-deploy-options), with the same SDK and
  agent requirements. The CI `build` job writes `~/.netrc` before
  `just package`, and the `check` job before `just check`, because lint
  type-checks packages that import a private module; each removes it
  afterward.
- The `terraform` and `terraform-module` justfiles have a `build` recipe that
  runs `terraform init -backend=false` and `terraform validate`, and a
  `package` recipe that runs `vorpal build --path` on the development
  environment. Their development environments carry `gitleaks`, and their
  `secret-scan` recipe matches the Go templates'. Generated CI runs
  `secret-scan` in the `check` job while `gitleaks` is in the development
  environment, and does not run `build`. `terraform` CI runs no `package`
  either, because every CI step already builds the development environment
  that `package` builds.

Every template's development environment carries `jq` 1.8.1 and `just`
1.45.0 by default. The Go defaults add `golangci-lint`, `govulncheck`,
`gitleaks` and `rumdl`; `go-lambda` adds `awscli2` 2.33.1 and `openjdk`
25.0.1, and `openapi-generator-cli` with `openapi.enable`. The `terraform`
defaults add `awscli2`, `gitleaks`, `rumdl` and `terraform` at
`vorpal.terraformVersion`;
`terraform-module` drops `awscli2`. Every template always includes its
defaults. A `go-cobra-cli` or `go-library` configuration that already adds
`jq` or `just`, or a `terraform` one that adds `jq`, now renders both
versions unless it lists the tool in `vorpal.excludeTools`. Likewise, a
`go-lambda` configuration that already adds a JDK other than
`openjdk:25.0.1` now renders both unless it lists `openjdk` in
`vorpal.excludeTools`. `vorpal.tools`
only appends Vorpal artifact aliases to the defaults and never replaces or
removes one.
`vorpal.excludeTools` opts out of defaults by name (`jq`) or full alias
(`jq:1.8.1`) and never drops an alias listed in `vorpal.tools`; the recipes
that use an excluded tool stop working. Names match the literal text before
`:`. To change a default's version, exclude it and add the new alias.
Earlier versions treated `vorpal.tools` as a replacement list, so a
configuration that left defaults out of it now gets them back unless it lists
them in `vorpal.excludeTools`:

```yaml
parameters:
  vorpal:
    tools:
      - protoc:31.1
    excludeTools:
      - govulncheck
```

Every generated workflow runs all of its checks as steps of one `check` job,
in this order: `secret-scan`, `doc-validate`, `check` (`lint` in `go-lambda`
and `go-service` with `privateModules`), then `vuln-scan`. `terraform` and `terraform-module`
run only `secret-scan` and `check`. Each scan step renders only while the
development environment carries its tool (`rumdl`, `gitleaks` or
`govulncheck`) at any version, so excluding one in `vorpal.excludeTools` also
drops its step. With `gitleaks`, the job checks out the full history so it
scans every commit; allowlist a false positive in a `.gitleaks.toml` or
`.gitleaksignore` at the repository root. With `privateModules` set, the job
writes the GitHub App token to `~/.netrc` after the credential-free scans,
before `just check` and `just vuln-scan`, and always removes it afterward.
A failed step stops the steps after it. Earlier versions ran
`doc-validate`, `secret-scan` and `vuln-scan` as separate jobs; a repository
that requires those status checks must require `check` instead.
Generated CI runs `test` only for `go-library`.

The job after `check` in every generated workflow needs `check`, so a failed
check or scan blocks it: `build` in `go-cobra-cli`, `go-lambda` and
`go-service`, `test` in `go-library`, `plan` in `terraform`, and `package` in
`terraform-module`.
Deploy and apply jobs follow those jobs, so they wait on the checks too.

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
`-tags='lambda.norpc'`. CI builds its Vorpal artifact with
`just package-artifact`, which compiles it, and runs no native `just build`:
there is no zip, upload or deploy step, and the `*-all` recipes in the
`justfile` leave it out. Each `artifacts` entry, an extra Vorpal artifact
name, also gets a `just package-artifact` step. CI builds every lambda,
service and artifact in one `build` job, one step each, rather than one
runner per entry: artifacts first, so a source that does not exclude `dist/`
never sees a lambda zip, then services, then lambdas. The job uploads each
lambda zip. Each `deploy-<environment>` job is likewise one job: it
downloads every zip, then runs `just deploy` for each lambda in order, and
queues behind the previous run for that environment. With `privateModules`
set, one GitHub App token, valid for an hour, covers every package step.
Earlier versions ran `build (<lambda>)`, `build-services (<name>)` and
`deploy-<environment> (<lambda>)` matrix jobs; a repository that requires
those status checks must require `build` and `deploy-<environment>` instead.
`just build <artifact>` no longer builds it, because `build` runs `go build`
on `cmd/<name>`.
`vorpal.extension`'s `Go` hook changes only the Vorpal build, so `just build`
does not see it. The template does not scaffold `cmd/<name>/main.go`.
A name listed in both `lambdas` and `services` is an error. `lambdas` defaults
to `[<name>]`, so set `lambdas: []` for a services-only project. That project
builds and uploads no lambda. `deploy` with an empty `lambdas` is an error,
whatever else the project builds, because there is no zip to deploy. A
project with no lambdas, services or artifacts renders no `build` job.

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
job runs `just build-client` right after checkout, before the GitHub App token
is written, then its package steps. With `generate` set, that job also runs
`just generate` right after checkout. Use it when the generated client is not
committed.

To share a client with other repositories, set `openapi.module` in the
repository that owns the spec. The output must sit outside `internal/`, and
the module path must end with `/<output>`:

```yaml
parameters:
  openapi:
    enable: true
    filename: ai.json
    output: client/ai
    module: github.com/ALT-F4-LLC/quirk-lambda-ai/client/ai
    packageName: client_ai
```

`build-client` then writes `<output>/go.mod`, and the client is committed.
`openapi.module` cannot be combined with `generateInCI`, and needs
`openapi.enable`. The CI `check` job runs `just check-client` before the
GitHub App token is written. It fails when `<output>/go.mod` is not committed,
or when regeneration changes the committed client. `Vorpal.toml` includes the
output, so the owner can import its own client with
`replace <module> => ./<output>`. The client's `go` directive is `goVersion`.

A consuming repository imports the module and adds it to `privateModules`,
for example
`privateModules: github.com/ALT-F4-LLC/quirk-service-kit,github.com/ALT-F4-LLC/quirk-lambda-ai/client/ai`.
It then runs `go get <module>@main`. The existing GitHub App token reads the
module in CI, so the App installation must include the owning repository.

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
  `just build-client` (when configured) before it writes the GitHub App token
  to `~/.netrc`, then runs `just lint` and `just vuln-scan` and removes the
  token, because lint type-checks packages that import a private module.
  `just lint` runs the linter alone, so no generator runs with the token on
  disk. `just check` still runs the
  generators and the linter locally.
- With `privateModules` set, `cmd/vorpal/main.go` also gives each lambda and
  service build what the Vorpal sandbox needs to fetch the module over HTTPS:
  `GOPRIVATE`, and on Linux targets the git CA bundle
  `/etc/ssl/certs/ca-certificates.crt`. When the build host's `~/.netrc` has a
  `github.com` entry, the program passes its login and password as the
  secrets `NETRC_USERNAME` and `NETRC_PASSWORD` to the artifact being built,
  and to no other. The build script reads them from
  `$VORPAL_SECRET_DIR/NETRC_USERNAME` and `$VORPAL_SECRET_DIR/NETRC_PASSWORD`
  and writes a `.netrc` into the sandbox workspace; no step names a secret in
  its arguments or environment, which the Vorpal worker refuses. A build that
  attaches the credential needs the Vorpal Go SDK in `go.mod` at or after
  `v0.0.0-20261112004409-56d9bfe3f08b`, and a Vorpal agent and worker at
  0.5.0 or later, which seal step secrets and deliver them as files. An older
  SDK carries secret values as strings and is not expected to read the sealed
  values back. The Vorpal agent encrypts every secret in the config it runs,
  so building the development environment, as the `check` job does, attaches
  none. The CI jobs write `~/.netrc` before the Vorpal `package` recipes, and a
  local
  `vorpal build` reads the developer's own `~/.netrc` the same way. Without
  the file the build runs with the other settings only. A config with no
  lambdas and no services gets none of this.
- `deploy[].if` is joined to the fork guard with `&&`. Use
  `github.event_name == 'push' && github.ref == 'refs/heads/main'` to deploy
  only from pushes to `main`. `deploy[].githubEnvironment` runs the job in that
  GitHub environment.

#### Vorpal Source

`cmd/vorpal/main.go` gives each lambda and service builder its own source
through `WithSource`: the project root without the top-level `dist/`
directory, where `just build` and `just package` write. Every other path
stays in the source by default. `vorpal.includes` narrows every lambda and
service source to the listed paths, relative to the project root, and `dist/`
stays excluded:

```yaml
parameters:
  vorpal:
    includes:
      - cmd
      - internal
      - go.mod
      - go.sum
```

The list applies to every lambda and service alike. Vorpal matches each entry
as a path prefix, so `cmd` covers everything under `cmd/`; write entries
without a leading `./`. The list must cover every path the Go build compiles
or embeds, including the OpenAPI client output (`pkg/client` by default) and
files `generate` writes. `vorpal.includes` does not change the
`[source] includes` list in `Vorpal.toml`, which scopes the Vorpal program
itself.

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
artifact in one chained call. Either way, the program builds lambdas in one
loop over their names and services in another. Setting `vorpal.extension`
does not change `vorpal.tools`. With `privateModules` set, `project.Go`
receives a builder that already carries the sandbox environment and
credential settings. Its `WithEnvironments` call replaces the environment
list and drops them, so avoid it in the hook. `WithSecrets` and
`WithSourceScript` merge safely.

`project.Go` receives a builder that already carries the
[Vorpal source](#vorpal-source), and a builder with a source ignores
`WithIncludes`, so a hook's `WithIncludes` call no longer takes effect. To
change a lambda's or service's source in the hook, call `WithSource` with a
complete source, and exclude `dist` in it to keep the digest stable. The
`dist/` exclusion and `vorpal.includes` apply only to the lambda and service
builders; artifacts that `project.Artifacts` registers keep whatever source
they declare.

If you would rather hand-write `cmd/vorpal/main.go`, list it in `.bcignore`
and the generator leaves it alone:

```text
cmd/vorpal/main.go
```

### `go-service`

`go-service` scaffolds a Go service repository: long-running binaries built
from `cmd/<name>` by Vorpal, optionally packed into one OCI image that CI
pushes to a container registry from `main`. It follows `go-lambda`'s
conventions for the fork guard, permissions, scans and the private module
credential, and takes the same `lint`, `privateModules`, `generate`,
`artifacts` and `vorpal` options (`extension`, `script`, `tools`,
`excludeTools` and `includes`). The project name, services, artifacts and
client names must be lowercase letters, digits, `_` and `-`, and
`privateModules` a comma-separated list of module path globs. The template
does not scaffold `cmd/<name>/main.go`. See
[`examples/go-service`](./examples/go-service/build-configs.yaml).

- `services` lists the binaries, and defaults to `[<name>]`. `build <service>`
  runs `go build` into `dist/<service>/<service>`, `install <service>` runs
  `go install`, and `package <name>` runs `vorpal build --path <name>` on a
  service or an `artifacts` entry. Every service builds with `CGO_ENABLED=0`
  and `-ldflags='-s -w'`.
- `clients` lists Vorpal artifacts that the source needs but the repository
  does not commit, such as generated API clients. Each entry has a `name`, an
  `alias` of the form `<name>:<version>`, and an `output` directory relative
  to the repository root. The Vorpal program wraps each alias in an artifact
  called `<name>-client-<client>` and copies it into `output` inside every
  service build. Each service source excludes the `output` directories, so a
  local copy never changes the artifact digest. `just clients` deletes each
  `output` and copies the client into it, so an `output` may not overlap
  another, `cmd`, `dist`, `.github`, or a top-level build file; `check`
  depends on `clients`. CI runs `just clients` before it writes the GitHub
  App token. That needs no cross-repository checkout token only when each
  alias is already published where the runner's Vorpal registry resolves it;
  a client built from a sibling repository in the same run needs its own
  step. Add each `output` to the project's `.gitignore`. A
  `vorpal.extension` `Go` hook must not call `WithArtifacts`: it replaces the
  client artifacts while their copy scripts remain, and the build then fails.
- `image.repository` turns on the image, such as `quirk/backend-discord-bot`,
  without the registry host. The Vorpal program builds `<name>-image` for
  Linux targets only, from the `image.rootfs` alias (default
  `linux-vorpal:latest`, which floats) with every service on it.
  `just package-image` builds it and copies `image.tar` to
  `dist/<name>-image/`, and `just push <registry> [tag]` loads that tarball,
  pushes `<registry>/<repository>:<tag>` with `tag` defaulting to `HEAD`'s
  commit, and removes both local tags on exit. Log in to the registry first.

The CI `check` job runs `secret-scan` and `doc-validate`, then, with
`privateModules`, `clients` and `generate`, writes the GitHub App token to
`~/.netrc`, runs `lint` and `vuln-scan`, and always removes `~/.netrc`.
Without `privateModules` it runs `just check` and `vuln-scan`. The `build` job
needs `check`, runs `clients` when `generate` is set and then `generate`,
writes `~/.netrc`, runs `just package` for every artifact and service and
`just package-image`, then always removes `~/.netrc`. Building the image under
the image artifact gives every service the private module credential, as
building a service alone does. The Vorpal agent seals each secret afresh per
invocation and the sealed value enters the artifact digest, so with
`privateModules` and `image.repository` both set, `just package-image`
rebuilds every service rather than reusing the `just package` result; this
follows from the Vorpal source and has not been observed in a CI run.

With `image.repository`, the `build` job then logs in with
`docker/login-action`, pushes `${{ github.sha }}` with `just push`, and runs
`docker logout` on every outcome. Those three steps run only when
`github.event_name == 'push' && github.ref == 'refs/heads/main'`, so a pull
request builds the image but never logs in or pushes. They run after
`~/.netrc` is removed, so the App token and the registry credential are never
on the runner together, and they set `DOCKER_CONFIG` to
`${{ runner.temp }}/docker`, so the credential never reaches the runner's
shared `~/.docker/config.json`; a credential helper on the runner's `PATH`
would still receive it. The `build` job carries a `concurrency` group per ref
without `cancel-in-progress`, so two pushes to `main` never tag and push
through the runner's Docker daemon at once. GitHub keeps one pending run per
group, so when three pushes land in quick succession the middle one is
cancelled and its commit gets no image tag. Unlike `go-lambda`'s deploy
jobs, the push runs in `build` with no GitHub `environment`, so a
same-repository pull request that edits the workflow is not stopped by an
environment rule. The login reads the password from
`secrets.ALTF4LLC_DOCKER_REGISTRY_PASSWORD` and the registry and username from
`vars.ALTF4LLC_DOCKER_REGISTRY_URL` and `vars.ALTF4LLC_DOCKER_REGISTRY_USERNAME`;
no `run:` line carries a GitHub expression. Every job carries the fork guard
and `permissions: contents: read`.

The generated `cmd/vorpal/main.go` needs the Vorpal Go SDK at or after
`v0.0.0-20260915234803-77ac6ad903ae` to compile, and at or after
`v0.0.0-20261112004409-56d9bfe3f08b` with a Vorpal agent and worker at 0.5.0
or later for a build that attaches the private module credential, as
`go-lambda` does.

### `go-library`

`go-library` scaffolds the Vorpal development environment and CI for a Go
library. A library produces no binary, so `cmd/vorpal/main.go` builds only the
development environment, and CI runs a `check` job and then a `test` job. The
`justfile` carries the shared Go recipes except `install`, `package` builds the
development environment with Vorpal, and the bodies of `build` and `check` are
these:

- `build` runs `go build ./...` as a compile check.
- `check` runs `golangci-lint run ./...`.

The development environment carries `golangci-lint`, `govulncheck`, `gitleaks`,
`jq`, `just` and `rumdl`. `vorpal.tools` appends Vorpal artifact aliases to
that list, and `vorpal.excludeTools` drops a default. Together they pin a
default to another version:

```yaml
parameters:
  vorpal:
    tools:
      - golangci-lint:2.11.4 # added
      - protoc:31.1          # added
    excludeTools:
      - golangci-lint        # drops golangci-lint:2.15.0
      - rumdl
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
