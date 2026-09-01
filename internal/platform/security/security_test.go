package security

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestHashPassword_RoundTrip(t *testing.T) {
	h, err := HashPassword("gado-gado-2026")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "$argon2id$") {
		t.Fatalf("hash does not carry the argon2id prefix the DB CHECK requires: %q", h)
	}
	ok, err := VerifyPassword("gado-gado-2026", h)
	if err != nil || !ok {
		t.Errorf("the correct password did not verify: %v", err)
	}
	ok, err = VerifyPassword("wrong", h)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("a wrong password verified")
	}
}

func TestHashPassword_SaltIsPerHash(t *testing.T) {
	a, _ := HashPassword("same")
	b, _ := HashPassword("same")
	if a == b {
		t.Error("two hashes of the same password are identical — the salt is not random")
	}
}

func TestHashPassword_RefusesEmpty(t *testing.T) {
	if _, err := HashPassword(""); err == nil {
		t.Error("hashing an empty password should be refused")
	}
}

func TestVerifyPassword_RejectsMalformed(t *testing.T) {
	for _, bad := range []string{"", "hunter2", "$2a$10$bcryptstyle", "$argon2id$broken"} {
		if _, err := VerifyPassword("x", bad); err == nil {
			t.Errorf("%q should not parse as a hash", bad)
		}
	}
}

func TestNeedsRehash(t *testing.T) {
	weak, err := HashPasswordWith("x", Argon2Params{
		Memory: 8 * 1024, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !NeedsRehash(weak) {
		t.Error("a hash below the current cost should be flagged for rehash")
	}
	current, _ := HashPassword("x")
	if NeedsRehash(current) {
		t.Error("a current-cost hash should not be flagged")
	}
}

func issuer(t *testing.T) *TokenIssuer {
	t.Helper()
	ti, err := NewTokenIssuer(strings.Repeat("k", 48), "evermore", "evermore-api", 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	return ti
}

func TestToken_RoundTrip(t *testing.T) {
	ti := issuer(t)
	uid := uuid.New()
	now := time.Now()
	raw, jti, err := ti.Issue(Claims{UserID: uid, Email: "a@b.com", Roles: []string{"customer"}}, now)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ti.Verify(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.UserID != uid {
		t.Errorf("uid = %s, want %s", got.UserID, uid)
	}
	if got.ID != jti.String() {
		t.Errorf("jti mismatch: %s vs %s", got.ID, jti)
	}
}

func TestToken_RejectsAlgNone(t *testing.T) {
	// The classic forgery: re-sign the payload with alg=none and hope the
	// verifier trusts the header.
	ti := issuer(t)
	raw, _, err := ti.Issue(Claims{UserID: uuid.New()}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	var c Claims
	parsed, _, err := jwt.NewParser().ParseUnverified(raw, &c)
	if err != nil {
		t.Fatal(err)
	}
	forged, err := jwt.NewWithClaims(jwt.SigningMethodNone, parsed.Claims).
		SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ti.Verify(forged); err == nil {
		t.Fatal("an alg=none token was ACCEPTED")
	}
}

func TestToken_RejectsWrongKey(t *testing.T) {
	ti := issuer(t)
	other, _ := NewTokenIssuer(strings.Repeat("z", 48), "evermore", "evermore-api", 15*time.Minute)
	raw, _, _ := other.Issue(Claims{UserID: uuid.New()}, time.Now())
	if _, err := ti.Verify(raw); err == nil {
		t.Error("a token signed with a different key was accepted")
	}
}

func TestToken_RejectsExpired(t *testing.T) {
	ti := issuer(t)
	raw, _, _ := ti.Issue(Claims{UserID: uuid.New()}, time.Now().Add(-2*time.Hour))
	if _, err := ti.Verify(raw); err == nil {
		t.Error("an expired token was accepted")
	}
}

func TestToken_RejectsWrongAudienceAndIssuer(t *testing.T) {
	ti := issuer(t)
	wrongAud, _ := NewTokenIssuer(strings.Repeat("k", 48), "evermore", "someone-else", 15*time.Minute)
	raw, _, _ := wrongAud.Issue(Claims{UserID: uuid.New()}, time.Now())
	if _, err := ti.Verify(raw); err == nil {
		t.Error("a token for another audience was accepted")
	}
	wrongIss, _ := NewTokenIssuer(strings.Repeat("k", 48), "elsewhere", "evermore-api", 15*time.Minute)
	raw, _, _ = wrongIss.Issue(Claims{UserID: uuid.New()}, time.Now())
	if _, err := ti.Verify(raw); err == nil {
		t.Error("a token from another issuer was accepted")
	}
}

func TestNewTokenIssuer_RefusesShortKey(t *testing.T) {
	if _, err := NewTokenIssuer("short", "a", "b", time.Minute); err == nil {
		t.Error("a signing key under 32 bytes should be refused")
	}
}

func TestRefreshToken_PlaintextIsNotTheStoredValue(t *testing.T) {
	plain, hash, err := NewRefreshToken()
	if err != nil {
		t.Fatal(err)
	}
	if plain == hash {
		t.Fatal("the stored value equals the token — a database read would yield a usable token")
	}
	if HashToken(plain) != hash {
		t.Error("HashToken does not reproduce the stored hash")
	}
	other, _, _ := NewRefreshToken()
	if other == plain {
		t.Error("two refresh tokens collided")
	}
}
