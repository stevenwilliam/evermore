package test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"testing"

	"github.com/stevenwilliam/evermore/internal/app/seed"
)

// loginWith signs in through the given client, so the cookie jar keeps what
// the server sets. The package-level login() helper uses a jar-less client and
// therefore cannot see the session cookie at all.
func loginWith(t *testing.T, client *http.Client, base, email, password string) (int, []*http.Cookie, error) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"email": email, "password": password})
	resp, err := client.Post(base+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if resp.StatusCode != http.StatusOK {
		t.Logf("login failed body: %v", out)
	}
	// Returned from the RESPONSE, not from the jar: net/http/cookiejar keeps
	// only name and value, so flags like HttpOnly are invisible through it and
	// an assertion made against the jar would silently always pass.
	return resp.StatusCode, resp.Cookies(), nil
}

// D37: the server-rendered surface must authenticate from COOKIES ALONE.
//
// D26 made the signed-in screens Go templates rather than a SPA, but
// Authenticate kept reading only the Authorization header. A browser cannot
// put a header on a link click or a form POST, so every signed-in page
// redirected a signed-in customer straight back to the login screen. The bug
// was invisible to the existing tests because all of them send a bearer token,
// which a browser never does.
//
// This test therefore sends NO Authorization header, exactly as a browser
// would not.
func TestSignedInPagesAuthenticateFromCookiesAlone(t *testing.T) {
	seedOnce(t)
	srv := testServer(t)

	jar, jerr := cookiejar.New(nil)
	if jerr != nil {
		t.Fatal(jerr)
	}
	client := srv.Client()
	client.Jar = jar
	// Do not follow the redirect: the redirect itself is the failure we are
	// asserting against, so it has to be observable.
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}

	code, setCookies, err := loginWith(t, client, srv.URL, "sinta@example.com", seed.DemoPassword)
	if err != nil {
		t.Fatal(err)
	}
	if code != http.StatusOK {
		t.Fatalf("login: got %d, want 200", code)
	}

	var gotSession bool
	for _, ck := range setCookies {
		if ck.Name == "evermore_session" {
			gotSession = true
			if !ck.HttpOnly {
				t.Error("the session cookie must be HttpOnly: it carries the access token")
			}
			if ck.Path != "/" {
				t.Errorf("session cookie path is %q, want \"/\" — it has to reach every page", ck.Path)
			}
			if ck.SameSite != http.SameSiteLaxMode {
				t.Errorf("session cookie SameSite is %v, want Lax: a cookie-borne session must not ride a cross-site POST", ck.SameSite)
			}
		}
	}
	if !gotSession {
		t.Fatal("login set no evermore_session cookie, so no browser navigation can authenticate")
	}

	for _, path := range []string{"/app/pesanan", "/app/paket"} {
		req, _ := http.NewRequest(http.MethodGet, srv.URL+path, nil)
		// Deliberately no Authorization header.
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		loc := resp.Header.Get("Location")
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s: got %d (Location %q), want 200 — a signed-in browser was bounced to the login page",
				path, resp.StatusCode, loc)
		}
		if strings.Contains(loc, "/app/masuk") {
			t.Errorf("%s: redirected to the login page while holding a valid session cookie", path)
		}
	}
}
