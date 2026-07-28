package twist

import "testing"

// abs is a tiny helper for the fuzz invariants.
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// FuzzEditDistance drives the bounded OSA distance with arbitrary input — the
// attacker-controlled surface, since a query is untrusted. It must never panic and
// must always report a value that is non-negative, never above the k+1 sentinel,
// symmetric, and — when within the cap — no smaller than the rune-length
// difference (a lower bound) nor larger than the longer operand.
func FuzzEditDistance(f *testing.F) {
	seeds := []struct {
		a, b string
		k    int
	}{
		{"paypal", "paypa1", 1},
		{"papyal", "paypal", 1},
		{"", "", 0},
		{"café", "cafe", 2},
		{"日本語", "日本国", 1},
		{"\x00\xff\xfe", "invalid \xc0 utf8", 3},
		{"😀 emoji", "😀 emojii", 2},
		{"a", "", 5},
	}
	for _, s := range seeds {
		f.Add(s.a, s.b, s.k)
	}
	f.Fuzz(func(t *testing.T, a, b string, k int) {
		// Keep the cap in a sane band; the clamp is exercised by unit tests.
		if k < 0 {
			k = 0
		}
		if k > 32 {
			k = 32
		}
		ar, br := []rune(a), []rune(b)
		got := editDistance(ar, br, k, nil)

		if got < 0 {
			t.Fatalf("negative distance %d for (%q,%q,%d)", got, a, b, k)
		}
		if got > k+1 {
			t.Fatalf("distance %d exceeds sentinel %d for (%q,%q,%d)", got, k+1, a, b, k)
		}
		// Symmetry: OSA is a metric-like symmetric function within the cap.
		if rev := editDistance(br, ar, k, nil); rev != got {
			t.Fatalf("asymmetric: (%q,%q)=%d vs reversed=%d (k=%d)", a, b, got, rev, k)
		}
		// Identity: a string is distance 0 from itself (0 <= k always here).
		if self := editDistance(ar, ar, k, nil); self != 0 {
			t.Fatalf("self-distance %d != 0 for %q (k=%d)", self, a, k)
		}
		// When the result is within the cap it is the true distance, so it must
		// respect the length-difference lower bound and the length upper bound.
		if got <= k {
			if lo := abs(len(ar) - len(br)); got < lo {
				t.Fatalf("distance %d below length-diff lower bound %d for (%q,%q)", got, lo, a, b)
			}
			if hi := max(len(ar), len(br)); got > hi {
				t.Fatalf("distance %d above length upper bound %d for (%q,%q)", got, hi, a, b)
			}
		}
	})
}

// FuzzNearest drives New+Nearest with arbitrary targets and queries: it must never
// panic and must honour its contract — a hit reports a distance in [1,k] to one of
// the actual targets, and an exact query never reports a hit.
func FuzzNearest(f *testing.F) {
	f.Add("paypal", "google", "paypa1")
	f.Add("", "", "")
	f.Add("café", "日本語", "cafe")
	f.Fuzz(func(t *testing.T, x, y, q string) {
		s := New([]string{x, y})
		got, d, ok := s.Nearest(q)
		if !ok {
			return
		}
		k := kFor(len([]rune(q)))
		if d < 1 || d > k {
			t.Fatalf("hit distance %d outside [1,%d] for query %q", d, k, q)
		}
		if got != x && got != y {
			t.Fatalf("hit %q is not one of the targets %q/%q", got, x, y)
		}
		if got == q {
			t.Fatalf("exact match %q reported as a squat", q)
		}
		// A non-empty query that equals a target must be a miss, never a hit.
		if q != "" && (q == x || q == y) {
			t.Fatalf("query %q equals a target but was reported as a hit", q)
		}
		// The reported distance must match a direct computation.
		if dd := editDistance([]rune(q), []rune(got), k, nil); dd != d {
			t.Fatalf("reported distance %d != recomputed %d for %q->%q", d, dd, q, got)
		}
	})
}
