package money_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/annmary-ann/recurin/internal/platform/money"
)

func TestParseCurrency(t *testing.T) {
	for _, c := range []money.Currency{money.INR, money.USD, money.EUR, money.GBP, money.JPY} {
		got, err := money.ParseCurrency(c.Code())
		if err != nil || got != c {
			t.Errorf("ParseCurrency(%q) = %v, %v", c.Code(), got, err)
		}
	}
	for _, code := range []string{"", "inr", "XYZ", "INRR"} {
		if _, err := money.ParseCurrency(code); !errors.Is(err, money.ErrInvalidCurrency) {
			t.Errorf("ParseCurrency(%q) error = %v, want ErrInvalidCurrency", code, err)
		}
	}
}

func TestCurrencyExponent(t *testing.T) {
	if money.INR.Exponent() != 2 || money.JPY.Exponent() != 0 {
		t.Fatal("wrong exponents")
	}
	var zero money.Currency
	if zero.Valid() || zero.Code() != "" || zero.String() != "Currency(0)" {
		t.Fatalf("zero currency: valid=%v code=%q string=%q", zero.Valid(), zero.Code(), zero.String())
	}
}

func TestCurrencyJSON(t *testing.T) {
	b, err := json.Marshal(struct{ C money.Currency }{money.INR})
	if err != nil || string(b) != `{"C":"INR"}` {
		t.Fatalf("Marshal = %s, %v", b, err)
	}
	var v struct{ C money.Currency }
	if err := json.Unmarshal([]byte(`{"C":"USD"}`), &v); err != nil || v.C != money.USD {
		t.Fatalf("Unmarshal = %v, %v", v.C, err)
	}
	if err := json.Unmarshal([]byte(`{"C":"BTC"}`), &v); !errors.Is(err, money.ErrInvalidCurrency) {
		t.Fatalf("Unmarshal BTC error = %v", err)
	}
	if _, err := json.Marshal(struct{ C money.Currency }{}); err == nil {
		t.Fatal("marshalling the zero currency should fail")
	}
}

func TestCurrencyDatabaseRoundTrip(t *testing.T) {
	v, err := money.INR.Value()
	if err != nil || v != "INR" {
		t.Fatalf("Value = %v, %v", v, err)
	}
	var c money.Currency
	if err := c.Scan("INR"); err != nil || c != money.INR {
		t.Fatalf("Scan(string) = %v, %v", c, err)
	}
	if err := c.Scan([]byte("EUR")); err != nil || c != money.EUR {
		t.Fatalf("Scan([]byte) = %v, %v", c, err)
	}
	if err := c.Scan(nil); !errors.Is(err, money.ErrInvalidCurrency) {
		t.Fatalf("Scan(nil) error = %v", err)
	}
	if _, err := money.Currency(0).Value(); err == nil {
		t.Fatal("Value of the zero currency should fail")
	}
}
