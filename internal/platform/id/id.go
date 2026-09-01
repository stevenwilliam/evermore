// Package id mints identifiers.
//
// CLAUDE.md §4: IDs are UUIDv7; human-facing codes use CSPRNG + Crockford
// base32. UUIDv7 is time-ordered, which keeps index locality on the hot
// insert paths without leaking a sequential count the way a bigserial does.
package id

import (
	"crypto/rand"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// New returns a UUIDv7.
func New() uuid.UUID {
	v, err := uuid.NewV7()
	if err != nil {
		// NewV7 only fails if the system CSPRNG fails, at which point this
		// process cannot safely mint tokens or hash passwords either.
		panic(fmt.Sprintf("id: the system CSPRNG failed: %v", err))
	}
	return v
}

// crockford is Crockford base32: no I, L, O or U, so a human reading a code
// aloud cannot turn it into a different valid code.
const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// Code returns a human-facing code of n characters drawn from a CSPRNG.
// Rejection sampling keeps the distribution uniform; taking a byte modulo 32
// would be uniform here only because 256 is a multiple of 32, but the
// rejection loop keeps it correct if the alphabet ever changes.
func Code(n int) (string, error) {
	if n <= 0 {
		return "", fmt.Errorf("id: code length must be positive, got %d", n)
	}
	var sb strings.Builder
	sb.Grow(n)
	buf := make([]byte, 1)
	// The largest multiple of the alphabet size that fits in a byte; values
	// at or above it are rejected so the distribution stays uniform.
	limit := 256 - (256 % len(crockford))
	for sb.Len() < n {
		if _, err := rand.Read(buf); err != nil {
			return "", err
		}
		if int(buf[0]) >= limit {
			continue
		}
		sb.WriteByte(crockford[int(buf[0])%len(crockford)])
	}
	return sb.String(), nil
}

// MustCode is Code for call sites where a failing CSPRNG is fatal anyway.
func MustCode(n int) string {
	c, err := Code(n)
	if err != nil {
		panic(fmt.Sprintf("id: %v", err))
	}
	return c
}
