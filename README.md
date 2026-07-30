# twist

Edit-distance typosquat detection for Go — **build a `Set` over your targets, query
for the nearest within a small edit budget**, pure standard library, no
dependencies. twist *detects* typosquats — query → nearest target; it is the counter
to `twister`, which *generates* the look-alike domains to hunt for. It is the
edit-distance sibling of `ditto` — the same build-index-then-query shape,
Damerau-Levenshtein (OSA) instead of a locality-sensitive hash.

```
targets []string ─▶ New ─▶ dedup ─▶ bucket by rune length ─▶ Set
                                                              │
query ─▶ k = 1 (short) | 2 (long) ─▶ scan buckets within k of len(q)
                                                              │
              bounded OSA distance (row-min abort)            ▼
                            keep smallest dist in [1,k] ─▶ Nearest ─▶ (target, dist, ok)
```

## Quick start

```go
s := twist.New([]string{"paypal", "google", "amazon", "apple"})

s.Nearest("paypa1")  // ("paypal", 1, true)  — substitution l -> 1
s.Nearest("papyal")  // ("paypal", 1, true)  — adjacent transposition
s.Nearest("paypal")  // ("", 0, false)       — exact match is the target, not a squat
s.Nearest("paypal-secure") // ("", 0, false) — combosquat, far past the budget
```

```sh
go run ./app/twist near -t targets.txt paypa1 papyal   # nearest per query
printf 'paypa1\n' | go run ./app/twist near -t targets.txt   # queries on stdin
```

## Documentation

- **Start here** — [docs/introduction.md](docs/introduction.md) ·
  [docs/executive-summary.md](docs/executive-summary.md)
- **Deep dive** — [docs/architecture.md](docs/architecture.md)
- **Operations** — [docs/userguide.md](docs/userguide.md)
- **Examples** — [example/README.md](example/README.md)

## Layout

| File | Purpose |
|---|---|
| [twist.go](twist.go) | `Set`, `New` (dedup + length buckets), `Nearest`, and the `kFor` length-relative budget policy |
| [distance.go](distance.go) | `editDistance` — bounded Damerau-Levenshtein (OSA) over runes; three-row DP with length and row-min prunes |
| [domainset.go](domainset.go) | v0.2 — `DomainSet.Check`: tld-swap (same label, other TLD) + typo detection over registrable domains |
| [weighted.go](weighted.go) | v0.2 — `NearestWeighted`: confusability-weighted OSA (caller-supplied per-class substitution cost) |
| [index.go](index.go) | v0.2 — `Set.Index`: a BK-tree for sublinear *exact* nearest lookup on a large target list |
| [doc.go](doc.go) | package doc — the name metaphor (detects, not generates) and the one-question scope |
| [app/twist/](app/twist/main.go) | the CLI — `near` (`-index`) · `domain` · `version` |

## Notes

- Go module `github.com/netstar-labs/twist`. **Standard library only** — no
  dependencies. Build standalone with `GOWORK=off`.
- twist normalizes nothing and knows no brands: it compares a raw query against a
  raw target list and returns nearest + distance. The caller normalizes (lowercase,
  eTLD+1, IDNA) and owns the target list; consumers wire it in a
  few lines.
- Combosquats (`paypal-secure`) and homoglyphs are out of scope by design — a
  different signal and a different metric; see
  [docs/architecture.md](docs/architecture.md) § "Deliberately out (YAGNI)".
