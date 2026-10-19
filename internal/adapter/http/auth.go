package http

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/stevenwilliam/evermore/internal/app/auth"
	"github.com/stevenwilliam/evermore/internal/platform/apierror"
	"github.com/stevenwilliam/evermore/internal/platform/sanitize"
	"github.com/stevenwilliam/evermore/internal/platform/security"
)

const principalKey = "principal"

// AuthHandler serves the identity endpoints.
type AuthHandler struct {
	svc       *auth.Service
	tokens    *security.TokenIssuer
	secure    bool // set Secure on the auth cookies
	accessTTL time.Duration
}

func NewAuthHandler(svc *auth.Service, tokens *security.TokenIssuer, secure bool, accessTTL time.Duration) *AuthHandler {
	if accessTTL <= 0 {
		accessTTL = 15 * time.Minute
	}
	return &AuthHandler{svc: svc, tokens: tokens, secure: secure, accessTTL: accessTTL}
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
	// The header wins; the cookie is what makes the server-rendered surface
	// work at all, because a browser navigation carries no header (D37).
	if v, err := c.Cookie(sessionCookie); err == nil && v != "" {
		return v
	}
	return ""
}

// --------------------------------------------------------------- endpoints

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

const refreshCookie = "evermore_refresh"

// sessionCookie carries the SHORT-LIVED access token for the server-rendered
// surface (D37).
//
// D26 made the signed-in screens Go templates rather than a SPA, but auth was
// left bearer-header-only. A browser cannot put an Authorization header on a
// link click or a form POST, so /app/orders, /app/packages and /app/checkout all
// redirected to the login page for a signed-in user — proven by requesting
// them with cookies and without the header.
//
// The token is HttpOnly, so this is stricter than the previous intent of
// holding it in JavaScript memory, and its lifetime is the access token's own:
// the cookie cannot outlive the credential inside it. SameSite=Lax is what
// keeps a cross-site form POST from riding on it, which matters now that a
// cookie — not a header the attacker cannot set — carries the session.
const sessionCookie = "evermore_session"

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
	h.setSessionCookie(c, tokens.AccessToken)
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
	h.setSessionCookie(c, tokens.AccessToken)
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
	// Clear the session too, or the browser keeps navigating as the signed-in
	// user until the access token expires on its own.
	h.clearSessionCookie(c)
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

	// Onboarding (D40). Both optional in the wire format so an existing client
	// keeps working, but an address that IS supplied is validated in full —
	// a half-filled address would fail the NOT NULL columns at insert time
	// with a driver error instead of a readable message.
	Address   *registerAddress `json:"address"`
	Allergens []string         `json:"allergens"`
}

type registerAddress struct {
	Label     string   `json:"label"`
	Recipient string   `json:"recipient_name"`
	Phone     string   `json:"recipient_phone"`
	Line      string   `json:"address_line"`
	District  string   `json:"district"`
	City      string   `json:"city"`
	Province  string   `json:"province"`
	Postal    string   `json:"postal_code"`
	Note      string   `json:"driver_note"`
	PlaceID   string   `json:"google_place_id"`
	Formatted string   `json:"formatted_address"`
	Lat       *float64 `json:"latitude"`
	Lng       *float64 `json:"longitude"`
}

// toRegisterAddress applies the same rules the address screen applies.
func toRegisterAddress(in *registerAddress) (*auth.RegisterAddress, error) {
	out := &auth.RegisterAddress{}
	var err error
	if out.Label, err = sanitize.Required("label", in.Label, 40); err != nil {
		return nil, err
	}
	if out.Recipient, err = sanitize.Required("recipient_name", in.Recipient, 120); err != nil {
		return nil, err
	}
	if out.Phone, err = sanitize.Phone("recipient_phone", in.Phone); err != nil {
		return nil, err
	}
	if out.Line, err = sanitize.Required("address_line", in.Line, 240); err != nil {
		return nil, err
	}
	if out.District, err = sanitize.Required("district", in.District, 80); err != nil {
		return nil, err
	}
	if out.City, err = sanitize.Required("city", in.City, 80); err != nil {
		return nil, err
	}
	if out.Province, err = sanitize.Required("province", in.Province, 80); err != nil {
		return nil, err
	}
	if out.Postal, err = sanitize.Text("postal_code", in.Postal, 0, 10); err != nil {
		return nil, err
	}
	if out.Note, err = sanitize.Text("driver_note", in.Note, 0, 240); err != nil {
		return nil, err
	}
	if out.Formatted, err = sanitize.Text("formatted_address", in.Formatted, 0, 300); err != nil {
		return nil, err
	}
	if out.PlaceID, err = sanitize.Text("google_place_id", in.PlaceID, 0, 120); err != nil {
		return nil, err
	}
	if in.Lat == nil || in.Lng == nil {
		return nil, apierror.Validation("Tandai lokasi di peta sebelum mendaftar.")
	}
	if err := sanitize.LatLng("latitude", *in.Lat, *in.Lng); err != nil {
		return nil, apierror.From(err)
	}
	if *in.Lat < idnMinLat || *in.Lat > idnMaxLat || *in.Lng < idnMinLng || *in.Lng > idnMaxLng {
		return nil, apierror.Validation("Titik peta berada di luar Indonesia.")
	}
	out.Lat, out.Lng = *in.Lat, *in.Lng
	return out, nil
}

// Register creates a customer account.
func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, apierror.Validation(""))
		return
	}
	in := auth.RegisterInput{
		Email: req.Email, Password: req.Password,
		FullName: req.FullName, Phone: req.Phone,
	}
	if req.Address != nil {
		addr, aerr := toRegisterAddress(req.Address)
		if aerr != nil {
			Fail(c, apierror.From(aerr))
			return
		}
		in.Address = addr
	}
	// Slugs are validated inside Register, which owns the write and has the
	// allergen table in the same transaction.
	in.Allergens = req.Allergens
	_, err := h.svc.Register(c.Request.Context(), in)
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

func (h *AuthHandler) setSessionCookie(c *gin.Context, token string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   int(h.accessTTL.Seconds()),
		HttpOnly: true,
		Secure:   h.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *AuthHandler) clearSessionCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name: sessionCookie, Value: "", Path: "/",
		MaxAge: -1, HttpOnly: true, Secure: h.secure, SameSite: http.SameSiteLaxMode,
	})
}

func (h *AuthHandler) clearRefreshCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name: refreshCookie, Value: "", Path: "/api/v1/auth",
		MaxAge: -1, HttpOnly: true, Secure: h.secure, SameSite: http.SameSiteStrictMode,
	})
}
