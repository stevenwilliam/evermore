package http

import "testing"

// Every key in the reference catalogue must be translated in every other
// locale. A missing key does not crash — T falls back to English — which is
// precisely why it needs a test: a Bahasa page quietly serving English words
// looks fine to anyone who does not read both.
//
// This is the test that would have caught "month.des", a December key that
// never matched the "month.dec" the lookup asks for.
func TestEveryLocaleIsComplete(t *testing.T) {
	for _, l := range messages.Set.Supported {
		if l == messages.Set.Default {
			continue
		}
		if missing := messages.Missing(l); len(missing) > 0 {
			t.Errorf("locale %q is missing %d key(s): %v", l, len(missing), missing)
		}
	}
}

// And the reverse: a key translated somewhere but absent from the reference
// catalogue is dead weight that no page can ever reach.
func TestNoOrphanKeys(t *testing.T) {
	def := messages.Catalogs[messages.Set.Default]
	for _, l := range messages.Set.Supported {
		if l == messages.Set.Default {
			continue
		}
		for k := range messages.Catalogs[l] {
			if _, ok := def[k]; !ok {
				t.Errorf("locale %q defines %q, which the %q catalogue does not have",
					l, k, messages.Set.Default)
			}
		}
	}
}
