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

## Onboarding Terraform repositories

Add a `build-configs.yaml` at the repository root, then run
`build-configs generate` from that repository. For example:

```yaml
name: terraform-aws-gha-iam
template: terraform
parameters:
  github:
    runsOn: altf4llc
    appToken: true
```

The templates do not generate `Vorpal.lock`. Copy it from a repository using
the same Vorpal SDK version (currently `terraform-github` for SDK 0.6.0), commit
it, and let CI run. Do not generate `Vorpal.lock` locally.

## Development

The Vorpal dev shell provides Go and golangci-lint. Enter it with direnv, then
lint and build:

```shell
direnv allow
just lint
just build
```

## Templates

The Vorpal config program is Go in the four `go-*` templates, which are tied to
the Go language. Every other template, `terraform`, `terraform-module` and
`typescript-frontend`, writes it in TypeScript on `@altf4llc/vorpal-sdk`, with
`language = "typescript"` in `Vorpal.toml`, and needs no Go toolchain. A new
template that is not tied to a programming language follows the TypeScript
rule; the template lists in `internal/config/vorpal_language_test.go` enforce
it.

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
- The Vorpal config program of `terraform` and `terraform-module` is
  TypeScript: `src/vorpal.ts` with `language = "typescript"` in
  `Vorpal.toml`, as in [`typescript-frontend`](#typescript-frontend). Vorpal
  compiles it with `bun build --compile` from `bun.lock`, `package.json`,
  `tsconfig.json` and `src/vorpal.ts`, so the repository needs no Go and no
  `go.mod`. `build-configs generate` also creates `tsconfig.json`, and
  `package.json` with `bun.lock`, when they are missing, and never replaces
  one that exists, so Renovate, bun and you own them afterward. `package.json`
  and `bun.lock` are written as a pair: with either one present or listed in
  `.bcignore`, or with a `bun.lockb`, `package-lock.json`, `yarn.lock` or
  `pnpm-lock.yaml` present, neither is written, and you add
  `@altf4llc/vorpal-sdk` 0.6.0 yourself and run `bun install`. The pair pins
  `@altf4llc/vorpal-sdk` 0.6.0, `typescript` 7.0.2 and `@types/bun` 1.4.4,
  named for the repository. The lock is the `bun.lock` the Vorpal CLI ships
  at its 0.6.0 tag in its TypeScript template, with every package pinned by
  integrity hash; `bun install --frozen-lockfile` accepts it and a plain
  `bun install` leaves it unchanged on bun 1.4.4. Then pin the sources in
  `Vorpal.lock` for every platform that builds, such as
  `vorpal prepare <name>-dev` and
  `vorpal prepare --system x86_64-linux <name>-dev`; the program declares only
  the development environment. `.gitignore` ignores `node_modules`. There
  is no `vorpal.script`, which the Vorpal CLI reads only for a Go config
  program. A repository that generated an earlier Go version keeps its
  `cmd/vorpal/main.go`, `go.mod` and `go.sum`; delete them. Both justfiles
  also render a `docs` recipe that runs `terraform-docs`. No Vorpal artifact
  provides that tool, so no development environment carries it, and the recipe
  works only where `terraform-docs` is already on the host `PATH`.
- The `typescript-frontend` justfile shares `doc-validate` and `secret-scan`
  and runs bun for the rest; see
  [`typescript-frontend`](#typescript-frontend).

Every template's development environment carries `jq` 1.8.1 and `just`
1.45.0 by default. The Go defaults add `golangci-lint`, `govulncheck`,
`gitleaks` and `rumdl`; `go-lambda` adds `awscli2` 2.33.1 and `openjdk`
25.0.1, and `openapi-generator-cli` with `openapi.enable`. The `terraform`
defaults add `awscli2`, `gitleaks`, `rumdl` and `terraform` at
`vorpal.terraformVersion`;
`terraform-module` drops `awscli2`. The `typescript-frontend` defaults add
`gitleaks` and `rumdl`. The development environments of `typescript-frontend`,
`terraform` and `terraform-module` always carry the Vorpal SDK's `bun` 1.4.4,
which `vorpal.excludeTools` cannot drop. Every
template always includes its
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
drops its step. `typescript-frontend`'s `vuln-scan` runs `bun audit`, so it
renders whatever `vorpal.excludeTools` holds.
With `gitleaks`, the job checks out the full history so it
scans every commit; allowlist a false positive in a `.gitleaks.toml` or
`.gitleaksignore` at the repository root. With `privateModules` set, the job
writes the GitHub App token to `~/.netrc` after the credential-free scans,
before `just check` and `just vuln-scan`, and always removes it afterward.
A failed step stops the steps after it. Earlier versions ran
`doc-validate`, `secret-scan` and `vuln-scan` as separate jobs; a repository
that requires those status checks must require `check` instead.
Generated CI runs `test` only for `go-library`, and for
`typescript-frontend` with `scripts.test` set, as the last `check` step.

The job after `check` in every generated workflow needs `check`, so a failed
check or scan blocks it: `build` in `go-cobra-cli`, `go-lambda`,
`go-service` and `typescript-frontend`, `test` in `go-library`, `plan` in
`terraform`, and `package` in
`terraform-module`.
Deploy and apply jobs follow those jobs, so they wait on the checks too.

### Repository Visibility and Runners

Every template picks its CI runner from the top-level `visibility` key in
`build-configs.yaml`, beside `name` and `template`:

```yaml
---
name: example
template: go-library
visibility: public
```

- `private`, the default when the key is absent, runs every job on the
  `altf4llc` runner group. That runner carries Vorpal, so a private render has
  no setup step.
- `public` runs every job on `ubuntu-latest`. A GitHub-hosted runner has no
  Vorpal, so each job starts with `ALT-F4-LLC/setup-vorpal-action`, pinned to
  Vorpal 0.6.0, before any step that runs through the Vorpal shell. The step
  selects the `altf4llc-vorpal-registry` S3 registry, because the local
  registry a fresh runner starts holds none of the tool aliases the
  development environment fetches. A registry needs AWS credentials, which
  come from GitHub OIDC and not from stored keys: ahead of the setup step,
  `aws-actions/configure-aws-credentials` assumes
  `arn:aws:iam::677459762413:role/altf4llc-gha-<name>-vorpal` in `us-west-2`,
  and the setup action passes the session it exports to the Vorpal services.
  Each job therefore carries `id-token: write`. The repository needs no AWS
  secret or variable, but the role must exist in terraform-aws-gha-iam: trusted
  by this repository's OIDC subjects (`ref:refs/heads/main`, `pull_request`,
  and any `environment:<name>` a job uses) and allowed `s3:GetObject` and
  `s3:PutObject` on `altf4llc-vorpal-registry/*`; Vorpal's worker pushes every
  artifact it builds, and a failed push fails the build. The Vorpal services keep
  the session for the whole job, so a job longer than the role's maximum
  session (one hour by default) loses registry access. Every job builds the
  development environment cold, which takes longer than on the altf4llc
  runner. A public render is tested here by its generated workflow only, not on
  a GitHub-hosted runner, and no run can pass until the role exists.
- Any other value is an error, and so is `visibility` under `parameters`. The
  `terraform` template also rejects `public`, because its plan artifact holds
  cleartext variable and state values.
- Build jobs take the architecture of the runner, so a deploying repository
  moved to a hosted runner builds for the architecture that runner has.
- Pull requests from forks still skip every job, because they carry neither
  secrets nor an OIDC token. A public repository keeps that guard.

### `terraform`

`terraform` also reads these options, under `parameters.github`:

- `runsOn` overrides the runner that `visibility` picks, for every job. A
  runner other than `altf4llc` gets the Vorpal setup step, whichever option
  chose it.
- `appToken: true` adds an `actions/create-github-app-token` step to the
  plan and apply jobs, after the role is assumed and before checkout. It
  reads the `ALTF4LLC_GITHUB_APP_CLIENT_ID` variable and the
  `ALTF4LLC_GITHUB_APP_PRIVATE_KEY` secret. Only the `just apply` step
  receives the token, as `GITHUB_TOKEN`: the plan job generates it but passes
  it to no step, because a plan runs the pull request's own code. The go-cobra-cli,
  go-lambda and go-service workflows read the same variable and secret.
- `env` sets environment variables on the plan and apply jobs. Values may be
  GitHub expressions such as `${{ secrets.TF_VAR_PROVIDER_TOKEN }}`.

`parameters.role.planArn` and `parameters.role.applyArn` override the OIDC
roles; otherwise they default to `altf4llc-gha-<name>-plan` and
`altf4llc-gha-<name>-apply`. `parameters.role.region` selects the AWS region
for both role assumptions and defaults to `us-west-2`.
`parameters.vorpal.tools` adds artifact aliases to the default development
tools, such as `terraform-docs:0.20.0`.

The plan job takes a concurrency group per pull request, `tf-plan-<number>`,
and runs `just plan -lock=false` on pull requests, so a pull request's plan
never waits for another pull request's plan or for an apply. Pushes and manual
runs share `tf-lock` with every apply and keep the state lock. The justfile
`plan` recipe forwards its arguments to `terraform plan`.

On pull requests the plan job also renders a summary table of the resource
changes with `jq`, and posts it with the full plan as one comment, which later
runs update in place. The job holds `pull-requests: write`; the other jobs do
not. The comment stays under the 65,536-character comment limit by cutting the
summary and then the plan, and the full plan stays in the workflow log. The
summary step needs `jq` in the Vorpal shell, so a config that excludes the `jq`
tool breaks pull request plans.

The plan job uploads the binary plan as the `tf-plan` artifact. It holds
variable and state values in cleartext, sensitive ones included, so the
template rejects `visibility: public`. The check reads the config, not
the repository, so keep a repository that uses this template private.

The `github.appToken` step in the plan job only generates the token. No plan
step receives it, so a plan that needs the token for a provider or a private
module source runs unauthenticated.

### `terraform-module`

`terraform-module` uses the visibility-selected runner. Its `check` job runs
`just check`; the dependent `build` job runs `just build`, which initializes
without a backend and validates the module.

### `go-cobra-cli`

`go-cobra-cli` scaffolds a Cobra CLI tool with Go that can be used to build out
internal tooling. This is the template this CLI uses.

### `go-lambda`

`go-lambda` scaffolds an lambda setup in Go.

To use it, you need to run the generator and then run `go mod init` and
`go mod tidy`.

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
directory, where `just build` and `just package` write, and without the
top-level `.env`, `.direnv` and `.claude`. Vorpal ignores `.gitignore` and
uploads the source to the registry. Excludes match whole path components from
the project root, not globs, so `.env.local` and a nested `cmd/api/.env`
still go in. Every other path stays in the source by
default. `vorpal.includes` narrows every lambda and service source to the
listed paths, relative to the project root, and the excludes still apply:

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
complete source, and exclude `dist` in it to keep the digest stable; that
source also replaces the `.env`, `.direnv` and `.claude` excludes and
`go-service`'s default allowlist, so carry them over. The
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
`excludeTools` and `includes`). The project name, services and artifacts
must be lowercase letters, digits, `_` and `-`, and
`privateModules` a comma-separated list of module path globs. The template
does not scaffold `cmd/<name>/main.go`. See
[`examples/go-service`](./examples/go-service/build-configs.yaml).

