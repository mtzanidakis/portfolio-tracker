package domain

import "time"

// User is a human principal that owns accounts, transactions, and tokens.
type User struct {
	ID           int64
	Email        string
	Name         string
	BaseCurrency Currency // reporting currency for totals, charts, PnL
	CreatedAt    time.Time
}

// Account is a label grouping for holdings (brokerage, exchange, wallet, cash).
// Accounts are not integrated with external providers; they are purely
// user-defined labels.
type Account struct {
	ID        int64
	UserID    int64
	Name      string
	Type      string   // free-text: "Taxable Brokerage", "Crypto Exchange", etc.
	Short     string   // 2–3 char label for UI badge
	Color     string   // hex like "#c8502a"
	Currency  Currency // default currency for cash in this account
	Connected bool     // purely informational
	CreatedAt time.Time
}

// Asset is a tradeable instrument identified by its ticker symbol. Cash
// "assets" use the currency code as the symbol (e.g., "USD", "EUR").
type Asset struct {
	Symbol     string    // primary key: "AAPL", "BTC", "USD"
	Name       string    // "Apple Inc.", "Bitcoin", "US Dollar"
	Type       AssetType // stock | etf | crypto | cash
	Currency   Currency  // native currency of the asset's price
	Color      string    // hex for UI
	Provider   string    // "yahoo" | "coingecko" | "" (for cash)
	ProviderID string    // external ID at provider (e.g., "bitcoin" for CoinGecko)
}

// Transaction records a single buy or sell. Prices and fees are denominated
// in the asset's native currency. FxToBase locks the FX rate from
// Asset.Currency to the user's BaseCurrency at the transaction time, so that
// historical base-currency cost-basis does not drift if FX history is revised.
type Transaction struct {
	ID          int64
	UserID      int64
	AccountID   int64
	AssetSymbol string
	Side        TxSide
	Qty         float64   // quantity of the asset
	Price       float64   // per-unit price in Asset.Currency
	Fee         float64   // fee amount in Asset.Currency
	FxToBase    float64   // 1 Asset.Currency = FxToBase * BaseCurrency, at OccurredAt
	OccurredAt  time.Time // when the trade happened (user-supplied date)
	Note        string
	CreatedAt   time.Time // when the tx was recorded in the DB
}

// Token is an API access credential. Only the hash is persisted; the raw
// token is returned exactly once, at creation time.
type Token struct {
	ID         int64
	UserID     int64
	Name       string
	Hash       string // hex-encoded SHA-256 of the raw token
	CreatedAt  time.Time
	LastUsedAt *time.Time
	RevokedAt  *time.Time
}
