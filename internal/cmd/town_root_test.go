package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

// atTownRoot is what lets `gt crew list` mean "every rig" at the town root
// instead of erroring out. The distinction it draws is narrow — town root yes,
// one directory into a rig no — so it is worth pinning both sides down.
// newTownRoot builds the minimum a town root needs to be recognized:
// mayor/town.json, workspace.PrimaryMarker. Deliberately not reusing
// setupTestTown from rig_integration_test.go — that one is behind the
// integration build tag and pulls in far more scaffolding than this needs.
func newTownRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mayorDir := filepath.Join(root, "mayor")
	if err := os.MkdirAll(mayorDir, 0755); err != nil {
		t.Fatalf("mkdir mayor: %v", err)
	}
	if err := os.WriteFile(filepath.Join(mayorDir, "town.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("write town.json: %v", err)
	}
	return root
}

func TestAtTownRoot(t *testing.T) {
	townRoot := newTownRoot(t)

	rigDir := filepath.Join(townRoot, "somerig")
	if err := os.MkdirAll(filepath.Join(rigDir, "crew", "someone"), 0755); err != nil {
		t.Fatalf("mkdir rig: %v", err)
	}

	// Resolve symlinks: on macOS t.TempDir() hands back /var/... while the
	// workspace lookup sees /private/var/..., and the comparison is on paths.
	resolvedTown, err := filepath.EvalSymlinks(townRoot)
	if err != nil {
		t.Fatalf("eval symlinks: %v", err)
	}

	tests := []struct {
		name string
		dir  string
		want bool
	}{
		{"town root itself", resolvedTown, true},
		{"inside a rig", filepath.Join(resolvedTown, "somerig"), false},
		{"deep inside a rig", filepath.Join(resolvedTown, "somerig", "crew", "someone"), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(tc.dir)
			if got := atTownRoot(); got != tc.want {
				t.Errorf("atTownRoot() in %s = %v, want %v", tc.dir, got, tc.want)
			}
		})
	}
}

// Outside a Gas Town workspace entirely there is no town root to be at, so the
// answer must be false rather than an accidental true that would silently turn
// a bare `gt crew list` into a town-wide scan.
func TestAtTownRootOutsideWorkspace(t *testing.T) {
	outside := t.TempDir()
	resolved, err := filepath.EvalSymlinks(outside)
	if err != nil {
		t.Fatalf("eval symlinks: %v", err)
	}
	t.Chdir(resolved)

	if atTownRoot() {
		t.Errorf("atTownRoot() = true outside any workspace, want false")
	}
}
