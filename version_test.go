package bast

import "testing"

func TestDetectVersion_NotHardcoded(t *testing.T) {
	// Inside the framework repo itself there is no dependency entry to read,
	// so detection must fall back to "dev" — never a stale literal like "0.1.0".
	v := detectVersion()
	if v == "0.1.0" {
		t.Fatal("bastVersion must be detected from build info, not hardcoded to 0.1.0")
	}
	if v == "" {
		t.Fatal("detectVersion must never return empty")
	}
}
