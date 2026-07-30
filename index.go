package snare

// Index is a BK-tree over a [Set]'s targets that answers the same near-match query as
// [Set.Nearest] in sublinear time, for corpora past the tens-of-thousands crossover
// where the brute-force length-bucket scan starts to cost. Build one with [Set.Index];
// query it with [Index.Nearest]. An Index is read-only after Index() and safe for
// concurrent [Index.Nearest].
//
// The tree is metric on true Levenshtein distance — a genuine metric, so its radius
// search is exact — while the answer is scored with the core's bounded OSA
// ([editDistance]), exactly as Nearest scores it. The two differ only by transpositions,
// and are pinned by OSA <= Lev <= 2*OSA (a transposition is one OSA edit but at most two
// Levenshtein edits): a candidate within OSA k therefore lies within Levenshtein 2k, so
// a radius-2k Levenshtein search returns a superset of every OSA-k neighbour, which
// [Index.Nearest] then re-ranks by OSA. This is why the tree cannot use OSA directly:
// OSA violates the triangle inequality (it is not a metric), so an OSA-keyed BK-tree
// could prune away a true neighbour. The Levenshtein detour is what buys exact parity.
type Index struct {
	root *bkNode
	// maxLen is the longest target rune length, used to size the Levenshtein scratch
	// once per query rather than per visited node.
	maxLen int
}

// bkNode is one node of the BK-tree. Each child edge is keyed by the exact Levenshtein
// distance from this node's target to the child's; by construction every target in a
// child's subtree sits at exactly that distance from this node, which is what makes the
// radius prune sound.
type bkNode struct {
	e        entry
	children map[int]*bkNode
}

// Index builds a BK-tree over the Set's targets. It runs in O(n * d) distance
// computations for n targets of average depth d in the tree; the tree shape depends on
// insertion order, but [Index.Nearest] re-ranks its candidates deterministically, so the
// answer does not.
func (s *Set) Index() *Index {
	idx := &Index{}
	for _, es := range s.buckets {
		for _, e := range es {
			if len(e.runes) > idx.maxLen {
				idx.maxLen = len(e.runes)
			}
			if idx.root == nil {
				idx.root = &bkNode{e: e}
				continue
			}
			idx.root.insert(e)
		}
	}
	return idx
}

// insert threads e down the tree, following the child at each node keyed by the exact
// Levenshtein distance to that node and creating the child where none exists.
func (n *bkNode) insert(e entry) {
	for {
		d := levenshtein(n.e.runes, e.runes, nil)
		if d == 0 {
			return // identical to an existing target; Set already dedups, so defensive
		}
		child, ok := n.children[d]
		if !ok {
			if n.children == nil {
				n.children = make(map[int]*bkNode)
			}
			n.children[d] = &bkNode{e: e}
			return
		}
		n = child
	}
}

// Nearest returns the target closest to q, identical in contract and result to
// [Set.Nearest] — same length-relative cap ([kFor]), same exclusion of an exact match,
// same (smallest distance, then lexicographically smallest target) tie-break — but found
// through the BK-tree instead of a full length-bucket scan.
//
// It walks the tree with a Levenshtein radius of 2k (k = kFor(len(q))): a node's children
// are visited only when their edge distance lies within 2k of the node's distance to q,
// the exact-radius prune the Levenshtein metric permits. Every node inside that radius is
// then scored with the bounded OSA distance and kept only if it lands in [1, k], so the
// winner is exactly Nearest's. Nearest is safe for concurrent use.
func (i *Index) Nearest(q string) (target string, dist int, ok bool) {
	if i.root == nil {
		return "", 0, false
	}
	qr := []rune(q)
	qlen := len(qr)
	k := kFor(qlen)
	r := 2 * k // Levenshtein radius whose ball is a superset of the OSA-k neighbours

	// OSA scratch, sized for the widest target that can still score within k: an OSA-k
	// neighbour differs in length by at most k, so len(target) <= qlen+k and this buffer
	// never reallocs. levBuf is the Levenshtein row, sized once for the longest target.
	scratch := make([]int, scratchLen(qlen+k))
	levBuf := make([]int, i.maxLen+1)

	bestDist := k + 1
	bestTarget := ""
	exact := false

	// Iterative DFS over an explicit stack — no recursion-depth surprise on a deep tree.
	stack := []*bkNode{i.root}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		dLev := levenshtein(qr, n.e.runes, levBuf)
		// Only nodes within the Levenshtein radius can be OSA-k neighbours (OSA >= Lev/2,
		// so Lev > 2k implies OSA > k); score just those.
		if dLev <= r {
			if n.e.s == q {
				exact = true // q IS a target — decided after the walk, it outranks any near-miss
			} else if d := editDistance(qr, n.e.runes, k, scratch); d >= 1 && d <= k {
				if d < bestDist || (d == bestDist && n.e.s < bestTarget) {
					bestDist = d
					bestTarget = n.e.s
				}
			}
		}
		// Triangle inequality: a neighbour within r of q sits within [dLev-r, dLev+r] of
		// this node, so children keyed outside that band cannot hold one — prune them.
		for w, child := range n.children {
			if w >= dLev-r && w <= dLev+r {
				stack = append(stack, child)
			}
		}
	}
	if exact {
		return "", 0, false
	}
	if bestTarget == "" {
		return "", 0, false
	}
	return bestTarget, bestDist, true
}

// levenshtein is the true (unbounded) Levenshtein distance between rune slices a and b:
// insertions, deletions, and substitutions each cost 1, with no transposition. Unlike the
// core's OSA it satisfies the triangle inequality, which is exactly what the BK-tree needs
// of its metric. One rolling row of lb+1 cells (with a saved diagonal) suffices, since
// without transpositions no row before i-1 is ever read. buf is optional scratch of at
// least len(b)+1 ints; a nil or short buf is allocated internally.
func levenshtein(a, b []rune, buf []int) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	if cap(buf) < lb+1 {
		buf = make([]int, lb+1)
	}
	row := buf[:lb+1]
	for j := 0; j <= lb; j++ {
		row[j] = j // turning the empty prefix of a into b[:j] costs j insertions
	}
	for i := 1; i <= la; i++ {
		prev := row[0] // d[i-1][j-1], the diagonal, before this row overwrites it
		row[0] = i     // deleting a[:i] down to the empty prefix of b
		for j := 1; j <= lb; j++ {
			diag := row[j] // d[i-1][j], saved before overwrite to become the next diagonal
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			row[j] = min(row[j]+1, row[j-1]+1, prev+cost)
			prev = diag
		}
	}
	return row[lb]
}
