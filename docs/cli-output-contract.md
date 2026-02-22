---
sidebar_position: 5
---

# Output Contract

For scriptability, command output is deterministic.

## Modes

- human-readable text is default
- JSON mode with `--format json` is available on:
  - `auth login`
  - `status`
  - `saldo`
  - all `worktimes` subcommands
  - `calendar overview`
  - `calendar detailed`
  - `holidays` and `holidays read`
  - all `absence` subcommands (`options`, `browse`, `read`, `add`, `update`, `delete`, `comment`)
- CSV mode is available only for `worktimes report --format csv`
- `--json` exists as a deprecated alias for `--format json`

## Exit Codes

- `0` success
- `1` validation/config/auth/API failure

## JSON Envelope

- top-level object: `ok`, `command`, `data`
- `data` fields are command-specific
- raw API payload is included where API schemas vary by tenant
- `worktimes list/read/browse/report` payloads are worktime-only and never include absences
- use `absence browse` or `calendar detailed/overview` when absence-aware schedule output is needed
- minute-based read commands support global `--duration-format` (`minutes|hours|days|hhmm`)
- duration conversion basis for `days` is fixed: `1 day = 24h = 1440 minutes`

## Duration Fields

When a command includes minute totals, JSON responses include:

- `duration_format`: normalized selected format
- duration summary objects (for example `total_duration`, `worktime_duration`, `absence_duration`, or `durations.*`) with:
  - `format`
  - `minutes` (raw canonical value)
  - `value` (converted value)
  - `text` (human-readable representation)

Example (`worktimes list`):

```json
{
  "ok": true,
  "command": "worktimes list",
  "data": {
    "date": "2026-02-20",
    "count": 0,
    "raw": {
      "count": 0,
      "worktimes": []
    }
  }
}
```

Example (`worktimes add`):

```json
{
  "ok": true,
  "command": "worktimes add",
  "data": {
    "raw": {
      "worktime": {
        "id": 12345678,
        "status": "open"
      }
    }
  }
}
```

Example (`absence browse`, excerpt):

```json
{
  "ok": true,
  "command": "absence browse",
  "data": {
    "from": "2026-02-01",
    "to": "2026-02-28",
    "days": 28,
    "count": 1,
    "total_minutes": 450,
    "duration_format": "hours",
    "total_duration": {
      "format": "hours",
      "minutes": 450,
      "value": 7.5,
      "text": "7.50 hours"
    }
  }
}
```

Example (`absence add`, excerpt):

```json
{
  "ok": true,
  "command": "absence add",
  "data": {
    "id": 51744722,
    "raw": {
      "abcense": {
        "id": 51744722,
        "status": "open"
      }
    }
  }
}
```

Example (`calendar overview`, excerpt):

```json
{
  "ok": true,
  "command": "calendar overview",
  "data": {
    "from": "2026-02-01",
    "to": "2026-02-28",
    "days": 28,
    "totals": {
      "worktime_hours": 105,
      "absence_hours": 7.5
    }
  }
}
```

Example (`calendar detailed`, excerpt):

```json
{
  "ok": true,
  "command": "calendar detailed",
  "data": {
    "from": "2026-02-01",
    "to": "2026-02-28",
    "worktime_group_id": 17910737,
    "totals": {
      "day_off_days": 8,
      "celebration_days": 0
    }
  }
}
```

Example (`saldo`, excerpt):

```json
{
  "ok": true,
  "command": "saldo",
  "data": {
    "user_id": 24352445,
    "from": "2024-09-01",
    "to": "2026-02-21",
    "cumulative_saldo_minutes": 1463,
    "cumulative_saldo_duration": {
      "format": "hours",
      "minutes": 1463,
      "value": 24.3833,
      "text": "24.38 hours"
    }
  }
}
```

## Error Output

- errors are returned as plain text on `stderr`
- there is no JSON error envelope on failure
