package snare

import "unicode/utf8"

// Set is an immutable collection of target strings prepared for near-match
// queries. New deduplicates the targets and buckets them by rune length so a query
// need only be compared against the few targets whose length is within the edit
// cap of the query. A Set is read-only after New and safe for concurrent
// [Set.Nearest] calls.
type Set struct {
	// buckets maps a rune length to the deduped targets of exactly that length.
	buckets map[int][]entry
}

// entry caches a target string alongside its rune decomposition so the distance
// function need not re-decode UTF-8 on every query.
type entry struct {
	s     string
	runes []rune
}

// shortLabel is the rune-length boundary of the length-relative edit cap: a query
// no longer than this admits a single edit, a longer one admits two. Short labels
// (brands, host labels) are dense — one edit already reaches many strings — so a
// tighter budget keeps precision; longer strings can afford two edits before the
// match stops being a plausible typo.
const shortLabel = 6

// New builds a Set from targets. Empty strings and exact duplicates are dropped
// (first occurrence wins), and the survivors are bucketed by rune length. The
// input slice is neither retained nor modified. New runs in time linear in the
// total length of the targets.
func New(targets []string) *Set {
	buckets := make(map[int][]entry)
	seen := make(map[string]struct{}, len(targets))
	for _, t := range targets {
		if t == "" {
			continue
		}
		if _, dup := seen[t]; dup {
			continue
		}
		seen[t] = struct{}{}
		r := []rune(t)
		buckets[len(r)] = append(buckets[len(r)], entry{s: t, runes: r})
	}
	return &Set{buckets: buckets}
}

// kFor returns the edit cap for a query of the given rune length: 1 for short
// labels (<= [shortLabel] runes), 2 above. The cap is length-relative because a
// fixed budget is too loose for short strings and too tight for long ones.
func kFor(qlen int) int {
	if qlen <= shortLabel {
		return 1
	}
	return 2
}

// Nearest returns the target closest to q as a genuine near-miss: the returned
// distance satisfies 1 <= dist <= k, where k is the length-relative cap
// ([kFor]: 1 for queries of <= 6 runes, else 2).
//
// It returns ("", 0, false) when q exactly equals some target — an exact match is
// the target itself, not a squat — or when no target lies within k edits. (Only an
// exact match is excluded, so a very short query can be a near-miss of a very short
// target — an empty query is one insertion from a 1-rune target.) When
// several targets are within k, the one at the smallest distance wins; ties are
// broken by lexicographically smallest target, so the result is deterministic
// regardless of target order. Distance is bounded Damerau-Levenshtein (OSA) over runes
// (see [editDistance]). Nearest is safe for concurrent use.
func (s *Set) Nearest(q string) (target string, dist int, ok bool) {
	qlen := utf8.RuneCountInString(q) // rune count without allocating the []rune yet
	k := kFor(qlen)

	// qr (the decoded query) and scratch are built lazily, on the first candidate
	// actually scored. A query whose length band selects no target — including a
	// pathologically long one — then does zero work and zero allocation, instead of
	// sizing a scratch to the query length up front. Once built, scratch is sized for
	// the widest in-band target (qlen+k runes) and reused across every candidate, so a
	// working query allocates a small constant — the decoded query plus one scratch
	// buffer — no matter how many targets it is scored against. The Set is immutable,
	// so Nearest is safe for concurrent use.
	var qr []rune
	var scratch []int

	bestDist := k + 1
	bestTarget := ""
	// Length-bucket pruning: edit distance is at least the length difference, so
	// only targets whose rune length is within k of the query can ever match. Scan
	// exactly those buckets, in ascending length for determinism.
	for dl := max(0, qlen-k); dl <= qlen+k; dl++ {
		for _, e := range s.buckets[dl] {
			// An exact match means q IS this target, not a look-alike. It outranks any
			// near-miss found in range, so bail out with "not a squat" immediately. The
			// query-length bucket is always within the band, so this is always reached.
			if e.s == q {
				return "", 0, false
			}
			if scratch == nil {
				qr = []rune(q)
				scratch = make([]int, scratchLen(qlen+k))
			}
			d := editDistance(qr, e.runes, k, scratch)
			if d < 1 || d > k {
				continue
			}
			if d < bestDist || (d == bestDist && e.s < bestTarget) {
				bestDist = d
				bestTarget = e.s
			}
		}
	}
	if bestTarget == "" {
		return "", 0, false
	}
	return bestTarget, bestDist, true
}