- `services` lists the binaries, and defaults to `[<name>]`. `build <service>`
  runs `go build` into `dist/<service>/<service>`, `install <service>` runs
  `go install`, and `package <name>` runs `vorpal build --path <name>` on a
  service or an `artifacts` entry. Every service builds with `CGO_ENABLED=0`
  and `-ldflags='-s -w'`.
- `vorpal.includes` defaults to `cmd`, `internal`, `pkg`, `go.mod` and
  `go.sum`, unlike `go-lambda`'s whole-root default: Vorpal ignores
  `.gitignore` and uploads the source to the registry, so an allowlist keeps
  a top-level `.env` out; one inside an allowed directory still goes in. Set
  it to the full list when the build compiles or embeds other paths. The
  default leaves out a committed `vendor/` and a root `go.work`, so the build
  downloads modules instead. Entries must be relative paths with no leading
  `./`; an empty entry would match every path.
- `privateModules` is also how a service gets API clients and other shared
  code: import them with `go get` as Go modules and list their repositories,
  such as
  `github.com/ALT-F4-LLC/quirk-service-kit,github.com/ALT-F4-LLC/quirk-lambda-ai`.
  Every build reads them through `GOPRIVATE` and the `~/.netrc` credential.
  Setting the list replaces the `quirk-service-kit` default, so keep it there
  when the service still imports it.
