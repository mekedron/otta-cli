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
- `otta config path`: print resolved config path.
- `otta config cache-path`: print resolved cache path.
- `otta worktimes list`: list entries for a specific date.
- `otta worktimes browse`: aggregate entries across a date range.
- `otta worktimes report`: export range data as JSON or CSV.
- `otta worktimes options`: fetch selectable IDs for add/update flows.
- `otta worktimes add`: create an entry.
- `otta worktimes update`: update an existing entry.
- `otta worktimes delete`: delete an entry.
- `otta calendar overview`: generate combined day-by-day report (worktimes + absences + holidays).
- `otta holidays`: fetch workday calendar/holiday rows.
- `otta absence options`: fetch absence type/user options.
- `otta absence browse`: aggregate absence entries across a date range.
- `otta absence comment`: generate absence comment text.

## Practical First-Run Sequence

```bash
otta auth login --username <username> --password <password>
otta status --format json
otta worktimes options --date 2026-02-20 --format json
otta worktimes list --date 2026-02-20 --format json
otta absence browse --from 2026-02-01 --to 2026-02-28 --format json
otta calendar overview --from 2026-02-01 --to 2026-02-28 --format json
```

## Validation Note

A real terminal E2E sweep was run on 2026-02-22 against the live API, covering:

- auth and status
- config path commands
- worktimes list/browse/report/options/add/update/delete
- calendar overview
- holidays
- absence options/browse/comment
