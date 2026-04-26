package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strings"
)

// HMAC-signed cookie values: "<value>.<base64-hmac>". The signature is
// over the literal value bytes; Verify rejects anything that doesn't
// round-trip with the configured secret. We sign rather than encrypt
// because the session id is already an opaque server-side random — the
// only thing a forger could try is guessing or replaying, both of which
// the server-side store and lifetime gate. Signing lets us reject
// tampered / unsigned cookies before any DB lookup.

const cookieSigSep = "."

// SignCookie returns "<value>.<sig>" using HMAC-SHA256 over value.
func SignCookie(secret []byte, value string) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(value))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return value + cookieSigSep + sig
}

// VerifyCookie validates the signature and returns the original value.
// ok==false means the cookie was missing, malformed, or signed with a
// different secret.
func VerifyCookie(secret []byte, signed string) (string, bool) {
	if signed == "" {
		return "", false
	}
	idx := strings.LastIndex(signed, cookieSigSep)
	if idx <= 0 || idx == len(signed)-1 {
		return "", false
	}
	value, sig := signed[:idx], signed[idx+1:]
	want, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil {
		return "", false
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(value))
	if !hmac.Equal(mac.Sum(nil), want) {
		return "", false
	}
	return value, true
}
