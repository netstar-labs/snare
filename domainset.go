package snare

import "strings"

// Kind classifies a [DomainSet.Check] hit by the mechanism that made the domain a
// look-alike: an edit-distance typo of a brand label, or a straight swap of the
// registrable TLD onto an exact brand label.
type Kind int

const (
	// KindTypo is a near-miss of a brand label found by the edit-distance core:
	// paypa1.com against paypal.com. The label differs from the brand's within the
	// length-relative edit budget (see [Set.Nearest]); the TLD is not considered.
	KindTypo Kind = iota
	// KindTLDSwap is an exact brand label carried onto a TLD the brand does not use:
	// google.net against google.com. Edit distance cannot see this — on the label it
	// is an exact match — so it is caught by set membership, not by [Set.Nearest].
	KindTLDSwap
)

// String renders a Kind for logs and test output.
func (k Kind) String() string {
	switch k {
	case KindTypo:
		return "typo"
	case KindTLDSwap:
		return "tld-swap"
	default:
		return "unknown"
	}
}

// Result describes a single [DomainSet.Check] hit: which brand the queried domain
// impersonates, by which mechanism, and how far the label sits from the brand's.
type Result struct {
	// Kind is the mechanism: KindTypo or KindTLDSwap.
	Kind Kind
	// Brand is the registrable brand domain that was impersonated, exactly as passed
	// to NewDomainSet. When several brands share a label (google.com and google.co.uk),
	// the lexicographically smallest is reported so the result is deterministic.
	Brand string
	// Dist is the label edit distance from the brand: the OSA distance for a KindTypo
	// hit, and 0 for a KindTLDSwap hit (the label is an exact match — only the TLD moved).
	Dist int
}

// DomainSet is a thin domain-level adapter over the edit-distance core. It splits a
// candidate domain into its registrable label and TLD, then answers two questions the
// bare label [Set] cannot: is the label a typo of a brand label ([Set.Nearest]), and —
// the structural blind spot of edit distance — is it an exact brand label wearing a
// TLD the brand does not use. A DomainSet is read-only after NewDomainSet and safe for
// concurrent [DomainSet.Check].
//
// Domains are compared case-insensitively: labels and TLDs are lower-cased on the way
// in, both for the brands and for each query. (The core [Set] is normalization-agnostic;
// this adapter folds case because DNS names are.)
type DomainSet struct {
	// labels holds the distinct brand labels for typo delegation to Set.Nearest.
	labels *Set
	// legit maps a brand label to the set of TLDs that label legitimately uses,
	// taken solely from the brand list — a TLD outside this set is a swap candidate.
	legit map[string]map[string]struct{}
	// brand maps a brand label to the registrable brand domain reported for it (the
	// lexicographically smallest, when a label spans several brands).
	brand map[string]string
}

// NewDomainSet builds a DomainSet from registrable brand domains — each a label plus
// an eTLD, e.g. "paypal.com" or "google.co.uk". Each brand's label is registered for
// typo detection and its TLD recorded as legitimate for that label; a label a brand
// legitimately uses under several TLDs must list each brand (google.com AND google.de)
// or the unlisted TLDs read as swaps. Empty entries and unsafely-splittable entries are
// skipped — the latter being a brand under a multi-part suffix multiSuffix does not
// list (see [suspectMisSplit]). NewDomainSet does not validate label/TLD syntax beyond
// that; callers are responsible for passing syntactically valid registrable domains.
// The input slice is neither retained nor modified.
func NewDomainSet(brands []string) *DomainSet {
	legit := make(map[string]map[string]struct{}, len(brands))
	brand := make(map[string]string, len(brands))
	var labels []string
	for _, b := range brands {
		label, tld, ambiguous := splitDomain(b)
		if label == "" || suspectMisSplit(label, ambiguous) {
			continue
		}
		tlds := legit[label]
		if tlds == nil {
			// First time this label is seen: register it for typo delegation once.
			tlds = make(map[string]struct{})
			legit[label] = tlds
			labels = append(labels, label)
		}
		if tld != "" {
			tlds[tld] = struct{}{}
		}
		// Keep the lexicographically (case-insensitively) smallest brand string as the
		// reported brand so a label shared across several brands resolves
		// deterministically — stored as passed, so Result.Brand's "exactly as passed"
		// promise holds regardless of which candidate wins the tie-break.
		if cur, ok := brand[label]; !ok || strings.ToLower(b) < strings.ToLower(cur) {
			brand[label] = b
		}
	}
	return &DomainSet{labels: New(labels), legit: legit, brand: brand}
}

// Check classifies a candidate domain against the brands. It returns a Result and true
// on a hit, or the zero Result and false when the domain is either a legitimate brand
// domain (exact label on a known TLD) or nothing close.
//
// The label takes precedence over the TLD. An exact brand label resolves by TLD alone:
// a known TLD is the brand itself — not a squat — while any other TLD is a KindTLDSwap.
// A label that is not an exact brand label is delegated to [Set.Nearest]; a near-miss
// within budget is a KindTypo (its TLD is not examined — a typo is a typo on any TLD).
func (d *DomainSet) Check(domain string) (Result, bool) {
	label, tld, _ := splitDomain(domain)
	if label == "" {
		return Result{}, false
	}
	if tlds, ok := d.legit[label]; ok {
		// Exact brand label. With no TLD to compare (a bare label) there is nothing to
		// swap, so it is not a squat; otherwise a known TLD is legitimate and any other
		// is a swap.
		if tld == "" {
			return Result{}, false
		}
		if _, legit := tlds[tld]; legit {
			return Result{}, false
		}
		return Result{Kind: KindTLDSwap, Brand: d.brand[label], Dist: 0}, true
	}
	if target, dist, ok := d.labels.Nearest(label); ok {
		return Result{Kind: KindTypo, Brand: d.brand[target], Dist: dist}, true
	}
	return Result{}, false
}

