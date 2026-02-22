---
sidebar_position: 2
---

# Authentication

`otta-cli` authenticates against `https://api.moveniumprod.com/login` using:

- `grant_type=password`
- `client_id=ember_app`
- username/password

`otta-cli` also renews access tokens against the same endpoint using:

- `grant_type=refresh_token`
- `client_id=ember_app`
- `refresh_token`

Before this guide:

1. Complete [Installation](./cli-installation.md)
2. Review [CLI Overview](./cli-overview.md)

## Commands

```bash
otta auth login --username <username>
otta auth login --username <username> --password <password>
otta auth login --username <username> --password-stdin
otta auth login --username <username> --password <password> --format json
otta status
otta status --format json
otta config path
otta config cache-path
```

Use either `--password` or `--password-stdin` (not both).

Recommended non-interactive login:

```bash
printf '%s\n' "$OTTA_CLI_PASSWORD" | otta auth login --username <username> --password-stdin
```

Argument-style login (convenient, but may be visible in shell history/process listing):

```bash
otta auth login --username <username> --password "$OTTA_CLI_PASSWORD"
```

`otta auth login` stores credentials/token in local config.
API-derived profile data is stored in a separate cache file.
`otta status` refreshes cached user metadata (`user.id`, `worktimegroup_id`) used by
`worktimes add`, `holidays`, and `calendar overview` fallback resolution.

Any authenticated command uses silent token renewal when possible:

- pre-request renewal when `token.expires_at` is near/past expiry
- fallback renewal on `401 Unauthorized` and transparent request retry once
- refreshed tokens are persisted back to config when token values came from config

`otta status` validates configured credentials by doing an authenticated API call and prints:

- auth validity
- token expiry data (if available)
- basic user data when present in API response
- worktimes count for the current UTC date

## Config Path

Default config path:

- `~/.otta-cli/config.json`

Override path with env var:

- `OTTA_CLI_CONFIG_PATH=/custom/path/config.json`
- `OTTA_CLI_CACHE_PATH=/custom/path/cache.json`

Resolve current path:

```bash
otta config path
otta config cache-path
```

## Environment Variables

Credentials can be provided fully via env vars (useful in Docker/CI):

- `OTTA_CLI_API_BASE_URL`
- `OTTA_CLI_CLIENT_ID`
- `OTTA_CLI_USERNAME`
- `OTTA_CLI_PASSWORD` (used by `auth login` if no flag/stdin password is provided)
- `OTTA_CLI_ACCESS_TOKEN`
- `OTTA_CLI_TOKEN_TYPE`
- `OTTA_CLI_REFRESH_TOKEN`
- `OTTA_CLI_TOKEN_SCOPE`

When token values are provided via `OTTA_CLI_*` env vars, runtime refresh does not
write refreshed values back to config files.

## Security Notes

- Credentials/tokens are local only.
- Do not commit config or cache files.
- Keep `OTTA_CLI_*` env vars in private runtime secrets handling.
