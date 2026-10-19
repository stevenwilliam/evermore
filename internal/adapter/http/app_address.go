package http

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/stevenwilliam/evermore/internal/platform/apierror"
	"github.com/stevenwilliam/evermore/internal/platform/database"
	"github.com/stevenwilliam/evermore/internal/platform/id"
	"github.com/stevenwilliam/evermore/internal/platform/sanitize"
)

// Delivery addresses and the allergen profile (D40).
//
// customer_address and customer.allergen_profile existed from the first
// migration, and the checkout screen already rendered an address picker — but
// nothing could ever WRITE an address, so the picker was permanently empty and
// its "belum ada alamat" disabled state was the only branch a customer could
// reach. These are the screens that fill it.

// Indonesia's bounding box. This lives in the app layer, not in
// platform/sanitize: the platform packages are business-agnostic and portable
// (CLAUDE.md §2), and "the customer is in Indonesia" is a product rule.
// sanitize.LatLng still runs first for the global -90..90 / -180..180 check.
const (
	idnMinLat, idnMaxLat = -11.5, 6.5
	idnMinLng, idnMaxLng = 94.5, 141.5
)

// AllergenView is one allergen with the customer's current choice.
type AllergenView struct {
	Slug     string
	Name     string
	Selected bool
}

func (h *AppHandler) allergens(ctx context.Context, chosen []string) ([]AllergenView, error) {
	rows, err := h.db.QueryContext(ctx,
		`SELECT slug, name_id FROM allergen ORDER BY name_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	set := make(map[string]bool, len(chosen))
	for _, s := range chosen {
		set[s] = true
	}
	var out []AllergenView
	for rows.Next() {
		var a AllergenView
		if err := rows.Scan(&a.Slug, &a.Name); err != nil {
			return nil, err
		}
		a.Selected = set[a.Slug]
		out = append(out, a)
	}
	return out, rows.Err()
}

// allergenProfile reads the customer's saved slugs.
func (h *AppHandler) allergenProfile(ctx context.Context, customerID uuid.UUID) ([]string, error) {
	var raw []byte
	if err := h.db.QueryRowContext(ctx,
		`SELECT allergen_profile FROM customer WHERE id = $1`, customerID).Scan(&raw); err != nil {
		return nil, err
	}
	var out []string
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out)
	}
	return out, nil
}

// validSlugs keeps an unknown slug out of the profile. Reject, never repair.
func (h *AppHandler) validSlugs(ctx context.Context, want []string) ([]string, error) {
	rows, err := h.db.QueryContext(ctx, `SELECT slug FROM allergen`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	known := map[string]bool{}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		known[s] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	out := []string{}
	for _, s := range want {
		if !known[s] {
			return nil, apierror.Validation("Unknown allergen: " + s)
		}
		if seen[s] {
			continue // a duplicated checkbox is not an error, just a duplicate
		}
		seen[s] = true
		out = append(out, s)
	}
	return out, nil
}

// ---------------------------------------------------------------- addresses

// Addresses lists the customer's delivery addresses.
func (h *AppHandler) Addresses(c *gin.Context) {
	ctx := c.Request.Context()
	custID, ok := CustomerIDOf(c)
	if !ok {
		c.Redirect(http.StatusSeeOther, "/app/sign-in?next=/app/addresses")
		return
	}
	params, _ := h.repo.Params(ctx)
	list, err := h.addresses(ctx, custID)
	if err != nil {
		Fail(c, apierror.Internal(err))
		return
	}
	d := h.base(c, params, "", "Alamat pengantaran — Evermore", "Kelola alamat pengantaran.")
	d.NoIndex = true
	d.Addresses = list
	d.MapsKey = h.cfg.GoogleMapsBrowserKey
	if msg := c.Query("ok"); msg != "" {
		d.Notice = msg
	}
	if msg := c.Query("err"); msg != "" {
		d.Message = msg
	}
	h.render(c, "app-addresses", http.StatusOK, d)
}

// addressForm is one submitted address, already validated.
type addressForm struct {
	Label, Recipient, Phone    string
	Line, District, City, Prov string
	Postal, Note               string
	Lat, Lng                   float64
	PlaceID, Formatted         string
	MakeDefault                bool
}

// readAddressForm validates every field on the SERVER. The browser validates
// for feedback; this validates because the browser can be bypassed with curl
// (CLAUDE.md §4).
func readAddressForm(c *gin.Context) (*addressForm, error) {
	f := &addressForm{}
	var err error
	if f.Label, err = sanitize.Required("label", c.PostForm("label"), 40); err != nil {
		return nil, err
	}
	if f.Recipient, err = sanitize.Required("recipient_name", c.PostForm("recipient_name"), 120); err != nil {
		return nil, err
	}
	if f.Phone, err = sanitize.Phone("recipient_phone", c.PostForm("recipient_phone")); err != nil {
		return nil, err
	}
	if f.Line, err = sanitize.Required("address_line", c.PostForm("address_line"), 240); err != nil {
		return nil, err
	}
	if f.District, err = sanitize.Required("district", c.PostForm("district"), 80); err != nil {
		return nil, err
	}
	if f.City, err = sanitize.Required("city", c.PostForm("city"), 80); err != nil {
		return nil, err
	}
	if f.Prov, err = sanitize.Required("province", c.PostForm("province"), 80); err != nil {
		return nil, err
	}
	if f.Postal, err = sanitize.Text("postal_code", c.PostForm("postal_code"), 0, 10); err != nil {
		return nil, err
	}
	if f.Note, err = sanitize.Text("driver_note", c.PostForm("driver_note"), 0, 240); err != nil {
		return nil, err
	}
	if f.Formatted, err = sanitize.Text("formatted_address", c.PostForm("formatted_address"), 0, 300); err != nil {
		return nil, err
	}
	if f.PlaceID, err = sanitize.Text("google_place_id", c.PostForm("google_place_id"), 0, 120); err != nil {
		return nil, err
	}

	// The pin is MANDATORY: latitude and longitude are NOT NULL in the schema
	// because routing is done from the coordinates, not from the text.
	lat, laterr := strconv.ParseFloat(c.PostForm("latitude"), 64)
	lng, lngerr := strconv.ParseFloat(c.PostForm("longitude"), 64)
	if laterr != nil || lngerr != nil {
		return nil, apierror.Validation("Mark your location on the map before saving the address.")
	}
	if err := sanitize.LatLng("latitude", lat, lng); err != nil {
		return nil, apierror.From(err)
	}
	if lat < idnMinLat || lat > idnMaxLat || lng < idnMinLng || lng > idnMaxLng {
		return nil, apierror.Validation("That map point is outside Indonesia.")
	}
	f.Lat, f.Lng = lat, lng
	f.MakeDefault = c.PostForm("is_default") != ""
	return f, nil
}

// AddAddress creates one.
func (h *AppHandler) AddAddress(c *gin.Context) {
	ctx := c.Request.Context()
	custID, ok := CustomerIDOf(c)
	if !ok {
		c.Redirect(http.StatusSeeOther, "/app/sign-in?next=/app/addresses")
		return
	}
	f, err := readAddressForm(c)
	if err != nil {
		redirectWithErr(c, "/app/addresses", err)
		return
	}
	err = database.InTx(ctx, h.db, nil, func(tx *sql.Tx) error {
		// The FIRST address is always the default, whatever the box said: a
		// customer with addresses but no default cannot check out, because the
		// picker preselects nothing and the form requires a choice.
		var n int
		if err := tx.QueryRowContext(ctx,
			`SELECT count(*) FROM customer_address WHERE customer_id = $1 AND is_active`,
			custID).Scan(&n); err != nil {
			return err
		}
		makeDefault := f.MakeDefault || n == 0
		if makeDefault {
			if _, err := tx.ExecContext(ctx,
				`UPDATE customer_address SET is_default = false WHERE customer_id = $1`, custID); err != nil {
				return err
			}
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO customer_address
			  (id, customer_id, label, recipient_name, recipient_phone,
			   address_line, district, city, province, postal_code,
			   latitude, longitude, google_place_id, formatted_address,
			   driver_note, is_default, is_active)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,
			        NULLIF($13,''), NULLIF($14,''), $15, $16, true)`,
			id.New(), custID, f.Label, f.Recipient, f.Phone,
			f.Line, f.District, f.City, f.Prov, f.Postal,
			f.Lat, f.Lng, f.PlaceID, f.Formatted, f.Note, makeDefault)
		return err
	})
	if err != nil {
		redirectWithErr(c, "/app/addresses", err)
		return
	}
	c.Redirect(http.StatusSeeOther, "/app/addresses?ok=Alamat+disimpan.")
}

