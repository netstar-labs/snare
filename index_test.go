package snare

import (
	"math/rand"
	"testing"
)

// TestIndexParity is the load-bearing test for the BK-tree: for a large, varied query
// stream the index must return exactly what the brute-force Set.Nearest returns — same
// target, same distance, same ok. The queries mix exact targets (must report not-a-squat
// in both), single-edit mutations of targets (the near-miss regime), and random strings
// (mostly misses), so the parity is exercised across hits, exact matches, and misses.
func TestIndexParity(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	targets := genTargets(500) // deterministic labels of length 4..14, plus "paypal"
	s := New(targets)
	idx := s.Index()

	var queries []string
	queries = append(queries, "", "paypal") // empty and an exact target
	for i := 0; i < len(targets); i += 5 {
		queries = append(queries, targets[i]) // more exact targets
	}
	for i := 0; i < len(targets); i += 2 {
		for m := 0; m < 3; m++ {
			queries = append(queries, mutate(rng, targets[i])) // near-misses
		}
	}
	for n := 0; n < 800; n++ {
		queries = append(queries, randLabel(rng, 3+rng.Intn(12))) // mostly misses
	}

	for _, q := range queries {
		wt, wd, wok := s.Nearest(q)
		it, id, iok := idx.Nearest(q)
		if wt != it || wd != id || wok != iok {
			t.Fatalf("parity mismatch for %q: brute=(%q,%d,%v) index=(%q,%d,%v)",
				q, wt, wd, wok, it, id, iok)
		}
	}
}

// TestIndexBrands is a compact, explicit parity anchor over the curated brand list and
// the same query shapes the core's TestNearest covers.
func TestIndexBrands(t *testing.T) {
	s := New(brands)
	idx := s.Index()
	for _, q := range []string{"paypa1", "papyal", "paypal", "gooogle", "appl", "microsfot", "xyzzy", ""} {
		wt, wd, wok := s.Nearest(q)
		it, id, iok := idx.Nearest(q)
		if wt != it || wd != id || wok != iok {
			t.Fatalf("brands parity mismatch for %q: brute=(%q,%d,%v) index=(%q,%d,%v)",
				q, wt, wd, wok, it, id, iok)
		}
	}
}

// TestIndexEmpty checks that an index over an empty Set is a clean miss, matching Nearest.
func TestIndexEmpty(t *testing.T) {
	idx := New(nil).Index()
	if _, _, ok := idx.Nearest("anything"); ok {
		t.Fatalf("empty index must return no hit")
	}
}

// TestBKNodeDuplicateInsert covers the defensive identical-target guard in insert: a
// Set dedups, so this cannot arise through the public API, but insert must still no-op.
func TestBKNodeDuplicateInsert(t *testing.T) {
	e := entry{s: "paypal", runes: []rune("paypal")}
	n := &bkNode{e: e}
	n.insert(e)
	if len(n.children) != 0 {
		t.Fatalf("duplicate insert created %d children, want 0", len(n.children))
	}
}

// TestLevenshtein drives the tree's metric directly, covering the empty-operand guards
// (never reached through Index, which sees no empty target) and the scratch-growth path.
// papyal->paypal is 2 here — a transposition costs two Levenshtein edits but one OSA
// edit, the OSA<=Lev<=2*OSA relation the 2k search radius rests on.
func TestLevenshtein(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"abc", "", 3},
		{"", "abc", 3},
		{"kitten", "sitting", 3},
		{"paypal", "paypa1", 1},
		{"papyal", "paypal", 2},
		{"café", "cafe", 1},
	}
	for _, c := range cases {
		if got := levenshtein([]rune(c.a), []rune(c.b), nil); got != c.want {
			t.Errorf("levenshtein(%q,%q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
	short := make([]int, 1) // too small -> internal realloc
	if got := levenshtein([]rune("abc"), []rune("abcdef"), short); got != 3 {
		t.Errorf("levenshtein with short buf = %d, want 3", got)
	}
}

// mutate applies one random single edit — substitution, insertion, deletion, or adjacent
// transposition — to s, producing a near-miss for the parity stream.
func mutate(rng *rand.Rand, s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return "x"
	}
	pos := rng.Intn(len(r))
	switch rng.Intn(4) {
	case 0:
		r[pos] = 'a' + rune(rng.Intn(26))
	case 1:
		r = append(r[:pos:pos], append([]rune{'a' + rune(rng.Intn(26))}, r[pos:]...)...)
	case 2:
		r = append(r[:pos:pos], r[pos+1:]...)
	case 3:
		if pos+1 < len(r) {
			r[pos], r[pos+1] = r[pos+1], r[pos]
		}
	}
	return string(r)
}

// randLabel builds a random lowercase label of length n.
func randLabel(rng *rand.Rand, n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = 'a' + rune(rng.Intn(26))
	}
	return string(b)
}
