# Portfolio Tracker

## Features

- Track stocks, ETFs, and crypto across multiple accounts.
- Multi-currency: every transaction locks its FX rate at trade time, so historical PnL stays accurate as today's rates drift.
- Performance dashboard with value, cost basis, realised + unrealised PnL, and interactive charts across 1D / 1W / 1M / 3M / 6M / 1Y / ALL.
- Allocation breakdowns by asset class, account, and currency.
- Auto-refreshed prices (Yahoo Finance, CoinGecko) and FX (ECB) — no manual entry once a transaction is logged.
- Import from Ghostfolio JSON via a guided wizard (per-account / per-asset review before applying); export full-snapshot JSON or transactions CSV.
- Multi-user with browser session auth and API tokens for `ptagent` / automation, self-serviced from the UI.
- Self-hosted, PWA-installable on Android, with three aesthetics (technical / editorial / forest), dark + light themes, a privacy mask, and a customisable date format.

## Quick start

```bash
# 1. Pick a compose overlay (symlink)
ln -sf compose.override.yaml-dev compose.override.yaml    # local build
# or:  ln -sf compose.override.yaml-prod compose.override.yaml

# 2. Build + start
make build
make run

# 3. Create a user (prompts twice for a password ≥ 8 chars)
make admin ARGS="user add --email you@example.com --name You --base-currency EUR"

# 4. Open the app and sign in with that email + password
xdg-open http://localhost:8082
```

Inside the app, the avatar menu (bottom-left of the sidebar) lets you
edit profile, open Settings (base currency, aesthetic, date format),
and create/revoke API tokens for `ptagent` and other automation.

The sidebar's **Import / Export** entry runs the import wizard
(currently Ghostfolio JSON; pluggable for future sources) and serves
backups: full-snapshot JSON or transactions-only CSV.

### API-only setup (no browser access)

```bash
make admin ARGS="user add --email bot@example.com --name Bot --no-password"
make admin ARGS="token create --user bot@example.com --name default"
# ↑ token is printed exactly once.
```

## Auth model

| Client    | Credentials                               | Auth mechanism                   |
|-----------|-------------------------------------------|----------------------------------|
| Browser   | email + password → session                | `pt_session` cookie + CSRF       |
| `ptagent` | API token from UI or `ptadmin`            | `Authorization: Bearer <token>`  |
| Admin ops | none — direct DB via `ptadmin`            | —                                |

Both auth paths coexist on the same API routes; the server accepts
whichever is present. Browser unsafe methods additionally require an
`X-CSRF-Token` header matching the `pt_csrf` cookie.

## Configuration

Server env vars (CLI flags override when both are set):

| Variable                   | Default              | Description                                       |
|----------------------------|----------------------|---------------------------------------------------|
| `PT_ADDR`                  | `:8082`              | HTTP listen address                               |
| `PT_DB`                    | `./data/pt.db`       | SQLite database path                              |
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
