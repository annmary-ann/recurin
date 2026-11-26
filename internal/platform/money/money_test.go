package money_test

import (
	"errors"
	"math"
	"slices"
	"testing"

	"github.com/annmary-ann/recurin/internal/platform/money"
)

func inr(paise int64) money.Amount { return money.New(paise, money.INR) }

func TestAddSub(t *testing.T) {
	got, err := inr(10050).Add(inr(-75))
	if err != nil || !got.Equal(inr(9975)) {
		t.Fatalf("Add = %v, %v; want INR 99.75", got, err)
	}
	got, err = inr(100).Sub(inr(250))
	if err != nil || !got.Equal(inr(-150)) {
		t.Fatalf("Sub = %v, %v; want INR -1.50", got, err)
	}
}

func TestOverflowIsReported(t *testing.T) {
	cases := []struct {
		name string
		op   func() (money.Amount, error)
	}{
		{"max + 1", func() (money.Amount, error) { return inr(math.MaxInt64).Add(inr(1)) }},
		{"min + -1", func() (money.Amount, error) { return inr(math.MinInt64).Add(inr(-1)) }},
		{"min - 1", func() (money.Amount, error) { return inr(math.MinInt64).Sub(inr(1)) }},
		{"max - -1", func() (money.Amount, error) { return inr(math.MaxInt64).Sub(inr(-1)) }},
		{"0 - min", func() (money.Amount, error) { return inr(0).Sub(inr(math.MinInt64)) }},
		{"neg min", func() (money.Amount, error) { return inr(math.MinInt64).Neg() }},
		{"mul", func() (money.Amount, error) { return inr(math.MaxInt64 / 2).Mul(3) }},
		{"mulfrac", func() (money.Amount, error) { return inr(math.MaxInt64).MulFrac(3, 2) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := tc.op(); !errors.Is(err, money.ErrOverflow) {
				t.Fatalf("got %v, %v; want ErrOverflow", got, err)
			}
		})
	}
}

func TestBoundariesDoNotOverflow(t *testing.T) {
	if got, err := inr(math.MaxInt64).Add(inr(math.MinInt64)); err != nil || got.Minor() != -1 {
		t.Fatalf("max + min = %v, %v; want -1", got, err)
	}
	if got, err := inr(-1).Sub(inr(math.MaxInt64)); err != nil || got.Minor() != math.MinInt64 {
		t.Fatalf("-1 - max = %v, %v; want min", got, err)
	}
	if got, err := inr(math.MaxInt64).Neg(); err != nil || got.Minor() != -math.MaxInt64 {
		t.Fatalf("neg max = %v, %v", got, err)
	}
	// The intermediate product overflows int64 but the result does not.
	if got, err := inr(math.MaxInt64).MulFrac(3, 3); err != nil || got.Minor() != math.MaxInt64 {
		t.Fatalf("max × 3/3 = %v, %v", got, err)
	}
}

func TestCurrenciesNeverMix(t *testing.T) {
	usd := money.New(100, money.USD)
	if _, err := inr(100).Add(usd); !errors.Is(err, money.ErrCurrencyMismatch) {
		t.Fatalf("Add error = %v, want ErrCurrencyMismatch", err)
	}
	if _, err := inr(100).Cmp(usd); !errors.Is(err, money.ErrCurrencyMismatch) {
		t.Fatalf("Cmp error = %v, want ErrCurrencyMismatch", err)
	}
	if _, err := money.Sum(money.INR, inr(1), usd); !errors.Is(err, money.ErrCurrencyMismatch) {
		t.Fatalf("Sum error = %v, want ErrCurrencyMismatch", err)
	}
	if inr(100).Equal(usd) {
		t.Fatal("INR 1.00 must not equal USD 1.00")
	}
}

func TestZeroValueHasNoCurrency(t *testing.T) {
	var a money.Amount
	if _, err := a.Add(inr(1)); !errors.Is(err, money.ErrInvalidCurrency) {
		t.Fatalf("Add error = %v, want ErrInvalidCurrency", err)
	}
	if _, err := a.MulBasisPoints(1800); !errors.Is(err, money.ErrInvalidCurrency) {
		t.Fatalf("MulBasisPoints error = %v, want ErrInvalidCurrency", err)
	}
	if _, err := a.Allocate(1, 1); !errors.Is(err, money.ErrInvalidCurrency) {
		t.Fatalf("Allocate error = %v, want ErrInvalidCurrency", err)
	}
	if _, err := money.Sum(0); !errors.Is(err, money.ErrInvalidCurrency) {
		t.Fatalf("Sum error = %v, want ErrInvalidCurrency", err)
	}
}

func TestCmpAndSign(t *testing.T) {
	for _, tc := range []struct {
		a, b money.Amount
		want int
	}{{inr(1), inr(2), -1}, {inr(2), inr(2), 0}, {inr(3), inr(-2), 1}} {
		if got, err := tc.a.Cmp(tc.b); err != nil || got != tc.want {
			t.Errorf("Cmp(%v, %v) = %d, %v; want %d", tc.a, tc.b, got, err, tc.want)
		}
	}
	if inr(-5).Sign() != -1 || inr(0).Sign() != 0 || inr(5).Sign() != 1 {
		t.Error("Sign is wrong")
	}
	if !money.Zero(money.INR).IsZero() || inr(1).IsZero() {
		t.Error("IsZero is wrong")
	}
}

