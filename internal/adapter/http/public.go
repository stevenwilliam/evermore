package http

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/stevenwilliam/evermore/internal/adapter/postgres"
	"github.com/stevenwilliam/evermore/internal/app/auth"
	"github.com/stevenwilliam/evermore/internal/app/ordering"
	"github.com/stevenwilliam/evermore/internal/app/payments"
	"github.com/stevenwilliam/evermore/internal/domain/money"
	"github.com/stevenwilliam/evermore/internal/platform/apierror"
	"github.com/stevenwilliam/evermore/internal/platform/config"
	"github.com/stevenwilliam/evermore/internal/platform/sanitize"
)

// PublicHandler serves the server-rendered, SEO-indexed surface.
//
// This half of the site is Go templates rather than the SPA because preview
// bots and crawlers do not run JavaScript: Open Graph tags and page copy have
// to be in the served HTML (99-steven-preference.md §13).
type PublicHandler struct {
	*renderer
	repo *postgres.PublicRepo
}

// weekdayID and monthID render Indonesian dates. A message catalogue covers
// interface strings; these are date parts, which Go's time package has no
// Indonesian locale for.
var weekdayID = map[time.Weekday]string{
	time.Monday: "Senin", time.Tuesday: "Selasa", time.Wednesday: "Rabu",
	time.Thursday: "Kamis", time.Friday: "Jumat", time.Saturday: "Sabtu",
	time.Sunday: "Minggu",
}

var weekdayShortID = map[time.Weekday]string{
	time.Monday: "Sen", time.Tuesday: "Sel", time.Wednesday: "Rab",
	time.Thursday: "Kam", time.Friday: "Jum", time.Saturday: "Sab",
	time.Sunday: "Min",
}

var monthID = map[time.Month]string{
	time.January: "Januari", time.February: "Februari", time.March: "Maret",
	time.April: "April", time.May: "Mei", time.June: "Juni",
	time.July: "Juli", time.August: "Agustus", time.September: "September",
	time.October: "Oktober", time.November: "November", time.December: "Desember",
}

var monthShortID = map[time.Month]string{
	time.January: "Jan", time.February: "Feb", time.March: "Mar",
	time.April: "Apr", time.May: "Mei", time.June: "Jun",
	time.July: "Jul", time.August: "Agu", time.September: "Sep",
	time.October: "Okt", time.November: "Nov", time.December: "Des",
}

// rupiah formats an amount the Indonesian way: "Rp 480.148", full stops as the
// thousands separator and no decimals, because the minor unit is the rupiah.
func rupiah(v money.IDR) string {
	n := int64(v)
	neg := n < 0
	if neg {
		n = -n
	}
	s := strconv.FormatInt(n, 10)
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, '.')
		}
		out = append(out, c)
	}
	if neg {
		return "-Rp " + string(out)
	}
	return "Rp " + string(out)
}

// gram renders milligrams as grams with one decimal, Indonesian style (comma).
// The stored value is an integer in mg; this is display only and never feeds
// back into arithmetic.
func gram(mg int) string {
	whole := mg / 1000
	tenth := (mg % 1000) / 100
	return fmt.Sprintf("%d,%d", whole, tenth)
}

func roleLabel(r string) string {
	switch r {
	case "MAIN":
		return "Utama"
	case "SIDE":
		return "Pendamping"
	case "DRINK":
		return "Minuman"
	case "DESSERT":
		return "Pencuci mulut"
	}
	return r
}

func tierRange(minQty int, maxQty *int) string {
	if maxQty == nil {
		return fmt.Sprintf("%d+ porsi", minQty)
	}
	return fmt.Sprintf("%d – %d porsi", minQty, *maxQty)
}

// bps renders a basis-point rate as a percentage: 1100 -> "11%".
func bps(v int) string {
	whole := v / 100
	frac := v % 100
	if frac == 0 {
		return fmt.Sprintf("%d%%", whole)
	}
	return fmt.Sprintf("%d,%02d%%", whole, frac)
}

// duration renders a countdown the way the artifact does: "2 jam 47 menit".
func duration(d time.Duration) string {
	if d <= 0 {
		return "0 menit"
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%d jam %d menit", h, m)
	}
	return fmt.Sprintf("%d menit", m)
}

// isodate renders a business date. It takes a *time.Time so a template can
// pass a nullable column without a nil check at every call site.
func isodate(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return fmt.Sprintf("%d %s %d", t.Day(), monthShortID[t.Month()], t.Year())
}

