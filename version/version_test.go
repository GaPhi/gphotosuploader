package version

import (
	"strings"
	"testing"
)

func TestPseudoVersionDate(t *testing.T) {
	if d := pseudoVersionDate("v0.0.0-20261111165802-c3ad335b83b8"); !strings.HasPrefix(d, "2026-11-11") {
		t.Errorf("unexpected date %q", d)
	}
	if d := pseudoVersionDate("v1.2.3"); d != "unknown" {
		t.Errorf("unexpected date %q for a tag", d)
	}
}
