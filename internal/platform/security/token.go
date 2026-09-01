package security

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Claims is the access-token payload. Roles and permissions travel in the
// token so that authorisation does not need a database round trip on every
// request, and the short TTL is what bounds how stale they can be.
type Claims struct {
	jwt.RegisteredClaims
	UserID      uuid.UUID  `json:"uid"`
	Email       string     `json:"email"`
	Roles       []string   `json:"roles"`
	Permissions []string   `json:"perms"`
	CustomerID  *uuid.UUID `json:"cid,omitempty"`
	KitchenID   *uuid.UUID `json:"kid,omitempty"`
	TOTPPassed  bool       `json:"totp,omitempty"`
}

// TokenIssuer mints and verifies access tokens.
type TokenIssuer struct {
	key      []byte
	issuer   string
	audience string
	ttl      time.Duration
}

func NewTokenIssuer(signingKey, issuer, audience string, ttl time.Duration) (*TokenIssuer, error) {
	if len(signingKey) < 32 {
		return nil, fmt.Errorf("security: signing key is %d bytes, at least 32 are required", len(signingKey))
	}
	return &TokenIssuer{key: []byte(signingKey), issuer: issuer, audience: audience, ttl: ttl}, nil
}

// Issue returns a signed access token and its jti.
func (ti *TokenIssuer) Issue(c Claims, now time.Time) (string, uuid.UUID, error) {
	jti := uuid.New()
	c.RegisteredClaims = jwt.RegisteredClaims{
		Issuer:    ti.issuer,
		Subject:   c.UserID.String(),
		Audience:  jwt.ClaimStrings{ti.audience},
		ExpiresAt: jwt.NewNumericDate(now.Add(ti.ttl)),
		NotBefore: jwt.NewNumericDate(now.Add(-30 * time.Second)),
		IssuedAt:  jwt.NewNumericDate(now),
		ID:        jti.String(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	signed, err := tok.SignedString(ti.key)
	return signed, jti, err
}

// ErrTokenInvalid is returned for any token that does not verify. The reason
// is deliberately not distinguished for the caller: telling an attacker
// whether a token was expired or forged is free information.
var ErrTokenInvalid = errors.New("security: token is not valid")

// Verify parses and validates a token.
func (ti *TokenIssuer) Verify(raw string) (*Claims, error) {
	var c Claims
	_, err := jwt.ParseWithClaims(raw, &c, func(t *jwt.Token) (any, error) {
		// Pin the algorithm. Accepting whatever the token's header claims is
		// how the alg=none and RS256->HS256 confusion attacks work.
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method %v", t.Header["alg"])
		}
		return ti.key, nil
	},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(ti.issuer),
		jwt.WithAudience(ti.audience),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, ErrTokenInvalid
	}
	return &c, nil
}

// NewRefreshToken returns an opaque refresh token and the hash to store.
// The plaintext is never persisted: a database read must not yield a usable
// token (CLAUDE.md §4, "stored hashed and revocable").
func NewRefreshToken() (plaintext, hash string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	plaintext = base64.RawURLEncoding.EncodeToString(buf)
	return plaintext, HashToken(plaintext), nil
}

// HashToken is the one-way function used for refresh tokens. SHA-256 is right
// here and argon2 would be wrong: the input is 256 bits of CSPRNG output, so
// there is no dictionary to slow down, and the check is on a hot path.
func HashToken(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}
