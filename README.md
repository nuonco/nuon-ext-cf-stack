<h1 align="center">Nuon Extension: cf-stack</h1>

<p align="center">
  <a href="https://github.com/nuonco/nuon-ext-cf-stack/releases"><img src="https://img.shields.io/github/v/release/nuonco/nuon-ext-cf-stack?display_name=tag&amp;sort=semver" alt="Release"></a>
  <a href="https://go.dev/doc/devel/release"><img src="https://img.shields.io/badge/Go-1.25.0-00ADD8?logo=go&amp;logoColor=white" alt="Go Version"></a>
  <a href="https://pkg.go.dev/github.com/nuonco/nuon-ext-cf-stack"><img src="https://img.shields.io/badge/module-github.com%2Fnuonco%2Fnuon--ext--api-2C6BED" alt="Go Module"></a>
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

# optional when your template uses secret-backed parameters
nuon cf-stack install --install-id inl_123 --inputs inputs.json --secrets secrets.json
```

## Run Locally

Use the helper script to run the extension with environment loaded from your Nuon config:

```bash
./scripts/run-local.sh install --install-id inl_123 --inputs inputs.json
./scripts/run-local.sh upgrade --install-id inl_123 --inputs inputs.json

# optional when needed
./scripts/run-local.sh install --install-id inl_123 --inputs inputs.json --secrets secrets.json
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

In non-interactive environments (`NUON_NO_TTY=true`, `NUON_NOTTY=true`, or `CI`), `--watch` automatically falls back to plain text progress output.

### What the extension does

1. Uses Nuon SDK (`GetInstall` + `GetInstallStack`) to load install stack metadata.
2. Resolves template URL and stack name from the install stack version.
3. Resolves region/account from install stack outputs.
4. Verifies caller AWS account matches install stack account (when available).
5. Applies CloudFormation stack (create or update) with parameters from:
   - `inputs.json` mapped to stack parameter names (for example `foo -> ParameterFoo`)
   - optional `secrets.json`
   - role toggle params (`EnableRunnerMaintenance`, `EnableRunnerProvision`, `EnableRunnerDeprovision`)

Input keys are only sent when they match an actual template parameter name (directly or via `Parameter<PascalCase>` mapping). Unmatched inputs are omitted.

### Debug logging

Set `NUON_DEBUG=true` to print additional logs to stderr.

For `install` and `upgrade`, debug logs include:

- install id and install name
- install stack id and stack status
- omitted input keys that did not match stack template parameters
