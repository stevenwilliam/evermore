// Package security holds password hashing, token issuing and TOTP.
package security

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2Params are the argon2id cost parameters. CLAUDE.md §4 requires
// argon2id; these values follow the OWASP Password Storage Cheat Sheet's
// second recommended configuration (64 MiB, t=3, p=4).
type Argon2Params struct {
	Memory      uint32
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

// DefaultArgon2 is what this service uses unless a caller says otherwise.
var DefaultArgon2 = Argon2Params{
	Memory:      64 * 1024,
	Iterations:  3,
	Parallelism: 4,
	SaltLength:  16,
	KeyLength:   32,
}

var (
	ErrInvalidHash        = errors.New("security: hash is not in the expected format")
	ErrIncompatibleParams = errors.New("security: hash was produced by a different argon2 version")
)

// HashPassword returns a PHC-format argon2id hash. The format carries the
// parameters, so a later cost increase can re-hash on next login without
// invalidating existing passwords.
func HashPassword(password string) (string, error) {
	return HashPasswordWith(password, DefaultArgon2)
}

func HashPasswordWith(password string, p Argon2Params) (string, error) {
	if password == "" {
		return "", errors.New("security: refusing to hash an empty password")
	}
	salt := make([]byte, p.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, p.Iterations, p.Memory, p.Parallelism, p.KeyLength)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.Memory, p.Iterations, p.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key)), nil
}

// VerifyPassword checks a password against a PHC hash in constant time.
func VerifyPassword(password, encoded string) (bool, error) {
	p, salt, key, err := decodeHash(encoded)
	if err != nil {
		return false, err
	}
	other := argon2.IDKey([]byte(password), salt, p.Iterations, p.Memory, p.Parallelism, uint32(len(key)))
	// ConstantTimeCompare, so the time taken does not reveal how much of the
	// hash matched.
	return subtle.ConstantTimeCompare(key, other) == 1, nil
}

// NeedsRehash reports whether a stored hash was made with weaker parameters
// than the current default, so a successful login can transparently upgrade.
func NeedsRehash(encoded string) bool {
	p, _, _, err := decodeHash(encoded)
	if err != nil {
		return true
	}
	d := DefaultArgon2
	return p.Memory < d.Memory || p.Iterations < d.Iterations || p.Parallelism < d.Parallelism
}

func decodeHash(encoded string) (p Argon2Params, salt, key []byte, err error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return p, nil, nil, ErrInvalidHash
	}
	var version int
	if _, err = fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return p, nil, nil, ErrInvalidHash
	}
	if version != argon2.Version {
		return p, nil, nil, ErrIncompatibleParams
	}
	if _, err = fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.Memory, &p.Iterations, &p.Parallelism); err != nil {
		return p, nil, nil, ErrInvalidHash
	}
	if salt, err = base64.RawStdEncoding.Strict().DecodeString(parts[4]); err != nil {
		return p, nil, nil, ErrInvalidHash
	}
	if key, err = base64.RawStdEncoding.Strict().DecodeString(parts[5]); err != nil {
		return p, nil, nil, ErrInvalidHash
	}
	p.SaltLength = uint32(len(salt))
	p.KeyLength = uint32(len(key))
	return p, salt, key, nil
}
