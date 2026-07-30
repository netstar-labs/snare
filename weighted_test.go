package snare

import (
	"math"
	"testing"
)

// leetSub is a stand-in for the confusable cost function a caller would wire from
// unmask/twister tables: the leet look-alikes a<->4, o<->0, e<->3 cost 0.4, every other
// substitution the full 1.0. It is only ever called for differing runes.
func leetSub(a, b rune) float64 {
	class := func(r rune) rune {
		switch r {
		case '4':
			return 'a'
		case '0':
			return 'o'
		case '3':
			return 'e'
		}
		return r
	}
	if class(a) == class(b) {
		return 0.4
	}
	return 1.0
}

// approx reports whether two weighted distances are equal within float slop.
func approx(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestNearestWeighted(t *testing.T) {
	s := New([]string{"paypal", "google", "microsoft"})

	// Two leet substitutions (4->a twice) cost 0.8, within a one-edit budget, so the
	// aggressive squat p4yp4l is caught where the uniform-cost core (two full edits on a
	// short label, k=1) would miss it.
	w := Weights{Sub: leetSub, Ins: 1, Del: 1, Transpose: 1, Budget: 1.0}
	if tgt, d, ok := s.NearestWeighted("p4yp4l", w); !ok || tgt != "paypal" || !approx(d, 0.8) {
		t.Fatalf("NearestWeighted(p4yp4l) = (%q, %v, %v), want (paypal, 0.8, true)", tgt, d, ok)
	}

	// Two RANDOM substitutions (x for a, twice) cost 2.0 and must NOT match under the
	// same budget — the point of weighting is that plausible edits are cheap, arbitrary
	// ones are not.
	if tgt, d, ok := s.NearestWeighted("pxypxl", w); ok {
		t.Fatalf("NearestWeighted(pxypxl) = (%q, %v, %v), want no hit", tgt, d, ok)
	}

	// An exact target is not a squat.
	if _, _, ok := s.NearestWeighted("paypal", w); ok {
		t.Fatalf("exact target must not be a weighted hit")
	}
}

// TestNearestWeightedDefaultSub checks that a nil Sub defaults to uniform cost 1.0,
// making NearestWeighted a float mirror of the integer core: one substitution costs 1.0.
func TestNearestWeightedDefaultSub(t *testing.T) {
	s := New([]string{"paypal"})
	w := Weights{Ins: 1, Del: 1, Transpose: 1, Budget: 1.0}
	if tgt, d, ok := s.NearestWeighted("paypa1", w); !ok || tgt != "paypal" || !approx(d, 1.0) {
		t.Fatalf("NearestWeighted(paypa1, nil Sub) = (%q, %v, %v), want (paypal, 1.0, true)", tgt, d, ok)
	}
}

// TestNearestWeightedTranspose checks that the transposition cost is honoured
// independently of the substitution weight: papyal is one adjacent swap from paypal.
func TestNearestWeightedTranspose(t *testing.T) {
	s := New([]string{"paypal"})
	base := Weights{Sub: leetSub, Ins: 1, Del: 1, Transpose: 0.5}
	hit := base
	hit.Budget = 0.6
	if tgt, d, ok := s.NearestWeighted("papyal", hit); !ok || tgt != "paypal" || !approx(d, 0.5) {
		t.Fatalf("NearestWeighted(papyal, T=0.5, B=0.6) = (%q, %v, %v), want (paypal, 0.5, true)", tgt, d, ok)
	}
	miss := base
	miss.Budget = 0.4
	if _, _, ok := s.NearestWeighted("papyal", miss); ok {
		t.Fatalf("transposition cost 0.5 must miss under budget 0.4")
	}
}

// TestNearestWeightedUnbounded covers the degenerate weighting where an insertion is
// free (min(Ins,Del) == 0): the length prune is disabled so every bucket is scanned,
// and a longer target forces the DP scratch to grow. papal reaches paypal by one free
// insertion, a zero-cost — but non-identical — hit.
func TestNearestWeightedUnbounded(t *testing.T) {
	s := New([]string{"paypal"})
	w := Weights{Ins: 0, Del: 1, Transpose: 1, Budget: 1.0} // nil Sub -> uniform 1.0
	if tgt, d, ok := s.NearestWeighted("papal", w); !ok || tgt != "paypal" || !approx(d, 0) {
		t.Fatalf("NearestWeighted(papal, Ins=0) = (%q, %v, %v), want (paypal, 0, true)", tgt, d, ok)
	}
}

// TestEditDistanceWeighted drives the weighted distance directly — the empty-operand
// closed forms (which New never feeds through NearestWeighted, since it drops "") and
// the scratch-growth path from a short buffer.
func TestEditDistanceWeighted(t *testing.T) {
	w := Weights{Sub: leetSub, Ins: 1, Del: 1, Transpose: 1, Budget: 10}
	if got := editDistanceWeighted([]rune(""), []rune("abc"), w, w.Sub, nil); !approx(got, 3) {
		t.Errorf("weighted(%q,%q) = %v, want 3", "", "abc", got)
	}
	if got := editDistanceWeighted([]rune("abc"), []rune(""), w, w.Sub, nil); !approx(got, 3) {
		t.Errorf("weighted(%q,%q) = %v, want 3", "abc", "", got)
	}
	// A deliberately undersized buffer must be grown internally and still be correct.
	short := make([]float64, 3)
	if got := editDistanceWeighted([]rune("p4yp4l"), []rune("paypal"), w, w.Sub, short); !approx(got, 0.8) {
		t.Errorf("weighted(%q,%q) with short buf = %v, want 0.8", "p4yp4l", "paypal", got)
	}
}

// TestNearestWeightedIndel checks a weighted insertion/deletion and the length-bucket
// prune: gooogle is one deletion from google (in band), while a target that differs in
// length by more than Budget/min(Ins,Del) is pruned before scoring.
func TestNearestWeightedIndel(t *testing.T) {
	s := New([]string{"google", "paypalpaypal"})
	w := Weights{Sub: leetSub, Ins: 1, Del: 1, Transpose: 1, Budget: 1.0}
	if tgt, d, ok := s.NearestWeighted("gooogle", w); !ok || tgt != "google" || !approx(d, 1.0) {
		t.Fatalf("NearestWeighted(gooogle) = (%q, %v, %v), want (google, 1.0, true)", tgt, d, ok)
	}
	// "paypal" (len 6) is six shorter than "paypalpaypal" (len 12); with Budget 1.0 and
	// unit indel cost only lengths within 1 are candidates, so the long target is pruned.
	if _, _, ok := s.NearestWeighted("paypal", w); ok {
		t.Fatalf("out-of-band-length target must be pruned under the weighted length bound")
	}
}
