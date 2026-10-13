package http

import "github.com/stevenwilliam/evermore/internal/platform/i18n"

// The message catalogue (D47).
//
// No literal copy in a template: every visible string is a key, which is what
// makes a third language a data edit rather than a rewrite (11-i18n.md §1).
//
// English is the DEFAULT and therefore the reference catalogue — `Missing`
// compares every other locale against it, and a key absent here cannot be
// translated anywhere.
//
// Keys are dotted and grouped by surface, not by page, so a string used in two
// places is defined once: `nav.menu` appears in the masthead, the float nav
// and the mobile nav.

var messages = i18n.Bundle{
	Set: i18n.NewSet([]string{"en", "id"}),
	Catalogs: map[i18n.Locale]i18n.Catalog{
		"en": {
			// --- chrome
			"skip":            "Skip to main content",
			"nav.aria":        "Main navigation",
			"nav.open":        "Open navigation menu",
			"nav.menu":        "This week's menu",
			"nav.how":         "How it works",
			"nav.packages":    "Packages",
			"nav.corporate":   "For offices",
			"nav.coverage":    "Delivery area",
			"nav.order":       "Order online",
			"nav.addresses":   "My addresses",
			"nav.allergies":   "My allergies",
			"nav.orders":      "My orders",
			"nav.signin":      "Sign in",
			"nav.lang":        "Bahasa Indonesia",
			"nav.lang.aria":   "Switch language to Bahasa Indonesia",
			"nav.lang.flag":   "\U0001F1EE\U0001F1E9",

			// --- footer
			"footer.explore":   "Explore",
			"footer.contact":   "Contact",
			"footer.privacy":   "Privacy policy",
			"footer.terms":     "Terms & conditions",

			// --- home
			"home.eyebrow":     "Delivered across Jakarta · since 2024",
			"home.title.a":     "Healthy can be",
			"home.title.b":     "fun.",
			"home.lede":        "A new menu every week, built by nutritionists, cooked the same morning, and delivered to your desk or your door at the hour you pick.",
			"home.cta.menu":    "See this week's menu",
			"home.cta.area":    "Check your area",
			"home.stat.diets":  "Diet types",
			"home.stat.kitchens": "Kitchens",
			"home.stat.days":   "Delivery a week",
			"home.stat.days.unit": "days",
			"home.stat.cutoff": "Cut-off, day before",
			"home.menu.title":  "Menu for",
			"home.menu.sub":    "Published every Friday for the week after",
			"home.menu.all":    "See all",

			// --- the hero box
			"box.aria":        "Evermore catering box. Drag to turn it, tap to open.",
			"box.label":       "catering",
			"box.drag":        "Drag",
			"box.drag.hint":   "to turn",
			"box.click":       "Click",
			"box.open":        "to open",
			"box.close":       "to close",

			// --- menu
			"menu.title":      "This week's menu",
			"menu.none":       "No menu published yet",
			"menu.none.date":  "No menu published for this date.",
			"menu.search":     "Search a dish or ingredient…",
			"menu.all":        "All",
			"menu.cutoff":     "Cut-off",

			// --- packages
			"pkg.title":       "Pay once, eat for 90 days",
			"pkg.sub":         "Credits work on any published menu.",
			"pkg.portions":    "portions",
			"pkg.per":         "per portion",
			"pkg.valid":       "valid %s days",
			"pkg.buy":         "Buy package",
			"pkg.note":        "Credits are non-refundable and expire at the end of the validity period. A kitchen cancellation always returns credit, never cash.",

			// --- generic
			"common.back":     "Back to home",
			"err.404.title":   "Page not found",
			"err.404.body":    "The page you're looking for doesn't exist or has moved.",
			"err.ref":         "Reference code",

			// --- the eight-day strip
			"week.title":      "What's cooking this week",
			"week.sub":        "Today and the next seven days. Order before %s the day before.",
			"week.today":      "Today",
			"week.empty":      "Menu not published yet",
			"week.view":       "See the full menu",
			"day.mon": "Monday", "day.tue": "Tuesday", "day.wed": "Wednesday",
			"day.thu": "Thursday", "day.fri": "Friday", "day.sat": "Saturday", "day.sun": "Sunday",
			"month.jan": "Jan", "month.feb": "Feb", "month.mar": "Mar", "month.apr": "Apr",
			"month.may": "May", "month.jun": "Jun", "month.jul": "Jul", "month.aug": "Aug",
			"month.sep": "Sep", "month.oct": "Oct", "month.nov": "Nov", "month.dec": "Dec",
		},
		"id": {
			"skip":            "Lewati ke konten utama",
			"nav.aria":        "Navigasi utama",
			"nav.open":        "Buka menu navigasi",
			"nav.menu":        "Menu minggu ini",
			"nav.how":         "Cara kerja",
			"nav.packages":    "Paket",
			"nav.corporate":   "Untuk kantor",
			"nav.coverage":    "Wilayah antar",
			"nav.order":       "Pesan online",
			"nav.addresses":   "Alamat saya",
			"nav.allergies":   "Alergi saya",
			"nav.orders":      "Pesanan saya",
			"nav.signin":      "Masuk",
			"nav.lang":        "English",
			"nav.lang.aria":   "Ganti bahasa ke English",
			"nav.lang.flag":   "\U0001F1EC\U0001F1E7",

			"footer.explore":   "Jelajahi",
			"footer.contact":   "Hubungi",
			"footer.privacy":   "Kebijakan privasi",
			"footer.terms":     "Syarat & ketentuan",

			"home.eyebrow":     "Diantar di Jakarta · sejak 2024",
			"home.title.a":     "Sehat itu bisa",
			"home.title.b":     "seru.",
			"home.lede":        "Menu baru tiap minggu, disusun ahli gizi, dimasak pagi hari, dan diantar ke meja kerja atau rumah pada jam yang kamu pilih.",
			"home.cta.menu":    "Lihat menu minggu ini",
			"home.cta.area":    "Cek wilayah antar",
			"home.stat.diets":  "Tipe diet",
			"home.stat.kitchens": "Dapur",
			"home.stat.days":   "Antar seminggu",
			"home.stat.days.unit": "hari",
			"home.stat.cutoff": "Cut-off H-1",
			"home.menu.title":  "Menu",
			"home.menu.sub":    "Terbit tiap Jumat untuk minggu berikutnya",
			"home.menu.all":    "Lihat semua",

			"box.aria":        "Kotak catering Evermore. Putar dengan menyeret, ketuk untuk membuka.",
			"box.label":       "catering",
			"box.drag":        "Seret",
			"box.drag.hint":   "untuk memutar",
			"box.click":       "Klik",
			"box.open":        "untuk membuka",
			"box.close":       "untuk menutup",

			"menu.title":      "Menu minggu ini",
			"menu.none":       "Belum ada menu yang terbit",
			"menu.none.date":  "Belum ada menu yang terbit untuk tanggal ini.",
			"menu.search":     "Cari menu atau bahan…",
			"menu.all":        "Semua",
			"menu.cutoff":     "Cut-off",

			"pkg.title":       "Bayar sekali, pakai 90 hari",
			"pkg.sub":         "Kredit dipakai pada menu apa pun yang sudah terbit.",
			"pkg.portions":    "porsi",
			"pkg.per":         "per porsi",
			"pkg.valid":       "berlaku %s hari",
			"pkg.buy":         "Beli paket",
			"pkg.note":        "Kredit tidak bisa diuangkan dan hangus setelah masa berlaku. Pengembalian karena dapur batal selalu masuk sebagai kredit, bukan tunai.",

			"common.back":     "Kembali ke beranda",
			"err.404.title":   "Halaman tidak ditemukan",
			"err.404.body":    "Halaman yang kamu cari tidak ada atau sudah dipindahkan.",
			"err.ref":         "Kode rujukan",

			"week.title":      "Menu minggu ini",
			"week.sub":        "Hari ini dan tujuh hari ke depan. Pesan sebelum %s sehari sebelumnya.",
			"week.today":      "Hari ini",
			"week.empty":      "Menu belum terbit",
			"week.view":       "Lihat menu lengkap",
			"day.mon": "Senin", "day.tue": "Selasa", "day.wed": "Rabu",
			"day.thu": "Kamis", "day.fri": "Jumat", "day.sat": "Sabtu", "day.sun": "Minggu",
			"month.jan": "Jan", "month.feb": "Feb", "month.mar": "Mar", "month.apr": "Apr",
			"month.may": "Mei", "month.jun": "Jun", "month.jul": "Jul", "month.aug": "Agu",
			"month.sep": "Sep", "month.oct": "Okt", "month.nov": "Nov", "month.dec": "Des",
		},
	},
}
