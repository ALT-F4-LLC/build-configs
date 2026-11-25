package config

import (
	"os"
	"path/filepath"
	"testing"
)

// Public renders get their own goldens, one workflow per template, so the
// existing private goldens stay as they are and prove a private render did not
// change. Regenerate with -update after reviewing the diff.
func TestPublicVisibilityWorkflowSnapshot(t *testing.T) {
	for _, tc := range visibilityTemplates {
		t.Run(tc.template, func(t *testing.T) {
			golden, err := filepath.Abs(filepath.Join("testdata", tc.template+"-public-workflow.golden"))
			if err != nil {
				t.Fatalf("resolve golden path: %v", err)
			}

			visibilityRender(t, Config{Name: "demo", Template: tc.template, Visibility: VisibilityPublic}, tc.workflow)

			got, err := os.ReadFile(tc.workflow) //nolint:gosec // reads a file the test rendered into its own temp dir
			if err != nil {
				t.Fatalf("read rendered workflow: %v", err)
			}
			if *terraformModuleUpdateGolden {
				if err := os.WriteFile(golden, got, 0o644); err != nil { //nolint:gosec // golden files are world-readable like the others
					t.Fatalf("write golden: %v", err)
				}
				return
			}
			want, err := os.ReadFile(golden) //nolint:gosec // golden path is built from a fixed template name
			if err != nil {
				t.Fatalf("read golden %s (regenerate with -update): %v", golden, err)
			}
			if string(got) != string(want) {
				t.Errorf("public workflow differs from %s (regenerate with -update after reviewing the change)\ngot:\n%s", golden, got)
			}
		})
	}
}
