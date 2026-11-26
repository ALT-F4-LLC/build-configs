package templates

import (
	"os"
	"slices"
	"testing"
)

func TestIgnoreEntryForms(t *testing.T) {
	const original = "user content\n"

	for _, tc := range []struct{ name, entry string }{
		{name: "bare", entry: "sample.cfg"},
		{name: "dot-slash", entry: "./sample.cfg"},
		{name: "surrounding-blanks", entry: "  sample.cfg  "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if err := os.WriteFile(".bcignore", []byte(tc.entry+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile("sample.cfg", []byte(original), 0o600); err != nil {
				t.Fatal(err)
			}

			err := WriteFiles(map[string]string{
				"./sample.cfg": "generated sample\n",
				"./other.cfg":  "generated other\n",
			})
			if err != nil {
				t.Fatalf("WriteFiles: %v", err)
			}

			sample, err := os.ReadFile("sample.cfg")
			if err != nil {
				t.Fatal(err)
			}
			if string(sample) != original {
				t.Errorf("sample.cfg overwritten despite .bcignore entry %q: got %q", tc.entry, sample)
			}

			other, err := os.ReadFile("other.cfg")
			if err != nil {
				t.Fatalf("other.cfg was not written: %v", err)
			}
			if string(other) != "generated other\n" {
				t.Errorf("other.cfg = %q, want %q", other, "generated other\n")
			}
		})
	}
}

func TestIgnoreGetIgnoredFiles(t *testing.T) {
	t.Run("comment-blank-line-and-trailing-newline", func(t *testing.T) {
		dir := t.TempDir()
		contents := "# kept by hand\njustfile\n\n./README.md\n"
		if err := os.WriteFile(dir+"/.bcignore", []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}

		got, err := GetIgnoredFiles(dir)
		if err != nil {
			t.Fatalf("GetIgnoredFiles: %v", err)
		}

		want := []string{"justfile", "README.md"}
		if !slices.Equal(got, want) {
			t.Errorf("GetIgnoredFiles = %q, want %q", got, want)
		}
	})
}
