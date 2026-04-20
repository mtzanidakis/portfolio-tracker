# Portfolio Tracker

Self-hosted portfolio tracker for stocks, ETFs, and crypto. Multi-user,
multi-currency, token-authenticated API, PWA-installable on Android.

- **Backend:** Go 1.26.2, stdlib-first, pure-Go SQLite (`modernc.org/sqlite`), single static binary (CGO-free).
- **Frontend:** Preact + esbuild, embedded with `go:embed`. Three aesthetics (technical / editorial / forest), dark + light themes, privacy mask.
- **Prices:** Yahoo Finance (stocks/ETFs) + CoinGecko (crypto, optional free API key). FX via Frankfurter (ECB).
- **Deploy:** one Alpine container (`ptd` server + `ptadmin` admin inside). `ptagent` CLI released separately via goreleaser.

## Quick start

```bash
# 1. Pick a compose overlay
ln -sf compose.override.yaml-dev compose.override.yaml   # local build
# or:  ln -sf compose.override.yaml-prod compose.override.yaml

# 2. Build + start
make build
make run

# 3. Create a user and a token
make admin ARGS="user add --email you@example.com --name You --base-currency EUR"
make admin ARGS="token create --user you@example.com --name default"
# ↑ prints the token once. Save it.

# 4. Open the app
open http://localhost:8080
# paste the token when prompted
```

Add an asset and a transaction through the UI (+ Add transaction), or with
`ptagent` (see `skill/SKILL.md`).

## Configuration

All server knobs come from environment variables (or CLI flags, which win).

| Variable                   | Default              | Description                                    |
|----------------------------|----------------------|------------------------------------------------|
| `PT_ADDR`                  | `:8080`              | HTTP listen address                            |
| `PT_DB`                    | `./data/pt.db`       | SQLite database path                           |
| `PT_PRICE_REFRESH_INTERVAL`| `15m`                | background refresh cadence (Go duration)       |
| `PT_COINGECKO_API_KEY`     | *(unset)*            | optional Demo tier key for dedicated quota     |
| `TZ`                       | `Europe/Athens`      | container timezone (for log timestamps)        |

`ptagent` (the standalone CLI) reads:

| Variable      | Default                 |
|---------------|-------------------------|
| `PT_API_URL`  | `http://localhost:8080` |
| `PT_TOKEN`    | *(required)*            |

## Layout

```
cmd/
  ptd/        HTTP server (inside container)
  ptadmin/    Admin CLI (inside container)
  ptagent/    API client CLI (outside, goreleaser)
internal/
  api/        HTTP handlers + router
  auth/       token auth middleware
  db/         SQLite + migrations + repositories
  domain/     core types (User, Account, Asset, Transaction, enums)
  portfolio/  pure logic: holdings, PnL, valuation
  prices/     Yahoo + CoinGecko + Frankfurter + refresh service
  version/    build-time version string
  web/        go:embed static handler
migrations/   (inside internal/db) SQL schema
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
