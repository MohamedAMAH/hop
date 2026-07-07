package ui

import (
	"strings"
	"testing"
)

func TestRenderSyncContainsCounts(t *testing.T) {
	out := RenderSync("push", 3, 4)
	if !strings.Contains(out, "push") || !strings.Contains(out, "3") || !strings.Contains(out, "4") {
		t.Fatalf("summary missing key content: %q", out)
	}
}

func TestRenderStatusContainsFields(t *testing.T) {
	out := RenderStatus("B", "demo", "folder", "manual", 2, true, "2026-07-08T00:00:00Z")
	for _, want := range []string{"B", "demo", "folder", "manual", "2"} {
		if !strings.Contains(out, want) {
			t.Fatalf("status missing %q in %q", want, out)
		}
	}
}
