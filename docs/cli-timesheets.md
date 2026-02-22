---
sidebar_position: 4
---

# Time Tracking Workflows

Before running these commands:

1. Complete [Installation](./cli-installation.md)
2. Authenticate with [Authentication](./cli-auth.md)

## Worktimes

Important: `worktimes list`, `worktimes browse`, and `worktimes report` return only worktime rows.
They do not include absences. Use `absence browse` or `calendar detailed` when schedule context matters.

Duration display on read commands can be changed with global `--duration-format`:

- supported formats: `minutes` (default), `hours`, `days`, `hhmm`
- `days` uses fixed conversion `1 day = 24h = 1440 minutes`
- for AI schedule analysis, prefer `calendar detailed --format json --duration-format <format>`

Collect worktimes for a day:

```bash
otta worktimes list --date 2026-02-20 --format json
```

Browse worktimes across a date range:

```bash
otta worktimes browse --from 2026-02-20 --to 2026-02-26 --format json
```

Generate a report for a date range as JSON:

```bash
otta worktimes report --from 2026-02-01 --to 2026-02-28 --format json
```

Generate a report for a date range as CSV:

```bash
otta worktimes report --from 2026-02-01 --to 2026-02-28 --format csv
```

List selectable IDs before `add`:

```bash
otta worktimes options --date 2026-02-20 --format json
```

Filter options when IDs are dependent:

```bash
otta worktimes options \
  --date 2026-02-20 \
  --project <project-id> \
  --worktype <worktype-id> \
  --task <task-id> \
  --format json
```

Add a worktime (required: `--project`, `--worktype`, plus resolved user):

```bash
otta worktimes add \
  --date 2026-02-20 \
  --start 09:00 \
  --end 17:00 \
  --pause 30 \
  --project <project-id> \
  --user <user-id> \
  --worktype <worktype-id> \
  --task <task-id> \
  --subtask <subtask-id> \
  --superior <superior-id> \
  --description "Example task description"
```

If `--user` is omitted in `worktimes add`, fallback order is:

- `OTTA_CLI_USER_ID`
- cached user profile (`~/.otta-cli/cache.json` by default)

Update a worktime:

```bash
otta worktimes update \
  --id <worktime-id> \
  --start 10:00 \
  --end 18:00 \
  --description "Shifted by one hour"
```

Delete a worktime:

```bash
otta worktimes delete --id <worktime-id>
```

Minimal CRUD smoke flow (same date used in live E2E validation):

```bash
DATE=2026-02-20
MARKER="smoke-$(date +%s)"

# 1) Fetch IDs
otta worktimes options --date "$DATE" --format json

# 2) Create one row (fill IDs from options output)
otta worktimes add --date "$DATE" --start 06:00 --end 06:30 --pause 0 \
  --project <project-id> --worktype <worktype-id> --description "$MARKER" --format json

# 3) Update then delete (replace <worktime-id>)
otta worktimes update --id <worktime-id> --description "${MARKER}-updated" --format json
otta worktimes delete --id <worktime-id> --format json
```

## Holidays

Fetch holidays/workday calendar range:

```bash
otta holidays --from 2026-02-20 --to 2026-02-20 --worktimegroup <worktimegroup-id> --format json
```

If `--worktimegroup` is omitted, CLI uses fallback values (if available).
Fallback order for `--worktimegroup`:

- `OTTA_CLI_WORKTIMEGROUP_ID`
- cached user profile (`~/.otta-cli/cache.json` by default)

## Saldo

Fetch current cumulative saldo:

```bash
otta saldo --format json
```

Optional explicit user id:

```bash
otta saldo --user <user-id> --format json
```

If `--user` is omitted in `saldo`, fallback order is:

- `OTTA_CLI_USER_ID`
- cached user profile (`~/.otta-cli/cache.json` by default)

`saldo` also supports `--duration-format` for converted saldo output.

## Absences

Browse absences across a date range (calendar-compatible split rows):

```bash
otta absence browse --from 2026-02-01 --to 2026-02-28 --format json
```

List selectable values for absence creation:

```bash
otta absence options --format json
```

Optional API filter for absence type mode:

```bash
otta absence options --type days --format json
```

Add an absence row (required: `--type`, plus resolved user):

```bash
otta absence add \
  --type <absence-type-id> \
  --from 2026-02-20 \
  --to 2026-02-20 \
  --description "sick leave" \
  --format json
```

If `--user` is omitted in `absence add`, fallback order is:

- `OTTA_CLI_USER_ID`
- cached user profile (`~/.otta-cli/cache.json` by default)

Read one absence row by id:

```bash
otta absence read --id <absence-id> --format json
```

Update an absence row:

```bash
otta absence update --id <absence-id> --description "sick leave" --format json
```

Delete an absence row:

```bash
otta absence delete --id <absence-id> --format json
```

Minimal absence CRUD smoke flow (date used in live validation):

```bash
DATE=2026-02-20

# 1) Resolve type id
otta absence options --format json

# 2) Create one row (fill <absence-type-id>)
otta absence add --type <absence-type-id> --from "$DATE" --to "$DATE" --description "tmp-smoke" --format json

# 3) Read/update/delete (replace <absence-id>)
otta absence read --id <absence-id> --format json
otta absence update --id <absence-id> --description "sick leave" --format json
otta absence delete --id <absence-id> --format json
```

Generate unified calendar overview (worktimes + absences + holidays):

```bash
otta calendar overview --from 2026-02-01 --to 2026-02-28 --format json
```

`calendar overview` returns one `items[]` row per day in range, including weekends and days without entries.

Generate detailed calendar view (day-by-day with events, day-off reasons, and celebrations):

```bash
otta calendar detailed --from 2026-02-01 --to 2026-02-28 --format json
```

For AI/automation schedule checks, prefer `calendar detailed --format json` first.

If `--worktimegroup` is omitted for `calendar overview` or `calendar detailed`, fallback order is:

- `OTTA_CLI_WORKTIMEGROUP_ID`
- cached user profile (`~/.otta-cli/cache.json` by default)

## Absence Comment Helper

Generate consistent comment text for absence submissions:

```bash
otta absence comment \
  --type sick \
  --from 2026-02-20 \
  --to 2026-02-20 \
  --details "Flu symptoms, no work capacity" \
  --format json
```

Use output text directly in Otta absence submission UI/workflow.
