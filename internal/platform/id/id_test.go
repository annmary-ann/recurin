package id_test

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"pgregory.net/rapid"

	"github.com/annmary-ann/recurin/internal/platform/id"
)

func TestNewHasPrefixAndRoundTrips(t *testing.T) {
	c := id.New[id.Customer]()
	s := c.String()
	if !strings.HasPrefix(s, "cus_") || len(s) != len("cus_")+26 {
		t.Fatalf("String() = %q", s)
	}
	if c.UUID().Version() != 7 {
		t.Fatalf("version = %d, want 7", c.UUID().Version())
	}
	back, err := id.Parse[id.Customer](s)
	if err != nil || back != c {
		t.Fatalf("Parse(%q) = %v, %v; want %v", s, back, err, c)
	}
}

func TestNewSortsInCreationOrder(t *testing.T) {
	ids := make([]string, 1000)
	for i := range ids {
		ids[i] = id.New[id.Invoice]().String()
	}
	if !slices.IsSorted(ids) {
		t.Fatal("IDs created in sequence are not in sorted order")
	}
}

// Vectors from the TypeID specification's valid.yml.
func TestTypeIDSpecVectors(t *testing.T) {
	cases := []struct{ suffix, uuid string }{
		{"01h455vb4pex5vsknk084sn02q", "01890a5d-ac96-774b-bcce-b302099a8057"},
	}
	for _, tc := range cases {
		u := uuid.MustParse(tc.uuid)
		x := id.FromUUID[id.Customer](u)
		if got := x.String(); got != "cus_"+tc.suffix {
			t.Errorf("encode %s = %q, want cus_%s", tc.uuid, got, tc.suffix)
		}
		back, err := id.Parse[id.Customer]("cus_" + tc.suffix)
		if err != nil || back.UUID() != u {
			t.Errorf("decode %s = %v, %v; want %s", tc.suffix, back.UUID(), err, tc.uuid)
		}
	}
}

func TestParseRejects(t *testing.T) {
	inv := id.New[id.Invoice]().String()
	good := id.New[id.Customer]().String()
	suffix := good[len("cus_"):]
	v4 := id.FromUUID[id.Customer](uuid.MustParse("6ba7b810-9dad-41d1-80b4-00c04fd430c8")).String()
	cases := map[string]string{
		"wrong prefix":      inv,
		"no prefix":         suffix,
		"empty":             "",
		"upper-case prefix": "CUS_" + suffix,
		"upper-case suffix": "cus_" + strings.ToUpper(suffix),
		"short":             good[:len(good)-1],
		"long":              good + "0",
		"bad character":     "cus_" + "u" + suffix[1:],
		"over 128 bits":     "cus_8" + suffix[1:],
		"not v7":            v4,
		"nil uuid":          "cus_00000000000000000000000000",
	}
	for name, s := range cases {
		t.Run(name, func(t *testing.T) {
			if got, err := id.Parse[id.Customer](s); !errors.Is(err, id.ErrInvalid) {
				t.Fatalf("Parse(%q) = %v, %v; want ErrInvalid", s, got, err)
			}
		})
	}
}

func TestZeroID(t *testing.T) {
	var z id.SubscriptionID
	if !z.IsZero() || z.String() != "" {
		t.Fatalf("zero ID: IsZero=%v String=%q", z.IsZero(), z.String())
	}
	if v, err := z.Value(); v != nil || err != nil {
		t.Fatalf("zero Value() = %v, %v; want NULL", v, err)
	}
	if z.Prefix() != "sub" {
		t.Fatalf("Prefix() = %q", z.Prefix())
	}
}

func TestJSON(t *testing.T) {
	type invoice struct {
		ID       id.InvoiceID  `json:"id"`
		Customer id.CustomerID `json:"customer"`
		Coupon   id.CouponID   `json:"coupon"`
	}
	in := invoice{ID: id.New[id.Invoice](), Customer: id.New[id.Customer]()}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out invoice
	if err := json.Unmarshal(b, &out); err != nil || out != in {
		t.Fatalf("round trip = %+v, %v; want %+v (json %s)", out, err, in, b)
	}
	// A customer ID in the invoice field is rejected.
	bad := strings.Replace(string(b), in.ID.String(), in.Customer.String(), 1)
	if err := json.Unmarshal([]byte(bad), &out); !errors.Is(err, id.ErrInvalid) {
		t.Fatalf("Unmarshal with swapped IDs error = %v, want ErrInvalid", err)
	}
}

func TestDatabaseRoundTrip(t *testing.T) {
	x := id.New[id.Mandate]()
	v, err := x.Value()
	if err != nil {
		t.Fatal(err)
	}
	if v != x.UUID().String() {
		t.Fatalf("Value() = %v, want the plain UUID %s", v, x.UUID())
	}
	raw := x.UUID()
	for name, src := range map[string]any{
		"text":        v,
		"text []byte": []byte(v.(string)),
		"raw bytes":   raw[:],
	} {
		var got id.MandateID
		if err := got.Scan(src); err != nil || got != x {
			t.Fatalf("Scan(%s) = %v, %v; want %v", name, got, err, x)
		}
	}
	var got id.MandateID
	if err := got.Scan(nil); err != nil || !got.IsZero() {
		t.Fatalf("Scan(nil) = %v, %v; want zero", got, err)
	}
	if err := got.Scan(42); !errors.Is(err, id.ErrInvalid) {
		t.Fatalf("Scan(int) error = %v, want ErrInvalid", err)
	}
}

func TestPropertyEncodingRoundTrips(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		var u uuid.UUID
		copy(u[:], rapid.SliceOfN(rapid.Byte(), 16, 16).Draw(t, "bytes"))
		// Make it a v7 UUID so Parse accepts it.
		u[6] = u[6]&0x0f | 0x70
		u[8] = u[8]&0x3f | 0x80
		if u == uuid.Nil {
			return
		}
		x := id.FromUUID[id.Price](u)
		back, err := id.Parse[id.Price](x.String())
		if err != nil || back != x {
			t.Fatalf("Parse(%q) = %v, %v; want %v", x.String(), back, err, x)
		}
	})
}

// Encoding preserves byte order, so text IDs sort like the UUIDs they hold.
func TestPropertyEncodingPreservesOrder(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		var a, b uuid.UUID
		copy(a[:], rapid.SliceOfN(rapid.Byte(), 16, 16).Draw(t, "a"))
		copy(b[:], rapid.SliceOfN(rapid.Byte(), 16, 16).Draw(t, "b"))
		sa := id.FromUUID[id.Plan](a).String()
		sb := id.FromUUID[id.Plan](b).String()
		if a == uuid.Nil || b == uuid.Nil {
			return
		}
		if (strings.Compare(string(a[:]), string(b[:])) < 0) != (sa < sb) {
			t.Fatalf("order differs: %s vs %s, %s vs %s", a, b, sa, sb)
		}
	})
}