// signed renders a ledger movement with its sign, so +1 and -2 read as
// movements rather than as quantities.
func signed(n int) string {
	if n > 0 {
		return fmt.Sprintf("+%d", n)
	}
	return strconv.Itoa(n)
}

var funcs = template.FuncMap{
	"rupiah":    rupiah,
	"gram":      gram,
	"role":      roleLabel,
	"tierRange": tierRange,
	"bps":       bps,
	"duration":  duration,
	"isodate":   isodate,
	"signed":    signed,
}

// NewPublicHandler parses every page template against the base layout.
//
// Each page gets its OWN template set. html/template refuses Parse after
// Execute, so a single shared root would make the first page render and every
// later one fail — the exact failure recorded as D18(c) in the decision log.
func NewPublicHandler(repo *postgres.PublicRepo, cfg *config.Config, fsys fs.FS) (*PublicHandler, error) {
	r, err := newRenderer(cfg, fsys, []string{
		"home", "menu", "meal", "packages", "coverage", "how", "corporate", "error",
	})
	if err != nil {
		return nil, err
	}
	return &PublicHandler{renderer: r, repo: repo}, nil
}

// renderer owns template parsing and the fields every page's layout needs. It
// is shared by the public and the signed-in handlers so the masthead, footer
// and SEO head are defined once.
type renderer struct {
	cfg *config.Config
	tpl map[string]*template.Template
}

// newRenderer parses each page against the base layout, into its OWN set.
//
// html/template refuses Parse after Execute, so a single shared root would
// make the first page render and every later one fail — the exact failure
// recorded as D18(c) in the decision log.
func newRenderer(cfg *config.Config, fsys fs.FS, pages []string) (*renderer, error) {
	set := make(map[string]*template.Template, len(pages))
	for _, p := range pages {
		t, err := template.New("base").Funcs(funcs).ParseFS(fsys,
			"templates/base.html", "templates/"+p+".html")
		if err != nil {
			return nil, fmt.Errorf("http: parsing template %s: %w", p, err)
		}
		// A page whose "content" block is missing would render the layout with
		// a hole in it. Assert it defined one.
		if t.Lookup("content") == nil {
			return nil, fmt.Errorf("http: template %s defines no \"content\" block", p)
		}
		set[p] = t
	}
	return &renderer{cfg: cfg, tpl: set}, nil
}

// pageData is the common shape every template's base layout needs.
type pageData struct {
	Lang         string
	Theme        string
	Title        string
	Description  string
	CanonicalURL string
	BaseURL      string
	SiteName     string
	OGType       string
	OGLocale     string
	NoIndex      bool
	JSONLD       template.JS
	Alternates   []alternate
	Nav          string
	Year         int
	Params       map[string]string

	// Page-specific fields. Templates only reach for what their page sets.
	Diets           []postgres.DietType
	DietNames       string
	Kitchens        []postgres.Kitchen
	KitchenNames    string
	Meals           []postgres.MealCard
	Meal            *postgres.MealDetail
	Packages        []postgres.PackageCard
	Tiers           []postgres.TierPrice
	Dates           []dayLink
	TodayLabel      string
	DateLabel       string
	PublishedUntil  string
	CutOff          string
	CutOffCountdown string
	Search          string
	SelectedDiet    string
	SelectedDate    string
	DateQuery       string
	DateQueryAmp    string

	Code    string
	Heading string
	Message string
	TraceID string

	// Signed-in surface.
	Anonymous        bool
	Next             string
	Quote            *ordering.Quote
	QuoteError       string
	Addresses        []AddressView
	Orders           []OrderView
	Payment          *payments.Instructions
	PaymentLines     []PaymentLine
	CustomerPackages []PackageView
	Ledger           []LedgerRow
	IdempotencyKey   string
	Principal        *auth.Principal

	// Back office.
	Dash      *DashboardView
	PayQueue  []payments.QueueItem
	MenuWeek  []MenuDay
	WeekLabel string
	ThisWeek  string
	PrevWeek  string
	NextWeek  string
	Flash     string
	FlashKind string
}

type alternate struct {
	Lang string
	URL  string
}

type dayLink struct {
	Weekday  string
	DayMonth string
	URL      string
	Selected bool
}

