// Package id provides recurin's typed, prefixed identifiers.
//
// Every public ID is a type prefix plus a UUIDv7, written as the prefix, an
// underscore and the UUID in 26 lower-case Crockford base32 characters:
//
//	cus_01jd8x3k5fe2s9v7q4w6ytbn0m
//
// This text form matches the TypeID specification (github.com/jetify-com/typeid).
// In Postgres the value is a plain uuid column; the prefix exists only at the
// boundary. Each kind of ID is its own Go type (CustomerID, InvoiceID, ...),
// so passing an invoice ID where a customer ID is expected does not compile,
// and parsing "inv_..." as a CustomerID fails before any query runs.
// See docs/adr/0005-type-prefixed-uuidv7-ids.md.
package id

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// Kind is the type parameter that names an ID's prefix. Its method is
// unexported, so only this package can define kinds: the prefixes are part of
// the public API contract and live in one list (kinds.go).
type Kind interface {
	prefix() string
}

// ID is an identifier of kind K. The zero value is "no ID": IsZero reports
// true, it encodes as SQL NULL, and it never parses from text.
type ID[K Kind] struct {
	u uuid.UUID
}

// ErrInvalid is returned when text or a database value is not a valid ID of
// the expected kind.
var ErrInvalid = errors.New("id: invalid ID")

// New returns a fresh ID of kind K. IDs from one process sort in creation
// order. The timestamp comes from the system clock, not clock.Clock: an ID's
// embedded time is for index locality, never for billing decisions.
func New[K Kind]() ID[K] {
	return ID[K]{u: uuid.Must(uuid.NewV7())}
}

// FromUUID wraps a UUID read from storage (for example a sqlc-generated row).
// It does not check the UUID version; data in the database is trusted.
func FromUUID[K Kind](u uuid.UUID) ID[K] { return ID[K]{u: u} }

// Parse reads the text form of an ID of kind K, such as "cus_01jd...".
// It rejects a different prefix, a malformed suffix and a UUID that is not v7.
func Parse[K Kind](s string) (ID[K], error) {
	var k K
	want := k.prefix()
	p, suffix, ok := strings.Cut(s, "_")
	if !ok || p != want {
		return ID[K]{}, fmt.Errorf("%w: %q is not a %s_ ID", ErrInvalid, s, want)
	}
	u, err := decode(suffix)
	if err != nil {
		return ID[K]{}, fmt.Errorf("%w: %q: %w", ErrInvalid, s, err)
	}
	if u.Version() != 7 || u.Variant() != uuid.RFC4122 {
		return ID[K]{}, fmt.Errorf("%w: %q is not a UUIDv7", ErrInvalid, s)
	}
	return ID[K]{u: u}, nil
}

// Prefix returns the kind's prefix, such as "cus".
func (ID[K]) Prefix() string {
	var k K
	return k.prefix()
}

// IsZero reports whether the ID is unset.
func (x ID[K]) IsZero() bool { return x.u == uuid.Nil }

// UUID returns the underlying UUID, for storage.
func (x ID[K]) UUID() uuid.UUID { return x.u }

// String returns the text form, such as "cus_01jd8x3k5fe2s9v7q4w6ytbn0m".
// The zero ID returns "".
func (x ID[K]) String() string {
	if x.IsZero() {
		return ""
	}
	return x.Prefix() + "_" + encode(x.u)
}

// MarshalText encodes the ID in its text form (used by encoding/json).
// The zero ID encodes as an empty string.
func (x ID[K]) MarshalText() ([]byte, error) { return []byte(x.String()), nil }

// UnmarshalText parses the text form. It accepts an empty string as the zero
// ID, so an absent optional ID round-trips; required IDs are checked by the
// API layer.
func (x *ID[K]) UnmarshalText(b []byte) error {
	if len(b) == 0 {
		*x = ID[K]{}
		return nil
	}
	parsed, err := Parse[K](string(b))
	if err != nil {
		return err
	}
	*x = parsed
	return nil
}

// Value stores the ID in a uuid column. The zero ID is stored as NULL.
func (x ID[K]) Value() (driver.Value, error) {
	if x.IsZero() {
		return nil, nil
	}
	return x.u.String(), nil
}

// Scan reads the ID from a uuid column. NULL scans as the zero ID.
func (x *ID[K]) Scan(src any) error {
	if src == nil {
		*x = ID[K]{}
		return nil
	}
	var u uuid.UUID
	if err := u.Scan(src); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	*x = ID[K]{u: u}
	return nil
}
