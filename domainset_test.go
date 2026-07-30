package snare

import "testing"

func TestDomainSetCheck(t *testing.T) {
	// example.co.uk exercises the multi-part-suffix split; google.com/google.org share
	// a label so the reported brand must resolve deterministically.
	d := NewDomainSet([]string{"paypal.com", "google.com", "google.org", "example.co.uk"})
	cases := []struct {
		name   string
		domain string
		kind   Kind
		brand  string
		dist   int
		ok     bool
	}{
		{"tld swap same label other tld", "google.net", KindTLDSwap, "google.com", 0, true},
		{"tld swap picks smallest shared brand", "google.io", KindTLDSwap, "google.com", 0, true},
		{"tld swap under a multi-part suffix brand", "example.com", KindTLDSwap, "example.co.uk", 0, true},
		{"legit domain is not a squat", "google.com", KindTypo, "", 0, false},
		{"legit second brand for shared label", "google.org", KindTypo, "", 0, false},
		{"legit multi-part suffix domain", "example.co.uk", KindTypo, "", 0, false},
		{"typo of a brand label", "paypa1.com", KindTypo, "paypal.com", 1, true},
		{"typo ignores the tld", "gogle.info", KindTypo, "google.com", 1, true},
		{"subdomain resolves to registrable label", "login.google.net", KindTLDSwap, "google.com", 0, true},
		{"nothing close", "totally-different.com", KindTypo, "", 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := d.Check(c.domain)
			if ok != c.ok {
				t.Fatalf("Check(%q) ok = %v, want %v (result %+v)", c.domain, ok, c.ok, got)
			}
			if !ok {
				return
			}
			if got.Kind != c.kind || got.Brand != c.brand || got.Dist != c.dist {
				t.Fatalf("Check(%q) = %+v, want {Kind:%v Brand:%q Dist:%d}",
					c.domain, got, c.kind, c.brand, c.dist)
			}
		})
	}
}

// TestSplitDomain checks the registrable label / TLD split, including the embedded
// multi-part suffixes, subdomain stripping, case folding, and the no-TLD fallback.
func TestSplitDomain(t *testing.T) {
	cases := []struct{ in, label, tld string }{
		{"google.com", "google", "com"},
		{"google.co.uk", "google", "co.uk"},
		{"GOOGLE.COM", "google", "com"},
		{"login.paypal.com", "paypal", "com"},
		{"example.com.au", "example", "com.au"},
		{"root-dot.com.", "root-dot", "com"},
		{"single", "single", ""},
		{"", "", ""},
	}
	for _, c := range cases {
		if label, tld := splitDomain(c.in); label != c.label || tld != c.tld {
			t.Errorf("splitDomain(%q) = (%q, %q), want (%q, %q)", c.in, label, tld, c.label, c.tld)
		}
	}
}

// TestDomainSetEmpty checks that a DomainSet over no brands is a clean miss.
func TestDomainSetEmpty(t *testing.T) {
	d := NewDomainSet(nil)
	if _, ok := d.Check("anything.com"); ok {
		t.Fatalf("empty DomainSet must return no hit")
	}
}

// TestDomainSetEdges covers the guard paths: an empty/malformed brand is skipped, an
// empty query misses, and an exact brand label carried with no TLD is not a swap.
func TestDomainSetEdges(t *testing.T) {
	d := NewDomainSet([]string{"google.com", ""}) // "" is skipped in construction
	if _, ok := d.Check(""); ok {
		t.Fatalf("empty domain must miss")
	}
	if _, ok := d.Check("google"); ok {
		t.Fatalf("bare brand label with no TLD must miss (nothing to swap)")
	}
	if r, ok := d.Check("google.net"); !ok || r.Kind != KindTLDSwap {
		t.Fatalf("google.net should still be a tld-swap, got %+v ok=%v", r, ok)
	}
}

// TestKindString guards the Kind labels used in logs and test output.
func TestKindString(t *testing.T) {
	if KindTypo.String() != "typo" || KindTLDSwap.String() != "tld-swap" || Kind(99).String() != "unknown" {
		t.Fatalf("Kind.String mismatch: %q %q %q", KindTypo, KindTLDSwap, Kind(99))
	}
}
