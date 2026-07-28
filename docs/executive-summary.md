# twist — executive summary

**What it is.** A small, dependency-free Go library that detects typosquats by edit
distance. Build a `Set` over a list of target strings, then query for the nearest
target within a small edit budget. It is the edit-distance sibling of `ditto` (which
finds near-duplicate documents by SimHash): the same build-index-then-query shape,
Damerau-Levenshtein (OSA) instead of a locality-sensitive hash. Named after `dnstwist`,
but where that tool *generates* look-alike domains to hunt for, twist *detects* them:
query → nearest target.

**Why it exists.** A typosquat is a name one keystroke off a trusted one —
`paypa1`, `papyal`, `gooogle`, `micros0ft` — the workhorse of phishing and brand
abuse. Catching it means asking, of every untrusted string, "is this a near-miss for
something I protect?" Done naively that is a distance computation against every
target, which is slow and does not scale. twist makes the question cheap enough to
ask inline, on every message or lookup, against a curated brand list.

**What you get.**
- A one-call query — `Nearest(q)` returns `(target, distance, ok)`: the closest
  target and how many edits away, or no hit.
- **Bounded Damerau-Levenshtein (OSA) over runes** — insertion, deletion, substitution,
  and adjacent transposition each cost one; correct for Unicode, and aborted the
  moment a candidate exceeds the budget so far-apart strings are nearly free.
- **Length-relative budget** — one edit for short labels (≤ 6 runes), two above —
  and it **excludes exact matches**: an exact hit is the target itself, not a squat.
- **Length-bucket pruning** — a query is compared only against targets whose length
  is within the budget, so brute force over a curated list stays microsecond-class
  with no index to build or maintain.
- **No dependencies, no state, no configuration** — standard library only, a pure
  function of its inputs, safe for concurrent queries.

**Where it fits.** twist is the *algorithm* layer, coupled to nothing. It takes a
raw `[]string` and a raw query and returns nearest + distance — data-agnostic (feed
it any target list) and normalization-agnostic (the caller normalizes first).
Consumers wire it in a few lines and own their own target projection — e.g. a mail-capture detector as a
`typosquat_suspect` feature over a brand corpus, or an identity service as a corpus near-miss
on the identity side. Each supplies the target list and the normalized query; twist
supplies only the distance.

**What it is not.** Not a combosquat detector (`paypal-secure` is a brand token plus
a keyword — a different signal, correctly not an edit hit). Not a homoglyph detector
(that is the Unicode-skeleton metric, a different distance). Not a domain resolver or
a classifier. It answers one question — nearest target within k edits — and only
that.
