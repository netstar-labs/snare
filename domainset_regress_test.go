package twist

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
