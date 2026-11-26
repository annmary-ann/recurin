// Package apikey makes and checks recurin's secret API keys.
//
// A key looks like sk_test_<52 characters> or sk_live_<52 characters>: the
// mode, then 32 random bytes (256 bits) in lower-case Crockford base32. Only
// the SHA-256 hash of the whole key is stored. A slow password hash (bcrypt,
// argon2) is unnecessary because the secret is random and long, not chosen by
// a person; SHA-256 lets the server find a key with one indexed lookup.
package apikey

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"fmt"
	"strings"
)

// Mode separates test data from live data. A test key never touches live
// objects and never moves real money.
type Mode string

// Modes.
const (
	Test Mode = "test"
	Live Mode = "live"
)

// ParseMode accepts "test" or "live".
func ParseMode(s string) (Mode, error) {
	switch m := Mode(s); m {
	case Test, Live:
		return m, nil
	default:
		return "", fmt.Errorf("apikey: mode must be test or live, got %q", s)
	}
}

const (
	randomBytes = 32
	// encodedLen is 32 bytes in unpadded base32.
	encodedLen = (randomBytes*8 + 4) / 5
)

var encoding = base32.NewEncoding("0123456789abcdefghjkmnpqrstvwxyz").WithPadding(base32.NoPadding)

// ErrMalformed means the text is not shaped like a recurin secret key.
var ErrMalformed = errors.New("apikey: not a recurin secret key (sk_test_... or sk_live_...)")

// Hash is the SHA-256 of a whole secret key, as stored.
type Hash [sha256.Size]byte

// Secret is a newly generated key. Text is shown to the user once and never
// stored; Hash and Hint are stored.
type Secret struct {
	Text string
	Mode Mode
	Hash Hash
	Hint string
}

// Generate makes a new secret key for mode.
func Generate(mode Mode) (Secret, error) {
	if _, err := ParseMode(string(mode)); err != nil {
		return Secret{}, err
	}
	b := make([]byte, randomBytes)
	_, _ = rand.Read(b) // crypto/rand never fails
	text := prefix(mode) + encoding.EncodeToString(b)
	return Secret{Text: text, Mode: mode, Hash: sha256.Sum256([]byte(text)), Hint: hint(text, mode)}, nil
}

// Parse checks the shape of a presented key and returns its mode and hash.
// It does not say whether the key exists; that is a database lookup.
func Parse(text string) (Mode, Hash, error) {
	var mode Mode
	switch {
	case strings.HasPrefix(text, prefix(Test)):
		mode = Test
	case strings.HasPrefix(text, prefix(Live)):
		mode = Live
	default:
		return "", Hash{}, ErrMalformed
	}
	body := text[len(prefix(mode)):]
	if len(body) != encodedLen {
		return "", Hash{}, ErrMalformed
	}
	if _, err := encoding.DecodeString(body); err != nil {
		return "", Hash{}, ErrMalformed
	}
	return mode, sha256.Sum256([]byte(text)), nil
}

func prefix(m Mode) string { return "sk_" + string(m) + "_" }

// HintOf returns the display hint (sk_test_...a1b2) for a key that Parse
// accepted. It never reveals more than the last four characters.
func HintOf(text string) string {
	mode, _, err := Parse(text)
	if err != nil {
		return "sk_..."
	}
	return hint(text, mode)
}

func hint(text string, m Mode) string {
	return prefix(m) + "..." + text[len(text)-4:]
}
