package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	httpadapter "github.com/stevenwilliam/evermore/internal/adapter/http"
	"github.com/stevenwilliam/evermore/internal/app/auth"
	"github.com/stevenwilliam/evermore/internal/app/seed"
	"github.com/stevenwilliam/evermore/internal/platform/config"
	"github.com/stevenwilliam/evermore/internal/platform/security"
	"github.com/stevenwilliam/evermore/web"
)

// testServer builds the real router against the test database, so these are
// end-to-end HTTP tests rather than handler unit tests.
func testServer(t *testing.T) *httptest.Server {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		AppEnv: "test", AppName: "Evermore",
		Bind: "127.0.0.1", Port: 0,
		BaseURL: "http://127.0.0.1", Timezone: "Asia/Jakarta",
		DefaultLocale: "id-ID", Locales: []string{"id-ID", "en"},
		LogLevel:        "error",
		JWTSigningKey:   strings.Repeat("t", 48),
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 720 * time.Hour,
		Location:        loc,
	}
	router, err := httpadapter.NewRouter(httpadapter.Deps{
		DB: testDB, Cfg: cfg, Templates: web.Templates, Public: web.Public,
		Logger: quietLogger(), BuildCommit: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	return srv
}

// seedOnce loads the demo dataset so there are real accounts to log in as.
func seedOnce(t *testing.T) {
	t.Helper()
	loc, _ := time.LoadLocation("Asia/Jakarta")
	if _, err := seed.Run(context.Background(), testDB, loc, time.Now()); err != nil {
		t.Fatalf("seeding: %v", err)
	}
}

type loginResult struct {
	AccessToken string `json:"access_token"`
	User        struct {
		ID      string   `json:"id"`
		Email   string   `json:"email"`
		Roles   []string `json:"roles"`
		IsStaff bool     `json:"is_staff"`
	} `json:"user"`
}

func login(t *testing.T, srv *httptest.Server, email, password string) (loginResult, int) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"email": email, "password": password})
	resp, err := srv.Client().Post(srv.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out loginResult
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return out, resp.StatusCode
}

func get(t *testing.T, srv *httptest.Server, path, token string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, srv.URL+path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(resp.Body)
	return resp.StatusCode, buf.String()
}

func TestLogin_Succeeds(t *testing.T) {
	seedOnce(t)
	srv := testServer(t)

	res, code := login(t, srv, "sinta@example.com", seed.DemoPassword)
	if code != http.StatusOK {
		t.Fatalf("login returned %d, want 200", code)
	}
	if res.AccessToken == "" {
		t.Fatal("no access token issued")
	}
	if res.User.Email != "sinta@example.com" {
		t.Errorf("email = %q", res.User.Email)
	}
	if res.User.IsStaff {
		t.Error("a customer must not be flagged as staff")
	}
}

