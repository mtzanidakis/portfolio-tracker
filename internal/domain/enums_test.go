package domain

import "testing"

func TestCurrency_Valid(t *testing.T) {
	for _, c := range AllCurrencies {
		if !c.Valid() {
			t.Errorf("%s should be valid", c)
		}
	}
	if Currency("XYZ").Valid() {
		t.Error("XYZ should not be valid")
	}
	if Currency("").Valid() {
		t.Error("empty currency should not be valid")
	}
}

func TestCurrency_Decimals(t *testing.T) {
	cases := map[Currency]int{
		USD: 2,
		EUR: 2,
		GBP: 2,
		JPY: 0,
		CHF: 2,
		CAD: 2,
		AUD: 2,
	}
	for c, want := range cases {
		if got := c.Decimals(); got != want {
			t.Errorf("%s decimals: got %d, want %d", c, got, want)
		}
	}
}

func TestParseCurrency(t *testing.T) {
	cases := []struct {
		in      string
		want    Currency
		wantErr bool
	}{
		{"USD", USD, false},
		{"usd", USD, false},
		{" eur ", EUR, false},
		{"JPY", JPY, false},
		{"xyz", "", true},
		{"", "", true},
	}
	for _, tc := range cases {
		got, err := ParseCurrency(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseCurrency(%q) expected error", tc.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseCurrency(%q) unexpected error: %v", tc.in, err)
		}
		if got != tc.want {
			t.Errorf("ParseCurrency(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestAssetType_Valid(t *testing.T) {
	for _, tp := range AllAssetTypes {
		if !tp.Valid() {
			t.Errorf("%s should be valid", tp)
		}
	}
	if AssetType("bond").Valid() {
		t.Error("bond should not be valid")
	}
}

func TestParseAssetType(t *testing.T) {
	got, err := ParseAssetType("  STOCK  ")
	if err != nil || got != AssetStock {
		t.Errorf("ParseAssetType STOCK: got %q, err %v", got, err)
	}
	if _, err := ParseAssetType("bond"); err == nil {
		t.Error("expected error for bond")
	}
}

func TestTxSide(t *testing.T) {
	for _, s := range AllTxSides {
		if !s.Valid() {
			t.Errorf("%s should be valid", s)
		}
	}
	if TxSide("swap").Valid() {
		t.Error("swap should not be valid")
	}

	got, err := ParseTxSide("BUY")
	if err != nil || got != SideBuy {
		t.Errorf("ParseTxSide BUY: got %q, err %v", got, err)
	}
	if _, err := ParseTxSide("hodl"); err == nil {
		t.Error("expected error for hodl")
	}

	// IsCash: only deposit/withdraw/interest.
	cashSides := map[TxSide]bool{
		SideDeposit: true, SideWithdraw: true, SideInterest: true,
	}
	for _, s := range AllTxSides {
		if got, want := s.IsCash(), cashSides[s]; got != want {
			t.Errorf("%s.IsCash() = %v, want %v", s, got, want)
		}
	}

	// IncreasesQty: buy/deposit/interest.
	addSides := map[TxSide]bool{
		SideBuy: true, SideDeposit: true, SideInterest: true,
	}
	for _, s := range AllTxSides {
		if got, want := s.IncreasesQty(), addSides[s]; got != want {
			t.Errorf("%s.IncreasesQty() = %v, want %v", s, got, want)
		}
	}
}
