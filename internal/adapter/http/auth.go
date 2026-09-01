package http

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/stevenwilliam/evermore/internal/app/auth"
	"github.com/stevenwilliam/evermore/internal/platform/apierror"
	"github.com/stevenwilliam/evermore/internal/platform/security"
)

const principalKey = "principal"

// AuthHandler serves the identity endpoints.
type AuthHandler struct {
	svc    *auth.Service
	tokens *security.TokenIssuer
	secure bool // set Secure on the refresh cookie
}

func NewAuthHandler(svc *auth.Service, tokens *security.TokenIssuer, secure bool) *AuthHandler {
	return &AuthHandler{svc: svc, tokens: tokens, secure: secure}
}

// Authenticate parses the bearer token and attaches the principal.
//
// It does NOT decide anything: a request with a valid token but no permission
// still reaches RequirePermission, which is the only place a decision is made.
func Authenticate(tokens *security.TokenIssuer, svc *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := bearer(c)
		if raw == "" {
			c.Next()
			return
		}
		claims, err := tokens.Verify(raw)
		if err != nil {
			// An invalid token is not "anonymous": it is a request that tried
			// to authenticate and failed, and letting it fall through as
			// anonymous would silently downgrade rather than refuse.
			Fail(c, apierror.Unauthenticated("Sesi tidak valid atau sudah berakhir."))
			return
		}
		// Permissions are re-read from the database rather than trusted from
		// the token. A role revoked one minute ago must not keep working for
		// the remaining life of an access token.
		p, err := svc.LoadPrincipal(c.Request.Context(), claims.UserID)
		if err != nil {
			Fail(c, apierror.Unauthenticated("Sesi tidak valid atau sudah berakhir."))
			return
		}
		c.Set(principalKey, p)
		c.Next()
	}
}

// RequireAuth refuses an unauthenticated request.
func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if PrincipalOf(c) == nil {
			Fail(c, apierror.Unauthenticated(""))
			return
		}
		c.Next()
	}
}

// RequirePermission is how every protected handler declares what it needs.
// Deny by default: no permission, no access, and the check is here rather
// than inside the handler so it cannot be forgotten.
func RequirePermission(perm string) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := PrincipalOf(c)
		if p == nil {
			Fail(c, apierror.Unauthenticated(""))
			return
		}
		if !p.Can(perm) {
			Fail(c, apierror.Forbidden("Anda tidak punya izin "+perm+"."))
			return
		}
		c.Next()
	}
}

// PrincipalOf returns the authenticated principal, or nil.
func PrincipalOf(c *gin.Context) *auth.Principal {
	v, ok := c.Get(principalKey)
	if !ok {
		return nil
	}
	p, _ := v.(*auth.Principal)
	return p
}

// CustomerIDOf returns the customer the request acts as. Every customer-scoped
// query takes this from the token, never from the URL — an id in a path is a
// suggestion, not an authorisation (IDOR).
func CustomerIDOf(c *gin.Context) (uuid.UUID, bool) {
	p := PrincipalOf(c)
	if p == nil || p.CustomerID == nil {
		return uuid.Nil, false
	}
	return *p.CustomerID, true
}

func bearer(c *gin.Context) string {
	h := c.GetHeader("Authorization")
	const prefix = "Bearer "
	if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return strings.TrimSpace(h[len(prefix):])
	}
	return ""
}

// --------------------------------------------------------------- endpoints

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

const refreshCookie = "evermore_refresh"

// Login issues tokens. The refresh token also goes into an HttpOnly cookie so
// a browser client never has to keep it in JavaScript-reachable storage.
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, apierror.Validation("Email dan kata sandi wajib diisi."))
		return
	}

	tokens, principal, err := h.svc.Login(c.Request.Context(), req.Email, req.Password,
		c.Request.UserAgent(), c.ClientIP())
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		Fail(c, apierror.Unauthenticated("Email atau kata sandi salah."))
		return
	case errors.Is(err, auth.ErrAccountLocked):
		Fail(c, apierror.New(http.StatusTooManyRequests, "ACCOUNT_LOCKED",
			"Terlalu banyak percobaan. Coba lagi dalam 15 menit."))
		return
	case errors.Is(err, auth.ErrAccountInactive):
		Fail(c, apierror.Forbidden("Akun ini dinonaktifkan."))
		return
	case err != nil:
		Fail(c, apierror.Internal(err))
		return
	}

	h.setRefreshCookie(c, tokens.RefreshToken)
	OK(c, http.StatusOK, gin.H{
		"access_token": tokens.AccessToken,
		"token_type":   tokens.TokenType,
		"expires_at":   tokens.ExpiresAt,
		"user": gin.H{
			"id":          principal.UserID,
			"email":       principal.Email,
			"roles":       principal.Roles,
			"is_staff":    principal.IsStaff,
			"customer_id": principal.CustomerID,
			"kitchen_id":  principal.KitchenID,
		},
	})
}

