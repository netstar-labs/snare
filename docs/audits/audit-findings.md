# snare — A1 adversarial audit + least-code pass, pre-public

Full-repo audit ahead of public release, 2026-09-30: four dimensions (A simpler · B
dedup · C correctness+optimization · D doc-drift) plus a separate least-code pass.
Every candidate reproduced against the code before repair, then independently
re-verified by a second, adversarial pass whose only job was to try to refute it, then
re-verified a third time after the fix to confirm it actually holds with no regression.

## Verdict summary

| # | Finding | Dimension | Severity | Verdict | Status |
|---|---|---|---|---|---|
| 1 | `suspectMisSplit` dropped legitimate brands split via a *listed* multi-part suffix whose label reads as a suffix word (`co.co.uk`) | C | Moderate | CONFIRMED | **Fixed** |
| 2 | `Result.Brand` doc promised original case; code always lower-cased it | D | Moderate | CONFIRMED | **Fixed** |
| 3 | `NewDomainSet` doc promised malformed entries are skipped; no such validation exists | D | Moderate | CONFIRMED | **Fixed (doc)** |
| 4 | `suffixSecondLevel` was a second, hand-maintained copy of information `multiSuffix` already contains | B | Low | CONFIRMED | **Fixed** |
| 5 | `docs/userguide.md` and `docs/architecture.md` omitted the entire v0.2 feature surface (`domain` subcommand, `domainset.go`/`weighted.go`/`index.go`) | D | Low–Moderate | CONFIRMED | **Fixed** |
| 6 | `NearestWeighted` has no CLI/example surface, unlike the other two v0.2 features | A | Low | Noted | **Documented as by-design** |

## Finding 1 — the false-positive drop (CONFIRMED, fixed)

**Root cause.** `suspectMisSplit` fired on any 3+-label domain whose registrable label
matched a `suffixSecondLevel` word, regardless of *how* the split was produced. It
could not distinguish "took the risky unlisted-suffix fallback" (genuinely ambiguous)
from "matched a listed `multiSuffix` entry and the label just happens to read `co`"
(fully trustworthy). `co.co.uk` splits correctly to label `"co"` via the listed `co.uk`
entry, but was dropped as if it had mis-split.

**Reproduced, both directions.** `NewDomainSet([]string{"co.co.uk"})` registered zero
TLDs for label `"co"`; `Check("co.net")` — a real TLD-swap of that brand — returned no
hit. Generalizes: `org.org.uk`, `id.id.au` show the identical drop.

**Fix.** `splitDomain` now returns a third value, `ambiguous`, true only when it took
the 3+-label fallback with no listed-suffix match. `suspectMisSplit(label, ambiguous)`
only fires when `ambiguous` is true. The existing unlisted-suffix regression test
(`TestDomainSetSuspectMisSplitNoFalsePositive`) still passes unchanged — the false
*negative* direction (the reason `suspectMisSplit` exists at all) is untouched.

**Verified, not just fixed.** New regression test
`TestDomainSetListedSuffixSecondLevelLabelNotDropped`. Independently re-verified by a
second pass with its own inputs (`org.org.uk`, `id.id.au`, plus confirming
`example.co.kr` — the genuinely ambiguous case — is still correctly dropped).

## Finding 2 — case not preserved (CONFIRMED, fixed)

`Result.Brand`'s doc promised the brand "exactly as passed to NewDomainSet"; the
tie-break store always lower-cased it (`brand[label] = strings.ToLower(b)`).
Reproduced: `NewDomainSet([]string{"PayPal.COM"})` → `Check("paypal.net").Brand ==
"paypal.com"`, not `"PayPal.COM"`. **Fix:** the tie-break comparison now compares
lower-cased strings but stores the original `b`. New regression test
`TestDomainSetResultBrandPreservesCase`, covering both the single-brand case and the
multi-brand tie-break (confirms the *winning* candidate's original case survives, not
just any brand's).

## Finding 3 — a promise the code never implemented (CONFIRMED, fixed as doc)

`NewDomainSet`'s doc said "empty, malformed, and unsafely-splittable entries are
skipped," but no syntax validation exists anywhere — `splitDomain` only splits on `.`,
so `"http://paypal.com/login"` registers as a live brand label. This is intentional
scope, not a missing feature: the caller supplies its own curated brand list, not
untrusted input, so validating domain shape is the caller's job. **Fix:** the doc now
promises only what's implemented (empty-label and unsafely-splittable skipping) and
states explicitly that callers are responsible for syntax validity.

## Finding 4 — a derived set kept in sync by hand (CONFIRMED, fixed)

`suffixSecondLevel` (16 entries) was a hand-written literal that is exactly the set of
first labels of every `multiSuffix` key — nothing enforced the relationship, so a new
`multiSuffix` entry with a novel second-level label would silently stop being caught by
`suspectMisSplit`. **Fix:** derived once at init via `strings.Cut` over `multiSuffix`.
Confirmed programmatically, twice independently, that the derived set is
entry-for-entry identical to the original hand-written one (same 16 words, no more, no
less).

## Finding 5 — doc drift across the entire v0.2 surface (CONFIRMED, fixed)

`docs/userguide.md`'s CLI reference covered only `near`/`version`, omitting the `domain`
subcommand entirely — a reader following "Operations" docs had no way to discover it.
`docs/architecture.md`'s Layout table listed only the v0.1 files, omitting
`domainset.go`, `weighted.go`, and `index.go` despite the same document's own prose
discussing all three as shipped v0.2 features two sections earlier — internally
inconsistent, not just stale. **Fix:** both docs updated; the Layout table entry for
`weighted.go` states its library-only, no-CLI-surface status explicitly (see Finding 6).

## Finding 6 — the one v0.2 feature with no CLI surface (noted, not a defect)

`NearestWeighted` has no caller anywhere outside its own tests — `domainset.go`
(tld-swap) and `index.go` (BK-tree) both got a CLI subcommand or example wiring;
`weighted.go` didn't. Its own design doc explicitly frames this as deliberate,
isolated, external-consumer-facing API (for `unmask`/`twister`), not an oversight. Left
as library-only by design; `docs/architecture.md`'s Layout table now says so
explicitly, so it doesn't read as a forgotten wire-up to a new reader.

## Re-validation gate

| Check | Result |
|---|---|
| `go build` / `go vet` / `gofmt -l` | clean |
| `go test -race -count=1 ./...` | all packages pass, including 3 new regression tests |
| `FuzzEditDistance`, `FuzzNearest` (15–21s each, both before and after the fix) | ~3.5–8.8M execs, 0 crashers, 0 new failures |
| Independent skeptic pass on all 3 correctness/doc claims, pre-fix | all 3 CONFIRMED, could not be refuted |
| Independent skeptic pass on all 3 fixes, post-fix, with its own fresh inputs | all 3 HOLD, no regressions found |

## Least-code pass (separate dimension)

Same repo, audited against the least-code ladder (need → reuse → stdlib → platform →
installed dependency → one line → minimum that works): no config knobs nothing varies,
no single-implementation interfaces, no wrapper-for-wrapping's-sake. `suffixSecondLevel`
(Finding 4) was the one real duplication found; `NearestWeighted`'s CLI asymmetry
(Finding 6) was the one design judgment call, resolved as documentation rather than a
forced feature addition.
