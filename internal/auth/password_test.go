package auth

import (
	"strings"
	"testing"
)

func TestHashPassword_RoundTrip(t *testing.T) {
	h, err := HashPassword("s3cret!")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !strings.HasPrefix(h, "$argon2id$v=19$") {
		t.Errorf("unexpected format: %q", h)
	}
	if !VerifyPassword("s3cret!", h) {
		t.Error("verify failed on correct password")
	}
	if VerifyPassword("wrong", h) {
		t.Error("verify succeeded on wrong password")
	}
}

func TestHashPassword_UniqueSalt(t *testing.T) {
	h1, _ := HashPassword("same")
	h2, _ := HashPassword("same")
	if h1 == h2 {
		t.Error("two hashes of the same password should differ (random salt)")
	}
}

func TestVerifyPassword_EmptyHashRejectsEverything(t *testing.T) {
	if VerifyPassword("", "") {
		t.Error("empty hash should not verify empty password")
	}
	if VerifyPassword("anything", "") {
		t.Error("empty hash should not verify any password")
	}
}

func TestVerifyPassword_MalformedHash(t *testing.T) {
	cases := []string{
		"garbage",
		"$argon2id$v=19$not-valid",
		"$argon2i$v=19$m=65536,t=3,p=2$aaaa$bbbb", // wrong algorithm
		"$argon2id$v=19$m=65536,t=3,p=2$!!!$bbbb", // bad base64 salt
	}
	for _, c := range cases {
		if VerifyPassword("x", c) {
			t.Errorf("malformed hash should not verify: %q", c)
		}
	}
}
