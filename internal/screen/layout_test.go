package screen

import "testing"

// A screen with a box under a long status is too tall for the space below the
// object, and would run under the progress line, so it sits low like naming.
func TestTallScreensSitLow(t *testing.T) {
	for _, id := range []ID{"naming", "identity-name", "identity-email", "github-code", "identity"} {
		def, ok := find(id)
		if !ok {
			t.Fatalf("no screen %s", id)
		}
		if def.layout != LayoutLow {
			t.Errorf("%s has layout %q, want %q", id, def.layout, LayoutLow)
		}
	}
}
