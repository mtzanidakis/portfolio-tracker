# Portfolio Tracker — agent notes

Self-hosted stocks / ETF / crypto portfolio tracker. Multi-user, token auth, PWA-installable.

## Stack

- **Backend**: Go 1.26.2, stdlib-first, pure-Go SQLite (`modernc.org/sqlite`), `CGO_ENABLED=0` everywhere.
- **Frontend**: Preact + esbuild, JSX pre-transpiled at build time, embedded via `go:embed`. Self-hosted fonts, no CDN.
- **Deploy**: Single Alpine container (`ptd` server + `ptadmin` CLI inside). `ptagent` CLI released separately via goreleaser.

## Build & run — container-only

Do **not** run `go build` or `npm run build` on the host. Everything goes through Docker.

- `make build` — build the container image
- `make run` / `make stop` / `make logs` — dev stack via docker compose
- `make test` — `go test -race -cover ./...` inside an ephemeral container
- `make lint` — `golangci-lint run ./...` inside an ephemeral container
- `make admin ARGS="user add …"` — invoke `ptadmin` inside the running container
- `make ptagent-build` — build `ptagent` locally (for testing; normally released via goreleaser)

Compose env selection: symlink `compose.override.yaml` → `compose.override.yaml-dev` (local build) or `compose.override.yaml-prod` (registry image + tsrp).

## Repo conventions

- **Latest versions** for every dependency (Go, npm, Docker base, GH Actions). Reproducibility comes from `go.sum` / `package-lock.json`, not hand-pinning.
- **Multi-currency**: user has `base_currency`; assets have native currency; FX rate at transaction time is **locked** in the tx row (`fx_to_base`). Cost basis is computed average in native currency.
- **Tokens**: Bearer auth. Stored as SHA-256 hash. Created via `ptadmin token create`.
- **Accounts**: labels only (no brokerage integration).
- **Prices**: Yahoo Finance (stocks/ETF) + CoinGecko (crypto, optional free API key via `PT_COINGECKO_API_KEY`). FX via frankfurter.app (ECB).

## Layout

```
cmd/{ptd,ptadmin,ptagent}/     # binaries
internal/
  api/        # HTTP handlers
  auth/       # token auth
  db/         # sqlite + migrations
  domain/     # types
  portfolio/  # pure logic: holdings, PnL, series
  prices/     # provider interface + implementations
  version/    # build version
  web/        # go:embed static handler
migrations/   # SQL
skill/SKILL.md
web/          # Preact source, esbuild-bundled into web/dist
```
