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
- `PT_TOKEN` — Bearer token, created with `ptadmin token create` (required)

If `PT_TOKEN` is missing, ask the user to run `ptadmin token create --user <email> --name <label>` inside the server container (`make admin ARGS="token create ..."`) and paste the token.

## Read commands (safe, no confirmation)

```bash
ptagent me                              # user profile + base currency
ptagent holdings                        # positions with value + PnL in base
ptagent performance --tf 1M             # total + PnL for a timeframe
ptagent allocations --group type        # stocks vs crypto vs cash, etc.
ptagent accounts                        # list accounts
ptagent assets --q apple                # search the asset catalog
ptagent transactions --side buy --limit 20
```

Add `--json` for raw JSON output (good for further processing).

## Write commands (REQUIRE `--yes`)

Never run a write command without `--yes`. The CLI refuses without it; do not add `--yes` unless the user has explicitly confirmed the specific mutation.

```bash
ptagent add-tx --account-id 1 --symbol AAPL --side buy \
  --qty 3 --price 198.20 --fx 0.92 --date 2026-04-18 --yes

ptagent add-account --name "Broker" --type Brokerage --short BR \
  --color "#c8502a" --currency USD --yes

ptagent add-asset --symbol AAPL --name "Apple Inc." --type stock \
  --currency USD --provider yahoo --provider-id AAPL --yes

ptagent delete-tx --id 42 --yes
ptagent delete-account --id 3 --yes
ptagent set-base-currency --currency EUR --yes
```

## Tips for the coding agent

- Before `add-tx`: verify the asset exists (`ptagent assets --q <sym>`); if not, add it first. Transactions FK-reference the asset.
- Before `add-tx`: get the account id via `ptagent accounts`.
- Use `ptagent me` to confirm base currency and interpret monetary values.
- Multi-currency: the asset's native currency may differ from the user's base. `add-tx --fx` is the FX rate from the asset's currency to the user's base at the trade time; it is locked per transaction so historical cost basis doesn't drift.
- On errors: the CLI prints "ptagent: <reason>". Common ones: `401` (bad token), `400` (validation — fix the flags), `404` (wrong id).
