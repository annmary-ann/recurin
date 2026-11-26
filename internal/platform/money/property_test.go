package money_test

import (
	"errors"
	"math"
	"math/big"
	"testing"

	"pgregory.net/rapid"

	"github.com/annmary-ann/recurin/internal/platform/money"
)

// minorUnits draws amounts across the whole int64 range, with the extremes
// and small values (where rounding edge cases live) drawn more often.
func minorUnits() *rapid.Generator[int64] {
	return rapid.OneOf(
		rapid.Int64(),
		rapid.Int64Range(-1000, 1000),
		rapid.SampledFrom([]int64{math.MinInt64, math.MinInt64 + 1, -1, 0, 1, math.MaxInt64 - 1, math.MaxInt64}),
	)
}

// The core promise of Allocate: money is never created or lost.
func TestPropertyAllocateSumsToInput(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		amount := minorUnits().Draw(t, "amount")
		ratios := rapid.SliceOfN(rapid.Int64Range(0, 1_000_000_000_000), 1, 20).
			Filter(anyPositive).
			Draw(t, "ratios")

		parts, err := inr(amount).Allocate(ratios...)
		if err != nil {
			t.Fatal(err)
		}
		if len(parts) != len(ratios) {
			t.Fatalf("got %d parts for %d ratios", len(parts), len(ratios))
		}

		total := new(big.Int)
		for _, r := range ratios {
			total.Add(total, big.NewInt(r))
		}
		sum := new(big.Int)
		for i, p := range parts {
			sum.Add(sum, big.NewInt(p.Minor()))
			if p.Sign()*sign(amount) < 0 {
				t.Fatalf("part %d (%d) has the opposite sign to %d", i, p.Minor(), amount)
			}
			// Each part is within one minor unit of its exact share:
			// |part × total - amount × ratio| < total.
			diff := new(big.Int).Mul(big.NewInt(p.Minor()), total)
			diff.Sub(diff, new(big.Int).Mul(big.NewInt(amount), big.NewInt(ratios[i])))
			if diff.Abs(diff).Cmp(total) >= 0 {
				t.Fatalf("part %d = %d is not within one unit of its share", i, p.Minor())
			}
		}
		if sum.Cmp(big.NewInt(amount)) != 0 {
			t.Fatalf("parts sum to %s, want %d", sum, amount)
		}
	})
}

func TestPropertyAddSubMatchExactArithmetic(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		a := minorUnits().Draw(t, "a")
		b := minorUnits().Draw(t, "b")

		check := func(op string, got money.Amount, err error, exact *big.Int) {
			if exact.IsInt64() {
				if err != nil || got.Minor() != exact.Int64() {
					t.Fatalf("%d %s %d = %v, %v; want %s", a, op, b, got.Minor(), err, exact)
				}
				return
			}
			if !errors.Is(err, money.ErrOverflow) {
				t.Fatalf("%d %s %d = %v, %v; want ErrOverflow", a, op, b, got.Minor(), err)
			}
		}
		sum, err := inr(a).Add(inr(b))
		check("+", sum, err, new(big.Int).Add(big.NewInt(a), big.NewInt(b)))
		diff, err := inr(a).Sub(inr(b))
		check("-", diff, err, new(big.Int).Sub(big.NewInt(a), big.NewInt(b)))
	})
}

func TestPropertyMulFracIsNearestHalfEven(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		a := minorUnits().Draw(t, "a")
		num := rapid.Int64Range(-1_000_000, 1_000_000).Draw(t, "num")
		den := rapid.Int64Range(1, 1_000_000).Draw(t, "den")

		got, err := inr(a).MulFrac(num, den)
		exact := new(big.Int).Mul(big.NewInt(a), big.NewInt(num))
		// The result can overflow only if the exact quotient is out of range.
		if err != nil {
			if !errors.Is(err, money.ErrOverflow) {
				t.Fatal(err)
			}
			q := new(big.Int).Quo(exact, big.NewInt(den))
			if q.IsInt64() && q.Int64() != math.MinInt64 && q.Int64() != math.MaxInt64 {
				t.Fatalf("%d × %d/%d reported overflow but %s fits", a, num, den, q)
			}
			return
		}
		// 2|got × den - exact| <= den, and an exact tie lands on an even value.
		dist := new(big.Int).Mul(big.NewInt(got.Minor()), big.NewInt(den))
		dist.Sub(dist, exact).Abs(dist).Lsh(dist, 1)
		switch dist.Cmp(big.NewInt(den)) {
		case 1:
			t.Fatalf("%d × %d/%d = %d is not the nearest integer", a, num, den, got.Minor())
		case 0:
			if got.Minor()%2 != 0 {
				t.Fatalf("%d × %d/%d = %d: a tie must round to even", a, num, den, got.Minor())
			}
		}
	})
}

func anyPositive(xs []int64) bool {
	for _, x := range xs {
		if x > 0 {
			return true
		}
	}
	return false
}

func sign(x int64) int {
	switch {
	case x < 0:
		return -1
	case x > 0:
		return 1
	default:
		return 0
	}
}
