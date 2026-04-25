# Portfolio Tracker — agent notes

Self-hosted stocks / ETF / crypto portfolio tracker. Multi-user, token auth, PWA-installable.

## Stack

- **Backend**: Go 1.26.2, stdlib-first, pure-Go SQLite (`modernc.org/sqlite`), `CGO_ENABLED=0` everywhere.
- **Frontend**: Preact + esbuild, JSX pre-transpiled at build time, bundled into `internal/web/dist/` by the Dockerfile web stage and embedded via `go:embed`. No CDN at runtime; system font stack for now (self-hosting woff2 is a future polish).
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
- **Refresh schedule**: two independent loops in `prices.Service.Run`. Live (every `PT_PRICE_REFRESH_INTERVAL`, default 15m) hits only the latest-quote endpoints. History runs at boot, then daily at 22:00 UTC — backfill window is computed dynamically from each asset's earliest transaction (per-asset for prices, global for FX), floored at 1 year.
- **Snapshot key**: `(asset_symbol, midnight_UTC)`. Live refresh updates today's row in place; the next history pass overwrites past days with the official close from the chart endpoint. Today's bar is intentionally skipped by history so the live loop is its sole writer.
- **User preferences split**: identity fields (name, email, password) live in the Profile modal; display preferences (base currency, aesthetic, date format) live in a separate Settings modal. The aesthetic + date format are client-side only (`localStorage`); base currency is the one that persists server-side.
- **Privacy mode**: every aggregate monetary number renders inside `<span class="masked">`. Percentages, counts, and instrument quantities stay legible — toggle is in the topbar.
- **Import / Export**:
  - `internal/importers/` defines a source-agnostic `ImportPlan` and per-source parsers (Ghostfolio today). The plan is round-tripped between `/api/v1/import/{source}/analyze` and `/api/v1/import/apply` so the server stays stateless.
  - `db.ApplyImport` is the single atomic write path: pre-fetched FX rates go in, account/asset reuse is honoured via `MapToID` / `MapToSymbol`, and the whole batch is one SQLite transaction — failure rolls back.
  - `internal/exporters/` covers the way out: `WriteJSON` for a self-describing snapshot envelope, `WriteTransactionsCSV` for spreadsheet use. Served by `GET /api/v1/export?format=…`.

## Layout

```
cmd/{ptd,ptadmin,ptagent}/     # binaries (ptd + ptadmin in container, ptagent standalone)
internal/
  api/         # HTTP handlers + Go 1.22 ServeMux (incl. import/export)
  auth/        # Bearer token middleware
  db/
    migrations/    # SQL, embedded
    *.go           # repositories per entity, plus ApplyImport
  domain/      # types + enums
  exporters/   # snapshot JSON + transactions CSV
  importers/   # ImportPlan + per-source parsers
  portfolio/   # pure logic: holdings, PnL, valuation, daily series
  prices/      # providers + split live/history refresh service
  version/     # build-time Version string
  web/
    dist/      # built frontend (gitignored except .gitkeep placeholder)
    web.go     # go:embed static handler
skill/SKILL.md  # Claude Code skill for ptagent
web/            # Preact source, esbuild-bundled into internal/web/dist
```
