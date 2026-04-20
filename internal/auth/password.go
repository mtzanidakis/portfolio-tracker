package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// argon2id parameters (OWASP 2023+ guidance, tuned for ~100ms on commodity HW).
const (
	argonTimeCost = 3
	argonMemKiB   = 64 * 1024 // 64 MiB
	argonThreads  = 2
	argonSaltLen  = 16
	argonKeyLen   = 32
)

// HashPassword returns an argon2id-encoded hash string in the canonical
// PHC format:
//
//	$argon2id$v=19$m=65536,t=3,p=2$<salt>$<key>
//
// Both salt and key are base64 (std, no padding). The encoded hash
// carries all parameters needed for verification, so the cost can be
// tuned over time without breaking existing hashes.
func HashPassword(plaintext string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("argon2 salt: %w", err)
	}
	key := argon2.IDKey([]byte(plaintext), salt, argonTimeCost, argonMemKiB, argonThreads, argonKeyLen)
	enc := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemKiB, argonTimeCost, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	)
	return enc, nil
}

// VerifyPassword returns true iff plaintext matches the encoded hash.
// Any parse/format error yields false. Empty encoded strings (users who
// have not set a password) always fail verification.
func VerifyPassword(plaintext, encoded string) bool {
	params, salt, key, err := parseArgon2Hash(encoded)
	if err != nil {
		return false
	}
	keyLen := len(key)
	if keyLen < 0 || keyLen > 1<<30 {
		return false
	}
	computed := argon2.IDKey([]byte(plaintext), salt,
		params.time, params.memory, params.threads, uint32(keyLen)) //nolint:gosec // bounded above
	return subtle.ConstantTimeCompare(key, computed) == 1
}

type argon2Params struct {
	version int
	memory  uint32
	time    uint32
	threads uint8
}

var errBadArgonHash = errors.New("invalid argon2 hash")

func parseArgon2Hash(encoded string) (p argon2Params, salt, key []byte, err error) {
	parts := strings.Split(encoded, "$")
	// Expected: "", "argon2id", "v=19", "m=...,t=...,p=...", salt, key
	if len(parts) != 6 {
		return p, nil, nil, errBadArgonHash
	}
	if parts[1] != "argon2id" {
		return p, nil, nil, errBadArgonHash
	}
	if _, err := fmt.Sscanf(parts[2], "v=%d", &p.version); err != nil {
		return p, nil, nil, errBadArgonHash
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.memory, &p.time, &p.threads); err != nil {
		return p, nil, nil, errBadArgonHash
	}
	salt, err = base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return p, nil, nil, errBadArgonHash
	}
	key, err = base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return p, nil, nil, errBadArgonHash
	}
	return p, salt, key, nil
}
