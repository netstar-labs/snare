package twist

import (
	"math"
	"unicode/utf8"
)

// Weights parameterizes [Set.NearestWeighted] with per-operation costs, turning the
// uniform-cost core into a confusability-aware one: a look-alike substitution can be
// made cheap and a random one full price, so a budget of "one edit's worth" admits
// p4yp4l → paypal (two homoglyph swaps) while still rejecting two arbitrary swaps.
//
// Sub returns the cost of substituting one rune for a different one and is the whole
// point of the type — the caller wires it. It is typically built from unmask's
// confusable classes plus twister's leet and keyboard tables: a homoglyph, leet, or
// keyboard-adjacent swap returns a small fraction (say 0.3–0.5) and any other swap
// returns 1.0. twist does not import those packages; it only calls Sub. Sub is invoked
// only for differing runes (an identical rune costs 0), must be non-negative, and may
// be nil — a nil Sub defaults to uniform cost 1.0, making NearestWeighted a float
// mirror of [Set.Nearest].
//
// Ins, Del, and Transpose are the flat costs of an insertion, a deletion, and an
// adjacent transposition; all must be non-negative. Budget is the inclusive maximum
// total cost for a match — the weighted analogue of the integer cap k.
type Weights struct {
	Sub                 func(a, b rune) float64
	Ins, Del, Transpose float64
	Budget              float64
}

