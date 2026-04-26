# Portfolio Tracker

## Features

- Track stocks, ETFs, and crypto across multiple accounts.
- Multi-currency: every transaction locks its FX rate at trade time, so historical PnL stays accurate as today's rates drift.
- Performance dashboard with value, cost basis, realised + unrealised PnL, and interactive charts across 1D / 1W / 1M / 3M / 6M / 1Y / ALL.
- Allocation breakdowns by asset class, account, and currency.
- Auto-refreshed prices (Yahoo Finance, CoinGecko) and FX (ECB) — no manual entry once a transaction is logged.
- Import from Ghostfolio JSON via a guided wizard (per-account / per-asset review before applying); export full-snapshot JSON or transactions CSV.
- Multi-user with browser session auth (HMAC-signed session cookies) and `pt_…` API tokens for `ptagent` / automation — optional expiry, soft-delete on revoke, self-serviced from the UI.
- Self-hosted, PWA-installable on Android, with three aesthetics (technical / editorial / forest), dark + light themes, a privacy mask, and a customisable date format.

## Quick start (local development)

Builds the image from source and runs it on `localhost:8082`. For a
real deployment, see [Production deployment](#production-deployment)
below.

```bash
# 1. Symlink the dev compose overlay
ln -sf compose.override.yaml-dev compose.override.yaml

# 2. Set the cookie-signing secret (required, ≥ 32 bytes).
#    Persist it in .env so the container picks it up:
echo "PT_SESSION_SECRET=$(openssl rand -base64 32)" >> .env

# 3. Build + start
make build
make run

# 4. Create a user (prompts twice for a password ≥ 8 chars)
make admin ARGS="user add --email you@example.com --name You --base-currency EUR"

# 5. Open the app and sign in with that email + password
xdg-open http://localhost:8082
```

Inside the app, the avatar menu (bottom-left of the sidebar) lets you
edit profile, open Settings (base currency, aesthetic, date format),
and manage API tokens for `ptagent` and other automation.

The sidebar's **Import / Export** entry runs the import wizard
(currently Ghostfolio JSON; pluggable for future sources) and serves
backups: full-snapshot JSON or transactions-only CSV.

## API tokens

Tokens authenticate `ptagent` and any other Bearer-using client. They
start with `pt_`, carry 32 random bytes, and are stored as SHA-256
hashes — the raw token is shown exactly once at creation time. Each row
supports an optional expiry (`Never` / 7d / 30d / 90d / 1y from the UI,
or `--expires-in` from `ptadmin`) and `last_used_at` is updated
asynchronously, at most once per minute per token, so a busy CLI
doesn't generate one DB write per call.

| Action | What it does |
|--------|---------------|
| **Revoke** | The credential stops authenticating immediately; the row stays in the list with status `Revoked` for audit. |
| **Delete** (trash icon) | Soft-delete: the row disappears from the list but is retained in the DB for forensics. |

Manage them from **avatar menu → API tokens** in the UI, or from the
admin CLI:

```bash
# Create (in the running container)
make admin ARGS="token create --user you@example.com --name laptop-cli"
# Optional expiry: any Go duration; e.g. 30 days
make admin ARGS="token create --user you@example.com --name ci --expires-in 720h"

# List, revoke, delete
make admin ARGS="token list --user you@example.com"
make admin ARGS="token revoke --id 3"
make admin ARGS="token delete --id 3"
```

### `ptagent` setup

`ptagent` is the standalone API client. Pre-built binaries for
linux / macOS / Windows × amd64 / arm64 ship with every GitHub release;
or build it locally with `make ptagent-build` (puts a binary in `bin/`).

1. **Get a token** from the UI (recommended) or `ptadmin`:

   ```bash
   make admin ARGS="user add --email bot@example.com --name Bot --no-password"
   make admin ARGS="token create --user bot@example.com --name default"
   # → pt_<43 chars>   (shown once — copy it now)
   ```

2. **Export the env vars** the agent reads:

   ```bash
   export PT_API_URL="https://portfolio.your-tailnet.ts.net"  # or http://localhost:8082 in dev
   export PT_TOKEN="pt_…"
   ```

   Drop them in `~/.zshrc` / `~/.bashrc` if you want them persistent.

3. **Try a call:**

   ```bash
   ptagent me
   ptagent holdings
   ptagent assets
   ptagent add-tx --account-id 1 --symbol AAPL --side buy \
     --qty 10 --price 192.50 --date 2026-04-01 --yes
   ```

   `ptagent help` lists every command. Mutating commands require
   `--yes` as a destructive-action gate.

### Claude Code skill

`skill/SKILL.md` ships in every release archive. Drop it at
`~/.claude/skills/ptagent/SKILL.md` and Claude Code will invoke
`ptagent` on its own when you ask in plain language to log a trade,
look up a holding, or check performance — provided `PT_API_URL` /
`PT_TOKEN` are exported in the shell that runs Claude Code.

## Production deployment

Tagged releases publish a multi-arch image to
`ghcr.io/mtzanidakis/portfolio-tracker`. The prod overlay pairs it with
a [tsrp](https://github.com/mtzanidakis/tsrp) sidecar so the app is
reachable only through your Tailscale network — no public ports.

1. Symlink the prod overlay:

   ```bash
   ln -sf compose.override.yaml-prod compose.override.yaml
   ```

2. Set required variables in `.env`:

   ```
   HOSTNAME=portfolio
   TS_AUTHKEY=tskey-auth-...
   PT_SESSION_SECRET=<output of `openssl rand -base64 32`>
   ```

   `HOSTNAME` is the Tailscale machine name the app will register as.
   Generate `TS_AUTHKEY` at
   <https://login.tailscale.com/admin/settings/keys>.
   `PT_SESSION_SECRET` is the HMAC key for browser session cookies — keep
   it stable across restarts (rotating it invalidates every active
   session).

3. Pull and start:

   ```bash
   docker compose pull
   docker compose up -d
   ```

The app will be available at `https://<hostname>.<your-tailnet>.ts.net`.
The tsrp sidecar persists its Tailscale state under `./config/tsrp/`,
so back that directory up alongside `./data/` (the SQLite database).

To upgrade, pull the new image tag and restart:

```bash
docker compose pull
docker compose up -d
```

## Auth model

| Client    | Credentials                               | Auth mechanism                              |
|-----------|-------------------------------------------|---------------------------------------------|
| Browser   | email + password → session                | HMAC-signed `pt_session` cookie + CSRF      |
| `ptagent` | API token from UI or `ptadmin`            | `Authorization: Bearer pt_<32 random bytes>`|
| Admin ops | none — direct DB via `ptadmin`            | —                                           |

Both auth paths coexist on the same API routes; the server accepts
whichever is present. Browser unsafe methods additionally require an
`X-CSRF-Token` header matching the `pt_csrf` cookie. The session cookie
is HMAC-SHA256 signed with `PT_SESSION_SECRET`; tampered or unsigned
cookies are rejected before any DB lookup.

## Configuration

Server env vars (CLI flags override when both are set):

| Variable                   | Default              | Description                                       |
|----------------------------|----------------------|---------------------------------------------------|
| `PT_ADDR`                  | `:8082`              | HTTP listen address                               |
| `PT_DB`                    | `./data/pt.db`       | SQLite database path                              |
| `PT_SESSION_SECRET`        | **required**         | HMAC key for signing browser session cookies (≥ 32 bytes). Generate with `openssl rand -base64 32`. Rotating it logs everyone out. |
| `PT_PRICE_REFRESH_INTERVAL`| `15m`                | live-quote refresh cadence (Go duration). The daily history backfill runs once at boot and again every 24h at 22:00 UTC. |
| `PT_SESSION_LIFETIME`      | `30d`                | browser session lifetime (accepts `30d` or `720h`)|
| `PT_COINGECKO_API_KEY`     | *(unset)*            | optional Demo tier key for dedicated quota        |
| `TZ`                       | `Europe/Athens`      | container timezone (for log timestamps)           |

`ptagent` reads:

| Variable     | Default                 |
|--------------|-------------------------|
| `PT_API_URL` | `http://localhost:8082` |
| `PT_TOKEN`   | *(required)*            |

## Stack

- **Backend:** Go 1.26.2, stdlib-first, pure-Go SQLite (`modernc.org/sqlite`), single static binary (CGO-free).
- **Frontend:** Preact + esbuild, embedded with `go:embed`. Three aesthetics (technical / editorial / forest), dark + light themes, privacy mask, custom date format.
- **Auth:** password + session cookie for the browser (argon2id, CSRF double-submit). API tokens for `ptagent` / automation — self-serviced from the UI.
- **Prices:** Yahoo Finance (stocks/ETFs) + CoinGecko (crypto, optional free API key). FX via Frankfurter (ECB). History backfilled dynamically from the earliest transaction; live quotes refreshed every 15 min, official closes locked once a day at 22:00 UTC.
- **Import / Export:** import from Ghostfolio JSON via a guided wizard (review per-account / per-asset matches before applying); export full snapshot (JSON) or transactions-only (CSV).
- **Deploy:** one Alpine container (`ptd` server + `ptadmin` admin inside). `ptagent` CLI released separately via goreleaser.

## Layout

```
cmd/
  ptd/        HTTP server (inside container)
  ptadmin/    Admin CLI (inside container)
  ptagent/    API client CLI (outside, goreleaser)
internal/
  api/         HTTP handlers + router
  auth/        password + session + CSRF + Bearer middleware
  db/          SQLite + migrations + repositories + atomic ApplyImport
  domain/      core types (User, Account, Asset, Transaction, Session, enums)
  exporters/   JSON snapshot + transactions CSV writers
  importers/   normalised ImportPlan + per-source parsers (Ghostfolio today)
  portfolio/   pure logic: holdings, PnL, valuation, daily series
  prices/      Yahoo + CoinGecko + Frankfurter + split live/history refresh
  version/     build-time version string
  web/         go:embed static handler
skill/
  SKILL.md    Claude Code skill for ptagent
web/          Preact source (bundled into internal/web/dist/ at build time)
```

## Development

All developer workflows go through Docker; nothing is built or tested on the host.

```bash
make build           # build container image
make run / stop      # up/down via compose (uses the symlinked override)
make logs
make shell           # shell inside the running container
make admin ARGS="…"  # run ptadmin inside the container
make test            # go test -race -cover inside an ephemeral container
make lint            # golangci-lint inside an ephemeral container
make ptagent-build   # build bin/ptagent (for local testing only)
make clean
```

CI mirrors these on push and PR (see `.github/workflows/ci.yml`).

## Releases

Pushing a `vX.Y.Z` tag fires two GitHub Actions:

- `release.yml` — builds and pushes the container to `ghcr.io/<user>/portfolio-tracker:<tag>`.
- `release-cli.yml` — goreleaser builds cross-platform `ptagent` archives and attaches them to the GitHub release.

## License

MIT — see [LICENSE](LICENSE).
