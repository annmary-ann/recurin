package money

import (
	"database/sql/driver"
	"errors"
	"fmt"
)

// Currency is an ISO 4217 currency that recurin supports. The zero value is
// not a currency: arithmetic on it returns ErrInvalidCurrency.
//
// Currency is a closed set so an unsupported code is rejected at the edge
// (ParseCurrency, Scan) and never reaches billing logic.
type Currency uint8

// Supported currencies. INR is the primary currency; the others exist so the
// exponent handling (0 and 2 minor-unit digits) is exercised from day one.
const (
	INR Currency = iota + 1 // Indian rupee, 2 digits (paise)
	USD                     // US dollar, 2 digits (cents)
	EUR                     // Euro, 2 digits
	GBP                     // Pound sterling, 2 digits
	JPY                     // Japanese yen, 0 digits
)

type currencyInfo struct {
	code     string
	exponent int
}

var currencies = [...]currencyInfo{
	INR: {"INR", 2},
	USD: {"USD", 2},
	EUR: {"EUR", 2},
	GBP: {"GBP", 2},
	JPY: {"JPY", 0},
}

// ErrInvalidCurrency is returned for the zero Currency or an unsupported code.
var ErrInvalidCurrency = errors.New("money: invalid or unsupported currency")

// ParseCurrency returns the currency for an upper-case ISO 4217 code.
func ParseCurrency(code string) (Currency, error) {
	for c := INR; int(c) < len(currencies); c++ {
		if currencies[c].code == code {
			return c, nil
		}
	}
	return 0, fmt.Errorf("%w: %q", ErrInvalidCurrency, code)
}

// Valid reports whether c is a supported currency.
func (c Currency) Valid() bool { return c > 0 && int(c) < len(currencies) }

// Code returns the ISO 4217 code, or "" for an invalid currency.
func (c Currency) Code() string {
	if !c.Valid() {
		return ""
	}
	return currencies[c].code
}

// Exponent returns the number of minor-unit digits: 2 for INR (100 paise to
// the rupee), 0 for JPY. It returns 0 for an invalid currency.
func (c Currency) Exponent() int {
	if !c.Valid() {
		return 0
	}
	return currencies[c].exponent
}

// String returns the ISO code, or "Currency(n)" for an invalid value.
func (c Currency) String() string {
	if !c.Valid() {
		return fmt.Sprintf("Currency(%d)", uint8(c))
	}
	return currencies[c].code
}

// MarshalText encodes the currency as its ISO code.
func (c Currency) MarshalText() ([]byte, error) {
	if !c.Valid() {
		return nil, fmt.Errorf("%w: %d", ErrInvalidCurrency, uint8(c))
	}
	return []byte(currencies[c].code), nil
}

// UnmarshalText decodes an ISO code.
func (c *Currency) UnmarshalText(b []byte) error {
	parsed, err := ParseCurrency(string(b))
	if err != nil {
		return err
	}
	*c = parsed
	return nil
}

// Value stores the currency as its ISO code (a char(3) column).
func (c Currency) Value() (driver.Value, error) {
	b, err := c.MarshalText()
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// Scan reads a currency from a char(3) or text column.
func (c *Currency) Scan(src any) error {
	switch v := src.(type) {
	case string:
		return c.UnmarshalText([]byte(v))
	case []byte:
		return c.UnmarshalText(v)
	default:
		return fmt.Errorf("%w: cannot scan %T", ErrInvalidCurrency, src)
	}
}