// NearestWeighted is the confusability-weighted counterpart of [Set.Nearest]: it
// returns the target closest to q under w, where "closest" is the weighted OSA distance
// (substitutions priced by w.Sub, the other operations by w's flat costs) bounded by
// w.Budget.
//
// It returns ("", 0, false) when q exactly equals a target — an exact match is the
// target, not a squat — or when no target lies within w.Budget. Unlike Nearest, which
// requires distance >= 1, a hit here only requires the target to be a different string:
// a confusable substitution priced at 0 makes a distinct look-alike a legitimate
// zero-distance hit. Among targets within budget the smallest distance wins; ties break
// on the lexicographically smallest target, so the result is deterministic regardless of
// target order. NearestWeighted is safe for concurrent use.
func (s *Set) NearestWeighted(q string, w Weights) (target string, dist float64, ok bool) {
	qlen := utf8.RuneCountInString(q)

	// Exact-match short circuit: q IS a target, not a squat. An exact match can only
	// live in the query-length bucket, so this one scan settles it — and it lets the
	// scoring loop below treat every survivor as a genuine (possibly zero-cost) squat.
	for _, e := range s.buckets[qlen] {
		if e.s == q {
			return "", 0, false
		}
	}

	sub := w.Sub
	if sub == nil {
		sub = uniformSub
	}

	// Length-bucket prune. Bridging a rune-length gap of g needs at least g
	// length-changing ops, each costing at least min(Ins, Del), so a bucket at gap g is
	// out of reach once g*min(Ins,Del) > Budget. The test multiplies by g rather than
	// comparing against int(Budget/min): that truncating division drops a bucket that
	// fits exactly (0.29/0.01 is 28.999… → 28, wrongly excluding the g=29 target whose
	// cost 0.29 <= 0.29), and a tiny min would blow an integer band up to millions of
	// empty lengths. min <= 0 means an insertion or deletion can be free — no length
	// bound at all, every bucket is in range.
	minMove := min(w.Ins, w.Del)
	inBand := func(dl int) bool {
		if minMove <= 0 {
			return true
		}
		g := dl - qlen
		if g < 0 {
			g = -g
		}
		return float64(g)*minMove <= w.Budget
	}

	// Size the reused scratch for the widest in-band target (one cheap pass over the
	// bucket lengths). A query whose band selects no target finds maxLen 0 and returns
	// before decoding q or allocating, so even a pathologically long out-of-band query
	// does no work — the same anti-amplification guard Nearest has.
	maxLen := 0
	for dl, es := range s.buckets {
		if len(es) > 0 && inBand(dl) && dl > maxLen {
			maxLen = dl
		}
	}
	if maxLen == 0 {
		return "", 0, false
	}

	qr := []rune(q)
	scratch := make([]float64, scratchLen(maxLen))
	bestDist := math.Inf(1)
	bestTarget := ""
	for dl, es := range s.buckets {
		if !inBand(dl) {
			continue
		}
		for _, e := range es {
			d := editDistanceWeighted(qr, e.runes, w, sub, scratch)
			if d > w.Budget {
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

// uniformSub is the default substitution cost when Weights.Sub is nil: every swap of
// differing runes costs a flat 1.0.
func uniformSub(a, b rune) float64 { return 1 }

// editDistanceWeighted computes the weighted Optimal String Alignment distance between
// rune slices a and b under w, with substitutions priced by sub. It mirrors
// [editDistance] — the same three rolling rows carved from one buffer, the same
// transposition step — but with float costs and a budget-based abort. The result is the
// true weighted distance when it is <= w.Budget; otherwise a value greater than Budget
// (possibly +Inf from the abort), which the caller discards.
//
// buf is optional scratch of at least 3*(len(b)+1) floats; a nil or short buf is
// allocated internally. Substitution and flat costs are assumed non-negative.
func editDistanceWeighted(a, b []rune, w Weights, sub func(a, b rune) float64, buf []float64) float64 {
	la, lb := len(a), len(b)

	// Empty operand: the distance is the other's length priced at the flat ins/del cost.
	if la == 0 {
		return float64(lb) * w.Ins
	}
	if lb == 0 {
		return float64(la) * w.Del
	}

	rl := lb + 1
	if cap(buf) < scratchLen(lb) {
		buf = make([]float64, scratchLen(lb))
	}
	prev2 := buf[0:rl]
	prev1 := buf[rl : 2*rl]
	cur := buf[2*rl : 3*rl]

	// Row 0: building b[:j] from the empty prefix of a costs j insertions.
	for j := 0; j <= lb; j++ {
		prev1[j] = float64(j) * w.Ins
	}

	// Two-row budget abort. Any path reaching row la passes through row i-1 or row i for
	// every i (a normal step advances one row; the only two-row jump is a transposition,
	// which skips exactly one row — landing in the other), and costs are non-negative, so
	// the values along it never fall. Hence min over rows i-1 and i is a lower bound on
	// the final distance: once BOTH exceed Budget the answer cannot come back under it.
	// (The core's single-row abort holds only because its substitution and transposition
	// costs are equal; with arbitrary weights the skipped row forces the two-row form.)
	prevRowMin := 0.0 // row 0's minimum is prev1[0] == 0
	for i := 1; i <= la; i++ {
		cur[0] = float64(i) * w.Del // deleting a[:i] down to the empty prefix of b
		rowMin := cur[0]
		for j := 1; j <= lb; j++ {
			var cost float64
			if a[i-1] != b[j-1] {
				cost = sub(a[i-1], b[j-1])
			}
			// deletion (from above) / insertion (from the left) / substitution (diagonal)
			v := min(prev1[j]+w.Del, cur[j-1]+w.Ins, prev1[j-1]+cost)
			// Adjacent transposition: a[i-1]a[i-2] read as b[j-2]b[j-1].
			if i > 1 && j > 1 && a[i-1] == b[j-2] && a[i-2] == b[j-1] {
				v = min(v, prev2[j-2]+w.Transpose)
			}
			cur[j] = v
			rowMin = min(rowMin, v)
		}
		if prevRowMin > w.Budget && rowMin > w.Budget {
			return math.Inf(1)
		}
		prevRowMin = rowMin
		// Rotate: cur becomes prev1, prev1 becomes prev2, the stale prev2 is recycled.
		prev2, prev1, cur = prev1, cur, prev2
	}
	return prev1[lb] // rotation left the final row in prev1
}
