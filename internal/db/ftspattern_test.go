package db

import "testing"

func TestFtsPattern(t *testing.T) {
	cases := map[string]string{
		"":          "",
		"   ":       "",
		"apple":     "apple*",
		"  apple  ": "apple*",
		"apple inc": "apple* inc*",
		// FTS-special characters get dropped but the rest of the token
		// survives.
		`"apple"`:       "apple*",
		`app*`:          "app*",
		`(app)`:         "app*",
		`buy:something`: "buy* something*",
		// Greek with diacritics passes through untouched; the tokenizer
		// in SQLite (remove_diacritics 2) normalises on the index side.
		"Άπλ":     "Άπλ*",
		"ΑΠΛ inc": "ΑΠΛ* inc*",
		// Only punctuation → empty.
		"!@#$%": "",
	}
	for in, want := range cases {
		if got := ftsPattern(in); got != want {
			t.Errorf("ftsPattern(%q) = %q, want %q", in, got, want)
		}
	}
}
