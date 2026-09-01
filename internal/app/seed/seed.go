// Package seed loads the demo dataset.
//
// Everything here mirrors the design artifact so the running site shows what
// was designed: PT Evermore Nutrisi Indonesia, three kitchens (Tebet KTC-01,
// Kebayoran KBY-02, Kelapa Gading KLG-03), four diet types, the tier prices
// 78k/75k/71k, the 10/20/40-portion packages, and Sinta Prameswari's account
// with 13 credits of a 20-portion Balanced package.
//
// It is idempotent: every insert is ON CONFLICT DO NOTHING or keyed on a
// stable slug, so running it twice changes nothing.
package seed

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/stevenwilliam/evermore/internal/platform/database"
	"github.com/stevenwilliam/evermore/internal/platform/id"
	"github.com/stevenwilliam/evermore/internal/platform/security"
)

// Result reports what the seed did, so the CLI can say something true.
type Result struct {
	Counts map[string]int
}

// Run loads the dataset. loc is the operating timezone; today anchors the menu
// calendar so the seeded week is always the current one.
func Run(ctx context.Context, db *sql.DB, loc *time.Location, today time.Time) (*Result, error) {
	res := &Result{Counts: map[string]int{}}
	err := database.InTx(ctx, db, nil, func(tx *sql.Tx) error {
		s := &seeder{ctx: ctx, tx: tx, loc: loc, today: today, res: res, ids: map[string]uuid.UUID{}}
		steps := []struct {
			name string
			fn   func() error
		}{
			{"sys_parameters", s.sysParameters},
			{"permissions", s.permissions},
			{"roles", s.roles},
			{"customer_types", s.customerTypes},
			{"allergens", s.allergens},
			{"diet_types", s.dietTypes},
			{"slots", s.slots},
			{"kitchens", s.kitchens},
			{"foods", s.foods},
			{"tiers", s.tiers},
			{"prices", s.prices},
			{"packages", s.packages},
			{"bank_account", s.bankAccount},
			{"delivery_fee", s.deliveryFee},
			{"users", s.users},
			{"menu", s.menu},
			{"capacity", s.capacity},
			{"demo_package", s.demoPackage},
		}
		for _, st := range steps {
			if err := st.fn(); err != nil {
				return fmt.Errorf("seed %s: %w", st.name, err)
			}
		}
		return nil
	})
	return res, err
}

type seeder struct {
	ctx   context.Context
	tx    *sql.Tx
	loc   *time.Location
	today time.Time
	res   *Result
	// ids caches natural-key -> actual id lookups.
	ids map[string]uuid.UUID
}

func (s *seeder) exec(table, q string, args ...any) error {
	r, err := s.tx.ExecContext(s.ctx, q, args...)
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	s.res.Counts[table] += int(n)
	return nil
}

// idFor returns a stable UUID for a natural key, so a fresh seed produces the
// same ids every time.
//
// It is the id the seed WANTS to use. It is not necessarily the id that ends
// up in the database: every insert here is ON CONFLICT (natural key) DO
// NOTHING, so if a row already exists under a different id — inserted by a
// test, a fixture or a human — the seed's own id was never written. Resolving
// references through idFor in that case produces a foreign-key violation, so
// references go through resolve() instead.
func idFor(ns, key string) uuid.UUID {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("evermore:"+ns+":"+key))
}

// resolve reads back the id that is actually in the database for a natural
// key, caching it. This is what makes the seed idempotent against a database
// that already holds some of these rows under other ids.
func (s *seeder) resolve(cacheKey, query string, args ...any) (uuid.UUID, error) {
	if id, ok := s.ids[cacheKey]; ok {
		return id, nil
	}
	var id uuid.UUID
	if err := s.tx.QueryRowContext(s.ctx, query, args...).Scan(&id); err != nil {
		return uuid.Nil, fmt.Errorf("resolving %s: %w", cacheKey, err)
	}
	if id == uuid.Nil {
		return uuid.Nil, fmt.Errorf("resolving %s: scanned a nil uuid", cacheKey)
	}
	s.ids[cacheKey] = id
	return id, nil
}

func (s *seeder) slotID(at string) (uuid.UUID, error) {
	return s.resolve("slot:"+at, `SELECT id FROM delivery_time_slot WHERE slot_time = $1::time`, at)
}
func (s *seeder) dietID(slug string) (uuid.UUID, error) {
	return s.resolve("diet:"+slug, `SELECT id FROM diet_type WHERE slug = $1`, slug)
}
func (s *seeder) foodID(slug string) (uuid.UUID, error) {
	return s.resolve("food:"+slug, `SELECT id FROM food WHERE slug = $1`, slug)
}
func (s *seeder) kitchenID(code string) (uuid.UUID, error) {
	return s.resolve("kitchen:"+code, `SELECT id FROM kitchen WHERE code = $1`, code)
}
func (s *seeder) packageID(slug string) (uuid.UUID, error) {
	return s.resolve("package:"+slug, `SELECT id FROM package WHERE slug = $1`, slug)
}
func (s *seeder) ctypeID(slug string) (uuid.UUID, error) {
	return s.resolve("ctype:"+slug, `SELECT id FROM customer_type WHERE slug = $1`, slug)
}
func (s *seeder) roleID(slug string) (uuid.UUID, error) {
	return s.resolve("role:"+slug, `SELECT id FROM role WHERE slug = $1`, slug)
}
func (s *seeder) permID(code string) (uuid.UUID, error) {
	return s.resolve("perm:"+code, `SELECT id FROM permission WHERE code = $1`, code)
}
func (s *seeder) allergenID(slug string) (uuid.UUID, error) {
	return s.resolve("allergen:"+slug, `SELECT id FROM allergen WHERE slug = $1`, slug)
}

// tierID resolves the tier band containing qty, whoever created it.
func (s *seeder) tierID(qty int) (uuid.UUID, error) {
	return s.resolve(fmt.Sprintf("tier:%d", qty), `
		SELECT id FROM meal_price_tier
		 WHERE is_active AND min_qty <= $1 AND (max_qty IS NULL OR max_qty >= $1)
		 ORDER BY min_qty LIMIT 1`, qty)
}

// ---------------------------------------------------------------------------

