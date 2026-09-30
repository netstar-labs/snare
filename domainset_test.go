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
// multi-part suffixes, subdomain stripping, case folding, the no-TLD fallback, and the
// ambiguous flag: false whenever the split is trustworthy (2-label, or matched a listed
// multiSuffix entry — including a suffix-second-level WORD as the label via a listed
// suffix, e.g. "co.co.uk"), true only for the risky 3+-label unlisted-suffix fallback.
func TestSplitDomain(t *testing.T) {
	cases := []struct {
		in         string
		label, tld string
		ambiguous  bool
	}{
		{"google.com", "google", "com", false},
		{"google.co.uk", "google", "co.uk", false},
		{"GOOGLE.COM", "google", "com", false},
		{"login.paypal.com", "paypal", "com", true}, // 3-label fallback; label isn't a suffix word, so not dropped
		{"example.com.au", "example", "com.au", false},
		{"root-dot.com.", "root-dot", "com", false},
		{"single", "single", "", false},
		{"", "", "", false},
		// A listed suffix whose second level is itself a suffixSecondLevel word: the
		// split is via the trusted multiSuffix branch, so it is NOT ambiguous even
		// though the label reads "co" — this is the audit regression (see
		// TestDomainSetListedSuffixSecondLevelLabelNotDropped).
		{"co.co.uk", "co", "co.uk", false},
		{"id.id.au", "id", "id.au", false},
		// An unlisted 3-label suffix takes the risky fallback and IS ambiguous.
		{"example.co.kr", "co", "kr", true},
	}
	for _, c := range cases {
		label, tld, ambiguous := splitDomain(c.in)
		if label != c.label || tld != c.tld || ambiguous != c.ambiguous {
			t.Errorf("splitDomain(%q) = (%q, %q, %v), want (%q, %q, %v)",
				c.in, label, tld, ambiguous, c.label, c.tld, c.ambiguous)
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