- `image.repository` turns on the image, such as `quirk/backend-discord-bot`,
  without the registry host. The Vorpal program builds `<name>-image` for
  Linux targets only, from the `image.rootfs` alias (default
  `linux-vorpal:latest`, which floats) with every service on it.
  `just package-image` builds it and copies `image.tar` to
  `dist/<name>-image/`, and `just push <registry> [tag]` loads that tarball,
  pushes `<registry>/<repository>:<tag>` with `tag` defaulting to `HEAD`'s
  commit, and removes both local tags on exit. Log in to the registry first;
  CI logs in to ECR (below).

The CI `check` job runs `secret-scan` and `doc-validate`, then, with
`privateModules`, runs `generate`, writes the GitHub App token to `~/.netrc`,
runs `lint` and `vuln-scan`, and always removes `~/.netrc`. Without
`privateModules` it runs `just check` and `vuln-scan`. The `build` job needs
`check`, runs `generate`, writes `~/.netrc`, runs `just package` for every
artifact and service and
`just package-image`, then always removes `~/.netrc`. Building the image under
the image artifact gives every service the private module credential, as
building a service alone does. The Vorpal agent seals each secret afresh per
invocation and the sealed value enters the artifact digest, so with
`privateModules` and `image.repository` both set, `just package-image`
rebuilds every service rather than reusing the `just package` result; this
follows from the Vorpal source and has not been observed in a CI run.

