package apikey_test

import (
	"crypto/sha256"
	"errors"
	"strings"
	"testing"

	"github.com/annmary-ann/recurin/internal/platform/apikey"
)

func TestGenerateAndParse(t *testing.T) {
	for _, mode := range []apikey.Mode{apikey.Test, apikey.Live} {
		s, err := apikey.Generate(mode)
		if err != nil {
			t.Fatal(err)
		}
		wantPrefix := "sk_" + string(mode) + "_"
		if !strings.HasPrefix(s.Text, wantPrefix) || len(s.Text) != len(wantPrefix)+52 {
			t.Fatalf("Text = %q", s.Text)
		}
		if s.Hash != sha256.Sum256([]byte(s.Text)) {
			t.Fatal("Hash is not the SHA-256 of the text")
		}
		if s.Hint != wantPrefix+"..."+s.Text[len(s.Text)-4:] {
			t.Fatalf("Hint = %q", s.Hint)
		}
		gotMode, gotHash, err := apikey.Parse(s.Text)
		if err != nil || gotMode != mode || gotHash != s.Hash {
			t.Fatalf("Parse = %v, %x, %v", gotMode, gotHash, err)
		}
	}
}

func TestGenerateIsRandom(t *testing.T) {
	a, _ := apikey.Generate(apikey.Test)
	b, _ := apikey.Generate(apikey.Test)
	if a.Text == b.Text {
		t.Fatal("two keys are identical")
	}
}

func TestGenerateRejectsUnknownMode(t *testing.T) {
	if _, err := apikey.Generate("prod"); err == nil {
		t.Fatal("want an error for mode prod")
	}
}

func TestParseRejectsMalformed(t *testing.T) {
	good, _ := apikey.Generate(apikey.Live)
	body := good.Text[len("sk_live_"):]
	for name, text := range map[string]string{
		"empty":            "",
		"publishable":      "pk_live_" + body,
		"unknown mode":     "sk_prod_" + body,
		"short":            good.Text[:len(good.Text)-1],
		"long":             good.Text + "a",
		"upper case":       "sk_live_" + strings.ToUpper(body),
		"bad character":    "sk_live_u" + body[1:],
		"bearer included":  "Bearer " + good.Text,
		"trailing newline": good.Text + "\n",
	} {
		if _, _, err := apikey.Parse(text); !errors.Is(err, apikey.ErrMalformed) {
			t.Errorf("%s: Parse error = %v, want ErrMalformed", name, err)
		}
	}
}

func TestParseMode(t *testing.T) {
	if m, err := apikey.ParseMode("live"); err != nil || m != apikey.Live {
		t.Fatalf("ParseMode(live) = %v, %v", m, err)
	}
	if _, err := apikey.ParseMode("LIVE"); err == nil {
		t.Fatal("ParseMode is case-sensitive")
	}
}
