---
sidebar_position: 3
---

# CLI Overview

`otta-cli` is a Go-based command-line application for automating workflows around `otta.fi`.
Primary API base is `https://api.moveniumprod.com`.

## Docs Map

Read the docs in this order:

1. [Installation](./cli-installation.md)
2. [Authentication](./cli-auth.md)
3. [CLI Overview](./cli-overview.md) (this page)
4. [Time Tracking Workflows](./cli-timesheets.md)
5. [Output Contract](./cli-output-contract.md)
6. [Roadmap](./cli-roadmap.md)

## First-Run Check

After installation and login, verify API access:

```bash
otta status
```

## Command Map

- `otta --version`: print CLI build version.
- `otta auth login`: authenticate and store token/config data.
- `otta status`: validate auth and refresh cached user/worktimegroup metadata.
- `otta saldo`: return current cumulative saldo for the resolved user id.
- `otta config path`: print resolved config path.
- `otta config cache-path`: print resolved cache path.
- `otta worktimes list`: list entries for a specific date.
- `otta worktimes read`: fetch one worktime row by id.
- `otta worktimes browse`: aggregate entries across a date range.
- `otta worktimes report`: export range data as JSON or CSV.
- `otta worktimes options`: fetch selectable IDs for add/update flows.
- `otta worktimes add`: create an entry.
- `otta worktimes update`: update an existing entry.
- `otta worktimes delete`: delete an entry.
- `otta calendar overview`: generate combined calendar totals with day rows.
- `otta calendar detailed`: generate full day-by-day detailed calendar report.
- `otta holidays`: fetch workday calendar/holiday rows.
- `otta holidays read`: fetch workday calendar/holiday rows (explicit read subcommand).
- `otta absence options`: fetch absence type/user options (supports mode-specific lookup with `--mode days|hours`).
- `otta absence browse`: aggregate absence entries across a date range.
- `otta absence read`: fetch one absence row by id.
- `otta absence add`: create an absence row (`--mode auto|days|hours`, default `auto`).
- `otta absence update`: update an existing absence row.
- `otta absence delete`: delete an absence row.
- `otta absence comment`: generate absence comment text.

Important: `worktimes list/read/browse/report` do not return absences.
For complete schedule interpretation, prefer `calendar detailed` (or `calendar overview` for lighter totals/day rows).

## Duration Formatting

Use global `--duration-format` on read commands with minute totals:

- values: `minutes` (default), `hours`, `days`, `hhmm`
- applies to: `worktimes list/read/browse/report`, `absence browse`, `saldo`, `holidays`/`holidays read`, `calendar overview`, `calendar detailed`
- conversion basis for `days`: `1 day = 24h = 1440 minutes`

Examples:

```bash
otta worktimes browse --from 2026-02-01 --to 2026-02-28 --format json --duration-format hours
otta saldo --format json --duration-format hhmm
otta calendar detailed --from 2026-02-01 --to 2026-02-28 --format json --duration-format hhmm
```

## Practical First-Run Sequence

```bash
otta auth login --username <username> --password <password>
otta status --format json
otta worktimes options --date 2026-02-20 --format json
otta saldo --format json
otta worktimes list --date 2026-02-20 --format json
otta worktimes read --id <worktime-id> --format json
otta holidays read --from 2026-02-20 --to 2026-02-20 --worktimegroup <id> --format json
otta absence options --mode days --format json
otta absence options --mode hours --format json
otta absence browse --from 2026-02-01 --to 2026-02-28 --format json
otta absence add --mode days --type <absence-type-id> --from 2026-02-20 --to 2026-02-20 --description "sick leave" --format json
otta absence add --mode hours --type <absence-type-id> --from 2026-02-20 --start 09:00 --end 11:30 --hours 2.5 --description "extra hours" --format json
otta absence read --id <absence-id> --format json
otta absence update --id <absence-id> --description "sick leave" --format json
otta absence delete --id <absence-id> --format json
otta calendar overview --from 2026-02-01 --to 2026-02-28 --format json
otta calendar detailed --from 2026-02-01 --to 2026-02-28 --format json
```

## Validation Note

A real terminal E2E sweep was run on 2026-02-22 against the live API, covering:

- auth and status
- saldo
- config path commands
- worktimes list/read/browse/report/options/add/update/delete
- calendar overview
- calendar detailed
- holidays/holidays read
- absence options/browse/read/add/update/delete/comment, including mode-specific type checks (`days` vs `hours`)