With `image.repository`, the image goes to Amazon ECR through GitHub OIDC,
with no stored registry credential. On a push to `main` the `build` job
uploads `image.tar` as the `image` artifact after `~/.netrc` is removed. A
separate `push` job then runs only when
`github.event_name == 'push' && github.ref == 'refs/heads/main'`, so a pull
request builds the image but never pushes, and on the altf4llc runner never
requests an AWS credential. The `push` job needs `build` and is the only job
with `id-token: write`, except that a `public` render adds it to every job for
the Vorpal registry role (see Repository Visibility and Runners). It holds
no GitHub App token and runs no generator or service build; its only Vorpal
build is the development environment behind `.github/vorpal-shell.sh`, which
runs the repository's `cmd/vorpal` program. It then:

1. downloads the tarball;
2. builds the development environment with `just --version`, so the role
   session covers only the login and the push;
3. assumes `image.roleArn` in `image.region` for 900 seconds with
   `aws-actions/configure-aws-credentials`;
4. logs in to `<image.account>.dkr.ecr.<image.region>.amazonaws.com` with
   `docker/login-action`, which fetches the ECR token with those credentials
   and passes it on stdin;
5. pushes `${{ github.sha }}` with `just push`;
6. runs `docker logout` on every outcome.

The Docker steps set `DOCKER_CONFIG` to `${{ runner.temp }}/docker`, so the
credential never reaches the runner's shared `~/.docker/config.json`; a
credential helper on the runner's `PATH` would still receive it. The `push`
job carries a `concurrency` group per ref with `queue: max` and without
`cancel-in-progress`, so pushes to `main` run one at a time through the
runner's Docker daemon and every commit gets its image tag. No `run:` line
carries a GitHub expression. Every job carries the fork guard, and `check` and
`build` have `permissions: contents: read` only.

- `image.roleArn` is required with `image.repository`, and set without it is
  an error; there is no default role name. Only the `aws` and `aws-us-gov`
  partitions are accepted.
- `image.account` defaults to `677459762413` and `image.region` to
  `us-west-2`. Quote `image.account` in YAML so it stays a string. The
  account is not read from `image.roleArn`: a role in another account pushes
  to `image.account`, which then needs a repository policy granting it.
