package snare

import (
	"strings"
	"testing"
)

// brands is a small curated target list used across the Nearest tests.
var brands = []string{"paypal", "google", "amazon", "apple", "microsoft", "netflix"}

func TestNearest(t *testing.T) {
	s := New(brands)
	cases := []struct {
		name string
		q    string
		want string
		dist int
		ok   bool
	}{
		{"substitution l->1", "paypa1", "paypal", 1, true},
		{"adjacent transposition", "papyal", "paypal", 1, true},
		{"exact match is not a squat", "paypal", "", 0, false},
		{"combosquat is too far", "paypal-secure", "", 0, false},
		{"insertion, long label k=2", "gooogle", "google", 1, true},
		{"deletion, short label k=1", "appl", "apple", 1, true},
		{"transposition on a long label", "microsfot", "microsoft", 1, true},
		{"nothing close", "xyzzy", "", 0, false},
		{"empty query", "", "", 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, dist, ok := s.Nearest(c.q)
			if got != c.want || dist != c.dist || ok != c.ok {
				t.Fatalf("Nearest(%q) = (%q, %d, %v), want (%q, %d, %v)",
					c.q, got, dist, ok, c.want, c.dist, c.ok)
			}
		})
	}
}

// TestExactMatchWinsOverNearNeighbor guards that an exact hit reports "not a squat"
// even when a different target sits one edit away: paypa1 is a target, and paypal
// is one substitution from it, but querying paypa1 must return no hit.
func TestExactMatchWinsOverNearNeighbor(t *testing.T) {
	s := New([]string{"paypal", "paypa1"})
	if got, dist, ok := s.Nearest("paypa1"); ok {
		t.Fatalf("Nearest(%q) = (%q, %d, %v), want no hit for an exact target", "paypa1", got, dist, ok)
	}
	// A query that is neither target but one edit from both must resolve
	// deterministically to the lexicographically smaller target ("paypa1" < "paypal").
	got, dist, ok := s.Nearest("paypai")
	if !ok || got != "paypa1" || dist != 1 {
		t.Fatalf("Nearest(%q) = (%q, %d, %v), want (paypa1, 1, true)", "paypai", got, dist, ok)
	}
}

// TestLengthPruning checks that a target whose rune length is outside the query's
// [len-k, len+k] band is never returned. Edit distance is at least the length
// difference, so such a target can never be within k; only in-band targets surface.
func TestLengthPruning(t *testing.T) {
	// "abc" is length 3, three shorter than the query — with k=1 it is out of band
	// and must be pruned, leaving no hit at all.
	s := New([]string{"abc"})
	if got, dist, ok := s.Nearest("abcdef"); ok {
		t.Fatalf("Nearest(%q) = (%q, %d, %v), want no hit (out-of-band target pruned)", "abcdef", got, dist, ok)
	}
	// Add an in-band near-miss and confirm it is the one returned, not the far one.
	s = New([]string{"abc", "abcdeZ"})
	got, dist, ok := s.Nearest("abcdef")
	if !ok || got != "abcdeZ" || dist != 1 {
		t.Fatalf("Nearest(%q) = (%q, %d, %v), want (abcdeZ, 1, true)", "abcdef", got, dist, ok)
	}
}

// TestShortLabelPolicy checks the length-relative cap: a short query (<= 6 runes)
// matches only at distance 1, while an otherwise-identical-shaped long query admits
// distance 2.
func TestShortLabelPolicy(t *testing.T) {
	// Short: len(q)=5 -> k=1. "abxye" differs from "abcde" in two positions (dist 2)
	// and must NOT match; "abcxe" differs in one (dist 1) and must.
	short := New([]string{"abxye", "abcxe"})
	if got, dist, ok := short.Nearest("abcde"); !ok || got != "abcxe" || dist != 1 {
		t.Fatalf("short Nearest(%q) = (%q, %d, %v), want (abcxe, 1, true)", "abcde", got, dist, ok)
	}
	if _, _, ok := New([]string{"abxye"}).Nearest("abcde"); ok {
		t.Fatalf("short query at distance 2 must not match under k=1")
	}
	// Long: len(q)=7 -> k=2. A two-substitution target now DOES match.
	long := New([]string{"abxdyfg"}) // differs from "abcdefg" at positions 2 and 4
	if got, dist, ok := long.Nearest("abcdefg"); !ok || got != "abxdyfg" || dist != 2 {
		t.Fatalf("long Nearest(%q) = (%q, %d, %v), want (abxdyfg, 2, true)", "abcdefg", got, dist, ok)
	}
}

// TestNearestPrefersSmallerDistance checks that among in-band candidates a
// distance-1 target beats a distance-2 target for the same long query (k=2, so
// both are eligible and the choice is by distance, not length pruning).
func TestNearestPrefersSmallerDistance(t *testing.T) {
	s := New([]string{"micros0ft", "micr0s0ft"}) // one and two substitutions from "microsoft"
	got, dist, ok := s.Nearest("microsoft")
	if !ok || got != "micros0ft" || dist != 1 {
		t.Fatalf("Nearest(%q) = (%q, %d, %v), want (micros0ft, 1, true)", "microsoft", got, dist, ok)
	}
}

// TestNewDedup checks that duplicate and empty targets collapse in New.
func TestNewDedup(t *testing.T) {
	s := New([]string{"paypal", "paypal", "", "google", "paypal"})
	total := 0
	for _, es := range s.buckets {
		total += len(es)
	}
	if total != 2 {
		t.Fatalf("deduped target count = %d, want 2 (paypal, google)", total)
	}
	// Behaviour is unchanged by the duplicates.
	if got, dist, ok := s.Nearest("paypa1"); !ok || got != "paypal" || dist != 1 {
		t.Fatalf("Nearest(%q) = (%q, %d, %v), want (paypal, 1, true)", "paypa1", got, dist, ok)
	}
}

// TestEmptySet checks that querying an empty Set is a clean miss.
func TestEmptySet(t *testing.T) {
	s := New(nil)
	if _, _, ok := s.Nearest("anything"); ok {
		t.Fatalf("empty Set must return no hit")
	}
}

// TestNearestNoAllocForOutOfBandQuery guards the lazy-allocation contract: a query
// whose length band selects no target — including a pathologically long one — must
// score no candidate and so allocate nothing. The decoded query and the DP scratch
// are built only on the first candidate actually compared; before that was made lazy
// a long query sized a scratch to its own length (O(len) memory) even though no
// target could be within k. The 4096-rune query below is far outside the [len-k,
// len+k] band of every 5..9-rune brand, so both stay unbuilt.
func TestNearestNoAllocForOutOfBandQuery(t *testing.T) {
	s := New(brands) // brand lengths 5..9
	q := strings.Repeat("a", 4096)
	if got, dist, ok := s.Nearest(q); ok {
		t.Fatalf("Nearest(<4096 a's>) = (%q, %d, %v), want no hit (out of band)", got, dist, ok)
	}
	if n := testing.AllocsPerRun(100, func() { s.Nearest(q) }); n != 0 {
		t.Fatalf("out-of-band query allocated %v times, want 0 (scratch must stay lazy)", n)
	}
}
