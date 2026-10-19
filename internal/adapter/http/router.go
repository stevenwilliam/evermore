package http

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/stevenwilliam/evermore/internal/adapter/postgres"
	"github.com/stevenwilliam/evermore/internal/app/auth"
	"github.com/stevenwilliam/evermore/internal/app/ordering"
	"github.com/stevenwilliam/evermore/internal/app/payments"
	"github.com/stevenwilliam/evermore/internal/platform/config"
	"github.com/stevenwilliam/evermore/internal/platform/security"
)

// Deps is everything the router needs wired in.
type Deps struct {
	DB          *sql.DB
	Cfg         *config.Config
	Templates   fs.FS
	Public      fs.FS
	Logger      *slog.Logger
	BuildCommit string
	// Limiter is optional; a nil one gets a fresh per-router limiter.
	Limiter *Limiter
	// Store is the object store for payment proofs. A nil store skips the
	// upload, which is what the tests use.
	Store payments.ObjectStore
}

// NewRouter builds the whole HTTP surface.
func NewRouter(d Deps) (*gin.Engine, error) {
	if d.Cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	// Trust nothing by default. Behind nginx on this box the proxy is local,
	// so only the loopback is allowed to set X-Forwarded-For; without this gin
	// believes any client that sends the header, which forges the rate-limit
	// key and the audit log's IP.
	if err := r.SetTrustedProxies([]string{"127.0.0.1", "::1"}); err != nil {
		return nil, err
	}
	r.RedirectTrailingSlash = true
	r.HandleMethodNotAllowed = true

	r.Use(RequestID())
	r.Use(Logger(d.Logger))
	r.Use(Recovery())
	r.Use(SecurityHeaders(d.Cfg.IsProduction()))
	r.Use(CORS(d.Cfg.CORSAllowedOrigins))
	r.Use(BodyLimit(6 << 20)) // 6 MiB: the 5 MB proof upload plus overhead

	repo := postgres.NewPublicRepo(d.DB)
	ph, err := NewPublicHandler(repo, d.Cfg, d.Templates)
	if err != nil {
		return nil, err
	}

	tokens, err := security.NewTokenIssuer(
		d.Cfg.JWTSigningKey, "evermore", "evermore-api", d.Cfg.AccessTokenTTL)
	if err != nil {
		return nil, err
	}
	authSvc := auth.NewService(auth.Options{
		DB: d.DB, Tokens: tokens, RefreshTTL: d.Cfg.RefreshTokenTTL,
	})
	ah := NewAuthHandler(authSvc, tokens, d.Cfg.IsProduction(), d.Cfg.AccessTokenTTL)

	orderSvc := ordering.NewService(d.DB, d.Cfg.Location, nil)
	paySvc := payments.NewService(d.DB, d.Store, d.Cfg.Location, nil)
	app, err := NewAppHandler(d.DB, repo, orderSvc, paySvc, authSvc, d.Cfg, d.Templates)
	if err != nil {
		return nil, err
	}

	// One limiter per router. Tests build a router per case and must not
	// inherit another case's counters.
	limiter := d.Limiter
	if limiter == nil {
		limiter = NewLimiter()
	}

	// --- operational endpoints ---
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "commit": d.BuildCommit})
	})
	r.GET("/readyz", func(c *gin.Context) {
		ctx, cancel := contextWithTimeout(c, 2*time.Second)
		defer cancel()
		if err := d.DB.PingContext(ctx); err != nil {
			// Readiness that never fails is not readiness. Say which
			// dependency is down, without leaking the DSN.
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "degraded", "dependency": "postgres"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	// --- static assets ---
	sub := func(dir string) http.FileSystem {
		f, err := fs.Sub(d.Public, "public/"+dir)
		if err != nil {
			panic(fmt.Sprintf("http: embedded public/%s missing: %v", dir, err))
		}
		return http.FS(f)
	}
	// Fonts and CSS are content-addressed by deploy, not by filename, so they
	// get a modest cache with revalidation rather than an immutable year.
	staticGroup := r.Group("/", cacheControl("public, max-age=3600"))
	staticGroup.StaticFS("/css", sub("css"))
	staticGroup.StaticFS("/js", sub("js"))
	staticGroup.StaticFS("/images", sub("images"))
	r.Group("/", cacheControl("public, max-age=31536000, immutable")).
		StaticFS("/fonts", sub("fonts"))

	// --- API v1 ---
	api := r.Group("/api/v1")
	api.Use(Authenticate(tokens, authSvc))

	authGroup := api.Group("/auth")
	// The endpoints that mint or spend credentials are rate limited hardest:
	// they are where a brute force or a credential-stuffing run lands.
	authGroup.POST("/login", RateLimit(limiter, rlLogin), ah.Login)
	authGroup.POST("/register", RateLimit(limiter, rlRegister), ah.Register)
	authGroup.POST("/refresh", RateLimit(limiter, rlRefresh), ah.Refresh)
	authGroup.POST("/logout", ah.Logout)
	authGroup.GET("/me", RequireAuth(), ah.Me)

	// --- SEO surface ---
	// D81 — the routes moved to standard English. Everything that existed
	// under the old Indonesian paths answers with a permanent redirect rather
	// than a 404, so a bookmark, a shared link or an indexed URL still lands.
	for old, dst := range map[string]string{
		"/paket":             "/packages",
		"/cara-kerja":        "/how-it-works",
		"/untuk-kantor":      "/corporate",
		"/wilayah-antar":     "/delivery-area",
		"/kebijakan-privasi": "/privacy-policy",
		"/syarat-ketentuan":  "/terms",
		"/app/masuk":         "/app/sign-in",
		"/app/daftar":        "/app/register",
		"/app/keranjang":     "/app/cart",
		"/app/pesanan":       "/app/orders",
		"/app/paket":         "/app/packages",
		"/app/alamat":        "/app/addresses",
		"/app/alergi":        "/app/allergies",
		"/bo/pembayaran":     "/bo/payments",
	} {
		to := dst
		r.GET(old, func(c *gin.Context) {
			c.Redirect(http.StatusMovedPermanently, to)
		})
	}

	r.GET("/robots.txt", ph.Robots)
	r.GET("/sitemap.xml", ph.Sitemap)

	// --- public pages ---
	// D82 — the public pages read the session too, so the masthead can show
	// the signed-in state. Optional: an expired cookie must not turn the home
	// page into a 401.
	pub := r.Group("/", AuthenticateOptional(tokens, authSvc, d.Cfg.IsProduction()))
	pub.GET("/", ph.Home)
	pub.GET("/menu", ph.Menu)
	pub.GET("/menu/:id", ph.Meal)
	pub.GET("/packages", ph.Packages)
	pub.GET("/how-it-works", ph.HowItWorks)
	pub.GET("/corporate", ph.Corporate)
	pub.GET("/delivery-area", ph.Coverage)

	// --- signed-in surface ---
	//
	// Authenticate runs on the whole group so a page can render for an
	// anonymous visitor where that makes sense (the cart) and redirect where
	// it does not. RequireAuth and RequirePermission are per-route, so every
	// protected handler declares what it needs.
	appGroup := r.Group("/app", Authenticate(tokens, authSvc))
	appGroup.GET("/sign-in", app.LoginPage)
	appGroup.POST("/sign-out", RateLimit(limiter, rlWrite), ah.SignOutPage)
	appGroup.GET("/register", app.RegisterPage)
	appGroup.GET("/cart", app.Cart)
	appGroup.POST("/cart/add", RateLimit(limiter, rlWrite), app.AddToCart)
	appGroup.GET("/checkout", app.Checkout)
	appGroup.POST("/checkout", RequireAuth(), RateLimit(limiter, rlWrite), app.PlaceOrder)
	appGroup.GET("/payment/:id", app.Payment)
	appGroup.POST("/payment/:id/proof", RequireAuth(), RateLimit(limiter, rlWrite), app.UploadProof)
	appGroup.GET("/orders", app.Orders)
	appGroup.GET("/packages", app.Packages)

	// Delivery addresses and the allergen profile (D40). No RequireAuth: each
	// handler checks the session and redirects a signed-out visitor to the
	// login page rather than answering a browser with a JSON 401 (D38).
	appGroup.GET("/addresses", app.Addresses)
	appGroup.POST("/addresses", RateLimit(limiter, rlWrite), app.AddAddress)
	appGroup.POST("/addresses/:id/edit", RateLimit(limiter, rlWrite), app.EditAddress)
	appGroup.POST("/addresses/:id/default", RateLimit(limiter, rlWrite), app.DefaultAddress)
	appGroup.POST("/addresses/:id/delete", RateLimit(limiter, rlWrite), app.RemoveAddress)
	appGroup.GET("/allergies", app.Allergies)
	appGroup.POST("/allergies", RateLimit(limiter, rlWrite), app.SaveAllergies)
	// No RequireAuth: /packages is a PUBLIC page, so a signed-out visitor sees
	// the buy button. RequireAuth would answer their click with a raw JSON
	// 401 before the handler ran; BuyPackage checks the session itself and
	// redirects them to sign in and come back (D38).
	appGroup.POST("/packages/buy", RateLimit(limiter, rlWrite), app.BuyPackage)

	// --- back office ---
	bo := r.Group("/bo", Authenticate(tokens, authSvc), RequireAuth())
	bo.GET("", RequirePermission("dashboard.view"), app.Dashboard)
	bo.GET("/menu", RequirePermission("menu.view"), app.MenuCalendar)
	bo.POST("/menu/publish", RequirePermission("menu.manage"), RateLimit(limiter, rlWrite), app.PublishMeal)
	bo.GET("/payments", RequirePermission("payment.view"), app.PaymentQueue)
	bo.POST("/payments/verify", RequirePermission("payment.verify"), RateLimit(limiter, rlWrite), app.VerifyPayment)
	bo.POST("/payments/reject", RequirePermission("payment.verify"), RateLimit(limiter, rlWrite), app.RejectPayment)

	r.NoRoute(ph.NotFound)
	r.NoMethod(func(c *gin.Context) {
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": gin.H{
			"code": "METHOD_NOT_ALLOWED", "message": "Metode HTTP tidak didukung untuk alamat ini.",
		}})
	})
	return r, nil
}

