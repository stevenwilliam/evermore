package http

import (
	"database/sql"
	"errors"
	"fmt"
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
	"github.com/stevenwilliam/evermore/internal/domain/order"
	"github.com/stevenwilliam/evermore/internal/platform/apierror"
	"github.com/stevenwilliam/evermore/internal/platform/config"
	"github.com/stevenwilliam/evermore/internal/platform/sanitize"
)

// AppHandler serves the signed-in surface: cart, checkout, payment, orders,
// packages, and the back office.
//
// Server-rendered like the public site. The cart lives in a signed cookie
// rather than a database row, so an anonymous visitor can build one before
// deciding to register.
type AppHandler struct {
	*renderer
	repo     *postgres.PublicRepo
	orders   *ordering.Service
	payments *payments.Service
	authSvc  *auth.Service
	db       *sql.DB
}

// NewAppHandler wires the signed-in surface.
func NewAppHandler(db *sql.DB, repo *postgres.PublicRepo, o *ordering.Service,
	pay *payments.Service, a *auth.Service, cfg *config.Config, fsys fsFS) (*AppHandler, error) {
	r, err := newRenderer(cfg, fsys, append(appPages, "error"))
	if err != nil {
		return nil, err
	}
	return &AppHandler{renderer: r, repo: repo, orders: o, payments: pay, authSvc: a, db: db}, nil
}

// appPages are the templates this handler owns.
var appPages = []string{
	"app-login", "app-register", "app-cart", "app-checkout", "app-payment",
	"app-orders", "app-packages", "bo-dashboard", "bo-payments", "bo-menu",
}

// --------------------------------------------------------------- the cart

// cartCookie holds the cart between requests. It is not signed because it
// contains no authority: every id in it is re-read and re-priced server-side
// at quote and at checkout, so tampering with it changes nothing except which
// meals get priced.
const cartCookie = "evermore_cart"

type cartEntry struct {
	MealID uuid.UUID
	Qty    int
}

func readCart(c *gin.Context) []cartEntry {
	raw, err := c.Cookie(cartCookie)
	if err != nil || raw == "" {
		return nil
	}
	var out []cartEntry
	for _, part := range strings.Split(raw, ",") {
		idStr, qtyStr, ok := strings.Cut(part, ":")
		if !ok {
			continue
		}
		id, err := uuid.Parse(idStr)
		if err != nil {
			continue
		}
		qty, err := strconv.Atoi(qtyStr)
		// A quantity outside the sane range is dropped rather than clamped:
		// reject, never silently repair.
		if err != nil || qty <= 0 || qty > 999 {
			continue
		}
		out = append(out, cartEntry{MealID: id, Qty: qty})
	}
	return out
}

