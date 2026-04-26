---
name: portfolio-tracker
description: Query and mutate a self-hosted portfolio tracker (stocks, ETFs, crypto) via the `ptagent` CLI. Trigger on questions about portfolio value, holdings, allocations, transactions, or requests to record a buy/sell.
---

# Portfolio Tracker skill

Use this skill when the user asks about their self-hosted portfolio tracker — for example:

- "what's my portfolio worth" / "show my holdings"
- "show performance this month"
- "how am I allocated between stocks and crypto"
- "list my latest transactions"
- "record a buy of 3 AAPL at 198"
- "what accounts do I have"

## Setup

The skill wraps the `ptagent` binary. It reads two environment variables:

- `PT_API_URL` — base URL of the tracker (default `http://localhost:8082`)
- `PT_TOKEN` — API token (required)

The browser UI uses password-based sessions — `ptagent` does **not**.
Tokens are only for CLIs and automation.

If `PT_TOKEN` is missing, ask the user to create one, either:

- **From the UI:** click the avatar at the bottom-left of the sidebar →
  "API tokens" → enter a name → copy the token (shown once).
- **Via ptadmin** (admin-only): `make admin ARGS="token create --user <email> --name <label>"`.

## Read commands (safe, no confirmation)

```bash
ptagent me                              # user profile + base currency
ptagent holdings                        # positions with value + PnL in base
ptagent performance --tf 1M             # total + PnL for a timeframe
ptagent allocations --group type        # stocks vs crypto vs cash, etc.
ptagent accounts                        # list accounts
ptagent assets --q apple                # search the local asset catalog
ptagent asset-lookup --symbol AAPL      # resolve via Yahoo (or --provider coingecko)
ptagent asset-price --symbol AAPL       # latest known price for one symbol
ptagent transactions --side buy --limit 20
ptagent tx-summary                      # totals: counts, buys, sells, deposits…
ptagent fx-rate --from USD --to EUR     # latest rate; add --at YYYY-MM-DD for historical
ptagent refresh-prices                  # force a server-side price + FX refresh
ptagent export --format json --out backup.json
ptagent export --format csv --out txs.csv
```

Add `--json` for raw JSON output (good for further processing). Note that `refresh-prices` posts to the server but does not mutate user data — it only refreshes the global price cache, so it is safe to run without `--yes`.

## Write commands (REQUIRE `--yes`)

Never run a write command without `--yes`. The CLI refuses without it; do not add `--yes` unless the user has explicitly confirmed the specific mutation.

```bash
ptagent add-tx --account-id 1 --symbol AAPL --side buy \
  --qty 3 --price 198.20 --fx 0.92 --date 2026-04-18 --yes

ptagent add-account --name "Broker" --type Brokerage --short BR \
  --color "#c8502a" --currency USD --yes

ptagent add-asset --symbol AAPL --name "Apple Inc." --type stock \
  --currency USD --provider yahoo --provider-id AAPL --yes

ptagent update-tx --id 42 --price 199.10 --note "fix typo" --yes
ptagent update-account --id 3 --name "Broker EU" --color "#3a7" --yes

ptagent delete-tx --id 42 --yes
ptagent delete-account --id 3 --yes
ptagent delete-asset --symbol OLDX --yes
ptagent set-base-currency --currency EUR --yes
```

`update-tx` and `update-account` are partial: only the flags you pass are
sent to the server; everything else is left untouched. To clear a note,
pass `--note ""` explicitly.

## Tips for the coding agent

- Before `add-tx`: verify the asset exists (`ptagent assets --q <sym>`); if not, look it up with `ptagent asset-lookup --symbol <sym>` to grab the canonical name / native currency / provider-id, then `add-asset`. Transactions FK-reference the asset.
- Before `add-tx`: get the account id via `ptagent accounts`.
- Use `ptagent me` to confirm base currency and interpret monetary values.
- Multi-currency: the asset's native currency may differ from the user's base. `add-tx --fx` is the FX rate from the asset's currency to the user's base at the trade time; it is locked per transaction so historical cost basis doesn't drift. Use `ptagent fx-rate --from <native> --to <base> --at <date>` to fetch the right rate before `add-tx`.
- To fix a mistaken transaction, prefer `update-tx` over `delete-tx` + `add-tx` so the original id and creation timestamp survive.
- On errors: the CLI prints "ptagent: <reason>". Common ones: `401` (bad token), `400` (validation — fix the flags), `404` (wrong id).
