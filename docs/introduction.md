# Meet twist — the near-miss, not the permutation

Every phishing kit begins with a lie told in a domain name. `paypa1.com` for
`paypal.com`; `papyal` for `paypal`; `micros0ft`, `gooogle`, `arnazon` — a single
keystroke off a name you trust, close enough that the eye slides right over it. Squat-hunting
has two directions. The attacker's way round is *generative* — give a tool a brand
and it *generates* the thousands of permutations a squatter might register, so you
can go hunting for them (that is twist's counter, `twister`). twist faces the other
direction. It does not generate — it **detects**. Hand it
the string in front of you and it answers one question: *is this a near-miss for
something on my list, and how near?*

## What it actually is

twist is a pure-Go, zero-dependency implementation of **bounded Damerau-Levenshtein (OSA)
edit distance** with a query interface. You build a `Set` over a list of target
strings — brands, domains, whatever you are protecting — and then call
`Nearest(query)`; it returns the closest target within a small edit budget, or
nothing. "Edit distance" is the number of single-character insertions, deletions,
substitutions, and adjacent transpositions that turn one string into another, so
`paypa1` is one edit from `paypal` (a substitution) and `papyal` is one edit too (a
swap of two neighbours). twist counts those edits over *runes*, not bytes, so an
accented or CJK character is one edit and not the several bytes of its encoding.
The budget is deliberately tight and length-relative — one edit for short labels,
two for longer ones — because past that the "typo" stops being plausible and starts
being a different word. An exact match is not a squat: it *is* the target, and twist
says so by returning no hit.

## The capability nobody hands you for free

The naive way to answer "nearest within k edits" is to run the distance function
against every target — quadratic in spirit and slow at scale. twist makes it cheap
two ways. It **bounds** the distance: the moment a candidate's running cost exceeds
the budget, the computation aborts, so a far-apart target costs almost nothing to
reject. And it **buckets targets by length**: since it takes at least *n* edits to
change a string's length by *n*, a query is only ever compared against targets whose
length is within the budget — the rest are skipped without a glance. Over a curated
list of hundreds to low thousands of targets, a single query resolves in
microseconds, with no specialized index to build, tune, or persist.

## The scope it keeps

twist answers one question — *nearest target, and how far* — and refuses the
neighbouring ones. It is not a combosquat detector: `paypal-secure` is a brand token
glued to a keyword, a different signal entirely, and twist correctly returns no hit
because it is far more than a typo. It has no opinion on homoglyphs (that is the
Unicode-skeleton metric, a different distance), no notion of what a domain resolves
to, and no idea what any string *means*. It takes a raw list and a raw query and
gives you a distance — nothing more. That discipline is what keeps it a pure
function you can drop into a detector, an identity service, or anything else in three
lines, and unit-test in isolation.

*Read next:* [executive-summary.md](executive-summary.md) ·
[architecture.md](architecture.md) · [userguide.md](userguide.md) ·
[../example/README.md](../example/README.md)