func writeCart(c *gin.Context, entries []cartEntry, secure bool) {
	parts := make([]string, 0, len(entries))
	for _, e := range entries {
		parts = append(parts, e.MealID.String()+":"+strconv.Itoa(e.Qty))
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name: cartCookie, Value: strings.Join(parts, ","), Path: "/",
		MaxAge: 7 * 24 * 3600, HttpOnly: true, Secure: secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *AppHandler) items(c *gin.Context) []ordering.CartItem {
	entries := readCart(c)
	out := make([]ordering.CartItem, 0, len(entries))
	for _, e := range entries {
		out = append(out, ordering.CartItem{ScheduledMealID: e.MealID, Qty: e.Qty})
	}
	return out
}

// AddToCart adds or replaces a line and returns to the cart.
func (h *AppHandler) AddToCart(c *gin.Context) {
	mealID, err := uuid.Parse(c.PostForm("meal_id"))
	if err != nil {
		Fail(c, apierror.Validation("Menu tidak dikenali."))
		return
	}
	qty, err := strconv.Atoi(c.DefaultPostForm("qty", "1"))
	if err != nil || qty < 0 || qty > 999 {
		Fail(c, apierror.Validation("Jumlah porsi harus antara 0 dan 999."))
		return
	}

	entries := readCart(c)
	found := false
	next := entries[:0]
	for _, e := range entries {
		if e.MealID == mealID {
			found = true
			if qty == 0 {
				continue // removing the line
			}
			e.Qty = qty
		}
		next = append(next, e)
	}
	if !found && qty > 0 {
		next = append(next, cartEntry{MealID: mealID, Qty: qty})
	}
	writeCart(c, next, h.cfg.IsProduction())
	c.Redirect(http.StatusSeeOther, "/app/keranjang")
}

// Cart renders the priced cart.
func (h *AppHandler) Cart(c *gin.Context) {
	ctx := c.Request.Context()
	params, err := h.repo.Params(ctx)
	if err != nil {
		Fail(c, apierror.Internal(err))
		return
	}
	d := h.base(c, params, "cart", "Keranjang — Evermore", "Keranjang belanja Evermore.")
	d.NoIndex = true

	items := h.items(c)
	if len(items) == 0 {
		h.render(c, "app-cart", http.StatusOK, d)
		return
	}

	// An anonymous visitor still gets a priced cart, using the retail scope,
	// so they can see what it costs before registering.
	custID, ok := CustomerIDOf(c)
	if !ok {
		custID, err = h.anyRetailCustomer(ctx)
		if err != nil {
			h.render(c, "app-cart", http.StatusOK, d)
			return
		}
		d.Anonymous = true
	}

	quote, err := h.orders.Quote(ctx, custID, items)
	if err != nil {
		d.QuoteError = quoteMessage(err)
		h.render(c, "app-cart", http.StatusOK, d)
		return
	}
	d.Quote = quote
	if tiers, err := h.repo.TierPrices(ctx, "balanced", time.Now().In(h.cfg.Location)); err == nil {
		d.Tiers = tiers
	}
	h.render(c, "app-cart", http.StatusOK, d)
}

// Checkout renders the address and slot confirmation step.
func (h *AppHandler) Checkout(c *gin.Context) {
	ctx := c.Request.Context()
	custID, ok := CustomerIDOf(c)
	if !ok {
		c.Redirect(http.StatusSeeOther, "/app/masuk?next=/app/checkout")
		return
	}
	params, err := h.repo.Params(ctx)
	if err != nil {
		Fail(c, apierror.Internal(err))
		return
	}
	items := h.items(c)
	if len(items) == 0 {
		c.Redirect(http.StatusSeeOther, "/app/keranjang")
		return
	}

	d := h.base(c, params, "cart", "Pengiriman — Evermore", "Konfirmasi alamat dan jam antar.")
	d.NoIndex = true

	quote, err := h.orders.Quote(ctx, custID, items)
	if err != nil {
		d.QuoteError = quoteMessage(err)
		h.render(c, "app-checkout", http.StatusOK, d)
		return
	}
	d.Quote = quote
	if d.Addresses, err = h.addresses(ctx, custID); err != nil {
		Fail(c, apierror.Internal(err))
		return
	}
	h.render(c, "app-checkout", http.StatusOK, d)
}

// PlaceOrder performs the checkout and redirects to the payment screen.
func (h *AppHandler) PlaceOrder(c *gin.Context) {
	ctx := c.Request.Context()
	custID, ok := CustomerIDOf(c)
	if !ok {
		Fail(c, apierror.Unauthenticated(""))
		return
	}
	addressID, err := uuid.Parse(c.PostForm("address_id"))
	if err != nil {
		Fail(c, apierror.Validation("Pilih alamat pengantaran."))
		return
	}
	note, err := sanitize.Text("courier_note", c.PostForm("courier_note"), 0, 240)
	if err != nil {
		Fail(c, apierror.From(err))
		return
	}

	items := h.items(c)
	if len(items) == 0 {
		c.Redirect(http.StatusSeeOther, "/app/keranjang")
		return
	}

	// The idempotency key comes from the form, so a double-submitted browser
	// POST places one order rather than two.
	key := c.PostForm("idempotency_key")
	if _, err := uuid.Parse(key); err != nil {
		key = uuid.NewString()
	}

	orderID, _, err := h.orders.Checkout(ctx, ordering.CheckoutInput{
		CustomerID: custID, AddressID: addressID, Items: items,
		CourierNote: note, IdempotencyKey: key,
	})
	if err != nil {
		Fail(c, checkoutError(err))
		return
	}

	writeCart(c, nil, h.cfg.IsProduction()) // the cart is now an order
	c.Redirect(http.StatusSeeOther, "/app/pembayaran/"+orderID.String())
}

// Payment renders the transfer instructions.
func (h *AppHandler) Payment(c *gin.Context) {
	ctx := c.Request.Context()
	custID, ok := CustomerIDOf(c)
	if !ok {
		c.Redirect(http.StatusSeeOther, "/app/masuk")
		return
	}
	orderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		h.NotFoundApp(c)
		return
	}
	params, err := h.repo.Params(ctx)
	if err != nil {
		Fail(c, apierror.Internal(err))
		return
	}

	in, err := h.payments.Instructions(ctx, custID, orderID)
	if errors.Is(err, payments.ErrPaymentNotFound) {
		h.NotFoundApp(c)
		return
	}
	if err != nil {
		Fail(c, apierror.Internal(err))
		return
	}

	d := h.base(c, params, "", "Pembayaran "+in.OrderNumber+" — Evermore",
		"Instruksi transfer untuk pesanan "+in.OrderNumber+".")
	d.NoIndex = true
	d.Payment = in
	d.PaymentLines, _ = h.orderLines(ctx, orderID)
	h.render(c, "app-payment", http.StatusOK, d)
}

