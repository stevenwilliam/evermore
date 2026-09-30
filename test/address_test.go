package test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"

	"github.com/stevenwilliam/evermore/internal/app/seed"
)

// D40: delivery addresses and the allergen profile.
//
// customer_address existed from the first migration and the checkout screen
// already rendered a picker, but nothing could WRITE an address — so the
// picker was permanently empty and checkout was unreachable.

func browser(t *testing.T, base string) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	c := &http.Client{Jar: jar}
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return c
}

func signIn(t *testing.T, c *http.Client, base, email, password string) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"email": email, "password": password})
	resp, err := c.Post(base+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("sign in %s: %d", email, resp.StatusCode)
	}
}

func addressForm() url.Values {
	return url.Values{
		"label": {"Rumah"}, "recipient_name": {"Uji Coba"},
		"recipient_phone": {"081234567890"},
		"address_line":    {"Jl. Tebet Raya No. 1"},
		"district":        {"Tebet"}, "city": {"Jakarta Selatan"},
		"province": {"DKI Jakarta"}, "postal_code": {"12820"},
		"latitude": {"-6.2264"}, "longitude": {"106.8553"},
	}
}

// The pin is the routing input, so it is mandatory and bounded — and both
// rules live on the SERVER, because the browser can be bypassed with curl.
func TestAddressRejectsAMissingOrImplausiblePin(t *testing.T) {
	seedOnce(t)
	srv := testServer(t)
	c := browser(t, srv.URL)
	signIn(t, c, srv.URL, "sinta@example.com", seed.DemoPassword)

	cases := []struct{ name, lat, lng, want string }{
		{"no pin at all", "", "", "Tandai lokasi"},
		{"outside Indonesia", "51.5074", "-0.1278", "luar Indonesia"},
		{"off the globe", "999", "106.8", "Lintang"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := addressForm()
			f.Set("latitude", tc.lat)
			f.Set("longitude", tc.lng)
			resp, err := c.PostForm(srv.URL+"/app/alamat", f)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			loc := resp.Header.Get("Location")
			if !strings.Contains(loc, "err=") {
				t.Fatalf("accepted %q/%q — Location %q", tc.lat, tc.lng, loc)
			}
			msg, _ := url.QueryUnescape(loc)
			if !strings.Contains(msg, tc.want) {
				t.Errorf("message %q does not mention %q", msg, tc.want)
			}
		})
	}
}

// Ownership is in the WHERE clause, not checked afterwards: another
// customer's id must simply match no row.
func TestAddressIsScopedToItsOwner(t *testing.T) {
	seedOnce(t)
	srv := testServer(t)

	owner := browser(t, srv.URL)
	signIn(t, owner, srv.URL, "sinta@example.com", seed.DemoPassword)
	resp, err := owner.PostForm(srv.URL+"/app/alamat", addressForm())
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if loc := resp.Header.Get("Location"); strings.Contains(loc, "err=") {
		m, _ := url.QueryUnescape(loc)
		t.Fatalf("the owner could not save an address: %s", m)
	}

	// Find what was just written, through the owner's own screen.
	page, err := owner.Get(srv.URL + "/app/alamat")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(page.Body)
	page.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	id := ""
	for _, part := range strings.Split(string(raw), `/app/alamat/`) {
		if i := strings.Index(part, "/hapus"); i > 0 {
			id = part[:i]
			break
		}
	}
	if id == "" {
		t.Fatal("no address id on the owner's page")
	}

	intruder := browser(t, srv.URL)
	signIn(t, intruder, srv.URL, "bagas@example.com", seed.DemoPassword)
	for _, action := range []string{"/utama", "/hapus"} {
		r, err := intruder.PostForm(srv.URL+"/app/alamat/"+id+action, url.Values{})
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		loc, _ := url.QueryUnescape(r.Header.Get("Location"))
		if !strings.Contains(loc, "tidak ditemukan") && r.StatusCode != http.StatusNotFound {
			t.Errorf("%s: an intruder was not refused (status %d, Location %q)", action, r.StatusCode, loc)
		}
	}
}

// An unrecognised slug is refused rather than dropped: a profile that quietly
// loses an allergy is worse than a rejected form.
func TestAllergyProfileRefusesAnUnknownSlug(t *testing.T) {
	seedOnce(t)
	srv := testServer(t)
	c := browser(t, srv.URL)
	signIn(t, c, srv.URL, "sinta@example.com", seed.DemoPassword)

	resp, err := c.PostForm(srv.URL+"/app/alergi",
		url.Values{"allergen": {"kacang", "uranium"}})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	loc, _ := url.QueryUnescape(resp.Header.Get("Location"))
	if !strings.Contains(loc, "tidak dikenali") {
		t.Fatalf("an unknown allergen was accepted: %q", loc)
	}
}
