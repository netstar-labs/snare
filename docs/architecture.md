# twist — architecture

A flat library (`package twist`) at the repo root, one concern per file, plus a thin
CLI under `app/twist`. The pipeline is: target list → dedup + length buckets (the
`Set`) → query compared only against in-band targets → bounded edit distance per
candidate → nearest within the budget.

## Data flow

```
targets []string ─▶ New ─▶ dedup ─▶ bucket by rune length ─▶ Set{ len -> []entry }
                                                                       │
query q ─▶ k = kFor(len)  ─▶ scan buckets in [len(q)-k, len(q)+k]  ────┤
                                                                       ▼
                     editDistance(q, target, k)  (bounded OSA, row-min abort)
                                                                       │
                          keep smallest dist in [1,k], lexicographic tie-break
                                                                       ▼
                                        Nearest ─▶ (target, dist, ok)
```

## The distance: bounded OSA over runes (`distance.go`)

`editDistance` computes **Optimal String Alignment** (OSA) distance — restricted
Damerau-Levenshtein. Insertion, deletion, substitution, and *adjacent*
transposition each cost 1, with the restriction that no substring is edited more
than once. That restriction is what keeps it a simple dynamic program while still
catching a swapped neighbour pair: `papyal` ↔ `paypal` is one transposition, cost 1,
which plain Levenshtein would score as two substitutions.

- **Runes, not bytes.** Both operands are decoded to `[]rune` before the DP, so an
  accented or multibyte character counts as one edit, not the several bytes of its
  UTF-8 encoding (`café` ↔ `cafe` is distance 1). Targets are decoded once, at
  `New`, and cached on the `entry`.
- **The DP.** The classic edit-distance matrix, computed with three rolling rows
  rather than a full matrix — the transposition step needs the row two back
  (`d[i-2][j-2]`), so three rows suffice and memory is O(len(target)). The three
  rows are carved from one scratch buffer and rotated by swapping slice headers.
- **Two prunes make far-apart pairs almost free.** (1) *Length lower bound* — edit
  distance is at least the length difference, so a pair whose lengths differ by more
  than `k` returns the over-cap sentinel before any DP runs. (2) *Row-min abort* —
  after filling each DP row, if the row's minimum already exceeds `k`, the final
  distance must too, so the computation stops. If the true distance is `≤ k` its
  optimal (non-decreasing) path leaves every row holding a cell `≤ k` — either one the
  path visits, or, when a transposition step skips a row entirely, that row's diagonal
  neighbour, whose value is at most the transposition's own cost `≤ k`. So the bound
  stays valid. The function returns the true
  distance when it is `≤ k`, and `k+1` (a sentinel meaning "farther than k")
  otherwise.

## The index: dedup + length buckets (`twist.go`)

`New` builds the `Set`: it drops empty strings and exact duplicates (first
occurrence wins) and buckets the survivors by rune length — a `map[int][]entry`.
There is no tree, no hash of contents, nothing to tune; the whole "index" is the
length histogram.

`Nearest(q)` decodes the query, picks the budget `k = kFor(len(q))`, and scans only
buckets `len(q)-k .. len(q)+k` (the **length-bucket prune** — every other bucket is
provably out of reach). For each in-band target it runs `editDistance` with the
shared per-query scratch buffer (so a query allocates a small constant — its decoded
runes plus one reused scratch buffer — regardless of how many targets it scans) and keeps
the smallest distance in `[1, k]`. Ties break on lexicographically smallest target,
and bucket/entry iteration order is deterministic, so the result never depends on map
ordering.

### The k policy

`kFor` returns the **length-relative** budget: `k = 1` for short labels
(`len(q) ≤ 6` runes), `k = 2` above. Short strings are dense — one edit already
reaches a large neighbourhood — so a tighter budget preserves precision; longer
strings can absorb two edits before the "typo" stops being plausible. The budget
**excludes distance 0** by construction: an exact match is the target itself, so
`Nearest` short-circuits to `("", 0, false)` when the query equals a target, and only
ever returns a hit for a genuine near-miss `1 ≤ dist ≤ k`. This is why `paypal`
queried against a set containing `paypal` is *not* a squat, and why `paypal-secure`
— many edits from any brand — is correctly no hit at all.

## Complexity

Let *m* = query length, *L* = target length, *B* = number of in-band targets.

- **`editDistance`** is O(*m*·*L*) worst case, but the length prune caps *L* at
  *m*+*k* and the row-min abort usually stops after a handful of rows for a
  non-match; a rejected candidate is effectively O(*m*). twist is built for short
  labels (caller-normalized host/brand labels); the quadratic term only bites if a
  caller curates a very long target and queries a length-matched near-miss of it —
  atypical and self-inflicted, not an attacker surface.
- **`Nearest`** is O(*B*·*m*) — and *B* is only the targets in a ±*k* length band, a
  small fraction of a curated list. In practice a query against hundreds to low
  thousands of targets resolves in microseconds (see `bench_test.go`), at one
  allocation per call.
- **`New`** is O(total target length) and builds the buckets once.

## Deliberately out (YAGNI)

The scope is the "smallest real capability." Three tempting extensions are left out
on purpose, each recorded as a future upgrade rather than pre-built:

- **No BK-tree / SymSpell index.** Brute force plus length-bucket pruning is
  microsecond-class over a curated list; a metric-tree or deletion-index only starts
  to pay when the target list or query volume grows large. `bench_test.go` at 1e2 /
  1e3 / 1e4 targets is the marker for when that day arrives — build it then, not now.
- **No combosquat / token logic.** `paypal-secure` is a brand token plus a keyword —
  a different signal (token match, not edit distance) — and is correctly not a twist
  hit. Combosquat detection belongs to a token-aware consumer, not this metric.
- **No skeleton fold.** twist is edit-distance only; homoglyph look-alikes are the
  Unicode UTS-39 *skeleton* metric, a different distance. When a skeleton function
  lands, `Nearest(skeleton(q))` gives the homoglyph ∪ typosquat union for free —
  deferred until that exists rather than half-built here.

## Consumer wiring (future work)

twist imports none of its consumers and they wire it in a few lines, each owning its
own target projection and query normalization:

| Consumer | Signal | Wiring |
|---|---|---|
| Mail-capture detector | a `typosquat_suspect` feature | normalize (eTLD+1), project a brand target list, then `twist.New(t).Nearest(host)`. Adding a scored feature to a trained model triggers a retrain + model-version bump on the consumer side, not twist. |
| Identity / corpus service | corpus near-miss | on a lookup miss, `Nearest(eTLD+1, corpusDomains)`; a hit demotes/attributes, never keyword-promotes — compatible with a one-way-trust rule. |

Homoglyph matching (the UTS-39 skeleton metric) is intentionally separate — a
different distance for a different problem. twist is edit-distance only.

## Layout

| File | Purpose |
|---|---|
| [twist.go](../twist.go) | `Set`, `New` (dedup + length buckets), `Nearest`, and the `kFor` budget policy |
| [distance.go](../distance.go) | `editDistance` — bounded OSA over runes, three-row DP with length and row-min prunes |
| [doc.go](../doc.go) | package doc — the name metaphor and the one-question scope |
| [app/twist/](../app/twist/main.go) | the CLI — `near` · `version` |