// multiSuffix is a SMALL, deliberately partial set of common multi-label public
// suffixes so that google.co.uk splits to the label "google", not "co". It is NOT the
// Public Suffix List and is not vendored from one: only the popular second-level
// registries are listed. The cost of the gap is BIDIRECTIONAL: an unlisted multi-label
// suffix (a rare ccTLD second level) mis-splits, taking the suffix's own second label
// (say "co" from an unlisted "co.kr") as the "registrable label". That both misses real
// squats AND — because two unlisted suffixes can share that second label (co.kr, co.ke)
// — could otherwise report unrelated domains as tld-swaps of each other. NewDomainSet
// closes the false-positive direction by dropping a brand whose label parses to such a
// suffix second level ([suspectMisSplit]); callers needing exact eTLD boundaries for
// every TLD should split upstream against a real PSL and feed labels to the bare Set.
// Everything here is lower-cased to match splitDomain.
var multiSuffix = map[string]struct{}{
	"co.uk": {}, "org.uk": {}, "me.uk": {}, "ltd.uk": {}, "plc.uk": {},
	"net.uk": {}, "sch.uk": {}, "ac.uk": {}, "gov.uk": {},
	"com.au": {}, "net.au": {}, "org.au": {}, "edu.au": {}, "gov.au": {}, "id.au": {},
	"co.nz": {}, "net.nz": {}, "org.nz": {}, "govt.nz": {},
	"com.br": {}, "net.br": {}, "org.br": {}, "gov.br": {},
	"co.jp": {}, "or.jp": {}, "ne.jp": {}, "go.jp": {},
	"co.in": {}, "net.in": {}, "org.in": {}, "gov.in": {},
	"co.za": {}, "org.za": {}, "gov.za": {},
	"com.cn": {}, "net.cn": {}, "org.cn": {}, "gov.cn": {},
	"com.mx": {}, "com.tr": {}, "com.sg": {}, "com.hk": {}, "com.tw": {},
}

// suffixSecondLevel is the set of labels that appear as the second level of a
// multi-part public suffix (the "co" in "co.uk", the "com" in "com.au"). When
// splitDomain hands one of these back as a registrable label from its ambiguous
// fallback path, the domain almost certainly used a multi-part suffix multiSuffix does
// not list, so the split is unreliable — see [suspectMisSplit]. Derived from multiSuffix
// once at init, rather than hand-maintained separately, so the two can never drift.
var suffixSecondLevel = func() map[string]struct{} {
	m := make(map[string]struct{}, len(multiSuffix))
	for suffix := range multiSuffix {
		second, _, _ := strings.Cut(suffix, ".")
		m[second] = struct{}{}
	}
	return m
}()

// suspectMisSplit reports whether splitDomain likely mis-parsed the domain that
// produced label: the split took the ambiguous 3+-label fallback path (see
// [splitDomain]'s ambiguous result) AND label came out as a known public-suffix second
// level (co, com, …) — the signature of a multi-part suffix multiSuffix does not list.
// Such a brand cannot be split safely without a real PSL, so NewDomainSet drops it
// rather than register it under a bogus label — which would let unrelated domains under
// a sibling suffix (co.kr vs co.ke) read as tld-swaps. A split that matched a *listed*
// multiSuffix entry is never ambiguous, however label reads: co.co.uk splits to label
// "co" via the trusted co.uk entry, not the risky fallback, and is kept.
func suspectMisSplit(label string, ambiguous bool) bool {
	if !ambiguous {
		return false
	}
	_, bad := suffixSecondLevel[label]
	return bad
}

// splitDomain separates a domain into its registrable label and registrable TLD. The
// TLD is the final two labels when they form a known [multiSuffix] (co.uk), else the
// final label alone (com); the registrable label is the single label immediately to its
// left. Any further subdomains are dropped — login.paypal.com and paypal.com both yield
// ("paypal", "com"). A trailing root dot is tolerated and input is lower-cased. A domain
// with no dot has no TLD to compare and yields (itself, "", false).
//
// ambiguous reports whether the split took the risky fallback: a 3+-label domain whose
// final two labels did NOT match a listed multiSuffix entry, so the final label alone was
// taken as the TLD on the (unverifiable) assumption that it is a single-label suffix
// rather than the second level of an unlisted multi-part one. A 2-label domain, and any
// split that matched a listed multiSuffix entry, is never ambiguous — see
// [suspectMisSplit], which is the only caller that needs this bit.
func splitDomain(domain string) (label, tld string, ambiguous bool) {
	domain = strings.ToLower(strings.TrimSuffix(domain, "."))
	labels := strings.Split(domain, ".")
	n := len(labels)
	if n < 2 {
		return domain, "", false
	}
	if n >= 3 {
		if _, ok := multiSuffix[labels[n-2]+"."+labels[n-1]]; ok {
			return labels[n-3], labels[n-2] + "." + labels[n-1], false
		}
		return labels[n-2], labels[n-1], true
	}
	return labels[n-2], labels[n-1], false
}
