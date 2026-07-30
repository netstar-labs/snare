# snare — user guide

## Library

```go
import "github.com/netstar-labs/snare"

s := snare.New([]string{"paypal", "google", "amazon", "apple", "microsoft"})

s.Nearest("paypa1")        // ("paypal", 1, true)  — substitution l -> 1
s.Nearest("papyal")        // ("paypal", 1, true)  — adjacent transposition
s.Nearest("gooogle")       // ("google", 1, true)  — one inserted letter
s.Nearest("paypal")        // ("", 0, false)       — exact match is the target, not a squat
s.Nearest("paypal-secure") // ("", 0, false)       — combosquat, far past the budget
s.Nearest("random")        // ("", 0, false)       — nothing within the budget
```

`New(targets)` builds an immutable, deduplicated, length-bucketed `Set`. `Nearest(q)`
returns `(target, dist, ok)`:

- `ok == true` only for a **genuine near-miss**: `1 ≤ dist ≤ k`, where `k` is the
  length-relative budget — **1** for queries of ≤ 6 runes, **2** for longer ones.
- `ok == false` when `q` exactly equals a target (an exact match *is* the target, not
  a squat) or when no target lies within `k` edits.
- Among targets within the budget, the smallest distance wins; ties break on the
  lexicographically smallest target, so the result is deterministic.

Distance is bounded Damerau-Levenshtein (Optimal String Alignment) over **runes** —
insertion, deletion, substitution, and adjacent transposition each cost 1 — so an
accented or multibyte character is one edit, not several bytes.

### Contract and cost

- **snare normalizes nothing.** It compares the raw query against the raw targets.
  The caller normalizes first — lowercase, strip to eTLD+1, IDNA-decode, whatever the
  domain rules require — and feeds snare the already-normalized strings.
- **snare is data-agnostic.** Any `[]string` is a valid target list; there is no
  brand-specific logic. The consumer owns the target projection.
- A `Set` is read-only after `New` and **safe for concurrent `Nearest` calls**; each
  call allocates a single scratch buffer and touches no shared state.
- Empty strings and exact duplicate targets are dropped by `New` (first occurrence
  wins).

## CLI

Build: `go build -o snare ./app/snare` (standalone: `GOWORK=off`).

```sh
# targets one per line; queries as arguments
snare near -t targets.txt paypa1 papyal paypal
#   paypa1  paypal  1
#   papyal  paypal  1
#   (paypal is an exact target — skipped, not a squat)

# queries one per line on stdin when no arguments are given
printf 'paypa1\npapyal\n' | snare near -t targets.txt

# also print the misses, marked with -
snare near -t targets.txt -all paypa1 paypal random
#   paypa1  paypal  1
#   paypal  -       -
#   random  -       -

snare version
```

Output is one tab-separated line per near-miss: `query <tab> nearest <tab> dist`.
Queries with no near-miss are skipped unless `-all` is set, which prints them with a
`-` in the nearest and distance columns.

| Command | Flags | Meaning |
|---|---|---|
| `near` | `-t <file>` (required), `-all` | print `query<tab>nearest<tab>dist` for each near-miss; queries from args or stdin |
| `version` | — | binary version + build revision |

## Build & deploy

`build/snare [--cli] [user@host]` cross-compiles the CLI to `linux/amd64` with a
`git describe` version stamp, packages a self-contained installer + `.tgz`, and —
given a host — scp's it over and installs the binary to `/usr/local/bin` via ssh.
With no host it just builds the package under `build/install/`.

## When to reach past snare

snare is brute force plus length-bucket pruning — microsecond-class over a curated
list of hundreds to low thousands of targets. If the target list or query volume
grows large enough that `Nearest` shows up in a profile, reach for the v0.2 BK-tree
`Set.Index` — the same exact result, sublinear lookup (`bench_test.go` at 1e2 / 1e3 /
1e4 targets is the yardstick). Combosquats (`paypal-secure`) and homoglyphs are out of
scope by design — a different signal and a different metric respectively; see
[architecture.md](architecture.md) § "Deliberately out (YAGNI)".
