package snare

import "testing"

// TestDomainSetSuspectMisSplitNoFalsePositive is the audit regression for the
// bidirectional eTLD-split gap. A brand under an unlisted multi-part suffix (co.kr — not
// in multiSuffix) mis-splits to the bogus label "co", which collides with any other
// unlisted co.* suffix (co.ke); registering it would flag the unrelated victim.co.ke as
// a tld-swap. NewDomainSet drops such brands (suspectMisSplit), so no false positive.
func TestDomainSetSuspectMisSplitNoFalsePositive(t *testing.T) {
	d := NewDomainSet([]string{"example.co.kr"})
	if r, ok := d.Check("victim.co.ke"); ok {
		t.Fatalf("Check(victim.co.ke) = (%+v, true); want no hit — co.kr brand must be dropped as unsafely-splittable", r)
	}
	// A brand under a LISTED multi-part suffix still detects a real tld-swap.
	d2 := NewDomainSet([]string{"google.co.uk"})
	if r, ok := d2.Check("google.net"); !ok || r.Kind != KindTLDSwap {
		t.Fatalf("Check(google.net) against google.co.uk = (%+v, %v); want a tld-swap hit", r, ok)
	}
}

// TestDomainSetListedSuffixSecondLevelLabelNotDropped is the audit regression for the
// false-positive direction of suspectMisSplit: a brand split via a LISTED multiSuffix
// entry whose label happens to equal a suffixSecondLevel word (co.co.uk splits to label
// "co" via the trusted co.uk entry) must be kept and fully protected, not dropped as if
// it had taken the risky unlisted-suffix fallback.
func TestDomainSetListedSuffixSecondLevelLabelNotDropped(t *testing.T) {
	d := NewDomainSet([]string{"co.co.uk"})
	if r, ok := d.Check("co.co.uk"); ok {
		t.Fatalf("Check(co.co.uk) against its own brand = (%+v, true); want no hit — an exact brand domain is not a squat", r)
	}
	if r, ok := d.Check("co.net"); !ok || r.Kind != KindTLDSwap {
		t.Fatalf("Check(co.net) against co.co.uk = (%+v, %v); want a tld-swap hit — the brand must not have been silently dropped", r, ok)
	}
}

// TestDomainSetResultBrandPreservesCase is the audit regression for Result.Brand's
// doc comment, which promises the brand is reported "exactly as passed to NewDomainSet."
func TestDomainSetResultBrandPreservesCase(t *testing.T) {
	d := NewDomainSet([]string{"PayPal.COM"})
	r, ok := d.Check("paypal.net")
	if !ok || r.Brand != "PayPal.COM" {
		t.Fatalf("Check(paypal.net) = (%+v, %v); want Brand %q preserved exactly as passed", r, ok, "PayPal.COM")
	}
	// The case-insensitive tie-break among several brands sharing a label must also
	// preserve the winning candidate's original case, not a lower-cased copy of it.
	d2 := NewDomainSet([]string{"Example.COM", "example.NET", "EXAMPLE.org"})
	r2, ok2 := d2.Check("exampel.io")
	if !ok2 || r2.Brand != "Example.COM" {
		t.Fatalf("Check(exampel.io) = (%+v, %v); want Brand %q (case-insensitively smallest, case preserved)", r2, ok2, "Example.COM")
	}
}
