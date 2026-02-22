# otta-cli

`otta-cli` is a community Go CLI for automating workflows around [otta.fi](https://otta.fi/).
It is intended for Finnish time tracking use cases.

## Current Scope

- auth command (`otta auth login`)
- status check command (`otta status`)
- worktime commands (`list/browse/report/options/add/update/delete`)
- calendar overview command (`otta calendar overview`)
- holidays retrieval command
- absence commands (`options/browse/comment`)
- configurable local config path
- separate cache file for API-derived user metadata
- environment variable credential overrides (Docker/CI friendly)
- refresh-token flow with silent access-token renewal
- Chrome DevTools MCP setup for browser automation/debugging

## Requirements

- Go `1.26+`
- Node.js `20+` (for docs site build)

## Recommended Install (Homebrew Tap)

Use the dedicated tap at `mekedron/tap`:

```bash
brew tap mekedron/tap
brew install otta-cli
```

Or install directly:

```bash
brew install mekedron/tap/otta-cli
```

Release tags (`v*`) automatically update
`mekedron/tap/Formula/otta-cli.rb`
via `.github/workflows/release.yml` (requires repository secret `TAP_REPO_TOKEN`).

## Build and Run

```bash
go build ./...
go build -o bin/otta ./cmd/otta
./bin/otta --help
```

Or without installing:

```bash
go run ./cmd/otta --help
```

## Useful Starter Commands

```bash
otta --version
otta auth login --username <username> --password <password>
otta status
otta config path
otta config cache-path
otta worktimes list --date 2026-02-20
otta worktimes browse --from 2026-02-20 --to 2026-02-26 --format json
otta worktimes report --from 2026-02-01 --to 2026-02-28 --format csv
otta calendar overview --from 2026-02-01 --to 2026-02-28 --format json
otta worktimes options --date 2026-02-20 --format json
otta holidays --from 2026-02-20 --to 2026-02-20 --worktimegroup <id> --format json
otta absence browse --from 2026-02-01 --to 2026-02-28 --format json
otta absence options --format json
otta absence comment --type sick --from 2026-02-20 --to 2026-02-20
```

For non-interactive scripts, prefer stdin or env secrets to reduce shell history exposure:

```bash
printf '%s\n' "$OTTA_CLI_PASSWORD" | otta auth login --username <username> --password-stdin
```

## Config Location

Default local profile config path:

- `~/.otta-cli/config.json`

Default cache path:

- `~/.otta-cli/cache.json`

Override with:

- `OTTA_CLI_CONFIG_PATH=/custom/path/config.json`
- `OTTA_CLI_CACHE_PATH=/custom/path/cache.json`

Example files:

- `configs/example.config.json` (credentials only)
- `configs/example.cache.json` (API-derived cache data)

Credential env vars:

- `OTTA_CLI_API_BASE_URL`
- `OTTA_CLI_CLIENT_ID`
- `OTTA_CLI_USERNAME`
- `OTTA_CLI_PASSWORD`
- `OTTA_CLI_ACCESS_TOKEN`
- `OTTA_CLI_TOKEN_TYPE`
- `OTTA_CLI_REFRESH_TOKEN`
- `OTTA_CLI_TOKEN_SCOPE`
- `OTTA_CLI_USER_ID` (optional convenience for `worktimes add`)
- `OTTA_CLI_WORKTIMEGROUP_ID` (optional convenience for `holidays` and `calendar overview`)

## Test and Lint

Quick local checks:

```bash
go test ./...
go test -race ./...
make lint
```

Full local validation gate (run before `git push`):

```bash
go mod download
go build ./...
BUILD_VERSION="$(git describe --tags --always --dirty)"
go build -trimpath -ldflags "-s -w -X main.version=${BUILD_VERSION}" -o bin/otta ./cmd/otta
test "$(./bin/otta --version | tr -d '\r\n')" = "${BUILD_VERSION}"
golangci-lint run
go test ./...
go test -race ./...
```

If `golangci-lint` is missing:

```bash
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
```

## Security

Local config stores credentials/tokens; cache stores API-derived profile metadata.
Keep both local and never commit them.
Ignored patterns are listed in `.gitignore`.

## Copyright

Copyright (c) 2026 otta-cli contributors.