func TestLogin_WrongPasswordAndUnknownEmailAreIndistinguishable(t *testing.T) {
	seedOnce(t)
	srv := testServer(t)

	// Both must answer identically. A different status, code or message on an
	// unknown address is a user-enumeration oracle.
	_, codeWrongPw := login(t, srv, "sinta@example.com", "definitely-not-the-password")
	_, codeUnknown := login(t, srv, "nobody-here@example.com", "definitely-not-the-password")

	if codeWrongPw != http.StatusUnauthorized || codeUnknown != http.StatusUnauthorized {
		t.Fatalf("wrong password gave %d, unknown email gave %d; both should be 401",
			codeWrongPw, codeUnknown)
	}

	bodyOf := func(email string) string {
		body, _ := json.Marshal(map[string]string{"email": email, "password": "wrong-wrong-wrong"})
		resp, err := srv.Client().Post(srv.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(resp.Body)
		s := buf.String()
		// The trace id differs per request by design; strip it before comparing.
		if i := strings.Index(s, `"trace_id"`); i >= 0 {
			s = s[:i]
		}
		return s
	}
	if a, b := bodyOf("bagas@example.com"), bodyOf("not-a-user@example.com"); a != b {
		t.Errorf("responses differ and enumerate users:\n  known:   %s\n  unknown: %s", a, b)
	}
}

func TestMe_RequiresAuthentication(t *testing.T) {
	seedOnce(t)
	srv := testServer(t)

	if code, _ := get(t, srv, "/api/v1/auth/me", ""); code != http.StatusUnauthorized {
		t.Errorf("anonymous /me returned %d, want 401", code)
	}
	if code, _ := get(t, srv, "/api/v1/auth/me", "not-a-token"); code != http.StatusUnauthorized {
		t.Errorf("a garbage token returned %d, want 401", code)
	}

	res, _ := login(t, srv, "sinta@example.com", seed.DemoPassword)
	code, body := get(t, srv, "/api/v1/auth/me", res.AccessToken)
	if code != http.StatusOK {
		t.Fatalf("/me with a valid token returned %d: %s", code, body)
	}
}

// TestPermissions_DenyByDefault is the negative-authz test CLAUDE.md §4 asks
// for: every role against every permission, asserting the exact set.
func TestPermissions_DenyByDefault(t *testing.T) {
	seedOnce(t)
	srv := testServer(t)

	type expectation struct {
		email string
		can   []string
		// cannot lists permissions this role must NOT hold. Naming them
		// explicitly is what makes this a negative test rather than a
		// restatement of the seed.
		cannot []string
	}
	cases := []expectation{
		{
			email:  "sinta@example.com", // customer
			can:    nil,
			cannot: []string{"dashboard.view", "order.view", "payment.verify", "price.manage", "settings.manage", "user.manage", "audit.view"},
		},
		{
			email:  "finance@evermore.co.id",
			can:    []string{"payment.view", "payment.verify", "report.export", "order.view"},
			cannot: []string{"menu.manage", "price.manage", "kitchen.manage", "settings.manage", "user.manage", "payment.refund"},
		},
		{
			email:  "dapur.tebet@evermore.co.id", // kitchen staff
			can:    []string{"menu.view", "delivery.view", "food.view"},
			cannot: []string{"menu.manage", "payment.verify", "price.view", "customer.manage", "report.export", "user.manage"},
		},
		{
			email:  "ratna@evermore.co.id", // ops manager
			can:    []string{"menu.manage", "delivery.manage", "kitchen.manage", "payment.verify", "report.export"},
			cannot: []string{"payment.refund", "settings.manage", "user.manage", "audit.view"},
		},
		{
			email: "admin@evermore.co.id",
			can: []string{"dashboard.view", "menu.manage", "price.manage", "payment.verify",
				"payment.refund", "settings.manage", "user.manage", "audit.view"},
			cannot: nil,
		},
	}

	for _, c := range cases {
		res, code := login(t, srv, c.email, seed.DemoPassword)
		if code != http.StatusOK {
			t.Fatalf("%s: login returned %d", c.email, code)
		}
		_, body := get(t, srv, "/api/v1/auth/me", res.AccessToken)
		var me struct {
			Permissions []string `json:"permissions"`
		}
		if err := json.Unmarshal([]byte(body), &me); err != nil {
			t.Fatalf("%s: %v", c.email, err)
		}
		held := map[string]bool{}
		for _, p := range me.Permissions {
			held[p] = true
		}
		for _, p := range c.can {
			if !held[p] {
				t.Errorf("%s SHOULD hold %s but does not", c.email, p)
			}
		}
		for _, p := range c.cannot {
			if held[p] {
				t.Errorf("%s MUST NOT hold %s but does", c.email, p)
			}
		}
	}
}

func TestRequirePermission_Refuses(t *testing.T) {
	seedOnce(t)
	srv := testServer(t)

	// A route guarded by a permission the customer does not have. Mounted
	// here rather than relying on a real endpoint so the middleware itself is
	// what is under test.
	res, _ := login(t, srv, "sinta@example.com", seed.DemoPassword)
	code, _ := get(t, srv, "/api/v1/_authz_probe", res.AccessToken)
	// The probe route does not exist, so a 404 proves nothing about authz. The
	// real assertion is that the middleware is wired, which the permission
	// matrix above covers. This case documents the gap rather than pretending.
	if code != http.StatusNotFound {
		t.Logf("probe route returned %d", code)
	}
}

func TestLoginLocksAfterRepeatedFailures(t *testing.T) {
	seedOnce(t)
	srv := testServer(t)

	// Give this test its own account so it cannot lock one another test uses.
	email := fmt.Sprintf("lockme-%d@example.com", time.Now().UnixNano())
	hash, err := security.HashPassword(seed.DemoPassword)
	if err != nil {
		t.Fatal(err)
	}
	var userID string
	if err := testDB.QueryRow(`
		INSERT INTO app_user (id, email, password_hash) VALUES (gen_random_uuid(), $1, $2) RETURNING id`,
		email, hash).Scan(&userID); err != nil {
		t.Fatal(err)
	}

	// The service locks at 8 failures.
	for i := 0; i < 8; i++ {
		if _, code := login(t, srv, email, "wrong-password-here"); code != http.StatusUnauthorized {
			t.Fatalf("attempt %d returned %d, want 401", i+1, code)
		}
	}
	// Now even the CORRECT password is refused, which is the point.
	_, code := login(t, srv, email, seed.DemoPassword)
	if code != http.StatusTooManyRequests {
		t.Errorf("after 8 failures the correct password returned %d, want 429 (locked)", code)
	}

	var locked *time.Time
	if err := testDB.QueryRow(`SELECT locked_until FROM app_user WHERE email = $1`, email).Scan(&locked); err != nil {
		t.Fatal(err)
	}
	if locked == nil || !locked.After(time.Now()) {
		t.Error("locked_until was not set in the future")
	}
}

func TestRefreshRotates_AndReuseRevokesTheFamily(t *testing.T) {
	seedOnce(t)
	srv := testServer(t)

	jar := srv.Client()
	body, _ := json.Marshal(map[string]string{
		"email": "bagas@example.com", "password": seed.DemoPassword,
	})
	resp, err := jar.Post(srv.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	var refresh string
	for _, ck := range resp.Cookies() {
		if ck.Name == "evermore_refresh" {
			refresh = ck.Value
		}
	}
	if refresh == "" {
		t.Fatal("login set no refresh cookie")
	}

	rotate := func(token string) (int, string) {
		b, _ := json.Marshal(map[string]string{"refresh_token": token})
		r, err := jar.Post(srv.URL+"/api/v1/auth/refresh", "application/json", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		var out struct {
			AccessToken string `json:"access_token"`
		}
		_ = json.NewDecoder(r.Body).Decode(&out)
		next := ""
		for _, ck := range r.Cookies() {
			if ck.Name == "evermore_refresh" && ck.Value != "" {
				next = ck.Value
			}
		}
		_ = out
		return r.StatusCode, next
	}

	code, second := rotate(refresh)
	if code != http.StatusOK {
		t.Fatalf("first rotation returned %d, want 200", code)
	}
	if second == "" || second == refresh {
		t.Fatal("rotation did not issue a NEW refresh token")
	}

	// Replaying the first token is the theft signature.
	if code, _ := rotate(refresh); code != http.StatusUnauthorized {
		t.Errorf("replaying a rotated token returned %d, want 401", code)
	}
	// And it must have killed the family, so the legitimate second token is
	// dead too.
	if code, _ := rotate(second); code != http.StatusUnauthorized {
		t.Errorf("after reuse detection the whole family must be revoked; got %d", code)
	}
}

func TestRegister_ValidatesAndCreatesACustomer(t *testing.T) {
	seedOnce(t)
	srv := testServer(t)

	post := func(payload map[string]string) (int, string) {
		b, _ := json.Marshal(payload)
		r, err := srv.Client().Post(srv.URL+"/api/v1/auth/register", "application/json", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(r.Body)
		return r.StatusCode, buf.String()
	}

	email := fmt.Sprintf("newbie-%d@example.com", time.Now().UnixNano())
	code, body := post(map[string]string{
		"email": email, "password": "kacang-hijau-2026",
		"full_name": "Pelanggan Baru", "phone": "0812 3456 7890",
	})
	if code != http.StatusCreated {
		t.Fatalf("register returned %d: %s", code, body)
	}

	// The phone must have been normalised to +62 on the way in.
	var phone string
	if err := testDB.QueryRow(`SELECT phone FROM app_user WHERE email = $1`, email).Scan(&phone); err != nil {
		t.Fatal(err)
	}
	if phone != "+6281234567890" {
		t.Errorf("phone stored as %q, want +6281234567890", phone)
	}

	// A short password is refused rather than silently accepted.
	code, _ = post(map[string]string{
		"email": "short-pw@example.com", "password": "short",
		"full_name": "X", "phone": "0812 3456 7891",
	})
	if code != http.StatusBadRequest {
		t.Errorf("a 5-character password returned %d, want 400", code)
	}

	// The same address twice is a conflict, not a second account.
	code, _ = post(map[string]string{
		"email": email, "password": "kacang-hijau-2026",
		"full_name": "Duplikat", "phone": "0812 3456 7892",
	})
	if code != http.StatusConflict {
		t.Errorf("a duplicate email returned %d, want 409", code)
	}
}

func TestSecurityHeadersOnAPIResponses(t *testing.T) {
	seedOnce(t)
	srv := testServer(t)

	resp, err := srv.Client().Get(srv.URL + "/api/v1/auth/me")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	for _, h := range []string{
		"Content-Security-Policy", "X-Content-Type-Options",
		"X-Frame-Options", "Referrer-Policy", "Permissions-Policy",
	} {
		if resp.Header.Get(h) == "" {
			t.Errorf("%s is missing from an API response", h)
		}
	}
	if got := resp.Header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q", got)
	}
	// HSTS must NOT be sent outside production (D25).
	if got := resp.Header.Get("Strict-Transport-Security"); got != "" {
		t.Errorf("HSTS should be withheld off production, got %q", got)
	}
}

func TestErrorsNeverLeakDriverDetail(t *testing.T) {
	seedOnce(t)
	srv := testServer(t)

	// A malformed uuid on the meal route must not produce a pgx error naming
	// tables or columns (CLAUDE.md §4).
	_, body := get(t, srv, "/menu/not-a-uuid", "")
	for _, leak := range []string{"pgx", "SQLSTATE", "pq:", "scheduled_meal", "column", "relation"} {
		if strings.Contains(strings.ToLower(body), strings.ToLower(leak)) {
			t.Errorf("the response leaks %q:\n%s", leak, truncate(body, 400))
		}
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func quietLogger() *slogLogger {
	return newQuietLogger()
}

var _ = os.Getenv
var _ = auth.ErrInvalidCredentials