// UploadProof accepts the transfer screenshot.
func (h *AppHandler) UploadProof(c *gin.Context) {
	ctx := c.Request.Context()
	custID, ok := CustomerIDOf(c)
	if !ok {
		Fail(c, apierror.Unauthenticated(""))
		return
	}
	orderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		h.NotFoundApp(c)
		return
	}

	file, err := c.FormFile("proof")
	if err != nil {
		Fail(c, apierror.Validation("Pilih berkas bukti transfer."))
		return
	}
	if file.Size > payments.MaxProofBytes {
		Fail(c, apierror.New(http.StatusRequestEntityTooLarge, apierror.CodePayloadTooLarge,
			"Bukti transfer maksimal 5 MB."))
		return
	}
	f, err := file.Open()
	if err != nil {
		Fail(c, apierror.Validation("Berkas tidak bisa dibaca."))
		return
	}
	defer f.Close()
	buf := make([]byte, file.Size)
	if _, err := readFull(f, buf); err != nil {
		Fail(c, apierror.Validation("Berkas tidak bisa dibaca."))
		return
	}

	var declared money.IDR
	if v := strings.TrimSpace(c.PostForm("declared_amount")); v != "" {
		n, err := strconv.ParseInt(strings.NewReplacer(".", "", ",", "", " ", "").Replace(v), 10, 64)
		if err != nil || n < 0 {
			Fail(c, apierror.Validation("Nominal tidak valid."))
			return
		}
		declared = money.IDR(n)
	}
	sender, err := sanitize.Text("sender_name", c.PostForm("sender_name"), 0, 120)
	if err != nil {
		Fail(c, apierror.From(err))
		return
	}

	err = h.payments.SubmitProof(ctx, custID, orderID, buf, declared, sender)
	switch {
	case errors.Is(err, payments.ErrProofWrongType):
		Fail(c, apierror.New(http.StatusUnsupportedMediaType, apierror.CodeUnsupportedMedia,
			"Bukti transfer harus berupa JPG atau PNG."))
		return
	case errors.Is(err, payments.ErrProofTooLarge):
		Fail(c, apierror.New(http.StatusRequestEntityTooLarge, apierror.CodePayloadTooLarge,
			"Bukti transfer maksimal 5 MB."))
		return
	case errors.Is(err, payments.ErrDeadlinePassed):
		Fail(c, apierror.Conflict("PAYMENT_DEADLINE_PASSED",
			"Batas waktu pembayaran sudah lewat dan slot dilepas."))
		return
	case errors.Is(err, payments.ErrPaymentNotFound):
		h.NotFoundApp(c)
		return
	case err != nil:
		Fail(c, apierror.From(err))
		return
	}
	c.Redirect(http.StatusSeeOther, "/app/pembayaran/"+orderID.String())
}

// Orders lists the customer's orders.
func (h *AppHandler) Orders(c *gin.Context) {
	ctx := c.Request.Context()
	custID, ok := CustomerIDOf(c)
	if !ok {
		c.Redirect(http.StatusSeeOther, "/app/masuk?next=/app/pesanan")
		return
	}
	params, err := h.repo.Params(ctx)
	if err != nil {
		Fail(c, apierror.Internal(err))
		return
	}
	search := c.Query("q")
	if search != "" {
		if search, err = sanitize.Text("q", search, 0, 60); err != nil {
			Fail(c, apierror.Validation("Kata kunci terlalu panjang."))
			return
		}
	}

	d := h.base(c, params, "", "Pesanan saya — Evermore", "Riwayat pesanan.")
	d.NoIndex = true
	d.Search = search
	if d.Orders, err = h.customerOrders(ctx, custID, search); err != nil {
		Fail(c, apierror.Internal(err))
		return
	}
	h.render(c, "app-orders", http.StatusOK, d)
}