func (s *seeder) sysParameters() error {
	type p struct{ key, val, typ, cat, label string }
	// CLAUDE.md §7: anything that could change without a code change lives
	// here. Every value the artifact shows as configurable is a row.
	params := []p{
		{"company.name", "PT Evermore Nutrisi Indonesia", "string", "company", "Nama perusahaan"},
		{"company.brand", "Evermore", "string", "company", "Nama merek"},
		{"company.phone", "+622129000123", "string", "company", "Telepon perusahaan"},
		{"company.email", "halo@evermore.co.id", "string", "company", "Email perusahaan"},
		{"company.address", "Jl. Tebet Raya No. 88, Jakarta Selatan 12820", "string", "company", "Alamat perusahaan"},
		{"company.npwp", "00.000.000.0-000.000", "string", "company", "NPWP"},

		{"tax.rate_bps", "1100", "int", "tax", "Tarif PPN dalam basis poin (1100 = 11%)"},
		{"tax.inclusive", "true", "bool", "tax", "Harga sudah termasuk pajak"},

		{"order.cutoff_time", "15:00", "time", "order", "Batas pesan untuk layanan besok (WIB)"},
		{"order.max_qty", "999", "int", "order", "Jumlah maksimum porsi per pesanan"},
		{"order.min_qty", "1", "int", "order", "Jumlah minimum porsi per pesanan"},
		{"order.payment_window_hours", "3", "int", "order", "Batas waktu transfer, dalam jam"},
		{"order.number_prefix", "EVM", "string", "order", "Awalan nomor pesanan"},
		{"package.number_prefix", "PKG", "string", "order", "Awalan nomor paket"},

		{"menu.publish_weekday", "5", "int", "menu", "Hari terbit menu minggu berikutnya (5 = Jumat)"},
		{"menu.autopublish_time", "09:00", "time", "menu", "Jam terbit otomatis"},
		{"menu.horizon_days", "14", "int", "menu", "Berapa hari ke depan menu ditampilkan"},

		{"delivery.free_all_distances", "true", "bool", "delivery", "Ongkos kirim gratis di semua jarak"},
		{"kitchen.default_capacity", "40", "int", "kitchen", "Kuota porsi per dapur per slot"},

		{"payment.proof_max_bytes", "5242880", "int", "payment", "Ukuran maksimum bukti transfer"},
		{"payment.verify_sla_minutes", "30", "int", "payment", "Target verifikasi manual"},

		{"notify.whatsapp_enabled", "false", "bool", "notify", "Kirim notifikasi WhatsApp"},
		{"notify.email_enabled", "true", "bool", "notify", "Kirim notifikasi email"},

		{"seo.site_name", "Evermore", "string", "seo", "Nama situs untuk Open Graph"},
		{"seo.default_title", "Evermore — katering sehat harian Jakarta", "string", "seo", "Judul bawaan"},
		{"seo.default_description", "Menu baru tiap minggu, disusun ahli gizi, dimasak pagi hari dan diantar ke meja kerja atau rumah pada jam yang kamu pilih.", "string", "seo", "Deskripsi bawaan"},
	}
	for _, x := range params {
		if err := s.exec("sys_parameters", `
			INSERT INTO sys_parameters (id, key, value, value_type, category, label_id)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (key) DO NOTHING`,
			idFor("param", x.key), x.key, x.val, x.typ, x.cat, x.label); err != nil {
			return err
		}
	}
	return nil
}

func (s *seeder) permissions() error {
	perms := []struct{ code, desc, cat string }{
		{"dashboard.view", "Lihat dasbor", "dashboard"},
		{"menu.view", "Lihat jadwal menu", "menu"},
		{"menu.manage", "Ubah dan terbitkan jadwal menu", "menu"},
		{"food.view", "Lihat makanan dan gizi", "catalogue"},
		{"food.manage", "Ubah makanan dan gizi", "catalogue"},
		{"price.view", "Lihat harga", "pricing"},
		{"price.manage", "Ubah harga", "pricing"},
		{"order.view", "Lihat pesanan", "order"},
		{"order.manage", "Ubah pesanan", "order"},
		{"payment.view", "Lihat pembayaran", "payment"},
		{"payment.verify", "Verifikasi dan tolak pembayaran", "payment"},
		{"payment.refund", "Kembalikan dana transfer keliru", "payment"},
		{"delivery.view", "Lihat pengantaran", "delivery"},
		{"delivery.manage", "Ubah pengantaran dan penugasan dapur", "delivery"},
		{"kitchen.view", "Lihat dapur dan wilayah", "kitchen"},
		{"kitchen.manage", "Ubah dapur, wilayah dan kapasitas", "kitchen"},
		{"customer.view", "Lihat pelanggan", "customer"},
		{"customer.manage", "Ubah pelanggan", "customer"},
		{"package.view", "Lihat paket dan kredit", "package"},
		{"package.manage", "Ubah paket dan kredit", "package"},
		{"report.view", "Lihat laporan", "report"},
		{"report.export", "Ekspor laporan ke CSV", "report"},
		{"settings.view", "Lihat pengaturan sistem", "settings"},
		{"settings.manage", "Ubah pengaturan sistem", "settings"},
		{"audit.view", "Lihat catatan audit", "audit"},
		{"user.manage", "Kelola pengguna dan peran", "identity"},
	}
	for _, p := range perms {
		if err := s.exec("permission", `
			INSERT INTO permission (id, code, description, category)
			VALUES ($1, $2, $3, $4) ON CONFLICT (code) DO NOTHING`,
			idFor("perm", p.code), p.code, p.desc, p.cat); err != nil {
			return err
		}
	}
	return nil
}

// rolePermissions is deny-by-default: a role has exactly what is listed.
var rolePermissions = map[string][]string{
	"admin": {"*"},
	"ops_manager": {
		"dashboard.view", "menu.view", "menu.manage", "food.view", "food.manage",
		"price.view", "order.view", "order.manage", "payment.view", "payment.verify",
		"delivery.view", "delivery.manage", "kitchen.view", "kitchen.manage",
		"customer.view", "customer.manage", "package.view", "package.manage",
		"report.view", "report.export", "settings.view",
	},
	"finance": {
		"dashboard.view", "order.view", "payment.view", "payment.verify",
		"report.view", "report.export", "customer.view",
	},
	"kitchen_staff": {
		"dashboard.view", "menu.view", "food.view", "delivery.view", "report.view",
	},
	"customer": {},
}

