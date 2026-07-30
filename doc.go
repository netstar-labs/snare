// Package snare detects typosquats by edit distance: build a [Set] over a list of
// target strings, then ask for the [Set.Nearest] target to a query within a small
// edit budget. snare *detects* typosquats: query → nearest target, the defensive
// side of the problem — its counter twister *generates* the look-alike permutations.
// It is the
// edit-distance sibling of ditto (near-duplicate documents by SimHash): the same
// build-index-then-query shape, Damerau-Levenshtein (OSA) instead of a locality-sensitive
// hash.
//
// A query is a near-miss when it lies within k edits of a target and is not the
// target itself. paypa1 and papyal are each one edit from paypal — a substitution
// and an adjacent transposition; paypal itself is not a squat (an exact match is
// the target, not a look-alike), and paypal-secure is far past the budget, so it is
// correctly not an edit hit at all. snare bounds the distance at k and buckets
// targets by rune length, so a query is only ever compared against the handful of
// targets that could possibly be within k — brute force over a curated list stays
// microsecond-fast without a specialized index.
//
// It is pure Go, standard library only, and operates on runes for correct Unicode
// edit distance. snare is data-agnostic (feed it any target list, not just brands)
// and normalization-agnostic (the caller normalizes the query first); it answers
// only "what is the nearest target, and how far?" — nothing about what the query
// means.
package snare
