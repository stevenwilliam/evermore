// Package i18n is the message-catalogue mechanism: locales, negotiation and
// lookup. It is business-agnostic and portable — it knows about languages and
// nothing about catering, so it carries to the next project unchanged
// (CLAUDE.md §2).
//
// The catalogue itself lives in the adapter layer, because the STRINGS are
// product content; only the machinery is here.
package i18n

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// Locale is a BCP-47 tag reduced to the part we actually switch on: "en",
// "id". Region subtags are accepted on the way in and folded away, so both
// "en-GB" and "en" resolve to the same catalogue.
type Locale string

// Base folds a tag to its language subtag, lower-cased.
func Base(tag string) Locale {
	tag = strings.TrimSpace(strings.ToLower(tag))
	if i := strings.IndexAny(tag, "-_"); i > 0 {
		tag = tag[:i]
	}
	return Locale(tag)
}

// Set is the locales a site serves, in preference order, with the first as
// the default.
type Set struct {
	Default   Locale
	Supported []Locale
}

// NewSet folds and de-duplicates the configured tags. The first surviving tag
// is the default, so changing the default is a config edit.
func NewSet(tags []string) Set {
	var out []Locale
	seen := map[Locale]bool{}
	for _, t := range tags {
		l := Base(t)
		if l == "" || seen[l] {
			continue
		}
		seen[l] = true
		out = append(out, l)
	}
	if len(out) == 0 {
		out = []Locale{"en"}
	}
	return Set{Default: out[0], Supported: out}
}

// Has reports whether the set serves a locale.
func (s Set) Has(l Locale) bool {
	for _, c := range s.Supported {
		if c == l {
			return true
		}
	}
	return false
}

// Cookie is where an explicit choice is remembered.
const Cookie = "evermore_lang"

// Negotiate picks the locale for a request, most explicit signal first:
//
//  1. ?lang= — the switcher link, so a shared URL carries its language
//  2. the cookie — a choice already made on this device
//  3. Accept-Language — what the browser asks for
//  4. the default
//
// A choice the visitor made outranks what their browser prefers, which is the
// whole reason the cookie sits above Accept-Language.
func (s Set) Negotiate(r *http.Request) Locale {
	if q := Base(r.URL.Query().Get("lang")); q != "" && s.Has(q) {
		return q
	}
	if c, err := r.Cookie(Cookie); err == nil {
		if l := Base(c.Value); s.Has(l) {
			return l
		}
	}
	if l, ok := s.FromAcceptLanguage(r.Header.Get("Accept-Language")); ok {
		return l
	}
	return s.Default
}

type weighted struct {
	loc Locale
	q   float64
}

// FromAcceptLanguage parses the header and returns the best supported match.
//
// Quality values are honoured, and an entry with q=0 is an explicit refusal
// rather than a weak preference, so it is dropped instead of ranked last.
func (s Set) FromAcceptLanguage(header string) (Locale, bool) {
	if strings.TrimSpace(header) == "" {
		return "", false
	}
	var list []weighted
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		tag, q := part, 1.0
		if i := strings.Index(part, ";"); i >= 0 {
			tag = strings.TrimSpace(part[:i])
			for _, p := range strings.Split(part[i+1:], ";") {
				p = strings.TrimSpace(p)
				if strings.HasPrefix(p, "q=") {
					if v, err := strconv.ParseFloat(strings.TrimPrefix(p, "q="), 64); err == nil {
						q = v
					}
				}
			}
		}
		if q <= 0 {
			continue // q=0 means "not this one"
		}
		if tag == "*" {
			continue
		}
		if l := Base(tag); l != "" && s.Has(l) {
			list = append(list, weighted{l, q})
		}
	}
	if len(list) == 0 {
		return "", false
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].q > list[j].q })
	return list[0].loc, true
}

// Catalog holds one locale's strings.
type Catalog map[string]string

// Bundle is every catalogue plus the set that governs fallback.
type Bundle struct {
	Set      Set
	Catalogs map[Locale]Catalog
}

// T looks up a key, falling back to the default locale and then to the key
// itself.
//
// Returning the KEY rather than an empty string is deliberate: a missing
// translation shows up as "menu.cutoff" on the page, which someone notices,
// instead of a blank space, which nobody does.
func (b Bundle) T(l Locale, key string) string {
	if c, ok := b.Catalogs[l]; ok {
		if v, ok := c[key]; ok && v != "" {
			return v
		}
	}
	if c, ok := b.Catalogs[b.Set.Default]; ok {
		if v, ok := c[key]; ok && v != "" {
			return v
		}
	}
	return key
}

// Missing lists keys present in the default catalogue but absent or empty in
// another. It is what a test asserts on, so a half-translated locale fails the
// build rather than shipping English into a Bahasa page.
func (b Bundle) Missing(l Locale) []string {
	def := b.Catalogs[b.Set.Default]
	other := b.Catalogs[l]
	var out []string
	for k, v := range def {
		if v == "" {
			continue
		}
		if other[k] == "" {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}