func (s *seeder) roles() error {
	roles := []struct {
		slug, name string
		isStaff    bool
	}{
		{"admin", "Administrator", true},
		{"ops_manager", "Ops Manager", true},
		{"finance", "Keuangan", true},
		{"kitchen_staff", "Staf Dapur", true},
		{"customer", "Pelanggan", false},
	}
	for _, r := range roles {
		if err := s.exec("role", `
			INSERT INTO role (id, name, slug, is_staff, is_system)
			VALUES ($1, $2, $3, $4, true) ON CONFLICT (slug) DO NOTHING`,
			idFor("role", r.slug), r.name, r.slug, r.isStaff); err != nil {
			return err
		}
	}
	for slug, codes := range rolePermissions {
		roleID, err := s.roleID(slug)
		if err != nil {
			return err
		}
		if len(codes) == 1 && codes[0] == "*" {
			if err := s.exec("role_permission", `
				INSERT INTO role_permission (role_id, permission_id)
				SELECT $1, id FROM permission
				ON CONFLICT DO NOTHING`, roleID); err != nil {
				return err
			}
			continue
		}
		for _, c := range codes {
			pid, err := s.permID(c)
			if err != nil {
				return err
			}
			if err := s.exec("role_permission", `
				INSERT INTO role_permission (role_id, permission_id)
				VALUES ($1, $2) ON CONFLICT DO NOTHING`, roleID, pid); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *seeder) customerTypes() error {
	types := []struct {
		slug, name  string
		isCorporate bool
		order       int
	}{
		{"retail", "Retail", false, 1},
		{"corporate", "Corporate", true, 2},
	}
	for _, t := range types {
		if err := s.exec("customer_type", `
			INSERT INTO customer_type (id, name, slug, is_corporate, sort_order)
			VALUES ($1, $2, $3, $4, $5) ON CONFLICT (slug) DO NOTHING`,
			idFor("ctype", t.slug), t.name, t.slug, t.isCorporate, t.order); err != nil {
			return err
		}
	}
	corpID, err := s.ctypeID("corporate")
	if err != nil {
		return err
	}
	return s.exec("organisation", `
		INSERT INTO organisation (id, customer_type_id, name, pic_name, billing_email, is_invoice_billing)
		VALUES ($1, $2, 'PT Sinar Mas', 'Dewi Anggraini', 'dewi@sinarmas.example', true)
		ON CONFLICT DO NOTHING`, idFor("org", "sinar-mas"), corpID)
}

func (s *seeder) allergens() error {
	list := []struct{ slug, idn, eng string }{
		{"kedelai", "Kedelai", "Soy"},
		{"wijen", "Wijen", "Sesame"},
		{"kacang", "Kacang", "Peanut"},
		{"telur", "Telur", "Egg"},
		{"susu", "Susu", "Milk"},
		{"gluten", "Gluten", "Gluten"},
		{"makanan-laut", "Makanan laut", "Seafood"},
		{"ikan", "Ikan", "Fish"},
	}
	for _, a := range list {
		if err := s.exec("allergen", `
			INSERT INTO allergen (id, name_id, name_en, slug)
			VALUES ($1, $2, $3, $4) ON CONFLICT (slug) DO NOTHING`,
			idFor("allergen", a.slug), a.idn, a.eng, a.slug); err != nil {
			return err
		}
	}
	return nil
}

func (s *seeder) dietTypes() error {
	// The four the artifact shows across every screen.
	diets := []struct {
		slug, name, desc string
		order            int
	}{
		{"balanced", "Balanced", "Gizi seimbang untuk hari kerja biasa.", 1},
		{"weight-loss", "Weight Loss", "Defisit kalori terkontrol, protein tetap tinggi.", 2},
		{"muscle-gain", "Muscle Gain", "Protein tinggi untuk yang sedang menambah massa otot.", 3},
		{"special-diet", "Special Diet", "Menu untuk kebutuhan khusus, disusun bersama ahli gizi.", 4},
	}
	for _, d := range diets {
		hasSub := d.slug == "special-diet"
		if err := s.exec("diet_type", `
			INSERT INTO diet_type (id, name, slug, description, sort_order, has_subtypes)
			VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT (slug) DO NOTHING`,
			idFor("diet", d.slug), d.name, d.slug, d.desc, d.order, hasSub); err != nil {
			return err
		}
	}
	subs := []struct{ slug, name string }{
		{"rendah-garam", "Rendah garam"},
		{"rendah-gula", "Rendah gula"},
		{"bebas-gluten", "Bebas gluten"},
		{"vegetarian", "Vegetarian"},
	}
	for _, x := range subs {
		sdID, err := s.dietID("special-diet")
		if err != nil {
			return err
		}
		if err := s.exec("diet_subtype", `
			INSERT INTO diet_subtype (id, diet_type_id, name, slug)
			VALUES ($1, $2, $3, $4) ON CONFLICT (diet_type_id, slug) DO NOTHING`,
			idFor("dietsub", x.slug), sdID, x.name, x.slug); err != nil {
			return err
		}
	}
	return nil
}

func (s *seeder) slots() error {
	// Exactly the slots the artifact shows.
	list := []struct {
		t, alias, period string
		order            int
	}{
		{"07:00:00", "Sarapan", "breakfast", 1},
		{"11:30:00", "Makan siang", "lunch", 2},
		{"12:00:00", "Makan siang", "lunch", 3},
		{"12:30:00", "Makan siang", "lunch", 4},
		{"17:30:00", "Makan malam", "dinner", 5},
		{"18:00:00", "Makan malam", "dinner", 6},
		{"18:30:00", "Makan malam", "dinner", 7},
	}
	for _, x := range list {
		if err := s.exec("delivery_time_slot", `
			INSERT INTO delivery_time_slot (id, slot_time, alias, meal_period, sort_order)
			VALUES ($1, $2::time, $3, $4, $5) ON CONFLICT (slot_time) DO NOTHING`,
			idFor("slot", x.t), x.t, x.alias, x.period, x.order); err != nil {
			return err
		}
	}
	return nil
}

func (s *seeder) kitchens() error {
	list := []struct {
		code, name, addr string
		lat, lng         float64
		radius           float64
		priority         int
	}{
		{"KTC-01", "Dapur Tebet", "Jl. Tebet Raya No. 88, Jakarta Selatan", -6.2260, 106.8480, 6.5, 1},
		{"KBY-02", "Dapur Kebayoran", "Jl. Gandaria Tengah III No. 4, Jakarta Selatan", -6.2440, 106.7980, 6.0, 2},
		{"KLG-03", "Dapur Kelapa Gading", "Jl. Boulevard Raya Blok C No. 12, Jakarta Utara", -6.1580, 106.9060, 7.0, 3},
	}
	for _, k := range list {
		if err := s.exec("kitchen", `
			INSERT INTO kitchen (id, code, name, address_line, latitude, longitude,
			                     service_radius_km, priority, default_daily_capacity, pic_name)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 40, 'Kepala Dapur')
			ON CONFLICT (code) DO NOTHING`,
			idFor("kitchen", k.code), k.code, k.name, k.addr, k.lat, k.lng, k.radius, k.priority); err != nil {
			return err
		}
		kid, err := s.kitchenID(k.code)
		if err != nil {
			return err
		}
		// Every kitchen serves every slot, except Kelapa Gading which the
		// dashboard shows as closed at 18.00.
		for _, st := range []string{"07:00:00", "11:30:00", "12:00:00", "12:30:00", "17:30:00", "18:00:00", "18:30:00"} {
			if k.code == "KLG-03" && (st == "18:00:00" || st == "18:30:00" || st == "17:30:00") {
				continue
			}
			sid, err := s.slotID(st)
			if err != nil {
				return err
			}
			if err := s.exec("kitchen_slot", `
				INSERT INTO kitchen_slot (kitchen_id, slot_id) VALUES ($1, $2)
				ON CONFLICT DO NOTHING`, kid, sid); err != nil {
				return err
			}
		}
		// D14: kitchens operate every day.
		for wd := 1; wd <= 7; wd++ {
			if err := s.exec("kitchen_operating_day", `
				INSERT INTO kitchen_operating_day (kitchen_id, weekday, is_open)
				VALUES ($1, $2, true) ON CONFLICT DO NOTHING`, kid, wd); err != nil {
				return err
			}
		}
	}
	return nil
}

// foodSpec is one catalogue item with its panel, in base units.
type foodSpec struct {
	slug, name, portion string
	kcal                int
	proteinMG, fatMG    int
	satFatMG, carbMG    int
	sugarMG, fibreMG    int
	sodiumMG, cholMG    int
	allergens           []string
	diets               []string
}

// foods are the components the artifact names, with panels that sum to the
// meal totals it shows (520 kkal / 38,2 g protein / 640 mg sodium for the
// Monday Balanced lunch).
var foods = []foodSpec{
	{"ayam-panggang-lemon", "Ayam panggang lemon", "150 g", 245, 31200, 9100, 2400, 2100, 800, 400, 380, 95000,
		nil, []string{"balanced", "muscle-gain", "weight-loss"}},
	{"quinoa-herba", "Quinoa herba", "120 g", 180, 5300, 5600, 700, 31200, 1200, 3800, 140, 0,
		nil, []string{"balanced", "weight-loss", "muscle-gain"}},
	{"brokoli-kukus", "Brokoli kukus", "100 g", 55, 1600, 3200, 400, 8300, 1700, 4600, 90, 0,
		nil, []string{"balanced", "weight-loss", "muscle-gain", "special-diet"}},
	{"infused-water-timun", "Infused water timun", "250 ml", 40, 100, 200, 0, 3000, 2100, 600, 30, 0,
		nil, []string{"balanced", "weight-loss", "muscle-gain", "special-diet"}},

	{"nasi-merah", "Nasi merah", "150 g", 165, 3800, 1300, 300, 34500, 400, 2800, 5, 0,
		nil, []string{"balanced", "muscle-gain"}},
	{"rendang-jamur", "Rendang jamur", "140 g", 245, 12400, 14200, 6100, 12800, 3200, 4200, 520, 0,
		[]string{"kedelai"}, []string{"balanced", "special-diet"}},

	{"dori-panggang", "Dori panggang", "140 g", 210, 28600, 8400, 1900, 1200, 300, 200, 310, 68000,
		[]string{"ikan"}, []string{"balanced", "weight-loss"}},
	{"kentang-herba", "Kentang herba", "130 g", 160, 3400, 4100, 600, 27800, 1500, 3100, 180, 0,
		nil, []string{"balanced", "weight-loss"}},

	{"salmon-teriyaki", "Salmon teriyaki", "130 g", 330, 32800, 18600, 4200, 8400, 6800, 300, 640, 82000,
		[]string{"ikan", "kedelai", "wijen"}, []string{"balanced", "muscle-gain"}},
	{"tumis-ayam-paprika", "Tumis ayam paprika", "140 g", 285, 30100, 14300, 3600, 7200, 4100, 1900, 480, 88000,
		[]string{"kedelai"}, []string{"weight-loss", "muscle-gain"}},

	{"ayam-kari-hijau", "Ayam kari hijau", "150 g", 290, 29800, 16200, 7400, 6800, 3400, 2100, 590, 91000,
		[]string{"ikan"}, []string{"balanced", "muscle-gain"}},
	{"brown-rice", "Brown rice", "150 g", 170, 3900, 1400, 300, 35200, 500, 2900, 8, 0,
		nil, []string{"balanced", "muscle-gain", "weight-loss"}},

	{"sop-iga-bening", "Sop iga bening", "200 g", 320, 26400, 19800, 8200, 4600, 1800, 900, 720, 78000,
		nil, []string{"balanced"}},
	{"ubi-kukus", "Ubi kukus", "120 g", 140, 2100, 200, 100, 32400, 8600, 4300, 40, 0,
		nil, []string{"balanced", "weight-loss", "special-diet"}},

	{"overnight-oats", "Overnight oats", "180 g", 290, 11200, 8600, 1800, 42300, 12400, 6800, 95, 0,
		[]string{"susu", "gluten"}, []string{"balanced", "weight-loss"}},
	{"buah-potong", "Buah potong", "120 g", 90, 1100, 300, 0, 22600, 18200, 2900, 5, 0,
		nil, []string{"balanced", "weight-loss", "muscle-gain", "special-diet"}},
}

func (s *seeder) foods() error {
	for _, f := range foods {
		fid := idFor("food", f.slug)

		if err := s.exec("food", `
			INSERT INTO food (id, name, slug, portion_size, description)
			VALUES ($1, $2, $3, $4, '') ON CONFLICT (slug) DO NOTHING`,
			fid, f.name, f.slug, f.portion); err != nil {
			return err
		}
		nutFID, err := s.foodID(f.slug)
		if err != nil {
			return err
		}
		if err := s.exec("food_nutrition", `
			INSERT INTO food_nutrition (id, food_id, calories_kcal, protein_mg, fat_mg,
			    saturated_fat_mg, carbohydrate_mg, sugar_mg, fibre_mg, sodium_mg, cholesterol_mg)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			ON CONFLICT (food_id) DO NOTHING`,
			idFor("nutrition", f.slug), nutFID, f.kcal, f.proteinMG, f.fatMG, f.satFatMG,
			f.carbMG, f.sugarMG, f.fibreMG, f.sodiumMG, f.cholMG); err != nil {
			return err
		}
		realFID, err := s.foodID(f.slug)
		if err != nil {
			return err
		}
		for _, a := range f.allergens {
			aid, err := s.allergenID(a)
			if err != nil {
				return err
			}
			if err := s.exec("food_allergen", `
				INSERT INTO food_allergen (food_id, allergen_id) VALUES ($1, $2)
				ON CONFLICT DO NOTHING`, realFID, aid); err != nil {
				return err
			}
		}
		for _, d := range f.diets {
			did, err := s.dietID(d)
			if err != nil {
				return err
			}
			if err := s.exec("food_diet_type", `
				INSERT INTO food_diet_type (food_id, diet_type_id) VALUES ($1, $2)
				ON CONFLICT DO NOTHING`, realFID, did); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *seeder) tiers() error {
	// 1-3, 4-9, 10+ — exactly the bands the cart screen shows.
	list := []struct {
		key, name string
		min       int
		max       *int
		order     int
	}{
		{"t1", "Tier 1", 1, intp(3), 1},
		{"t2", "Tier 2", 4, intp(9), 2},
		{"t3", "Tier 3", 10, nil, 3},
	}
	// meal_price_tier has no unique natural key, and its EXCLUDE constraint
	// means an existing ladder cannot be inserted over. If any active band
	// exists, that ladder is the one in force and this is a no-op.
	var existing int
	if err := s.tx.QueryRowContext(s.ctx,
		`SELECT count(*) FROM meal_price_tier WHERE is_active`).Scan(&existing); err != nil {
		return err
	}
	if existing > 0 {
		return nil
	}
	for _, t := range list {
		if err := s.exec("meal_price_tier", `
			INSERT INTO meal_price_tier (id, name, min_qty, max_qty, sort_order)
			VALUES ($1, $2, $3, $4, $5) ON CONFLICT (id) DO NOTHING`,
			idFor("tier", t.key), t.name, t.min, t.max, t.order); err != nil {
			return err
		}
	}
	return nil
}

func intp(i int) *int { return &i }

func (s *seeder) prices() error {
	// DEFAULT scope: 78k / 75k / 71k, tax-inclusive, open-ended from the start
	// of the current month so every order date resolves.
	from := time.Date(s.today.Year(), s.today.Month(), 1, 0, 0, 0, 0, s.loc)
	diets := []string{"balanced", "weight-loss", "muscle-gain", "special-diet"}
	prices := map[string]int64{"t1": 78000, "t2": 75000, "t3": 71000}

	tierQty := map[string]int{"t1": 1, "t2": 4, "t3": 10}
	for _, d := range diets {
		did, err := s.dietID(d)
		if err != nil {
			return err
		}
		for tierKey, price := range prices {
			tid, err := s.tierID(tierQty[tierKey])
			if err != nil {
				return err
			}
			if err := s.exec("meal_price_normal", `
				INSERT INTO meal_price_normal (id, customer_type_id, diet_type_id, tier_id, unit_price_idr, validity)
				VALUES ($1, NULL, $2, $3, $4, daterange($5::date, NULL, '[)'))
				ON CONFLICT ON CONSTRAINT meal_price_normal_no_overlap DO NOTHING`,
				idFor("mpn", d+":"+tierKey), did, tid,
				price, from.Format("2006-01-02")); err != nil {
				return err
			}
		}
	}
	// Corporate scope for PT Sinar Mas, matching the price-resolver screen:
	// 6 meals resolves to Rp 75.000 on the corporate Balanced row.
	corpPrices := map[string]int64{"t1": 78000, "t2": 75000, "t3": 71000}
	corpID, err := s.ctypeID("corporate")
	if err != nil {
		return err
	}
	balID, err := s.dietID("balanced")
	if err != nil {
		return err
	}
	for tierKey, price := range corpPrices {
		tid, err := s.tierID(tierQty[tierKey])
		if err != nil {
			return err
		}
		if err := s.exec("meal_price_normal", `
			INSERT INTO meal_price_normal (id, customer_type_id, diet_type_id, tier_id, unit_price_idr, validity)
			VALUES ($1, $2, $3, $4, $5, daterange($6::date, NULL, '[)'))
			ON CONFLICT ON CONSTRAINT meal_price_normal_no_overlap DO NOTHING`,
			idFor("mpn-corp", "balanced:"+tierKey), corpID, balID, tid, price,
			from.Format("2006-01-02")); err != nil {
			return err
		}
	}
	return nil
}

func (s *seeder) packages() error {
	from := time.Date(s.today.Year(), s.today.Month(), 1, 0, 0, 0, 0, s.loc)
	list := []struct {
		slug, name string
		credits    int
		days       int
		price      int64
		featured   bool
		order      int
	}{
		{"paket-10", "Paket 10 porsi", 10, 60, 750000, false, 1},
		{"paket-20", "Paket 20 porsi", 20, 90, 1420000, true, 2},
		{"paket-40", "Paket 40 porsi", 40, 120, 2720000, false, 3},
	}
	for _, p := range list {
		pid := idFor("package", p.slug)
		if err := s.exec("package", `
			INSERT INTO package (id, name, slug, meal_credits, validity_days, is_featured, sort_order, description)
			VALUES ($1, $2, $3, $4, $5, $6, $7, '')
			ON CONFLICT (slug) DO NOTHING`,
			pid, p.name, p.slug, p.credits, p.days, p.featured, p.order); err != nil {
			return err
		}
		realPID, err := s.packageID(p.slug)
		if err != nil {
			return err
		}
		if err := s.exec("package_price_normal", `
			INSERT INTO package_price_normal (id, customer_type_id, package_id, total_price_idr, validity)
			VALUES ($1, NULL, $2, $3, daterange($4::date, NULL, '[)'))
			ON CONFLICT ON CONSTRAINT package_price_normal_no_overlap DO NOTHING`,
			idFor("ppn", p.slug), realPID, p.price, from.Format("2006-01-02")); err != nil {
			return err
		}
	}
	return nil
}

func (s *seeder) bankAccount() error {
	// D14: a dummy account, replaceable in the back office. These are the
	// digits the artifact shows.
	return s.exec("bank_account", `
		INSERT INTO bank_account (id, bank_name, account_number, account_holder, branch, sort_order)
		VALUES ($1, 'BCA', '5391184402', 'PT Evermore Nutrisi Indonesia', 'KCP Tebet', 1)
		ON CONFLICT (bank_name, account_number) DO NOTHING`,
		idFor("bank", "bca-5391184402"))
}

func (s *seeder) deliveryFee() error {
	// D14: free at every distance, but the band engine still runs on every
	// order so charging later is one settings edit.
	return s.exec("delivery_fee_band", `
		INSERT INTO delivery_fee_band (id, min_distance_m, max_distance_m, fee_idr)
		VALUES ($1, 0, NULL, 0) ON CONFLICT (id) DO NOTHING`,
		idFor("fee", "free-all"))
}

// DemoPassword is the password every seeded account uses. Development only —
// the deployment handbook rotates it before anything real.
const DemoPassword = "Evermore#2026"

func (s *seeder) users() error {
	hash, err := security.HashPassword(DemoPassword)
	if err != nil {
		return err
	}
	staff := []struct {
		email, name, role, kitchen, title string
	}{
		{"admin@evermore.co.id", "Administrator", "admin", "", "Administrator"},
		{"ratna@evermore.co.id", "Ratna Wijaya", "ops_manager", "KTC-01", "Ops Manager"},
		{"finance@evermore.co.id", "Bagus Setiawan", "finance", "", "Staf Keuangan"},
		{"dapur.tebet@evermore.co.id", "Kepala Dapur Tebet", "kitchen_staff", "KTC-01", "Kepala Dapur"},
	}
	for _, u := range staff {
		uid := idFor("user", u.email)
		if err := s.exec("app_user", `
			INSERT INTO app_user (id, email, password_hash, is_active, email_verified_at)
			VALUES ($1, $2, $3, true, now()) ON CONFLICT (email) DO NOTHING`,
			uid, u.email, hash); err != nil {
			return err
		}
		rid, err := s.roleID(u.role)
		if err != nil {
			return err
		}
		if err := s.exec("user_role", `
			INSERT INTO user_role (user_id, role_id) VALUES ($1, $2)
			ON CONFLICT DO NOTHING`, uid, rid); err != nil {
			return err
		}
		var kitchenID any
		if u.kitchen != "" {
			kid, err := s.kitchenID(u.kitchen)
			if err != nil {
				return err
			}
			kitchenID = kid
		}
		if err := s.exec("staff_profile", `
			INSERT INTO staff_profile (id, user_id, kitchen_id, full_name, job_title)
			VALUES ($1, $2, $3, $4, $5) ON CONFLICT (user_id) DO NOTHING`,
			idFor("staffprofile", u.email), uid, kitchenID, u.name, u.title); err != nil {
			return err
		}
	}

	customers := []struct {
		email, name, ctype, org, phone string
		addrLabel, addr, district      string
		lat, lng                       float64
	}{
		{"sinta@example.com", "Sinta Prameswari", "retail", "", "+6281288994410",
			"Rumah", "Jl. Wijaya IX No. 12, Petogogan, Kebayoran Baru, Jakarta Selatan 12170", "Kebayoran Baru",
			-6.2400, 106.7980},
		{"bagas@example.com", "Bagas Nugroho", "retail", "", "+6281377112233",
			"Rumah", "Jl. Tebet Barat Dalam Raya No. 30, Tebet, Jakarta Selatan 12810", "Tebet",
			-6.2300, 106.8450},
		{"dewi@sinarmas.example", "Dewi Anggraini", "corporate", "sinar-mas", "+6281199887766",
			"Kantor", "Menara Sudirman lt. 18, Jl. Jend. Sudirman Kav. 60, Jakarta Selatan 12190", "Setiabudi",
			-6.2260, 106.8080},
	}
	for _, c := range customers {
		uid := idFor("user", c.email)
		cid := idFor("customer", c.email)
		if err := s.exec("app_user", `
			INSERT INTO app_user (id, email, password_hash, phone, is_active, email_verified_at)
			VALUES ($1, $2, $3, $4, true, now()) ON CONFLICT (email) DO NOTHING`,
			uid, c.email, hash, c.phone); err != nil {
			return err
		}
		custRole, err := s.roleID("customer")
		if err != nil {
			return err
		}
		if err := s.exec("user_role", `
			INSERT INTO user_role (user_id, role_id) VALUES ($1, $2)
			ON CONFLICT DO NOTHING`, uid, custRole); err != nil {
			return err
		}
		var orgID any
		if c.org != "" {
			orgID = idFor("org", c.org)
		}
		ctID, err := s.ctypeID(c.ctype)
		if err != nil {
			return err
		}
		if err := s.exec("customer", `
			INSERT INTO customer (id, user_id, customer_type_id, organisation_id, full_name, notify_channels)
			VALUES ($1, $2, $3, $4, $5, 'email') ON CONFLICT (user_id) DO NOTHING`,
			cid, uid, ctID, orgID, c.name); err != nil {
			return err
		}
		if err := s.exec("customer_address", `
			INSERT INTO customer_address (id, customer_id, label, recipient_name, recipient_phone,
			    address_line, district, city, province, postal_code, latitude, longitude, is_default, driver_note)
			VALUES ($1, $2, $3, $4, $5, $6, $7, 'Jakarta', 'DKI Jakarta', '', $8, $9, true, 'titip resepsionis')
			ON CONFLICT (id) DO NOTHING`,
			idFor("addr", c.email), cid, c.addrLabel, c.name, c.phone, c.addr, c.district,
			c.lat, c.lng); err != nil {
			return err
		}
	}
	return nil
}

// mealPlan is one scheduled meal in the seeded week.
type mealPlan struct {
	dayOffset int // days from Monday of the current week
	slot      string
	diet      string
	name      string
	items     []string
	published bool
}

// week mirrors the menu calendar the back-office screen shows: Monday to
// Saturday, lunch and dinner, most PUBLISHED and Thursday still DRAFT.
var week = []mealPlan{
	{0, "11:30:00", "balanced", "Ayam panggang lemon & quinoa",
		[]string{"ayam-panggang-lemon", "quinoa-herba", "brokoli-kukus", "infused-water-timun"}, true},
	{0, "18:00:00", "balanced", "Salmon teriyaki",
		[]string{"salmon-teriyaki", "nasi-merah", "brokoli-kukus", "infused-water-timun"}, true},
	{0, "07:00:00", "balanced", "Overnight oats & buah potong",
		[]string{"overnight-oats", "buah-potong"}, true},
	{0, "11:30:00", "weight-loss", "Tumis ayam paprika",
		[]string{"tumis-ayam-paprika", "brokoli-kukus", "infused-water-timun"}, true},

	{1, "11:30:00", "balanced", "Nasi merah rendang jamur",
		[]string{"rendang-jamur", "nasi-merah", "brokoli-kukus", "infused-water-timun"}, true},
	{1, "18:00:00", "balanced", "Tumis ayam paprika",
		[]string{"tumis-ayam-paprika", "nasi-merah", "brokoli-kukus", "infused-water-timun"}, true},

	{2, "11:30:00", "balanced", "Dori panggang & kentang herba",
		[]string{"dori-panggang", "kentang-herba", "brokoli-kukus", "infused-water-timun"}, true},
	{2, "18:00:00", "balanced", "Sop iga bening & ubi",
		[]string{"sop-iga-bening", "ubi-kukus", "brokoli-kukus", "infused-water-timun"}, false},

	// Thursday is the DRAFT the dashboard flags as "masih DRAFT".
	{3, "11:30:00", "balanced", "Ayam kari hijau & brown rice",
		[]string{"ayam-kari-hijau", "brown-rice", "brokoli-kukus"}, false},

	{5, "11:30:00", "balanced", "Ayam panggang lemon & quinoa",
		[]string{"ayam-panggang-lemon", "quinoa-herba", "brokoli-kukus", "infused-water-timun"}, true},
	{5, "18:00:00", "balanced", "Salmon teriyaki",
		[]string{"salmon-teriyaki", "nasi-merah", "brokoli-kukus", "infused-water-timun"}, true},
}

// mondayOf returns the Monday of the week containing t, in the operating zone.
func mondayOf(t time.Time, loc *time.Location) time.Time {
	d := t.In(loc)
	offset := (int(d.Weekday()) + 6) % 7 // Monday = 0
	return time.Date(d.Year(), d.Month(), d.Day()-offset, 0, 0, 0, 0, loc)
}

func (s *seeder) menu() error {
	monday := mondayOf(s.today, s.loc)
	// Seed this week and the next, so the site always has a forward calendar.
	for _, weekOffset := range []int{0, 7} {
		for _, m := range week {
			date := monday.AddDate(0, 0, m.dayOffset+weekOffset)
			key := fmt.Sprintf("%s:%s:%s", date.Format("2006-01-02"), m.diet, m.slot)
			status, published := "DRAFT", any(nil)
			if m.published {
				status, published = "PUBLISHED", time.Now().UTC()
			}
			did, err := s.dietID(m.diet)
			if err != nil {
				return err
			}
			sid, err := s.slotID(m.slot)
			if err != nil {
				return err
			}
			if err := s.exec("scheduled_meal", `
				INSERT INTO scheduled_meal (id, service_date, diet_type_id, slot_id, name, status, published_at, qty_capacity)
				VALUES ($1, $2::date, $3, $4, $5, $6, $7, 40)
				ON CONFLICT (service_date, diet_type_id, slot_id) DO NOTHING`,
				idFor("meal", key), date.Format("2006-01-02"), did, sid,
				m.name, status, published); err != nil {
				return err
			}
			// Read back the meal that is actually there: a prior run or a
			// fixture may own this (date, diet, slot) under another id.
			mid, err := s.resolve("meal:"+key, `
				SELECT id FROM scheduled_meal
				 WHERE service_date = $1::date AND diet_type_id = $2 AND slot_id = $3`,
				date.Format("2006-01-02"), did, sid)
			if err != nil {
				return err
			}
			for i, f := range m.items {
				role := "SIDE"
				switch {
				case i == 0:
					role = "MAIN"
				case f == "infused-water-timun":
					role = "DRINK"
				case f == "buah-potong":
					role = "DESSERT"
				}
				fid, err := s.foodID(f)
				if err != nil {
					return err
				}
				if err := s.exec("scheduled_meal_item", `
					INSERT INTO scheduled_meal_item (id, scheduled_meal_id, food_id, item_role, sort_order)
					VALUES ($1, $2, $3, $4, $5)
					ON CONFLICT (scheduled_meal_id, food_id) DO NOTHING`,
					idFor("mealitem", key+":"+f), mid, fid, role, i); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (s *seeder) capacity() error {
	monday := mondayOf(s.today, s.loc)
	kitchens := []string{"KTC-01", "KBY-02", "KLG-03"}
	caps := map[string]int{"KTC-01": 40, "KBY-02": 35, "KLG-03": 30}
	slots := []string{"07:00:00", "11:30:00", "12:00:00", "18:00:00"}
	// Reservations that reproduce the dashboard's capacity grid for today.
	reserved := map[string]map[string]int{
		"KTC-01": {"07:00:00": 18, "11:30:00": 40, "12:00:00": 31, "18:00:00": 22},
		"KBY-02": {"07:00:00": 12, "11:30:00": 33, "12:00:00": 28, "18:00:00": 19},
		"KLG-03": {"07:00:00": 6, "11:30:00": 24, "12:00:00": 17},
	}
	for d := 0; d < 14; d++ {
		date := monday.AddDate(0, 0, d)
		isToday := date.Format("2006-01-02") == s.today.In(s.loc).Format("2006-01-02")
		for _, k := range kitchens {
			for _, sl := range slots {
				if k == "KLG-03" && sl == "18:00:00" {
					continue // closed, as the dashboard shows
				}
				res := 0
				if isToday {
					res = reserved[k][sl]
				}
				kid, err := s.kitchenID(k)
				if err != nil {
					return err
				}
				sid, err := s.slotID(sl)
				if err != nil {
					return err
				}
				if err := s.exec("kitchen_capacity", `
					INSERT INTO kitchen_capacity (id, kitchen_id, service_date, slot_id, max_portions, reserved_portions)
					VALUES ($1, $2, $3::date, $4, $5, $6)
					ON CONFLICT (kitchen_id, service_date, slot_id) DO NOTHING`,
					idFor("cap", k+":"+date.Format("2006-01-02")+":"+sl),
					kid, date.Format("2006-01-02"), sid, caps[k], res); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// demoPackage gives Sinta the 20-portion Balanced package with 13 credits
// remaining that her account screen shows, built from real ledger entries so
// the balance is derived rather than asserted.
func (s *seeder) demoPackage() error {
	custID := idFor("customer", "sinta@example.com")
	pkgID := idFor("cpkg", "sinta-balanced-20")
	activated := s.today.In(s.loc).AddDate(0, 0, -8)
	expires := activated.AddDate(0, 0, 90)

	p20, err := s.packageID("paket-20")
	if err != nil {
		return err
	}
	if err := s.exec("customer_package", `
		INSERT INTO customer_package (id, customer_id, package_id, package_number,
		    meal_credits, validity_days, price_paid_idr, status, activated_at, expires_at)
		VALUES ($1, $2, $3, 'PKG-2608-0042', 20, 90, 1420000, 'ACTIVE', $4, $5::date)
		ON CONFLICT (id) DO NOTHING`,
		pkgID, custID, p20, activated, expires.Format("2006-01-02")); err != nil {
		return err
	}

	// The ledger from the account screen: +20, -2, -4, +1, -2 = 13.
	entries := []struct {
		key       string
		entryType string
		qty       int
		days      int
		note      string
	}{
		{"purchase", "PURCHASE", +20, -8, "Pembelian paket"},
		{"redeem-1", "REDEEM", -2, -6, ""},
		{"redeem-2", "REDEEM", -4, -4, ""},
		{"refund-1", "REFUND", +1, -3, ""},
		{"redeem-3", "REDEEM", -2, -1, ""},
	}
	for _, e := range entries {
		if err := s.exec("credit_ledger", `
			INSERT INTO credit_ledger (id, customer_id, customer_package_id, entry_type, qty, occurred_at, note, reference_type)
			VALUES ($1, $2, $3, $4, $5, $6, $7, 'seed')
			ON CONFLICT (id) DO NOTHING`,
			idFor("ledger", "sinta:"+e.key), custID, pkgID, e.entryType, e.qty,
			s.today.In(s.loc).AddDate(0, 0, e.days), e.note); err != nil {
			return err
		}
	}

	// Assert the balance came out at 13. A seed that silently produced a
	// different number would make every screenshot disagree with the design.
	var balance int
	if err := s.tx.QueryRowContext(s.ctx,
		`SELECT COALESCE(SUM(qty), 0) FROM credit_ledger WHERE customer_package_id = $1`,
		pkgID).Scan(&balance); err != nil {
		return err
	}
	if balance != 13 {
		return fmt.Errorf("demo package balance is %d, expected 13 — the ledger entries do not match the design", balance)
	}
	return nil
}

// Verify re-reads the seeded data and checks the facts the UI depends on. It
// runs after Run so the CLI can report something it has actually confirmed
// rather than something it merely attempted.
func Verify(ctx context.Context, db *sql.DB) (map[string]int, error) {
	checks := map[string]string{
		"sys_parameters":   `SELECT count(*) FROM sys_parameters`,
		"permissions":      `SELECT count(*) FROM permission`,
		"roles":            `SELECT count(*) FROM role`,
		"diet_types":       `SELECT count(*) FROM diet_type WHERE is_active`,
		"foods":            `SELECT count(*) FROM food WHERE is_active`,
		"foods_with_panel": `SELECT count(*) FROM food f JOIN food_nutrition n ON n.food_id = f.id`,
		"kitchens":         `SELECT count(*) FROM kitchen WHERE is_active`,
		"slots":            `SELECT count(*) FROM delivery_time_slot WHERE is_active`,
		"price_rows":       `SELECT count(*) FROM meal_price_normal WHERE is_active`,
		"packages":         `SELECT count(*) FROM package WHERE is_active`,
		"published_meals":  `SELECT count(*) FROM scheduled_meal WHERE status = 'PUBLISHED'`,
		"draft_meals":      `SELECT count(*) FROM scheduled_meal WHERE status = 'DRAFT'`,
		"customers":        `SELECT count(*) FROM customer`,
		"staff":            `SELECT count(*) FROM staff_profile`,
		"capacity_rows":    `SELECT count(*) FROM kitchen_capacity`,
		"credit_balance":   `SELECT COALESCE(SUM(qty),0) FROM credit_ledger`,
	}
	out := map[string]int{}
	for name, q := range checks {
		var n int
		if err := db.QueryRowContext(ctx, q).Scan(&n); err != nil {
			return nil, fmt.Errorf("verify %s: %w", name, err)
		}
		out[name] = n
	}
	// Every food must have a panel, or a meal's aggregate silently
	// under-reports (nutrition.Aggregate marks it incomplete, but the seed
	// should not be producing incomplete meals in the first place).
	if out["foods"] != out["foods_with_panel"] {
		return out, fmt.Errorf("%d foods but only %d have a nutrition panel", out["foods"], out["foods_with_panel"])
	}
	return out, nil
}

var _ = id.New // keep the platform id package wired in for future use
