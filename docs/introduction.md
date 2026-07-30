# snare — set the trap, catch the near-miss

Every phishing kit opens with a lie told in a domain name. `paypa1.com` for
`paypal.com`; `papyal` for `paypal`; `micros0ft`, `gooogle`, `arnazon` — one keystroke
off a name you trust, close enough that the eye slides right over it. Squat-hunting runs
in two directions. One is *generative*: hand a tool a brand and it spins out the
thousands of look-alikes a squatter might register, so you can go hunting — that is
snare's counterpart, [`twister`](https://github.com/netstar-labs/twister). snare faces
the other way. It does not generate the variants; it is the **trap you set to catch
them**. Show it the string in front of you — a domain from a CT-log firehose, a sender,
a referrer — and it answers one question: *is this a near-miss for something I protect,
and how near?*

## What it actually is

snare is a pure-Go, zero-dependency implementation of **bounded Damerau-Levenshtein (OSA)
edit distance** with a query interface. You build a `Set` over the strings you're
protecting — brands, domains, whatever's on your watchlist — and then call
`Nearest(query)`; it returns the closest protected name within a small edit budget, or
nothing. "Edit distance" is the count of single-character insertions, deletions,
substitutions, and adjacent transpositions that turn one string into another: `paypa1`
is one edit from `paypal` (a substitution), `papyal` one edit too (a neighbour swap).
snare counts those over *runes*, not bytes, so an accented or CJK character is a single
edit, not the several bytes of its encoding. The budget is deliberately tight and
length-relative — one edit for short labels, two for longer ones — because past that a
"typo" stops being plausible and becomes a different word. An exact match is not a squat:
it *is* the protected name, and snare says so by catching nothing.

## The capability nobody hands you for free

The naive way to answer "nearest within k edits" is to run the distance function against
every target — quadratic in spirit, slow at scale. snare makes it cheap two ways. It
**bounds** the distance: the instant a candidate's running cost exceeds the budget the
computation aborts, so a far-apart name costs almost nothing to reject. And it **buckets
targets by length**: since changing a string's length by *n* takes at least *n* edits, a
query is only ever compared against names whose length is within the budget — the rest
are never looked at. Over a curated list of hundreds to low thousands of names, a query
resolves in microseconds, with no specialized index to build, tune, or persist.

## Where it sits

snare is the **detect** half of a matched pair: `twister` walks the edit-distance space
outward to *generate* the candidates a squatter would register; snare walks it inward to
*catch* the ones that actually appear. Same metric, opposite directions. It is also the
edit-distance sibling of [`ditto`](https://github.com/netstar-labs/ditto) (SimHash
near-dup) and [`akin`](https://github.com/netstar-labs/akin) (MinHash set-similarity) —
the same build-an-index-then-query shape, a different notion of "close."

## The scope it keeps

snare answers one question — *nearest protected name, and how far* — and refuses the
neighbouring ones. It is not a combosquat detector: `paypal-secure` is a brand token glued
to a keyword, a different signal, and snare correctly catches nothing because it is far
more than a typo. It has no opinion on homoglyphs (that's the Unicode-skeleton metric — see
[`unmask`](https://github.com/netstar-labs/unmask)), no notion of what a domain resolves
to, and no idea what any string *means*. It takes a raw list and a raw query and gives you
a distance — nothing more. That discipline is what keeps it a pure function you can drop
into a detector, an identity service, or a CT-log watcher in three lines, and unit-test in
isolation.

*Read next:* [executive-summary.md](executive-summary.md) ·
[architecture.md](architecture.md) · [userguide.md](userguide.md) ·
[../example/README.md](../example/README.md)
