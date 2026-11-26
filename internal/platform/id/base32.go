package id

import (
	"encoding/binary"
	"errors"

	"github.com/google/uuid"
)

// alphabet is Crockford's base32 in lower case, as the TypeID spec uses.
const alphabet = "0123456789abcdefghjkmnpqrstvwxyz"

// encodedLen is the length of a 128-bit value in base32: 26 characters carry
// 130 bits, so the first character only holds the top 3 bits (0-7).
const encodedLen = 26

var decodeTable = func() [256]byte {
	var t [256]byte
	for i := range t {
		t[i] = 0xff
	}
	for i := range len(alphabet) {
		t[alphabet[i]] = byte(i)
	}
	return t
}()

var (
	errLength   = errors.New("suffix must be 26 characters")
	errChar     = errors.New("suffix has a character outside lower-case Crockford base32")
	errOverflow = errors.New("suffix exceeds 128 bits (first character must be 0-7)")
)

func encode(u uuid.UUID) string {
	hi, lo := split(u)
	var out [encodedLen]byte
	for i := encodedLen - 1; i >= 0; i-- {
		out[i] = alphabet[lo&31]
		lo = lo>>5 | hi<<59
		hi >>= 5
	}
	return string(out[:])
}

func decode(s string) (uuid.UUID, error) {
	if len(s) != encodedLen {
		return uuid.Nil, errLength
	}
	var hi, lo uint64
	for i := range encodedLen {
		v := decodeTable[s[i]]
		if v == 0xff {
			return uuid.Nil, errChar
		}
		if i == 0 && v > 7 {
			return uuid.Nil, errOverflow
		}
		hi = hi<<5 | lo>>59
		lo = lo<<5 | uint64(v)
	}
	var u uuid.UUID
	binary.BigEndian.PutUint64(u[:8], hi)
	binary.BigEndian.PutUint64(u[8:], lo)
	return u, nil
}

func split(u uuid.UUID) (hi, lo uint64) {
	return binary.BigEndian.Uint64(u[:8]), binary.BigEndian.Uint64(u[8:])
}
