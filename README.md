<h1 align="center">Nuon Extension: cf-stack</h1>

<p align="center">
  <a href="https://github.com/nuonco/nuon-ext-cf-stack/releases"><img src="https://img.shields.io/github/v/release/nuonco/nuon-ext-cf-stack?display_name=tag&amp;sort=semver" alt="Release"></a>
  <a href="https://go.dev/doc/devel/release"><img src="https://img.shields.io/badge/Go-1.25.0-00ADD8?logo=go&amp;logoColor=white" alt="Go Version"></a>
  <a href="https://pkg.go.dev/github.com/nuonco/nuon-ext-cf-stack"><img src="https://img.shields.io/badge/module-github.com%2Fnuonco%2Fnuon--ext--cf--stack-2C6BED" alt="Go Module"></a>
</p>

<p align="center">
  <a href="https://docs.nuon.co/guides">Nuon Docs</a>
  |
  <a href="https://docs.nuon.co/guides/cli-extensions">Nuon Extension Docs</a>
</p>

Nuon extension to install and upgrade CF stacks.

## Usage

```bash
nuon cf-stack install --install-id inl_123 --inputs inputs.json
nuon cf-stack upgrade --install-id inl_123 --inputs inputs.json

# show live apply progress (spinner in TTY mode)
nuon cf-stack install --watch --install-id inl_123 --inputs inputs.json

# install create: provide required secret-backed parameters
nuon cf-stack install --install-id inl_123 --inputs inputs.json --secrets secrets.json

# stack update (`upgrade`, or `install` against an existing stack):
# provide changed secrets only; omitted or empty template secret values keep existing values
nuon cf-stack upgrade --install-id inl_123 --inputs inputs.json --secrets rotated-secrets.json
```

## Run Locally

Use the helper script to run the extension with environment loaded from your Nuon config:

```bash
./scripts/run-local.sh install --install-id inl_123 --inputs inputs.json
./scripts/run-local.sh upgrade --install-id inl_123 --inputs inputs.json

# optional when needed
./scripts/run-local.sh install --install-id inl_123 --inputs inputs.json --secrets secrets.json

# optional on upgrade when rotating secret values
./scripts/run-local.sh upgrade --install-id inl_123 --inputs inputs.json --secrets rotated-secrets.json
```

By default, the script reads `~/.nuon`. To use a different config file:

```bash
NUON_CONFIG_FILE=~/.nuon-staging ./scripts/run-local.sh install --install-id inl_123 --inputs inputs.json
```

### Role flags

All roles are enabled by default. Disable individual roles with:

- `--disable-maintenance`
- `--disable-provision`
- `--disable-deprovision`

`--install-id` falls back to `NUON_INSTALL_ID` if omitted.

### AWS profile

Use `--profile` to select an AWS shared config profile explicitly:

```bash
nuon cf-stack install --install-id inl_123 --inputs inputs.json --secrets secrets.json --profile prod
```

If omitted, the extension uses default AWS credential/provider resolution.

### Watch Mode

Use `--watch` to show a live spinner while CloudFormation applies stack changes.

`install` and `upgrade` always print a stdout line when the CloudFormation apply begins.

In non-interactive environments (`NUON_NO_TTY=true`, `NUON_NOTTY=true`, or `CI`), `--watch` runs without the spinner and
keeps plain text progress output.

### What the extension does

1. Uses Nuon SDK (`GetInstall` + `GetInstallStack`) to load install stack metadata.
2. Resolves template URL and stack name from the install stack version.
3. Resolves region/account from install stack outputs.
4. Verifies caller AWS account matches install stack account (when available).
5. Applies CloudFormation stack (create or update) with parameters from:
   - `inputs.json` mapped to stack parameter names (for example `foo -> ParameterFoo`)
   - `secrets.json` (create: include required secrets; updates: include only changed secrets)
   - role toggle params (`EnableRunnerMaintenance`, `EnableRunnerProvision`, `EnableRunnerDeprovision`)

Input keys are only sent when they match an actual template parameter name (directly or via `Parameter<PascalCase>`
mapping). Unmatched inputs are omitted.

For stack updates (`upgrade`, or `install` against an existing stack), template `NoEcho` parameters omitted from
`--secrets` are sent with `UsePreviousValue=true` so CloudFormation keeps existing stack values.

When a template `NoEcho` secret key is provided with an empty string on stack updates, it is treated as
"keep existing value" (also sent as `UsePreviousValue=true`).

### Debug logging

Set `NUON_DEBUG=true` to print additional logs to stderr.

For `install` and `upgrade`, debug logs include:

- install id and install name
- install stack id and stack status
- secret parameter handling for provided values and missing prior values (`updated from provided ...` and `no provided value ...`)

The extension always prints non-debug `[info]` stderr lines for omitted input keys and secret keys that keep existing
stack values.
