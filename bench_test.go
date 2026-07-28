package twist

import (
	"strconv"
	"testing"
)

// genTargets builds n deterministic pseudo-random lowercase labels of length 4..14,
// mimicking a curated brand/domain list, and always includes "paypal" so a fixed
// near-miss query has exactly one hit to find.
func genTargets(n int) []string {
	out := make([]string, 0, n+1)
	out = append(out, "paypal")
	x := uint64(0x9e3779b97f4a7c15)
	for i := 0; i < n; i++ {
		x = x*6364136223846793005 + 1442695040888963407
		length := 4 + int(x>>60)%11 // 4..14
		b := make([]byte, length)
		y := x
		for j := range b {
			y = y*2862933555777941757 + 3037000493
			b[j] = byte('a' + int(y>>59)%26)
		}
		out = append(out, string(b))
	}
	return out
}

// BenchmarkNearest measures a single query against curated target lists of 1e2,
// 1e3, and 1e4 entries. Length-bucket pruning means only same-length-band targets
// are ever scored, so brute force stays microsecond-class — this is also the marker
// for where a BK-tree/SymSpell index would begin to pay off.
func BenchmarkNearest(b *testing.B) {
	const query = "paypa1" // one substitution from the injected "paypal"
	for _, n := range []int{100, 1000, 10000} {
		s := New(genTargets(n))
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _, _ = s.Nearest(query)
			}
		})
	}
}

// BenchmarkNew measures index construction (dedup + length bucketing) at the same
// scales.
func BenchmarkNew(b *testing.B) {
	for _, n := range []int{100, 1000, 10000} {
		targets := genTargets(n)
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = New(targets)
			}
		})
	}
}
