package snare

import "testing"

// TestNearestWeightedLengthPruneBoundary is the regression for the audit's confirmed
// length-prune bug. The old prune computed maxLenDiff = int(Budget/min(Ins,Del)); for
// Budget 0.9 and cost 0.3 that is int(0.9/0.3) = int(2.999…) = 2 (float truncation),
// which dropped the length-3 bucket even though an empty query is exactly within
// budget of a 3-rune target (3*0.3 = 0.9 <= 0.9). The fixed prune multiplies by the
// gap with the same arithmetic the scoring uses, so it keeps exactly what scores
// within budget.
func TestNearestWeightedLengthPruneBoundary(t *testing.T) {
	const target = "aaa"
	s := New([]string{target})
	w := Weights{Ins: 0.3, Del: 0.3, Budget: 0.9}
	got, _, ok := s.NearestWeighted("", w)
	if !ok || got != target {
		t.Fatalf("NearestWeighted(%q) = (%q, _, %v); want (%q, found) — length-prune boundary", "", got, ok, target)
	}
}