// base fills the fields every page shares.
func (h *renderer) base(c *gin.Context, params map[string]string, nav, title, desc string) pageData {
	// params keys use dots; templates cannot index a dotted key with .Foo, so
	// the map is re-keyed with underscores for template convenience.
	tplParams := make(map[string]string, len(params))
	for k, v := range params {
		tplParams[strings.ReplaceAll(k, ".", "_")] = v
	}
	site := params["seo.site_name"]
	if site == "" {
		site = h.cfg.AppName
	}
	canonical := strings.TrimRight(h.cfg.BaseURL, "/") + c.Request.URL.Path
	return pageData{
		Lang:         "id",
		Theme:        "",
		Title:        title,
		Description:  desc,
		CanonicalURL: canonical,
		BaseURL:      strings.TrimRight(h.cfg.BaseURL, "/"),
		SiteName:     site,
		OGType:       "website",
		OGLocale:     "id_ID",
		Nav:          nav,
		Year:         time.Now().In(h.cfg.Location).Year(),
		Params:       tplParams,
		CutOff:       params["order.cutoff_time"],
	}
}

func (h *renderer) render(c *gin.Context, page string, status int, data pageData) {
	t, ok := h.tpl[page]
	if !ok {
		Fail(c, apierror.Internal(fmt.Errorf("no template %q", page)))
		return
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Status(status)
	if err := t.ExecuteTemplate(c.Writer, "base", data); err != nil {
		// The status and some bytes are already written, so this cannot become
		// a clean error page. Log it loudly instead of pretending it rendered.
		Fail(c, apierror.Internal(fmt.Errorf("rendering %s: %w", page, err)))
	}
}

// today is the current business date in the operating timezone. Business-day
// logic converts explicitly and never uses the server's zone (CLAUDE.md §4).
func (h *renderer) today() time.Time {
	n := time.Now().In(h.cfg.Location)
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, h.cfg.Location)
}

func (h *renderer) longDate(t time.Time) string {
	t = t.In(h.cfg.Location)
	return fmt.Sprintf("%s, %d %s %d", weekdayID[t.Weekday()], t.Day(), monthID[t.Month()], t.Year())
}

// cutOffCountdown renders how long is left before the cut-off for tomorrow's
// service, or empty once it has passed.
func (h *renderer) cutOffCountdown(params map[string]string) string {
	raw := params["order.cutoff_time"]
	hh, mm, ok := parseHHMM(raw)
	if !ok {
		return ""
	}
	now := time.Now().In(h.cfg.Location)
	cut := time.Date(now.Year(), now.Month(), now.Day(), hh, mm, 0, 0, h.cfg.Location)
	if !now.Before(cut) {
		return ""
	}
	d := cut.Sub(now)
	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	if hours > 0 {
		return fmt.Sprintf("%d jam %d menit", hours, mins)
	}
	return fmt.Sprintf("%d menit", mins)
}

func parseHHMM(s string) (h, m int, ok bool) {
	parts := strings.Split(strings.TrimSpace(s), ":")
	if len(parts) != 2 {
		return 0, 0, false
	}
	hh, err1 := strconv.Atoi(parts[0])
	mm, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || hh < 0 || hh > 23 || mm < 0 || mm > 59 {
		return 0, 0, false
	}
	return hh, mm, true
}

// ---------------------------------------------------------------- handlers

func (h *PublicHandler) Home(c *gin.Context) {
	ctx := c.Request.Context()
	params, err := h.repo.Params(ctx)
	if err != nil {
		Fail(c, apierror.Internal(err))
		return
	}
	today := h.today()

	d := h.base(c, params, "home",
		params["seo.default_title"],
		params["seo.default_description"])
	d.OGType = "website"

	if d.Diets, err = h.repo.DietTypes(ctx); err != nil {
		Fail(c, apierror.Internal(err))
		return
	}
	if d.Kitchens, err = h.repo.Kitchens(ctx); err != nil {
		Fail(c, apierror.Internal(err))
		return
	}
	if d.Packages, err = h.repo.Packages(ctx, today); err != nil {
		Fail(c, apierror.Internal(err))
		return
	}
	if d.Meals, err = h.repo.PublishedMeals(ctx, today, "", ""); err != nil {
		Fail(c, apierror.Internal(err))
		return
	}
	if err := h.attachPrices(ctx, d.Meals, today); err != nil {
		Fail(c, apierror.Internal(err))
		return
	}
	if len(d.Meals) > 3 {
		d.Meals = d.Meals[:3]
	}

	d.DietNames = joinNames(d.Diets)
	d.KitchenNames = joinKitchens(d.Kitchens)
	d.TodayLabel = h.longDate(today)
	d.JSONLD = h.orgJSONLD(params)
	h.render(c, "home", http.StatusOK, d)
}