// EditAddress updates one the customer owns.
func (h *AppHandler) EditAddress(c *gin.Context) {
	ctx := c.Request.Context()
	custID, ok := CustomerIDOf(c)
	if !ok {
		c.Redirect(http.StatusSeeOther, "/app/sign-in?next=/app/addresses")
		return
	}
	addrID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		Fail(c, apierror.NotFound("Address not found."))
		return
	}
	f, err := readAddressForm(c)
	if err != nil {
		redirectWithErr(c, "/app/addresses", err)
		return
	}
	// Ownership is in the WHERE clause, not checked afterwards (CLAUDE.md §4):
	// an id belonging to another customer simply matches no row.
	res, err := h.db.ExecContext(ctx, `
		UPDATE customer_address
		   SET label=$3, recipient_name=$4, recipient_phone=$5, address_line=$6,
		       district=$7, city=$8, province=$9, postal_code=$10,
		       latitude=$11, longitude=$12,
		       google_place_id=NULLIF($13,''), formatted_address=NULLIF($14,''),
		       driver_note=$15, updated_at=now()
		 WHERE id=$1 AND customer_id=$2 AND is_active`,
		addrID, custID, f.Label, f.Recipient, f.Phone, f.Line,
		f.District, f.City, f.Prov, f.Postal, f.Lat, f.Lng,
		f.PlaceID, f.Formatted, f.Note)
	if err != nil {
		redirectWithErr(c, "/app/addresses", err)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		Fail(c, apierror.NotFound("Address not found."))
		return
	}
	if f.MakeDefault {
		if err := h.setDefaultAddress(ctx, custID, addrID); err != nil {
			redirectWithErr(c, "/app/addresses", err)
			return
		}
	}
	c.Redirect(http.StatusSeeOther, "/app/addresses?ok=Alamat+diperbarui.")
}

