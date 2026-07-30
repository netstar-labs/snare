// Example near builds a Set over a few brand targets and queries a handful of
// look-alikes, showing what does and does not count as an edit-distance typosquat.
//
//	go run ./example/near
package main

import (
	"fmt"

	"github.com/netstar-labs/snare"
)

func main() {
	targets := []string{"paypal", "google", "amazon", "apple", "microsoft"}
	s := snare.New(targets)

	fmt.Printf("targets: %v\n\n", targets)

	queries := []struct {
		q, note string
	}{
		{"paypa1", "substitution: l -> 1"},
		{"papyal", "adjacent transposition"},
		{"gooogle", "an extra inserted letter"},
		{"paypal", "exact brand — the target itself, not a squat"},
		{"paypal-secure", "combosquat — far past the edit budget, not snare's job"},
		{"random", "nothing close"},
	}

	fmt.Printf("%-15s %-11s %s\n", "query", "result", "why")
	fmt.Printf("%-15s %-11s %s\n", "-----", "------", "---")
	for _, c := range queries {
		near, dist, ok := s.Nearest(c.q)
		result := "no hit"
		if ok {
			result = fmt.Sprintf("%s (%d)", near, dist)
		}
		fmt.Printf("%-15s %-11s %s\n", c.q, result, c.note)
	}
}
