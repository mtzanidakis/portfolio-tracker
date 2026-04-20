package domain

import "time"

// User is a human principal that owns accounts, transactions, and tokens.
type User struct {
	ID           int64     `json:"id"`
	Email        string    `json:"email"`
	Name         string    `json:"name"`
	BaseCurrency Currency  `json:"base_currency"`
	CreatedAt    time.Time `json:"created_at"`
}

// Account is a label grouping for holdings (brokerage, exchange, wallet, cash).
// Accounts are not integrated with external providers; they are purely
// user-defined labels.
type Account struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"` // free-text
	Short     string    `json:"short"`
	Color     string    `json:"color"`
	Currency  Currency  `json:"currency"`
	Connected bool      `json:"connected"`
	CreatedAt time.Time `json:"created_at"`
}

// Asset is a tradeable instrument identified by its ticker symbol. Cash
// "assets" use the currency code as the symbol (e.g., "USD", "EUR").
type Asset struct {
	Symbol     string    `json:"symbol"`
	Name       string    `json:"name"`
	Type       AssetType `json:"type"`
	Currency   Currency  `json:"currency"`
	Color      string    `json:"color"`
	Provider   string    `json:"provider"`
	ProviderID string    `json:"provider_id"`
}

// Transaction records a single buy or sell. Prices and fees are denominated
// in the asset's native currency. FxToBase locks the FX rate from
// Asset.Currency to the user's BaseCurrency at the transaction time, so that
// historical base-currency cost-basis does not drift if FX history is revised.
type Transaction struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"user_id"`
	AccountID   int64     `json:"account_id"`
	AssetSymbol string    `json:"asset_symbol"`
	Side        TxSide    `json:"side"`
	Qty         float64   `json:"qty"`
	Price       float64   `json:"price"`
	Fee         float64   `json:"fee"`
	FxToBase    float64   `json:"fx_to_base"`
	OccurredAt  time.Time `json:"occurred_at"`
	Note        string    `json:"note"`
	CreatedAt   time.Time `json:"created_at"`
}

// Token is an API access credential. Only the hash is persisted; the raw
// token is returned exactly once, at creation time.
type Token struct {
	ID         int64      `json:"id"`
	UserID     int64      `json:"user_id"`
	Name       string     `json:"name"`
	Hash       string     `json:"-"` // never serialize the hash
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}