func (h *AppHandler) setDefaultAddress(ctx context.Context, custID, addrID uuid.UUID) error {
	return database.InTx(ctx, h.db, nil, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`UPDATE customer_address SET is_default = true, updated_at = now()
			  WHERE id = $1 AND customer_id = $2 AND is_active`, addrID, custID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return apierror.NotFound("Address not found.")
		}
		_, err = tx.ExecContext(ctx,
			`UPDATE customer_address SET is_default = false
			  WHERE customer_id = $1 AND id <> $2`, custID, addrID)
		return err
	})
}

// DefaultAddress marks one as the default.
func (h *AppHandler) DefaultAddress(c *gin.Context) {
	custID, ok := CustomerIDOf(c)
	if !ok {
		c.Redirect(http.StatusSeeOther, "/app/sign-in?next=/app/addresses")
		return
	}
	addrID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		Fail(c, apierror.NotFound("Address not found."))
		return
	}
	if err := h.setDefaultAddress(c.Request.Context(), custID, addrID); err != nil {
		redirectWithErr(c, "/app/addresses", err)
		return
	}
	c.Redirect(http.StatusSeeOther, "/app/addresses?ok=Alamat+utama+diubah.")
}

// RemoveAddress soft-deletes one.
//
// SOFT, because an address is referenced by past deliveries: a hard delete
// would either break that reference or rewrite delivery history, and history
// is append-only (CLAUDE.md §4).
func (h *AppHandler) RemoveAddress(c *gin.Context) {
	ctx := c.Request.Context()
	custID, ok := CustomerIDOf(c)
	if !ok {
		c.Redirect(http.StatusSeeOther, "/app/sign-in?next=/app/addresses")
		return
	}
	addrID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		Fail(c, apierror.NotFound("Address not found."))
		return
	}
	err = database.InTx(ctx, h.db, nil, func(tx *sql.Tx) error {
		var wasDefault bool
		err := tx.QueryRowContext(ctx,
			`UPDATE customer_address SET is_active = false, is_default = false, updated_at = now()
			  WHERE id = $1 AND customer_id = $2 AND is_active
			  RETURNING is_default`, addrID, custID).Scan(&wasDefault)
		if errors.Is(err, sql.ErrNoRows) {
			return apierror.NotFound("Address not found.")
		}
		if err != nil {
			return err
		}
		// Promote another one, or the customer is left with addresses and no
		// default and cannot check out.
		_, err = tx.ExecContext(ctx, `
			UPDATE customer_address SET is_default = true
			 WHERE id = (SELECT id FROM customer_address
			              WHERE customer_id = $1 AND is_active
			              ORDER BY created_at LIMIT 1)
			   AND NOT EXISTS (SELECT 1 FROM customer_address
			                    WHERE customer_id = $1 AND is_active AND is_default)`, custID)
		return err
	})
	if err != nil {
		redirectWithErr(c, "/app/addresses", err)
		return
	}
	c.Redirect(http.StatusSeeOther, "/app/addresses?ok=Alamat+dihapus.")
}

