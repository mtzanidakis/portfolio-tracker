package db

import (
	"strings"
	"unicode"
)

// ftsPattern turns a free-text search string into a MATCH expression
// safe for the tx_fts virtual table. The rules:
//
//   - Only letters, digits and whitespace survive — FTS5 operators
//     ('"', '*', '(', ')', ':', etc.) would otherwise let a user
//     trigger syntax errors by typing a quote.
//   - Whitespace splits the input into tokens.
//   - Each token gets a trailing '*' so "app" matches "apple"; this
//     mirrors the spirit of the old LIKE '%foo%' behaviour without
//     being as pathologically slow on large datasets.
//   - Empty input (or input that collapses to nothing after
//     sanitisation) returns "" — the caller checks and skips the
//     filter rather than running MATCH "" (which FTS5 rejects).
func ftsPattern(q string) string {
	var b strings.Builder
	b.Grow(len(q))
	for _, r := range q {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) {
			b.WriteRune(r)
		} else {
			b.WriteRune(' ')
		}
	}
	toks := strings.Fields(b.String())
	if len(toks) == 0 {
		return ""
	}
	for i, t := range toks {
		toks[i] = t + "*"
	}
	return strings.Join(toks, " ")
}