// Refresh rotates the refresh token.
func (h *AuthHandler) Refresh(c *gin.Context) {
	raw, err := c.Cookie(refreshCookie)
	if err != nil || raw == "" {
		// Fall back to the body, for non-browser clients.
		var body struct {
			RefreshToken string `json:"refresh_token"`
		}
		_ = c.ShouldBindJSON(&body)
		raw = body.RefreshToken
	}
	if raw == "" {
		Fail(c, apierror.Unauthenticated("Tidak ada sesi untuk diperbarui."))
		return
	}

	tokens, principal, err := h.svc.Refresh(c.Request.Context(), raw,
		c.Request.UserAgent(), c.ClientIP())
	if err != nil {
		h.clearRefreshCookie(c)
		Fail(c, apierror.Unauthenticated("Sesi tidak valid. Silakan masuk lagi."))
		return
	}

	h.setRefreshCookie(c, tokens.RefreshToken)
	OK(c, http.StatusOK, gin.H{
		"access_token": tokens.AccessToken,
		"token_type":   tokens.TokenType,
		"expires_at":   tokens.ExpiresAt,
		"user":         gin.H{"id": principal.UserID, "email": principal.Email, "roles": principal.Roles},
	})
}

// Logout revokes the current session.
func (h *AuthHandler) Logout(c *gin.Context) {
	if raw, err := c.Cookie(refreshCookie); err == nil && raw != "" {
		_ = h.svc.Logout(c.Request.Context(), raw)
	}
	h.clearRefreshCookie(c)
	OK(c, http.StatusOK, gin.H{"status": "logged_out"})
}

// Me returns the current principal.
func (h *AuthHandler) Me(c *gin.Context) {
	p := PrincipalOf(c)
	perms := make([]string, 0, len(p.Permissions))
	for code := range p.Permissions {
		perms = append(perms, code)
	}
	OK(c, http.StatusOK, gin.H{
		"id": p.UserID, "email": p.Email, "roles": p.Roles,
		"permissions": perms, "is_staff": p.IsStaff,
		"customer_id": p.CustomerID, "kitchen_id": p.KitchenID,
	})
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	FullName string `json:"full_name"`
	Phone    string `json:"phone"`
}

// Register creates a customer account.
func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, apierror.Validation(""))
		return
	}
	_, err := h.svc.Register(c.Request.Context(), auth.RegisterInput{
		Email: req.Email, Password: req.Password,
		FullName: req.FullName, Phone: req.Phone,
	})
	switch {
	case errors.Is(err, auth.ErrEmailTaken):
		// Registration cannot avoid revealing that an address is taken — the
		// account either gets created or it does not. The mitigation is rate
		// limiting on this endpoint, not a vague message.
		Fail(c, apierror.Conflict("EMAIL_ALREADY_REGISTERED", "Email ini sudah terdaftar."))
		return
	case err != nil:
		Fail(c, apierror.From(err))
		return
	}
	OK(c, http.StatusCreated, gin.H{"status": "registered"})
}

func (h *AuthHandler) setRefreshCookie(c *gin.Context, token string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     refreshCookie,
		Value:    token,
		Path:     "/api/v1/auth",
		MaxAge:   30 * 24 * 3600,
		HttpOnly: true,
		Secure:   h.secure,
		SameSite: http.SameSiteStrictMode,
	})
}

func (h *AuthHandler) clearRefreshCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name: refreshCookie, Value: "", Path: "/api/v1/auth",
		MaxAge: -1, HttpOnly: true, Secure: h.secure, SameSite: http.SameSiteStrictMode,
	})
}
