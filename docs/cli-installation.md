---
sidebar_position: 1
---

# Installation

For a high-level command map, see [CLI Overview](./cli-overview.md).

## Requirements

- Go `1.26+`
- Node.js `20+` (only for docs-site tasks)

## Option 1: Homebrew Tap (recommended)

Use the tap at `mekedron/tap`:

```bash
brew tap mekedron/tap
brew install otta-cli
```

Or install directly:

```bash
brew install mekedron/tap/otta-cli
```

## Option 2: Build from source (manual)

Build a local binary with embedded version metadata:

```bash
git clone git@github.com:mekedron/otta-cli.git
cd otta-cli
BUILD_VERSION="$(git describe --tags --always --dirty)" # requires a git checkout
go build -trimpath -ldflags "-s -w -X main.version=${BUILD_VERSION}" -o bin/otta ./cmd/otta
```

Optional: install to `PATH` manually:

```bash
install -m 0755 ./bin/otta /usr/local/bin/otta
```

## Verify install

Choose one binary path, then run:

```bash
OTTA_BIN=./bin/otta
# If installed to PATH:
# OTTA_BIN=otta

$OTTA_BIN --version
$OTTA_BIN --help
$OTTA_BIN config path
$OTTA_BIN config cache-path
$OTTA_BIN worktimes read --id <worktime-id> --format json
$OTTA_BIN holidays read --from 2026-02-20 --to 2026-02-20 --worktimegroup <id> --format json
$OTTA_BIN absence browse --from 2026-02-01 --to 2026-02-28 --format json
$OTTA_BIN absence options --format json
$OTTA_BIN absence add --type <absence-type-id> --from 2026-02-20 --to 2026-02-20 --description "sick leave" --format json
$OTTA_BIN absence read --id <absence-id> --format json
$OTTA_BIN calendar overview --from 2026-02-01 --to 2026-02-28 --format json
$OTTA_BIN calendar detailed --from 2026-02-01 --to 2026-02-28 --format json --duration-format hours
```

Note: `worktimes` commands are worktime-only and do not return absences.
Minute-based read commands support `--duration-format minutes|hours|days|hhmm`.

## Next Steps

1. [Authentication](./cli-auth.md)
2. [CLI Overview](./cli-overview.md)
3. [Time Tracking Workflows](./cli-timesheets.md)
