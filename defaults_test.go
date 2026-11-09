package main

import "testing"

func TestParseDomains(t *testing.T) {
	got := parseDomains("com.apple.dock, com.apple.Safari ,, NSGlobalDomain")
	want := []string{"com.apple.dock", "com.apple.Safari", "NSGlobalDomain"}
	if len(got) != len(want) {
		t.Fatalf("parseDomains() = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("parseDomains()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