func cacheControl(v string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", v)
		c.Next()
	}
}

// Robots disallows the transactional surface and points at the sitemap
// (99-steven-preference.md §13).
func (h *PublicHandler) Robots(c *gin.Context) {
	base := strings.TrimRight(h.cfg.BaseURL, "/")
	var b strings.Builder
	b.WriteString("User-agent: *\n")
	if h.cfg.IsProduction() {
		b.WriteString("Allow: /\n")
	} else {
		// A staging or dev host must never be indexed. Serving the production
		// robots.txt from a dev box is how a staging URL ends up in search.
		b.WriteString("Disallow: /\n")
	}
	for _, p := range []string{"/app/", "/api/", "/healthz", "/readyz"} {
		b.WriteString("Disallow: " + p + "\n")
	}
	b.WriteString("\nSitemap: " + base + "/sitemap.xml\n")
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, b.String())
}

// Sitemap lists the indexable pages plus every published meal.
func (h *PublicHandler) Sitemap(c *gin.Context) {
	ctx := c.Request.Context()
	base := strings.TrimRight(h.cfg.BaseURL, "/")
	today := h.today()

	type entry struct {
		loc      string
		lastmod  string
		priority string
		freq     string
	}
	entries := []entry{
		{base + "/", today.Format("2006-01-02"), "1.0", "daily"},
		{base + "/menu", today.Format("2006-01-02"), "0.9", "daily"},
		{base + "/packages", today.Format("2006-01-02"), "0.8", "weekly"},
		{base + "/how-it-works", today.Format("2006-01-02"), "0.6", "monthly"},
		{base + "/corporate", today.Format("2006-01-02"), "0.6", "monthly"},
		{base + "/delivery-area", today.Format("2006-01-02"), "0.7", "weekly"},
	}

	dates, err := h.repo.ServiceDates(ctx, today, 14)
	if err == nil {
		for _, d := range dates {
			meals, err := h.repo.PublishedMeals(ctx, d, "", "")
			if err != nil {
				continue
			}
			for _, m := range meals {
				entries = append(entries, entry{
					loc:      base + "/menu/" + m.ID.String(),
					lastmod:  d.Format("2006-01-02"),
					priority: "0.5",
					freq:     "weekly",
				})
			}
		}
	}

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	for _, e := range entries {
		b.WriteString("  <url>\n")
		b.WriteString("    <loc>" + xmlEscape(e.loc) + "</loc>\n")
		b.WriteString("    <lastmod>" + e.lastmod + "</lastmod>\n")
		b.WriteString("    <changefreq>" + e.freq + "</changefreq>\n")
		b.WriteString("    <priority>" + e.priority + "</priority>\n")
		b.WriteString("  </url>\n")
	}
	b.WriteString("</urlset>\n")

	c.Header("Content-Type", "application/xml; charset=utf-8")
	c.String(http.StatusOK, b.String())
}

func xmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;")
	return r.Replace(s)
}

// contextWithTimeout bounds a readiness probe so a hung database cannot hold
// the health endpoint open indefinitely.
func contextWithTimeout(c *gin.Context, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.Request.Context(), d)
}