func (h *PublicHandler) Menu(c *gin.Context) {
	ctx := c.Request.Context()
	params, err := h.repo.Params(ctx)
	if err != nil {
		Fail(c, apierror.Internal(err))
		return
	}

	// Every query parameter is validated. A search term goes into an ILIKE
	// with a placeholder, so it cannot inject, but it is still length-capped
	// and normalised before use.
	search := c.Query("q")
	if search != "" {
		search, err = sanitize.Text("q", search, 0, 80)
		if err != nil {
			Fail(c, apierror.Validation("Kata kunci pencarian terlalu panjang."))
			return
		}
	}
	dietSlug := c.Query("diet")
	if dietSlug != "" {
		if dietSlug, err = sanitize.Slug("diet", dietSlug); err != nil {
			Fail(c, apierror.Validation("Filter diet tidak valid."))
			return
		}
	}

	today := h.today()
	selected := today
	if raw := c.Query("tanggal"); raw != "" {
		parsed, perr := time.ParseInLocation("2006-01-02", raw, h.cfg.Location)
		if perr != nil {
			Fail(c, apierror.Validation("Format tanggal tidak valid."))
			return
		}
		selected = parsed
	}

	horizon := 14
	if v, ok := params["menu.horizon_days"]; ok {
		if n, cerr := strconv.Atoi(v); cerr == nil && n > 0 && n <= 60 {
			horizon = n
		}
	}

	d := h.base(c, params, "menu",
		"Menu minggu ini — "+params["seo.site_name"],
		"Menu katering sehat Evermore untuk minggu ini: "+
			"pilihan Balanced, Weight Loss, Muscle Gain dan Special Diet, dengan gizi lengkap per porsi.")
	d.Search = search
	d.SelectedDiet = dietSlug
	d.CutOffCountdown = h.cutOffCountdown(params)

	if d.Diets, err = h.repo.DietTypes(ctx); err != nil {
		Fail(c, apierror.Internal(err))
		return
	}

	dates, err := h.repo.ServiceDates(ctx, today, horizon)
	if err != nil {
		Fail(c, apierror.Internal(err))
		return
	}
	// If the selected date has no published menu, fall back to the first one
	// that does rather than showing an empty page with no explanation.
	if len(dates) > 0 {
		found := false
		for _, dt := range dates {
			if sameDay(dt, selected) {
				found = true
				break
			}
		}
		if !found {
			selected = dates[0]
		}
		last := dates[len(dates)-1].In(h.cfg.Location)
		d.PublishedUntil = fmt.Sprintf("%s, %d %s", weekdayID[last.Weekday()], last.Day(), monthID[last.Month()])
	}

	for _, dt := range dates {
		l := dt.In(h.cfg.Location)
		q := "/menu?tanggal=" + l.Format("2006-01-02")
		if dietSlug != "" {
			q += "&diet=" + dietSlug
		}
		d.Dates = append(d.Dates, dayLink{
			Weekday:  weekdayShortID[l.Weekday()],
			DayMonth: fmt.Sprintf("%d %s", l.Day(), monthShortID[l.Month()]),
			URL:      q,
			Selected: sameDay(dt, selected),
		})
	}

	d.SelectedDate = selected.Format("2006-01-02")
	d.DateQuery = "?tanggal=" + d.SelectedDate
	d.DateQueryAmp = "&tanggal=" + d.SelectedDate

	if d.Meals, err = h.repo.PublishedMeals(ctx, selected, dietSlug, search); err != nil {
		Fail(c, apierror.Internal(err))
		return
	}
	if err := h.attachPrices(ctx, d.Meals, selected); err != nil {
		Fail(c, apierror.Internal(err))
		return
	}
	h.render(c, "menu", http.StatusOK, d)
}

