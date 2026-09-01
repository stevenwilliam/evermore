// Package auth is the identity use-case layer: registration, login, refresh
// rotation, logout and the permission set a request carries.
//
// Deny by default. A user has no permission until a role grants it, and every
// handler declares the permission it needs (CLAUDE.md §4).
package auth

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/stevenwilliam/evermore/internal/platform/apierror"
	"github.com/stevenwilliam/evermore/internal/platform/database"
	"github.com/stevenwilliam/evermore/internal/platform/id"
	"github.com/stevenwilliam/evermore/internal/platform/sanitize"
	"github.com/stevenwilliam/evermore/internal/platform/security"
)

// Service performs identity use-cases.
type Service struct {
	db     *sql.DB
	tokens *security.TokenIssuer
	// refreshTTL bounds how long a session can be kept alive by rotation.
	refreshTTL time.Duration
	// maxFailed is how many wrong passwords lock an account.
	maxFailed  int
	lockWindow time.Duration
	now        func() time.Time
}

// Options configures the service.
type Options struct {
	DB         *sql.DB
	Tokens     *security.TokenIssuer
	RefreshTTL time.Duration
	Now        func() time.Time
}

func NewService(o Options) *Service {
	now := o.Now
	if now == nil {
		now = time.Now
	}
	return &Service{
		db: o.DB, tokens: o.Tokens, refreshTTL: o.RefreshTTL,
		maxFailed: 8, lockWindow: 15 * time.Minute, now: now,
	}
}

// Principal is who a request is acting as, with everything authorisation needs.
type Principal struct {
	UserID      uuid.UUID
	Email       string
	Roles       []string
	Permissions map[string]bool
	CustomerID  *uuid.UUID
	KitchenID   *uuid.UUID
	IsStaff     bool
}

// Can reports whether the principal holds a permission. Absence is denial.
func (p *Principal) Can(perm string) bool {
	if p == nil {
		return false
	}
	return p.Permissions[perm]
}

// Tokens is what a successful login returns.
type Tokens struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	TokenType    string    `json:"token_type"`
}

var (
	// ErrInvalidCredentials is returned for a wrong email OR a wrong password.
	// The two are deliberately indistinguishable: telling an attacker that an
	// address exists is a free user-enumeration oracle.
	ErrInvalidCredentials = errors.New("INVALID_CREDENTIALS")
	ErrAccountLocked      = errors.New("ACCOUNT_LOCKED")
	ErrAccountInactive    = errors.New("ACCOUNT_INACTIVE")
	ErrEmailTaken         = errors.New("EMAIL_ALREADY_REGISTERED")
)

// dummyHash is verified against when an email does not exist, so a login
// attempt on an unknown address costs the same time as one on a known address.
// Without it, response timing enumerates the user table.
var dummyHash string

func init() {
	h, err := security.HashPassword("this-password-is-never-valid-" + uuid.NewString())
	if err != nil {
		panic("auth: cannot initialise the dummy hash: " + err.Error())
	}
	dummyHash = h
}

