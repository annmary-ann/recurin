// Package money is recurin's only representation of an amount of money.
//
// An Amount is an int64 count of a currency's minor unit (paise for INR) plus
// its Currency. There are no floats anywhere in this package (a test enforces
// it), every operation that could overflow reports it, amounts in different
// currencies never mix, and all rounding goes through one half-even function.
// See docs/adr/0003-money-as-int64-paise.md.
package money

import (
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

// Errors returned by Amount operations.
var (
	ErrCurrencyMismatch = errors.New("money: currency mismatch")
	ErrOverflow         = errors.New("money: amount overflows int64 minor units")
	ErrDivideByZero     = errors.New("money: division by zero")
	ErrInvalidRatios    = errors.New("money: allocation ratios must be non-negative and not all zero")
)

// BasisPointsPerUnit is 100%, expressed in basis points (1 bp = 0.01%).
const BasisPointsPerUnit = 10_000

// Amount is an amount of money in a currency's minor unit. The zero value has
// no currency, and arithmetic on it returns ErrInvalidCurrency.
type Amount struct {
	minor    int64
	currency Currency
}

// New returns minor units of c: New(12345, INR) is ₹123.45.
func New(minor int64, c Currency) Amount { return Amount{minor: minor, currency: c} }

// Zero returns a zero amount in c.
func Zero(c Currency) Amount { return Amount{currency: c} }

// Minor returns the amount in minor units (paise for INR).
func (a Amount) Minor() int64 { return a.minor }

// Currency returns the amount's currency.
func (a Amount) Currency() Currency { return a.currency }

// IsZero reports whether the amount is zero (in any currency).
func (a Amount) IsZero() bool { return a.minor == 0 }

// Sign returns -1, 0 or +1.
func (a Amount) Sign() int {
	switch {
	case a.minor < 0:
		return -1
	case a.minor > 0:
		return 1
	default:
		return 0
	}
}

// Equal reports whether a and b have the same currency and value.
func (a Amount) Equal(b Amount) bool { return a == b }

// Cmp compares a and b, which must share a currency: -1 if a < b, 0 if equal,
// +1 if a > b.
func (a Amount) Cmp(b Amount) (int, error) {
	if err := sameCurrency(a, b); err != nil {
		return 0, err
	}
	switch {
	case a.minor < b.minor:
		return -1, nil
	case a.minor > b.minor:
		return 1, nil
	default:
		return 0, nil
	}
}

// Add returns a + b.
func (a Amount) Add(b Amount) (Amount, error) {
	if err := sameCurrency(a, b); err != nil {
		return Amount{}, err
	}
	sum := a.minor + b.minor
	// Overflow happened if both operands share a sign the result does not.
	if (a.minor >= 0) == (b.minor >= 0) && (sum >= 0) != (a.minor >= 0) {
		return Amount{}, fmt.Errorf("%w: %s + %s", ErrOverflow, a, b)
	}
	return Amount{minor: sum, currency: a.currency}, nil
}

// Sub returns a - b.
func (a Amount) Sub(b Amount) (Amount, error) {
	if err := sameCurrency(a, b); err != nil {
		return Amount{}, err
	}
	diff := a.minor - b.minor
	// Overflow happened if the operands differ in sign and the result's sign
	// differs from a's.
	if (a.minor >= 0) != (b.minor >= 0) && (diff >= 0) != (a.minor >= 0) {
		return Amount{}, fmt.Errorf("%w: %s - %s", ErrOverflow, a, b)
	}
	return Amount{minor: diff, currency: a.currency}, nil
}

// Neg returns -a.
func (a Amount) Neg() (Amount, error) {
	return Zero(a.currency).Sub(a)
}

// Sum adds amounts, all of which must be in c. Sum of nothing is Zero(c).
func Sum(c Currency, amounts ...Amount) (Amount, error) {
	if !c.Valid() {
		return Amount{}, ErrInvalidCurrency
	}
	total := Zero(c)
	for _, x := range amounts {
		var err error
		if total, err = total.Add(x); err != nil {
			return Amount{}, err
		}
	}
	return total, nil
}

// Mul returns a × n, for example a unit price times a quantity. It is exact.
func (a Amount) Mul(n int64) (Amount, error) {
	if !a.currency.Valid() {
		return Amount{}, ErrInvalidCurrency
	}
	p := new(big.Int).Mul(big.NewInt(a.minor), big.NewInt(n))
	if !p.IsInt64() {
		return Amount{}, fmt.Errorf("%w: %s × %d", ErrOverflow, a, n)
	}
	return Amount{minor: p.Int64(), currency: a.currency}, nil
}

// MulFrac returns a × num / den, rounded half-even to the minor unit. It is
// the building block for proration (days used / days in period) and rates.
func (a Amount) MulFrac(num, den int64) (Amount, error) {
	if !a.currency.Valid() {
		return Amount{}, ErrInvalidCurrency
	}
	if den == 0 {
		return Amount{}, ErrDivideByZero
	}
	p := new(big.Int).Mul(big.NewInt(a.minor), big.NewInt(num))
	q := roundHalfEven(p, big.NewInt(den))
	if !q.IsInt64() {
		return Amount{}, fmt.Errorf("%w: %s × %d / %d", ErrOverflow, a, num, den)
	}
	return Amount{minor: q.Int64(), currency: a.currency}, nil
}

// MulBasisPoints returns a × bps / 10 000, rounded half-even: 18% GST on a is
// a.MulBasisPoints(1800).
func (a Amount) MulBasisPoints(bps int64) (Amount, error) {
	return a.MulFrac(bps, BasisPointsPerUnit)
}

// Allocate splits a into len(ratios) parts proportional to ratios, without
// creating or losing a single minor unit: the parts always sum exactly to a.
//
// Each part first gets the floor of its exact share; the minor units left over
// go one each to the parts with the largest remainders (ties to the earlier
// part). Negative amounts are split by magnitude, so every part keeps a's sign.
// A zero ratio gets a zero part. Ratios must be non-negative and not all zero.
func (a Amount) Allocate(ratios ...int64) ([]Amount, error) {
	if !a.currency.Valid() {
		return nil, ErrInvalidCurrency
	}
	total := new(big.Int)
	for _, r := range ratios {
		if r < 0 {
			return nil, fmt.Errorf("%w: got %d", ErrInvalidRatios, r)
		}
		total.Add(total, big.NewInt(r))
	}
	if total.Sign() == 0 {
		return nil, ErrInvalidRatios
	}

	magnitude := new(big.Int).Abs(big.NewInt(a.minor))
	shares := make([]*big.Int, len(ratios))
	remainders := make([]*big.Int, len(ratios))
	left := new(big.Int).Set(magnitude)
	for i, r := range ratios {
		shares[i], remainders[i] = new(big.Int).QuoRem(
			new(big.Int).Mul(magnitude, big.NewInt(r)), total, new(big.Int))
		left.Sub(left, shares[i])
	}

	// left < len(ratios), because each share lost less than one minor unit.
	for n := left.Int64(); n > 0; n-- {
		best := -1
		for i, rem := range remainders {
			if rem.Sign() > 0 && (best < 0 || rem.Cmp(remainders[best]) > 0) {
				best = i
			}
		}
		shares[best].Add(shares[best], big.NewInt(1))
		remainders[best].SetInt64(0)
	}

	parts := make([]Amount, len(ratios))
	for i, s := range shares {
		if a.minor < 0 {
			s.Neg(s)
		}
		// Each part's magnitude is at most |a|, so it fits unless a is
		// math.MinInt64, whose magnitude is still representable when negated.
		parts[i] = Amount{minor: s.Int64(), currency: a.currency}
	}
	return parts, nil
}

// String formats the amount as "INR 1234.56" (or "INR -0.05"). It is for logs
// and errors; invoices format amounts for people elsewhere.
func (a Amount) String() string {
	exp := a.currency.Exponent()
	digits := strconv.FormatInt(a.minor, 10)
	sign := ""
	if a.minor < 0 {
		sign, digits = "-", digits[1:]
	}
	if exp > 0 {
		if len(digits) <= exp {
			digits = strings.Repeat("0", exp-len(digits)+1) + digits
		}
		digits = digits[:len(digits)-exp] + "." + digits[len(digits)-exp:]
	}
	return a.currency.String() + " " + sign + digits
}

func sameCurrency(a, b Amount) error {
	if !a.currency.Valid() || !b.currency.Valid() {
		return ErrInvalidCurrency
	}
	if a.currency != b.currency {
		return fmt.Errorf("%w: %s and %s", ErrCurrencyMismatch, a.currency, b.currency)
	}
	return nil
}

// roundHalfEven returns num / den rounded to the nearest integer, with exact
// halves going to the even neighbour (banker's rounding). It is the single
// rounding point in recurin: change the policy here and nowhere else.
func roundHalfEven(num, den *big.Int) *big.Int {
	if den.Sign() < 0 {
		num, den = new(big.Int).Neg(num), new(big.Int).Neg(den)
	}
	q, r := new(big.Int).QuoRem(num, den, new(big.Int)) // truncates toward zero
	twiceRem := new(big.Int).Abs(r)
	twiceRem.Lsh(twiceRem, 1)
	// Round away from zero past the half, and at exactly half when q is odd.
	// Bit(0) is the parity for negative q too (two's complement).
	if c := twiceRem.Cmp(den); c > 0 || (c == 0 && q.Bit(0) == 1) {
		if num.Sign() < 0 {
			q.Sub(q, big.NewInt(1))
		} else {
			q.Add(q, big.NewInt(1))
		}
	}
	return q
}