func (h *PublicHandler) Meal(c *gin.Context) {
	ctx := c.Request.Context()
	mealID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		h.NotFound(c)
		return
	}
	params, err := h.repo.Params(ctx)
	if err != nil {
		Fail(c, apierror.Internal(err))
		return
	}
	meal, err := h.repo.Meal(ctx, mealID)
	if err == sql.ErrNoRows {
		h.NotFound(c)
		return
	}
	if err != nil {
		Fail(c, apierror.Internal(err))
		return
	}

	price, err := h.repo.SinglePortionPrice(ctx, meal.DietSlug, meal.ServiceDate)
	if err != nil {
		Fail(c, apierror.Internal(err))
		return
	}
	meal.PriceIDR = price

	desc := fmt.Sprintf("%s — %d kkal, %s g protein per porsi. %s, diantar %s pukul %s.",
		meal.Name, meal.Panel.CaloriesKcal, gram(meal.Panel.ProteinMG),
		meal.DietName, h.longDate(meal.ServiceDate), meal.SlotTime)

	d := h.base(c, params, "menu", meal.Name+" — "+params["seo.site_name"], desc)
	d.OGType = "product"
	d.Meal = meal
	d.DateLabel = h.longDate(meal.ServiceDate)
	d.JSONLD = h.mealJSONLD(meal, params)
	h.render(c, "meal", http.StatusOK, d)
}

func (h *PublicHandler) Packages(c *gin.Context) {
	ctx := c.Request.Context()
	params, err := h.repo.Params(ctx)
	if err != nil {
		Fail(c, apierror.Internal(err))
		return
	}
	d := h.base(c, params, "packages",
		"Paket &amp; kredit — "+params["seo.site_name"],
		"Beli kredit sekali, pakai pada menu apa pun yang sudah terbit. Paket 10, 20 dan 40 porsi.")
	d.Title = "Paket & kredit — " + params["seo.site_name"]
	if d.Packages, err = h.repo.Packages(ctx, h.today()); err != nil {
		Fail(c, apierror.Internal(err))
		return
	}
	h.render(c, "packages", http.StatusOK, d)
}

func (h *PublicHandler) Coverage(c *gin.Context) {
	ctx := c.Request.Context()
	params, err := h.repo.Params(ctx)
	if err != nil {
		Fail(c, apierror.Internal(err))
		return
	}
	d := h.base(c, params, "coverage",
		"Wilayah antar — "+params["seo.site_name"],
		"Dapur Evermore di Tebet, Kebayoran dan Kelapa Gading, dengan radius layanan masing-masing.")
	if d.Kitchens, err = h.repo.Kitchens(ctx); err != nil {
		Fail(c, apierror.Internal(err))
		return
	}
	h.render(c, "coverage", http.StatusOK, d)
}

func (h *PublicHandler) HowItWorks(c *gin.Context) {
	ctx := c.Request.Context()
	params, err := h.repo.Params(ctx)
	if err != nil {
		Fail(c, apierror.Internal(err))
		return
	}
	d := h.base(c, params, "how",
		"Cara kerja — "+params["seo.site_name"],
		"Pilih menu, pesan sebelum cut-off, transfer manual, kami verifikasi dan antar pada jam yang kamu pilih.")
	if d.Diets, err = h.repo.DietTypes(ctx); err != nil {
		Fail(c, apierror.Internal(err))
		return
	}
	if d.Tiers, err = h.repo.TierPrices(ctx, "balanced", h.today()); err != nil {
		Fail(c, apierror.Internal(err))
		return
	}
	h.render(c, "how", http.StatusOK, d)
}

func (h *PublicHandler) Corporate(c *gin.Context) {
	ctx := c.Request.Context()
	params, err := h.repo.Params(ctx)
	if err != nil {
		Fail(c, apierror.Internal(err))
		return
	}
	d := h.base(c, params, "corporate",
		"Untuk kantor — "+params["seo.site_name"],
		"Katering harian untuk tim: harga korporat, satu tagihan, satu titik antar, laporan bulanan.")
	h.render(c, "corporate", http.StatusOK, d)
}

func (h *PublicHandler) NotFound(c *gin.Context) {
	ctx := c.Request.Context()
	params, _ := h.repo.Params(ctx)
	if params == nil {
		params = map[string]string{}
	}
	d := h.base(c, params, "", "Halaman tidak ditemukan — Evermore",
		"Halaman yang kamu cari tidak ada.")
	d.NoIndex = true
	d.Code = "404"
	d.Heading = "Halaman tidak ditemukan"
	d.Message = "Halaman yang kamu cari tidak ada atau sudah dipindahkan."
	if id, ok := c.Get("trace_id"); ok {
		d.TraceID, _ = id.(string)
	}
	h.render(c, "error", http.StatusNotFound, d)
}