- `image.githubEnvironment` runs the `push` job in that GitHub environment.
- The job `if` is not a security boundary against a same-repository pull
  request that edits the workflow; the role's trust policy is. It must use
  `StringEquals` on `token.actions.githubusercontent.com:aud` =
  `sts.amazonaws.com` and on `token.actions.githubusercontent.com:sub` =
  `repo:<owner>/<repo>:ref:refs/heads/main`. With `image.githubEnvironment`
  the subject is `repo:<owner>/<repo>:environment:<name>` instead, so a policy
  pinned to the ref stops matching. Give that environment a deployment-branch
  rule limiting it to `main`: without one, a workflow on any branch that
  names the environment gets the same subject. An organization or repository
  with a customized OIDC subject template issues a different `sub`.
- The role needs `ecr:GetAuthorizationToken` on `Resource: "*"`, plus
  `ecr:BatchCheckLayerAvailability`, `ecr:InitiateLayerUpload`,
  `ecr:UploadLayerPart`, `ecr:CompleteLayerUpload` and `ecr:PutImage` on the
  one repository. The repository must already exist as `image.repository` in
  `image.account` and `image.region`; the template creates neither it nor the
  role.

The generated `cmd/vorpal/main.go` needs the Vorpal Go SDK at or after
`v0.0.0-20260915234803-77ac6ad903ae` to compile, and at or after
`v0.0.0-20261112004409-56d9bfe3f08b` with a Vorpal agent and worker at 0.5.0
or later for a build that attaches the private module credential, as
`go-lambda` does.

### `typescript-frontend`

`typescript-frontend` scaffolds a static TypeScript frontend built with bun,
such as a Vite single-page app. A `package.json` build script writes static
files, and Vorpal packages that output directory as one artifact called
`<name>`. It follows `go-service`'s conventions for the fork guard,
permissions and scans, and has no deploy or publish job. The name must be
lowercase letters, digits, `_` and `-`. See
[`examples/typescript-frontend`](./examples/typescript-frontend/build-configs.yaml).

Like `terraform` and `terraform-module`, its Vorpal config program is TypeScript:
`src/vorpal.ts`, with `language = "typescript"` in `Vorpal.toml`. Vorpal
compiles it with `bun build --compile` from `bun.lock`, `package.json`,
`tsconfig.json` and `src/vorpal.ts`, so the repository needs no Go and no
`go.mod`. The template does not write `package.json`: add
`@altf4llc/vorpal-sdk` as a devDependency yourself. The program is written
against 0.6.0; other versions are untested. 0.6.0 does not export the SDK's
`Bun` builder, so the program defines bun 1.4.4 with the same inputs. An SDK
upgrade that changes the SDK's bun leaves the build step on 1.4.4 and the
development environment on the new version. Compiling the program runs
`bun install --frozen-lockfile` over the whole `package.json` from the public
npm registry, so any change to `package.json`, `bun.lock` or `tsconfig.json`
recompiles it before the next dev shell or CI job. After the first
`build-configs generate`, pin the sources in `Vorpal.lock` for every platform
that builds, such as `vorpal prepare <name>` and
`vorpal prepare --system x86_64-linux <name>`, and the same for `<name>-dev`.
A repository that generated an earlier Go version keeps its
`cmd/vorpal/main.go`; delete it.

`src/vorpal.ts` sits under `src`, so the project's own `scripts.lint` and
`scripts.typecheck` check it when their globs cover `src`. A strict lint
setup, or a typecheck without `skipLibCheck`, can reject it, and the file
cannot be edited; list it in `.bcignore` to own it instead. It stays out of
the frontend artifact's source, so editing it never rebuilds the frontend.

- `scripts.build` (default `build`), `scripts.lint` (default `lint`),
  `scripts.typecheck` (default `typecheck`) and `scripts.test` (default
  unset) name `package.json` scripts. A name holds letters, digits, `:`,
  `.`, `_` and `-`. With `scripts.test` unset, no `test` recipe or CI step
  renders.
