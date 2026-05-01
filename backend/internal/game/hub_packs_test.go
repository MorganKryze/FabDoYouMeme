package game

import (
	"testing"
)

// TestWeightedShuffle_ReturnsAllEntries asserts the no-drop / no-dup
// invariant. Probabilistic order is exercised separately.
func TestWeightedShuffle_ReturnsAllEntries(t *testing.T) {
	in := []WeightedPackRef{
		{PackID: [16]byte{1}, Weight: 1},
		{PackID: [16]byte{2}, Weight: 5},
		{PackID: [16]byte{3}, Weight: 3},
	}
	out := weightedShuffle(in)
	if len(out) != len(in) {
		t.Fatalf("len(out) = %d, want %d", len(out), len(in))
	}
	seen := map[[16]byte]int{}
	for _, e := range out {
		seen[e.PackID]++
	}
	for _, e := range in {
		if seen[e.PackID] != 1 {
			t.Errorf("pack %x: appeared %d times, want 1", e.PackID, seen[e.PackID])
		}
	}
}

// TestWeightedShuffle_SingleEntry collapses to identity.
func TestWeightedShuffle_SingleEntry(t *testing.T) {
	in := []WeightedPackRef{{PackID: [16]byte{42}, Weight: 7}}
	out := weightedShuffle(in)
	if len(out) != 1 || out[0] != in[0] {
		t.Fatalf("single-entry shuffle deformed: %+v", out)
	}
}

// TestWeightedShuffle_HeavyWinsOften samples the shuffle 10k times and
// asserts the heavily-weighted entry wins the first slot the vast majority
// of the time. With weights 100:1 the Efraimidis–Spirakis sampler should
// pick the heavy entry with probability 100/101 ≈ 99%; we leave plenty of
// headroom to keep the test non-flaky.
func TestWeightedShuffle_HeavyWinsOften(t *testing.T) {
	heavy := [16]byte{1}
	light := [16]byte{2}
	in := []WeightedPackRef{
		{PackID: heavy, Weight: 100},
		{PackID: light, Weight: 1},
	}
	wins := 0
	const trials = 10_000
	for i := 0; i < trials; i++ {
		out := weightedShuffle(in)
		if out[0].PackID == heavy {
			wins++
		}
	}
	// Expect ~99% heavy wins; require ≥90% to leave room for variance.
	if wins < trials*9/10 {
		t.Fatalf("heavy entry won %d/%d trials, want ≥ %d", wins, trials, trials*9/10)
	}
}

// TestWeightedShuffle_ZeroWeightDefaultsToOne keeps a misconfigured zero
// weight from producing NaN/Inf keys. The validator rejects zero weights at
// room creation, but the shuffle helper guards as a belt-and-braces.
func TestWeightedShuffle_ZeroWeightDefaultsToOne(t *testing.T) {
	in := []WeightedPackRef{
		{PackID: [16]byte{1}, Weight: 0},
		{PackID: [16]byte{2}, Weight: 1},
	}
	out := weightedShuffle(in)
	if len(out) != 2 {
		t.Fatalf("len(out) = %d, want 2", len(out))
	}
}
