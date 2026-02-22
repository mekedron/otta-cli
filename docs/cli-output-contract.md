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
  - all `worktimes` subcommands
  - `calendar overview`
  - `holidays`
  - `absence options`
  - `absence browse`
  - `absence comment`
- CSV mode is available only for `worktimes report --format csv`
- `--json` exists as a deprecated alias for `--format json`

## Exit Codes

- `0` success
- `1` validation/config/auth/API failure

## JSON Envelope

- top-level object: `ok`, `command`, `data`
- `data` fields are command-specific
- raw API payload is included where API schemas vary by tenant

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
    "total_minutes": 450
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

## Error Output

- errors are returned as plain text on `stderr`
- there is no JSON error envelope on failure