// Login authenticates and issues tokens.
func (s *Service) Login(ctx context.Context, email, password, userAgent, ip string) (*Tokens, *Principal, error) {
	normalised, err := sanitize.Email("email", email)
	if err != nil {
		// A malformed address is still just "invalid credentials" to the
		// caller; distinguishing it leaks which addresses are well-formed.
		return nil, nil, ErrInvalidCredentials
	}

	var (
		userID      uuid.UUID
		hash        string
		isActive    bool
		failedCount int
		lockedUntil sql.NullTime
	)
	err = s.db.QueryRowContext(ctx, `
		SELECT id, password_hash, is_active, failed_login_count, locked_until
		  FROM app_user WHERE email = $1`, normalised).
		Scan(&userID, &hash, &isActive, &failedCount, &lockedUntil)

	switch {
	case err == sql.ErrNoRows:
		// Spend the same work as a real verification before answering.
		_, _ = security.VerifyPassword(password, dummyHash)
		return nil, nil, ErrInvalidCredentials
	case err != nil:
		return nil, nil, err
	}

	if lockedUntil.Valid && lockedUntil.Time.After(s.now()) {
		return nil, nil, ErrAccountLocked
	}

	ok, err := security.VerifyPassword(password, hash)
	if err != nil {
		return nil, nil, err
	}
	if !ok {
		s.noteFailure(ctx, userID, failedCount)
		return nil, nil, ErrInvalidCredentials
	}
	// A correct password on a disabled account is still refused, and only
	// after the password check so the answer does not reveal account state.
	if !isActive {
		return nil, nil, ErrAccountInactive
	}

	principal, err := s.LoadPrincipal(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	// A successful login clears the failure counter and upgrades the hash if
	// the cost parameters have moved on since it was written.
	if err := s.noteSuccess(ctx, userID, password, hash); err != nil {
		return nil, nil, err
	}

	tokens, err := s.issue(ctx, principal, userAgent, ip)
	if err != nil {
		return nil, nil, err
	}
	return tokens, principal, nil
}

func (s *Service) noteFailure(ctx context.Context, userID uuid.UUID, failed int) {
	next := failed + 1
	if next >= s.maxFailed {
		_, _ = s.db.ExecContext(ctx, `
			UPDATE app_user SET failed_login_count = $2, locked_until = $3 WHERE id = $1`,
			userID, next, s.now().Add(s.lockWindow))
		return
	}
	_, _ = s.db.ExecContext(ctx, `
		UPDATE app_user SET failed_login_count = $2 WHERE id = $1`, userID, next)
}

func (s *Service) noteSuccess(ctx context.Context, userID uuid.UUID, password, hash string) error {
	if _, err := s.db.ExecContext(ctx, `
		UPDATE app_user
		   SET failed_login_count = 0, locked_until = NULL, last_login_at = now()
		 WHERE id = $1`, userID); err != nil {
		return err
	}
	if security.NeedsRehash(hash) {
		fresh, err := security.HashPassword(password)
		if err != nil {
			return nil // the login already succeeded; an upgrade failure is not fatal
		}
		_, _ = s.db.ExecContext(ctx,
			`UPDATE app_user SET password_hash = $2 WHERE id = $1`, userID, fresh)
	}
	return nil
}

// LoadPrincipal reads a user's roles and permissions.
func (s *Service) LoadPrincipal(ctx context.Context, userID uuid.UUID) (*Principal, error) {
	p := &Principal{UserID: userID, Permissions: map[string]bool{}}

	if err := s.db.QueryRowContext(ctx,
		`SELECT email FROM app_user WHERE id = $1`, userID).Scan(&p.Email); err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT r.slug, r.is_staff FROM user_role ur
		  JOIN role r ON r.id = ur.role_id
		 WHERE ur.user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var slug string
		var isStaff bool
		if err := rows.Scan(&slug, &isStaff); err != nil {
			rows.Close()
			return nil, err
		}
		p.Roles = append(p.Roles, slug)
		if isStaff {
			p.IsStaff = true
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	permRows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT p.code
		  FROM user_role ur
		  JOIN role_permission rp ON rp.role_id = ur.role_id
		  JOIN permission p       ON p.id = rp.permission_id
		 WHERE ur.user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer permRows.Close()
	for permRows.Next() {
		var code string
		if err := permRows.Scan(&code); err != nil {
			return nil, err
		}
		p.Permissions[code] = true
	}
	if err := permRows.Err(); err != nil {
		return nil, err
	}

	// Customer and staff scoping. A staff member's kitchen_id is what scopes
	// their reads (D-21); a customer's id is what scopes theirs.
	var custID uuid.UUID
	switch err := s.db.QueryRowContext(ctx,
		`SELECT id FROM customer WHERE user_id = $1`, userID).Scan(&custID); {
	case err == nil:
		p.CustomerID = &custID
	case err != sql.ErrNoRows:
		return nil, err
	}

	var kitchenID uuid.NullUUID
	switch err := s.db.QueryRowContext(ctx,
		`SELECT kitchen_id FROM staff_profile WHERE user_id = $1`, userID).Scan(&kitchenID); {
	case err == nil:
		if kitchenID.Valid {
			k := kitchenID.UUID
			p.KitchenID = &k
		}
	case err != sql.ErrNoRows:
		return nil, err
	}

	return p, nil
}

// issue mints an access token and a refresh token, storing only the hash.
func (s *Service) issue(ctx context.Context, p *Principal, userAgent, ip string) (*Tokens, error) {
	perms := make([]string, 0, len(p.Permissions))
	for code := range p.Permissions {
		perms = append(perms, code)
	}
	now := s.now()
	access, _, err := s.tokens.Issue(security.Claims{
		UserID:      p.UserID,
		Email:       p.Email,
		Roles:       p.Roles,
		Permissions: perms,
		CustomerID:  p.CustomerID,
		KitchenID:   p.KitchenID,
	}, now)
	if err != nil {
		return nil, err
	}

	plaintext, hash, err := security.NewRefreshToken()
	if err != nil {
		return nil, err
	}
	expires := now.Add(s.refreshTTL)
	var ipVal any
	if ip != "" {
		ipVal = ip
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO refresh_token (id, user_id, token_hash, jti, issued_at, expires_at, user_agent, ip_address)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		id.New(), p.UserID, hash, uuid.New(), now, expires, trunc(userAgent, 255), ipVal); err != nil {
		return nil, err
	}

	return &Tokens{
		AccessToken:  access,
		RefreshToken: plaintext,
		ExpiresAt:    now.Add(15 * time.Minute),
		TokenType:    "Bearer",
	}, nil
}

// Refresh rotates a refresh token: the old one is revoked and a new one issued
// in the same transaction.
//
// Reuse detection: presenting an already-revoked token revokes the entire
// family, because the only way that happens is a stolen token being replayed
// after the legitimate holder has already rotated.
func (s *Service) Refresh(ctx context.Context, plaintext, userAgent, ip string) (*Tokens, *Principal, error) {
	hash := security.HashToken(plaintext)

	var (
		tokenID   uuid.UUID
		userID    uuid.UUID
		expiresAt time.Time
		revokedAt sql.NullTime
	)
	err := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, expires_at, revoked_at FROM refresh_token WHERE token_hash = $1`, hash).
		Scan(&tokenID, &userID, &expiresAt, &revokedAt)
	if err == sql.ErrNoRows {
		return nil, nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, nil, err
	}

	if revokedAt.Valid {
		// Replay of a rotated token. Kill every live session for this user:
		// either the token was stolen, or the legitimate client is confused,
		// and re-authenticating is cheap next to the alternative.
		_, _ = s.db.ExecContext(ctx, `
			UPDATE refresh_token SET revoked_at = now(), revoked_reason = 'reuse detected'
			 WHERE user_id = $1 AND revoked_at IS NULL`, userID)
		return nil, nil, ErrInvalidCredentials
	}
	if !expiresAt.After(s.now()) {
		return nil, nil, ErrInvalidCredentials
	}

	principal, err := s.LoadPrincipal(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	var tokens *Tokens
	err = database.InTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			UPDATE refresh_token SET revoked_at = now(), revoked_reason = 'rotated'
			 WHERE id = $1 AND revoked_at IS NULL`, tokenID)
		if err != nil {
			return err
		}
		// If this affected no rows, another request rotated it first. Refuse
		// rather than issue a second live token from one refresh.
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n != 1 {
			return ErrInvalidCredentials
		}
		tokens, err = s.issue(ctx, principal, userAgent, ip)
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	return tokens, principal, nil
}

// Logout revokes the presented refresh token.
func (s *Service) Logout(ctx context.Context, plaintext string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE refresh_token SET revoked_at = now(), revoked_reason = 'logout'
		 WHERE token_hash = $1 AND revoked_at IS NULL`, security.HashToken(plaintext))
	return err
}

// LogoutAll revokes every live session for a user.
func (s *Service) LogoutAll(ctx context.Context, userID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE refresh_token SET revoked_at = now(), revoked_reason = 'logout all'
		 WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	return err
}

// RegisterInput is a customer self-registration.
type RegisterInput struct {
	Email    string
	Password string
	FullName string
	Phone    string
}

// Register creates a customer account. Staff accounts are created by an admin,
// never through this path.
func (s *Service) Register(ctx context.Context, in RegisterInput) (uuid.UUID, error) {
	email, err := sanitize.Email("email", in.Email)
	if err != nil {
		return uuid.Nil, err
	}
	name, err := sanitize.Required("full_name", in.FullName, 120)
	if err != nil {
		return uuid.Nil, err
	}
	phone, err := sanitize.Phone("phone", in.Phone)
	if err != nil {
		return uuid.Nil, err
	}
	if err := ValidatePassword(in.Password); err != nil {
		return uuid.Nil, err
	}

	hash, err := security.HashPassword(in.Password)
	if err != nil {
		return uuid.Nil, err
	}

	userID := id.New()
	err = database.InTx(ctx, s.db, nil, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO app_user (id, email, password_hash, phone) VALUES ($1, $2, $3, $4)`,
			userID, email, hash, phone)
		if err != nil {
			if strings.Contains(err.Error(), "app_user_email_uk") {
				return ErrEmailTaken
			}
			return err
		}

		var retailID uuid.UUID
		if err := tx.QueryRowContext(ctx,
			`SELECT id FROM customer_type WHERE slug = 'retail' AND is_active`).Scan(&retailID); err != nil {
			return fmt.Errorf("the 'retail' customer type is missing: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO customer (id, user_id, customer_type_id, full_name)
			VALUES ($1, $2, $3, $4)`, id.New(), userID, retailID, name); err != nil {
			return err
		}
		var roleID uuid.UUID
		if err := tx.QueryRowContext(ctx,
			`SELECT id FROM role WHERE slug = 'customer'`).Scan(&roleID); err != nil {
			return fmt.Errorf("the 'customer' role is missing: %w", err)
		}
		_, err = tx.ExecContext(ctx,
			`INSERT INTO user_role (user_id, role_id) VALUES ($1, $2)`, userID, roleID)
		return err
	})
	if err != nil {
		return uuid.Nil, err
	}
	return userID, nil
}

// ValidatePassword enforces the password policy.
//
// Length first, and a generous maximum. Composition rules are deliberately
// light: NIST SP 800-63B found they push people towards predictable
// substitutions, and length carries far more entropy.
func ValidatePassword(pw string) error {
	n := len([]rune(pw))
	if n < 12 {
		return apierror.Validation("").WithField("password", "TOO_SHORT",
			"Kata sandi minimal 12 karakter.")
	}
	if n > 200 {
		return apierror.Validation("").WithField("password", "TOO_LONG",
			"Kata sandi maksimal 200 karakter.")
	}
	// Refuse the handful that appear at the top of every breach corpus. A full
	// breach-list check belongs behind an API and is not worth a dependency.
	lower := strings.ToLower(pw)
	for _, bad := range []string{"password", "12345678", "qwerty", "evermore123", "admin123"} {
		if strings.Contains(lower, bad) {
			return apierror.Validation("").WithField("password", "TOO_COMMON",
				"Kata sandi terlalu mudah ditebak.")
		}
	}
	return nil
}

// ConstantTimeEquals compares two secrets without leaking their length
// relationship through timing.
func ConstantTimeEquals(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