func TestSum(t *testing.T) {
	got, err := money.Sum(money.INR, inr(100), inr(250), inr(-50))
	if err != nil || !got.Equal(inr(300)) {
		t.Fatalf("Sum = %v, %v; want INR 3.00", got, err)
	}
	got, err = money.Sum(money.INR)
	if err != nil || !got.Equal(money.Zero(money.INR)) {
		t.Fatalf("empty Sum = %v, %v; want zero INR", got, err)
	}
}

func TestMul(t *testing.T) {
	got, err := inr(49900).Mul(3)
	if err != nil || !got.Equal(inr(149700)) {
		t.Fatalf("Mul = %v, %v; want INR 1497.00", got, err)
	}
}

func TestMulFracRoundsHalfEven(t *testing.T) {
	cases := []struct {
		minor, num, den, want int64
	}{
		{5, 1, 2, 2},    // 2.5 -> 2 (even)
		{7, 1, 2, 4},    // 3.5 -> 4 (even)
		{-5, 1, 2, -2},  // -2.5 -> -2
		{-7, 1, 2, -4},  // -3.5 -> -4
		{5, -1, 2, -2},  // negative numerator
		{5, 1, -2, -2},  // negative denominator
		{10, 1, 3, 3},   // 3.33 -> 3
		{20, 1, 3, 7},   // 6.67 -> 7
		{-20, 1, 3, -7}, // -6.67 -> -7
		{100, 0, 7, 0},
		// Proration: ₹999.00 for 17 of 31 days = 54783.87 paise -> 54784.
		{99900, 17, 31, 54784},
	}
	for _, tc := range cases {
		got, err := inr(tc.minor).MulFrac(tc.num, tc.den)
		if err != nil || got.Minor() != tc.want {
			t.Errorf("%d × %d/%d = %v, %v; want %d", tc.minor, tc.num, tc.den, got.Minor(), err, tc.want)
		}
	}
	if _, err := inr(1).MulFrac(1, 0); !errors.Is(err, money.ErrDivideByZero) {
		t.Errorf("MulFrac(1, 0) error = %v, want ErrDivideByZero", err)
	}
}

func TestMulBasisPoints(t *testing.T) {
	// 18% GST on ₹1,234.57 = 22222.26 paise -> 22222.
	got, err := inr(123457).MulBasisPoints(1800)
	if err != nil || got.Minor() != 22222 {
		t.Fatalf("18%% of INR 1234.57 = %v, %v; want 22222 paise", got, err)
	}
	// 9% on 50 paise = 4.5 -> 4 (half-even), the CGST/SGST edge case.
	got, err = inr(50).MulBasisPoints(900)
	if err != nil || got.Minor() != 4 {
		t.Fatalf("9%% of 50 paise = %v, %v; want 4", got, err)
	}
}

func TestAllocate(t *testing.T) {
	cases := []struct {
		name   string
		amount int64
		ratios []int64
		want   []int64
	}{
		{"three ways", 10000, []int64{1, 1, 1}, []int64{3334, 3333, 3333}},
		{"halves of an odd amount", 4501, []int64{1, 1}, []int64{2251, 2250}},
		{"exact split", 100, []int64{1, 2, 3, 4}, []int64{10, 20, 30, 40}},
		{"largest remainder wins over position", 5, []int64{1, 3}, []int64{1, 4}},
		{"two leftovers", 11, []int64{1, 1, 1}, []int64{4, 4, 3}},
		{"negative", -10000, []int64{1, 1, 1}, []int64{-3334, -3333, -3333}},
		{"zero ratio gets nothing", 100, []int64{0, 1, 0, 2}, []int64{0, 33, 0, 67}},
		{"single part", math.MinInt64, []int64{7}, []int64{math.MinInt64}},
		{"zero amount", 0, []int64{1, 2}, []int64{0, 0}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parts, err := inr(tc.amount).Allocate(tc.ratios...)
			if err != nil {
				t.Fatal(err)
			}
			got := make([]int64, len(parts))
			for i, p := range parts {
				got[i] = p.Minor()
				if p.Currency() != money.INR {
					t.Errorf("part %d currency = %v", i, p.Currency())
				}
			}
			if !slices.Equal(got, tc.want) {
				t.Fatalf("Allocate(%d, %v) = %v, want %v", tc.amount, tc.ratios, got, tc.want)
			}
		})
	}
}

func TestAllocateRejectsBadRatios(t *testing.T) {
	for _, ratios := range [][]int64{nil, {0, 0}, {1, -1}} {
		if _, err := inr(100).Allocate(ratios...); !errors.Is(err, money.ErrInvalidRatios) {
			t.Errorf("Allocate(%v) error = %v, want ErrInvalidRatios", ratios, err)
		}
	}
}

func TestString(t *testing.T) {
	cases := []struct {
		a    money.Amount
		want string
	}{
		{inr(123456), "INR 1234.56"},
		{inr(5), "INR 0.05"},
		{inr(-5), "INR -0.05"},
		{inr(0), "INR 0.00"},
		{inr(-100), "INR -1.00"},
		{inr(math.MinInt64), "INR -92233720368547758.08"},
		{money.New(1500, money.JPY), "JPY 1500"},
	}
	for _, tc := range cases {
		if got := tc.a.String(); got != tc.want {
			t.Errorf("String() = %q, want %q", got, tc.want)
		}
	}
}
