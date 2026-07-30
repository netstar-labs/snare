package snare

import "testing"

// dist is a test shorthand for the bounded OSA distance over strings, allocating
// its own scratch.
func dist(a, b string, k int) int {
	return editDistance([]rune(a), []rune(b), k, nil)
}

func TestEditDistance(t *testing.T) {
	cases := []struct {
		a, b string
		k    int
		want int // the true distance; if want > k, editDistance must report the k+1 sentinel
	}{
		{"paypal", "paypa1", 1, 1},        // substitution
		{"paypal", "papyal", 1, 1},        // adjacent transposition
		{"paypal", "paypal", 1, 0},        // identical
		{"", "", 1, 0},                    // both empty
		{"abc", "", 2, 3},                 // deletions only (3 > cap 2 -> sentinel)
		{"", "abc", 3, 3},                 // insertions only
		{"kitten", "sitting", 3, 3},       // classic Levenshtein 3
		{"kitten", "sitting", 2, 3},       // same pair, tighter cap -> over-cap sentinel
		{"café", "cafe", 1, 1},            // one rune substitution (é->e), not two bytes
		{"日本語", "日本国", 1, 1},              // multibyte substitution, distance 1
		{"paypal", "paypal-secure", 2, 7}, // combosquat: far past the cap
		{"ab", "ba", 1, 1},                // single transposition
		{"ca", "abc", 3, 3},               // no cheap transposition path here
	}
	for _, c := range cases {
		got := dist(c.a, c.b, c.k)
		if c.want <= c.k {
			if got != c.want {
				t.Errorf("editDistance(%q,%q,%d) = %d, want %d", c.a, c.b, c.k, got, c.want)
			}
		} else if got != c.k+1 {
			t.Errorf("editDistance(%q,%q,%d) = %d, want over-cap sentinel %d", c.a, c.b, c.k, got, c.k+1)
		}
	}
}

// TestEditDistanceSymmetric checks that OSA is symmetric within the cap.
func TestEditDistanceSymmetric(t *testing.T) {
	pairs := [][2]string{
		{"paypal", "papyal"},
		{"kitten", "sitting"},
		{"café", "coffee"},
		{"abc", "cab"},
	}
	for _, p := range pairs {
		k := 4
		if ab, ba := dist(p[0], p[1], k), dist(p[1], p[0], k); ab != ba {
			t.Errorf("asymmetric: editDistance(%q,%q)=%d vs (%q,%q)=%d", p[0], p[1], ab, p[1], p[0], ba)
		}
	}
}

// TestEditDistanceScratchReuse checks that reusing one scratch buffer across
// candidates of different lengths yields the same results as fresh allocation —
// the reuse path Nearest depends on for its single-allocation guarantee.
func TestEditDistanceScratchReuse(t *testing.T) {
	buf := make([]int, 3*(12+1)) // sized for the longest b below
	targets := []string{"a", "abc", "abcdef", "abcdefghij", "paypal", "papyal"}
	q := []rune("paypal")
	for _, tg := range targets {
		reused := editDistance(q, []rune(tg), 3, buf)
		fresh := editDistance(q, []rune(tg), 3, nil)
		if reused != fresh {
			t.Errorf("scratch reuse for %q: got %d, fresh %d", tg, reused, fresh)
		}
	}
}