// Packages renders the credit balance and ledger.
func (h *AppHandler) Packages(c *gin.Context) {
	ctx := c.Request.Context()
	custID, ok := CustomerIDOf(c)
	if !ok {
		c.Redirect(http.StatusSeeOther, "/app/masuk?next=/app/paket")
		return
	}
	params, err := h.repo.Params(ctx)
	if err != nil {
		Fail(c, apierror.Internal(err))
		return
	}
	d := h.base(c, params, "", "Paket & kredit — Evermore", "Saldo kredit dan riwayatnya.")
	d.NoIndex = true
	if d.CustomerPackages, d.Ledger, err = h.creditView(ctx, custID); err != nil {
		Fail(c, apierror.Internal(err))
		return
	}
	if d.Packages, err = h.repo.Packages(ctx, time.Now().In(h.cfg.Location)); err != nil {
		Fail(c, apierror.Internal(err))
		return
	}
	h.render(c, "app-packages", http.StatusOK, d)
}

// LoginPage and RegisterPage render the forms.
func (h *AppHandler) LoginPage(c *gin.Context) { h.simplePage(c, "app-login", "Masuk — Evermore") }
func (h *AppHandler) RegisterPage(c *gin.Context) {
	h.simplePage(c, "app-register", "Daftar — Evermore")
}

func (h *AppHandler) simplePage(c *gin.Context, page, title string) {
	params, err := h.repo.Params(c.Request.Context())
	if err != nil {
		Fail(c, apierror.Internal(err))
		return
	}
	d := h.base(c, params, "", title, "")
	d.NoIndex = true
	d.Next = c.Query("next")
	h.render(c, page, http.StatusOK, d)
}

func (h *AppHandler) NotFoundApp(c *gin.Context) {
	params, _ := h.repo.Params(c.Request.Context())
	if params == nil {
		params = map[string]string{}
	}
	d := h.base(c, params, "", "Tidak ditemukan — Evermore", "")
	d.NoIndex = true
	d.Code = "404"
	d.Heading = "Tidak ditemukan"
	d.Message = "Halaman atau data yang kamu cari tidak ada."
	h.render(c, "error", http.StatusNotFound, d)
}

// quoteMessage turns a pricing failure into something a customer can act on.
func quoteMessage(err error) string {
	switch {
	case errors.Is(err, ordering.ErrMealNotPublished):
		return "Salah satu menu di keranjang sudah tidak tersedia. Hapus lalu pilih lagi."
	case strings.Contains(err.Error(), "PRICE_NOT_CONFIGURED"):
		return "Harga untuk menu ini belum diatur. Tim kami sudah diberi tahu."
	case errors.Is(err, ordering.ErrQtyOutOfRange):
		return "Jumlah porsi di luar batas yang diizinkan."
	default:
		return "Keranjang tidak bisa dihitung sekarang. Coba lagi sebentar."
	}
}

// checkoutError maps a checkout failure onto the JSON error model.
func checkoutError(err error) error {
	switch {
	case errors.Is(err, ordering.ErrPastCutOff):
		return apierror.Conflict(apierror.CodePastCutOff,
			"Batas pesan untuk salah satu tanggal sudah lewat.")
	case errors.Is(err, ordering.ErrCapacityFull):
		return apierror.Conflict(apierror.CodeCapacityFull,
			"Kapasitas untuk tanggal dan jam itu sudah penuh.")
	case errors.Is(err, ordering.ErrAddressNotOwned):
		return apierror.NotFound("Alamat tidak ditemukan.")
	case errors.Is(err, ordering.ErrMealNotPublished):
		return apierror.Conflict(apierror.CodeMenuNotPublished,
			"Menu itu belum terbit atau sudah ditarik.")
	case strings.Contains(err.Error(), "ADDRESS_NOT_SERVICEABLE"):
		return apierror.Conflict(apierror.CodeNotServiceable,
			"Alamat ini di luar wilayah antar kami.")
	case strings.Contains(err.Error(), "PRICE_NOT_CONFIGURED"):
		return apierror.Conflict(apierror.CodePriceNotConfigured,
			"Harga belum diatur untuk menu ini.")
	default:
		return apierror.From(err)
	}
}

var _ = order.StatusPaid
var _ = fmt.Sprintf