// ---------------------------------------------------------------- allergies

// Allergies renders the allergen profile.
func (h *AppHandler) Allergies(c *gin.Context) {
	ctx := c.Request.Context()
	custID, ok := CustomerIDOf(c)
	if !ok {
		c.Redirect(http.StatusSeeOther, "/app/sign-in?next=/app/allergies")
		return
	}
	chosen, err := h.allergenProfile(ctx, custID)
	if err != nil {
		Fail(c, apierror.Internal(err))
		return
	}
	list, err := h.allergens(ctx, chosen)
	if err != nil {
		Fail(c, apierror.Internal(err))
		return
	}
	params, _ := h.repo.Params(ctx)
	d := h.base(c, params, "", "Alergi saya — Evermore", "Kelola profil alergi.")
	d.NoIndex = true
	d.Allergens = list
	if msg := c.Query("ok"); msg != "" {
		d.Notice = msg
	}
	h.render(c, "app-allergies", http.StatusOK, d)
}

// SaveAllergies replaces the profile with the submitted set.
func (h *AppHandler) SaveAllergies(c *gin.Context) {
	ctx := c.Request.Context()
	custID, ok := CustomerIDOf(c)
	if !ok {
		c.Redirect(http.StatusSeeOther, "/app/sign-in?next=/app/allergies")
		return
	}
	slugs, err := h.validSlugs(ctx, c.PostFormArray("allergen"))
	if err != nil {
		redirectWithErr(c, "/app/allergies", err)
		return
	}
	blob, err := json.Marshal(slugs)
	if err != nil {
		Fail(c, apierror.Internal(err))
		return
	}
	if _, err := h.db.ExecContext(ctx,
		`UPDATE customer SET allergen_profile = $2 WHERE id = $1`, custID, blob); err != nil {
		Fail(c, apierror.Internal(err))
		return
	}
	c.Redirect(http.StatusSeeOther, "/app/allergies?ok=Profil+alergi+disimpan.")
}

// redirectWithErr sends a browser back to the page with a readable message,
// rather than replacing the screen with a JSON error object.
func redirectWithErr(c *gin.Context, path string, err error) {
	msg := "Tidak bisa disimpan. Periksa isian lalu coba lagi."
	var ae *apierror.Error
	if errors.As(err, &ae) {
		msg = ae.Message
	}
	var se *sanitize.Error
	if errors.As(err, &se) {
		msg = se.Message
	}
	c.Redirect(http.StatusSeeOther, path+"?err="+url.QueryEscape(msg))
}
