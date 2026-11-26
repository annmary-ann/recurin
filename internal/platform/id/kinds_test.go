package id

import (
	"regexp"
	"testing"
)

// allPrefixes lists every kind. A new kind must be added here.
var allPrefixes = []string{
	Account{}.prefix(),
	Customer{}.prefix(),
	Subscription{}.prefix(),
	Invoice{}.prefix(),
	Payment{}.prefix(),
	Mandate{}.prefix(),
	Plan{}.prefix(),
	Price{}.prefix(),
	Coupon{}.prefix(),
}

// TypeID allows up to 63 lower-case letters; recurin keeps prefixes short.
var prefixPattern = regexp.MustCompile(`^[a-z]{2,8}$`)

func TestPrefixesAreValidAndUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range allPrefixes {
		if !prefixPattern.MatchString(p) {
			t.Errorf("prefix %q must be 2-8 lower-case letters", p)
		}
		if seen[p] {
			t.Errorf("prefix %q is used by two kinds", p)
		}
		seen[p] = true
	}
}