// attachPrices fills in each card's price with one query per distinct diet,
// rather than one per meal.
func (h *PublicHandler) attachPrices(ctx context.Context, meals []postgres.MealCard, on time.Time) error {
	cache := map[string]money.IDR{}
	for i := range meals {
		slug := meals[i].DietSlug
		if _, ok := cache[slug]; !ok {
			p, err := h.repo.SinglePortionPrice(ctx, slug, on)
			if err != nil {
				return err
			}
			cache[slug] = p
		}
		meals[i].PriceIDR = cache[slug]
	}
	return nil
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

func joinNames(d []postgres.DietType) string {
	names := make([]string, 0, len(d))
	for _, x := range d {
		names = append(names, x.Name)
	}
	return strings.Join(names, ", ")
}

func joinKitchens(k []postgres.Kitchen) string {
	names := make([]string, 0, len(k))
	for _, x := range k {
		names = append(names, strings.TrimPrefix(x.Name, "Dapur "))
	}
	return strings.Join(names, ", ")
}

// --------------------------------------------------------------- JSON-LD

func (h *renderer) orgJSONLD(params map[string]string) template.JS {
	doc := map[string]any{
		"@context":      "https://schema.org",
		"@type":         "Restaurant",
		"name":          params["company.brand"],
		"legalName":     params["company.name"],
		"url":           h.cfg.BaseURL,
		"image":         strings.TrimRight(h.cfg.BaseURL, "/") + "/images/og-default.png",
		"telephone":     params["company.phone"],
		"email":         params["company.email"],
		"servesCuisine": "Healthy",
		"priceRange":    "Rp",
		"address": map[string]any{
			"@type":           "PostalAddress",
			"streetAddress":   params["company.address"],
			"addressLocality": "Jakarta",
			"addressCountry":  "ID",
		},
		"areaServed": []string{"Jakarta Selatan", "Jakarta Pusat", "Jakarta Utara"},
	}
	return marshalJSONLD(doc)
}

func (h *renderer) mealJSONLD(m *postgres.MealDetail, params map[string]string) template.JS {
	doc := map[string]any{
		"@context":    "https://schema.org",
		"@type":       "Product",
		"name":        m.Name,
		"description": fmt.Sprintf("%s, %d kkal, %s g protein per porsi.", m.DietName, m.Panel.CaloriesKcal, gram(m.Panel.ProteinMG)),
		"brand":       map[string]any{"@type": "Brand", "name": params["company.brand"]},
		"nutrition": map[string]any{
			"@type":               "NutritionInformation",
			"calories":            fmt.Sprintf("%d kcal", m.Panel.CaloriesKcal),
			"proteinContent":      fmt.Sprintf("%s g", gram(m.Panel.ProteinMG)),
			"carbohydrateContent": fmt.Sprintf("%s g", gram(m.Panel.CarbohydrateMG)),
			"fatContent":          fmt.Sprintf("%s g", gram(m.Panel.FatMG)),
			"fiberContent":        fmt.Sprintf("%s g", gram(m.Panel.FibreMG)),
			"sodiumContent":       fmt.Sprintf("%d mg", m.Panel.SodiumMG),
		},
	}
	if m.PriceIDR > 0 {
		doc["offers"] = map[string]any{
			"@type":         "Offer",
			"price":         strconv.FormatInt(int64(m.PriceIDR), 10),
			"priceCurrency": "IDR",
			"availability":  "https://schema.org/InStock",
		}
	}
	return marshalJSONLD(doc)
}

// marshalJSONLD serialises and neutralises the sequences that would let a
// value break out of the <script> element. template.JS bypasses contextual
// escaping, so this is the only thing standing between a food name containing
// "</script>" and an injected script tag.
func marshalJSONLD(doc map[string]any) template.JS {
	b, err := json.Marshal(doc)
	if err != nil {
		return ""
	}
	s := string(b)
	// encoding/json escapes <, > and & to \u003c, \u003e and \u0026 by default,
	// which is what closes the "</script>" breakout. It does NOT escape U+2028
	// and U+2029: those are valid inside a JSON string but terminate a line in
	// JavaScript, so a name containing one would break the script element.
	s = strings.ReplaceAll(s, "\u2028", `\\u2028`)
	s = strings.ReplaceAll(s, "\u2029", `\\u2029`)
	return template.JS(s)
}