- `output` (default `dist`) is the directory the build script writes,
  relative to the root.
- `generate` lists shell lines for a `generate` recipe. `check` depends on
  it, and the CI `build` job runs it before `just package`, so a generated,
  gitignored source such as an API client reaches the Vorpal source. A tool a
  line needs goes in `vorpal.tools`; the example adds `openapi-generator-cli`
  and `openjdk`.
- `audit.level` (`low`, `moderate`, `high` or `critical`) passes
  `--audit-level`, and `audit.production: true` passes `--production`, to
  `bun audit`. Unset, any advisory in any locked package fails `vuln-scan`.
- `vorpal.tools` and `vorpal.excludeTools` work as in the other templates.
  There is no `vorpal.extension`, and no `vorpal.script`, which the Vorpal
  CLI reads only for a Go config program.
- `vorpal.includes` defaults to `bun.lock`, `bunfig.toml`, `index.html`,
  `package.json`, `public`, `src`, `tsconfig.json`, `tsconfig.app.json`,
  `tsconfig.node.json`, and `postcss.config`, `tailwind.config` and
  `vite.config` in their `.js`, `.cjs`, `.mjs` and `.ts` forms (`vite.config`
  also `.mts`). An `.npmrc` stays out, so a private registry configured there
  does not reach the build. Vorpal keeps a path only when an include equals
  it or one of its leading directories: `src` keeps `src/main.tsx`, but
  `tsconfig` keeps no `tsconfig.json`. List every file by its exact name; an
  entry that matches nothing keeps nothing. Setting the list replaces the
  default: list what the build reads, including Vite's `envDir`, such as
  `env`. A `.env` file inside an included directory, such as
  `env/.env.local`, is uploaded to the registry. The `output` directory,
  `node_modules` and `src/vorpal.ts` are excluded either way.

The `justfile` recipes:

- `install` runs `bun install --frozen-lockfile`. `build`, `check`,
  `generate`, `test` and `vuln-scan` depend on it.
- `check` runs `scripts.lint`, then `scripts.typecheck`.
- `build` runs `scripts.build` natively into `output`. CI never runs it.
- `package [name]` runs `vorpal build --path <name>`, defaulting to the
  frontend artifact.
- `test` runs `scripts.test`, and `vuln-scan` runs `bun audit` with the
  `audit` flags. `doc-validate` and `secret-scan` match the Go templates'.

Every script runs through `bun --bun run`, so package binaries such as `tsc`
and `vite` run under bun even where `node` is on `PATH`. Local runs then
match the Linux Vorpal sandbox, which has no `node`. No frontend has been
built in a Vorpal sandbox with this template yet. A script that calls `npm`
or `node` itself, such as a `build` that chains `npm run build:tsc`, depends
on bun substituting for them, which is untested.

The Vorpal artifact installs from `bun.lock` with `--frozen-lockfile`, runs
`bun --bun run <scripts.build>`, fails when `output` is missing, and copies
`output` into the artifact. `bun install` fetches from the public npm
registry inside the build. The template configures no private registry and
no credential. Vite builds in its default mode, `production`, unless
`scripts.build` names a script that passes `--mode`. The CI artifact checks
that the frontend builds; nothing deploys it.

The CI `check` job runs `secret-scan`, `doc-validate`, `check`, `vuln-scan`
and, with `scripts.test`, `test`. The `build` job needs `check`, runs
`generate`, then `just package <name>`. Both jobs carry the fork guard and
`permissions: contents: read` only. On the altf4llc runner no job requests an
OIDC token, reads a secret, or deploys; a `public` render adds
`id-token: write` for the Vorpal registry role.

There is no deploy job because quirk-frontend-portal, the repository this
template was shaped on, deploys by hand from a workstation (`aws s3 sync` and
a CloudFront invalidation) with the workstation's ambient AWS credentials. It
has no CI, no GitHub OIDC role and no stored secret. A deploy job from CI
would first need a GitHub OIDC role for that bucket and distribution.
Generation overwrites the `justfile`, so keep project recipes such as a manual
deploy in another file, or list `justfile` in `.bcignore`.

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
