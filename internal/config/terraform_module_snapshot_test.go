package config

import (
	"bytes"
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"gopkg.in/yaml.v3"
)

var terraformModuleUpdateGolden = flag.Bool("update", false, "regenerate golden snapshot files under testdata")

const terraformModuleSnapshotConfig = `name: example-module
template: terraform-module
`

func TestTerraformModuleSnapshot(t *testing.T) {
	golden, err := filepath.Abs(filepath.Join("testdata", "terraform-module.golden"))
	if err != nil {
		t.Fatalf("resolve golden path: %v", err)
	}

	var cfg Config
	if err := yaml.Unmarshal([]byte(terraformModuleSnapshotConfig), &cfg); err != nil {
		t.Fatalf("parse config: %v", err)
	}

	root := t.TempDir()
	out := filepath.Join(root, "out")
	if err := os.Mkdir(out, 0o750); err != nil {
		t.Fatalf("create render dir: %v", err)
	}
	t.Chdir(out)

	tpl, err := cfg.GetTemplater()
	if err != nil {
		t.Fatalf("GetTemplater: %v", err)
	}
	if err := tpl.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}

	terraformModuleCompareGolden(t, golden, terraformModuleSnapshot(t, root))
}

func TestTerraformModuleSnapshotRecordsMissingFinalNewline(t *testing.T) {
	without, with := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(without, "file"), []byte("abc"), 0o600); err != nil {
		t.Fatalf("write file without final newline: %v", err)
	}
	if err := os.WriteFile(filepath.Join(with, "file"), []byte("abc\n"), 0o600); err != nil {
		t.Fatalf("write file with final newline: %v", err)
	}

	if bytes.Equal(terraformModuleSnapshot(t, without), terraformModuleSnapshot(t, with)) {
		t.Errorf("snapshots of %q and %q are identical; a dropped final newline would go undetected", "abc", "abc\n")
	}
}

// terraformModuleSnapshot lists every regular file under root in sorted path
// order, each as a "-- <path> --" header line followed by the file's content.
// Content that lacks a final newline is followed by a newline and the line
// "\ No newline at end of file", so the snapshot keeps that difference.
// Paths are slash-separated and relative to root.
func terraformModuleSnapshot(t *testing.T, root string) []byte {
	t.Helper()

	var paths []string
	err := fs.WalkDir(os.DirFS(root), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	slices.Sort(paths)

	var snap bytes.Buffer
	for _, path := range paths {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path))) //nolint:gosec // reads a file the test rendered into its own temp dir
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		snap.WriteString("-- " + path + " --\n")
		snap.Write(content)
		if len(content) > 0 && content[len(content)-1] != '\n' {
			snap.WriteString("\n\\ No newline at end of file\n")
		}
	}
	return snap.Bytes()
}

// terraformModuleCompareGolden fails the test when got differs from the golden
// file. With -update it rewrites the golden file from got instead.
func terraformModuleCompareGolden(t *testing.T, golden string, got []byte) {
	t.Helper()

	if *terraformModuleUpdateGolden {
		if err := os.MkdirAll(filepath.Dir(golden), 0o750); err != nil {
			t.Fatalf("create golden dir: %v", err)
		}
		if err := os.WriteFile(golden, got, 0o600); err != nil {
			t.Fatalf("update golden: %v", err)
		}
		return
	}

	want, err := os.ReadFile(golden) //nolint:gosec // golden path is fixed by the test
	if err != nil {
		t.Fatalf("read golden (regenerate with -update): %v", err)
	}
	if bytes.Equal(got, want) {
		return
	}
	gotLines, wantLines := bytes.Split(got, []byte("\n")), bytes.Split(want, []byte("\n"))
	line := 0
	for line < len(gotLines) && line < len(wantLines) && bytes.Equal(gotLines[line], wantLines[line]) {
		line++
	}
	t.Errorf("render differs from %s at line %d (regenerate with -update after reviewing the change):\ngot:  %q\nwant: %q",
		golden, line+1, terraformModuleLineAt(gotLines, line), terraformModuleLineAt(wantLines, line))
}

func terraformModuleLineAt(lines [][]byte, i int) string {
	if i >= len(lines) {
		return "<end of file>"
	}
	return string(lines[i])
}
