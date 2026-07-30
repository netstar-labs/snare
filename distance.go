package snare

// editDistance computes the Optimal String Alignment (OSA) distance between rune
// slices a and b, bounded by k. OSA is restricted Damerau-Levenshtein: insertions,
// deletions, substitutions, and adjacent transpositions all cost 1, with the
// restriction that no substring is edited more than once — enough to catch a
// transposed pair (papyal ↔ paypal) at cost 1 while staying a simple three-row DP.
//
// The result is the true distance when it is <= k, and k+1 (a sentinel meaning
// "farther than k") otherwise. Two prunes make a far-apart pair almost free: the
// length difference is a lower bound on the distance, checked before any DP; and
// the moment an entire DP row's minimum exceeds k the answer cannot come back under
// k, so the computation aborts. Operating on runes gives correct Unicode distance —
// an accented rune is one edit, not the two-or-more bytes of its encoding.
//
// buf is optional scratch of at least 3*(len(b)+1) ints; a nil or short buf is
// allocated internally. Callers reuse buf across candidates to avoid per-call
// allocation.
func editDistance(a, b []rune, k int, buf []int) int {
	la, lb := len(a), len(b)

	// Length difference alone is a lower bound on the edit distance.
	if la-lb > k || lb-la > k {
		return k + 1
	}
	// Empty operand: the distance is the other's length (all insertions/deletions).
	if la == 0 {
		return capped(lb, k)
	}
	if lb == 0 {
		return capped(la, k)
	}

	// Three rolling rows carved from one buffer: prev2 is DP row i-2 (needed only by
	// the transposition step), prev1 is row i-1, cur is the row being filled.
	rl := lb + 1
	if cap(buf) < scratchLen(lb) {
		buf = make([]int, scratchLen(lb))
	}
	prev2 := buf[0:rl]
	prev1 := buf[rl : 2*rl]
	cur := buf[2*rl : 3*rl]

	// Row 0: turning the empty prefix of a into b[:j] costs j insertions.
	for j := 0; j <= lb; j++ {
		prev1[j] = j
	}

	for i := 1; i <= la; i++ {
		cur[0] = i // deleting a[:i] down to the empty prefix of b
		rowMin := cur[0]
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			// deletion / insertion / substitution (cost 0 on a match)
			v := min(prev1[j]+1, cur[j-1]+1, prev1[j-1]+cost)
			// Adjacent transposition: a[i-1]a[i-2] read as b[j-2]b[j-1].
			if i > 1 && j > 1 && a[i-1] == b[j-2] && a[i-2] == b[j-1] {
				v = min(v, prev2[j-2]+1)
			}
			cur[j] = v
			rowMin = min(rowMin, v)
		}
		// If the true distance is <= k it is reached by a non-decreasing optimal path,
		// so every row holds a cell <= k: either one the path visits directly, or — when
		// a transposition step (i-1,j-2)->(i+1,j) skips row i entirely — the diagonal
		// neighbour (i,j-1), whose value <= d[i-1][j-2]+1 equals that transposition's own
		// cost <= k. So once this whole row's minimum exceeds k, the final distance does
		// too — abort.
		if rowMin > k {
			return k + 1
		}
		// Rotate the rows: the just-filled cur becomes prev1, prev1 becomes prev2, and
		// the stale prev2 buffer is recycled as the next cur.
		prev2, prev1, cur = prev1, cur, prev2
	}
	return capped(prev1[lb], k) // rotation left the final row in prev1
}

// capped returns d if it is within the budget k, else the over-cap sentinel k+1.
func capped(d, k int) int {
	if d > k {
		return k + 1
	}
	return d
}

// scratchLen is the reusable DP scratch size for comparing against a target of n
// runes: three rolling rows of n+1 cells. Nearest sizes one buffer with this and
// reuses it across every candidate, so the scratch is a single allocation however
// many targets are scored; keeping the formula here — not duplicated at the call
// site — keeps that contract from silently drifting.
func scratchLen(n int) int { return 3 * (n + 1) }
