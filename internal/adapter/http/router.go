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
	"github.com/stevenwilliam/evermore/internal/platform/config"
)

// Deps is everything the router needs wired in.
type Deps struct {
	DB          *sql.DB
	Cfg         *config.Config
	Templates   fs.FS
	Public      fs.FS
	Logger      *slog.Logger
	BuildCommit string
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

	// --- SEO surface ---
	r.GET("/robots.txt", ph.Robots)
	r.GET("/sitemap.xml", ph.Sitemap)

	// --- public pages ---
	r.GET("/", ph.Home)
	r.GET("/menu", ph.Menu)
	r.GET("/menu/:id", ph.Meal)
	r.GET("/paket", ph.Packages)
	r.GET("/cara-kerja", ph.HowItWorks)
	r.GET("/untuk-kantor", ph.Corporate)
	r.GET("/wilayah-antar", ph.Coverage)

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
		{base + "/paket", today.Format("2006-01-02"), "0.8", "weekly"},
		{base + "/cara-kerja", today.Format("2006-01-02"), "0.6", "monthly"},
		{base + "/untuk-kantor", today.Format("2006-01-02"), "0.6", "monthly"},
		{base + "/wilayah-antar", today.Format("2006-01-02"), "0.7", "weekly"},
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
