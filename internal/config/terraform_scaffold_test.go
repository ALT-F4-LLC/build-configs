package config

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// terraformScaffoldRender renders template (terraform or terraform-module)
// named name into the current directory, which the caller has made.
func terraformScaffoldRender(t *testing.T, template, name string) {
	t.Helper()

	tpl, err := Config{Name: name, Template: template}.GetTemplater()
	if err != nil {
		t.Fatalf("GetTemplater: %v", err)
	}
	if err := tpl.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
}

type terraformScaffoldPackage struct {
	Name            string            `json:"name"`
	Private         bool              `json:"private"`
	Type            string            `json:"type"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

// bunLockTrailingComma matches the trailing commas bun.lock allows and JSON
// does not.
var bunLockTrailingComma = regexp.MustCompile(`,(\s*[}\]])`)

type terraformScaffoldLock struct {
	LockfileVersion int `json:"lockfileVersion"`
	Workspaces      map[string]struct {
		Name            string            `json:"name"`
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	} `json:"workspaces"`
	Packages map[string]json.RawMessage `json:"packages"`
}

func terraformScaffoldReadLock(t *testing.T) terraformScaffoldLock {
	t.Helper()

	raw, err := os.ReadFile("bun.lock")
	if err != nil {
		t.Fatalf("bun.lock was not written: %v", err)
	}
	var lock terraformScaffoldLock
	if err := json.Unmarshal(bunLockTrailingComma.ReplaceAll(raw, []byte("$1")), &lock); err != nil {
		t.Fatalf("bun.lock is not JSON with trailing commas: %v", err)
	}
	return lock
}

func TestTerraformScaffoldWritesBunProject(t *testing.T) {
	for _, template := range []string{TerraformName, TerraformModuleName} {
		t.Run(template, func(t *testing.T) {
			t.Chdir(t.TempDir())
			terraformScaffoldRender(t, template, "demo-infra")

			var pkg terraformScaffoldPackage
			raw, err := os.ReadFile("package.json")
			if err != nil {
				t.Fatalf("package.json was not written: %v", err)
			}
			if err := json.Unmarshal(raw, &pkg); err != nil {
				t.Fatalf("package.json is not JSON: %v\n%s", err, raw)
			}
			if pkg.Name != "demo-infra" || !pkg.Private || pkg.Type != "module" {
				t.Errorf("package.json = %+v, want name demo-infra, private, type module", pkg)
			}
			if got := pkg.Dependencies["@altf4llc/vorpal-sdk"]; got != "0.6.0" {
				t.Errorf("@altf4llc/vorpal-sdk = %q, want the exact version 0.6.0", got)
			}
			for name, version := range pkg.Dependencies {
				if strings.ContainsAny(version, "^~*<>= ") {
					t.Errorf("dependency %s = %q, want an exact version", name, version)
				}
			}
			for name, version := range pkg.DevDependencies {
				if strings.ContainsAny(version, "^~*<>= ") {
					t.Errorf("devDependency %s = %q, want an exact version", name, version)
				}
			}

			var tsconfig struct {
				CompilerOptions map[string]any `json:"compilerOptions"`
				Include         []string       `json:"include"`
			}
			raw, err = os.ReadFile("tsconfig.json")
			if err != nil {
				t.Fatalf("tsconfig.json was not written: %v", err)
			}
			if err := json.Unmarshal(raw, &tsconfig); err != nil {
				t.Fatalf("tsconfig.json is not JSON: %v\n%s", err, raw)
			}
			if tsconfig.CompilerOptions["strict"] != true || tsconfig.CompilerOptions["noEmit"] != true {
				t.Errorf("tsconfig compilerOptions = %v, want strict and noEmit", tsconfig.CompilerOptions)
			}
			if len(tsconfig.Include) != 1 || tsconfig.Include[0] != "src" {
				t.Errorf("tsconfig include = %q, want [src]", tsconfig.Include)
			}
		})
	}
}

// bun install --frozen-lockfile fails when the lockfile and package.json
// disagree, so the rendered pair must agree on name and every dependency.
func TestTerraformScaffoldLockMatchesPackage(t *testing.T) {
	for _, template := range []string{TerraformName, TerraformModuleName} {
		t.Run(template, func(t *testing.T) {
			t.Chdir(t.TempDir())
			terraformScaffoldRender(t, template, "demo-infra")

			var pkg terraformScaffoldPackage
			raw, err := os.ReadFile("package.json")
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(raw, &pkg); err != nil {
				t.Fatal(err)
			}
			lock := terraformScaffoldReadLock(t)

			root, ok := lock.Workspaces[""]
			if !ok || len(lock.Workspaces) != 1 {
				t.Fatalf("bun.lock workspaces = %v, want only the root workspace", lock.Workspaces)
			}
			if root.Name != pkg.Name {
				t.Errorf("bun.lock workspace name = %q, package.json name = %q", root.Name, pkg.Name)
			}
			for _, group := range []struct {
				kind      string
				lock, pkg map[string]string
			}{
				{"dependencies", root.Dependencies, pkg.Dependencies},
				{"devDependencies", root.DevDependencies, pkg.DevDependencies},
			} {
				if len(group.lock) != len(group.pkg) {
					t.Errorf("%s: bun.lock has %v, package.json has %v", group.kind, group.lock, group.pkg)
				}
				for name, version := range group.pkg {
					if group.lock[name] != version {
						t.Errorf("%s %s: bun.lock %q, package.json %q", group.kind, name, group.lock[name], version)
					}
					entry, ok := lock.Packages[name]
					if !ok {
						t.Errorf("bun.lock has no packages entry for %s", name)
						continue
					}
					if !strings.Contains(string(entry), "\""+name+"@"+version+"\"") || !strings.Contains(string(entry), "sha512-") {
						t.Errorf("bun.lock entry for %s = %s, want %s@%s and a sha512 integrity", name, entry, name, version)
					}
				}
			}
		})
	}
}

// sha512Integrity matches the integrity field of a locked package.
var sha512Integrity = regexp.MustCompile(`^sha512-[A-Za-z0-9+/]{86}==$`)

// Every locked package is a registry package pinned by integrity hash, so a
// registry cannot swap a tarball unnoticed and nothing resolves from a URL, a
// path or a git ref.
func TestTerraformScaffoldLockPinsEveryPackage(t *testing.T) {
	t.Chdir(t.TempDir())
	terraformScaffoldRender(t, TerraformName, "demo-infra")

	lock := terraformScaffoldReadLock(t)
	if len(lock.Packages) == 0 {
		t.Fatal("bun.lock has no packages")
	}
	if lock.LockfileVersion != 1 {
		t.Errorf("lockfileVersion = %d, want 1, which bun 1.2 and later read", lock.LockfileVersion)
	}
	for name, raw := range lock.Packages {
		var entry []json.RawMessage
		if err := json.Unmarshal(raw, &entry); err != nil || len(entry) != 4 {
			t.Errorf("bun.lock entry for %s = %s, want [name@version, registry, metadata, integrity]", name, raw)
			continue
		}
		var ident, registry, integrity string
		if err := json.Unmarshal(entry[0], &ident); err != nil || !strings.HasPrefix(ident, name+"@") {
			t.Errorf("bun.lock entry for %s has identifier %s, want %s@<version>", name, entry[0], name)
		}
		if err := json.Unmarshal(entry[1], &registry); err != nil || registry != "" {
			t.Errorf("bun.lock entry for %s has registry %s, want the default registry (empty)", name, entry[1])
		}
		if err := json.Unmarshal(entry[3], &integrity); err != nil || !sha512Integrity.MatchString(integrity) {
			t.Errorf("bun.lock entry for %s has integrity %s, want sha512-<base64>", name, entry[3])
		}
		version := strings.TrimPrefix(ident, name+"@")
		if version == "" || strings.ContainsAny(version, ":/^~*<>= ") {
			t.Errorf("bun.lock entry for %s = %s, want an exact registry version", name, ident)
		}
	}
}

// package.json and bun.lock are written together or not at all, since a lock
// that does not match its manifest fails bun install --frozen-lockfile.
func TestTerraformScaffoldWritesTheBunPairTogether(t *testing.T) {
	for _, tc := range []struct {
		name     string
		existing map[string]string
		ignore   string
		wantPkg  bool
		wantLock bool
	}{
		{name: "empty", wantPkg: true, wantLock: true},
		{name: "own package.json", existing: map[string]string{"package.json": "{}\n"}},
		{name: "own bun.lock", existing: map[string]string{"bun.lock": "{}\n"}},
		{name: "bun.lockb", existing: map[string]string{"bun.lockb": "x"}},
		{name: "package-lock.json", existing: map[string]string{"package-lock.json": "{}\n"}},
		{name: "yarn.lock", existing: map[string]string{"yarn.lock": "x"}},
		{name: "package.json ignored", ignore: "package.json\n", wantLock: false},
		{name: "bun.lock ignored", ignore: "bun.lock\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			for name, content := range tc.existing {
				if err := os.WriteFile(name, []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if tc.ignore != "" {
				if err := os.WriteFile(".bcignore", []byte(tc.ignore), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			terraformScaffoldRender(t, TerraformName, "demo-infra")

			for name, want := range map[string]bool{"package.json": tc.wantPkg, "bun.lock": tc.wantLock} {
				got, err := fs.ReadFile(os.DirFS("."), name)
				_, existed := tc.existing[name]
				switch {
				case want && err != nil:
					t.Errorf("%s was not written: %v", name, err)
				case !want && !existed && err == nil:
					t.Errorf("%s was written beside the developer's files:\n%s", name, got)
				case existed && string(got) != tc.existing[name]:
					t.Errorf("%s = %q, want the existing %q", name, got, tc.existing[name])
				}
			}
			if _, err := os.Stat("tsconfig.json"); err != nil {
				t.Errorf("tsconfig.json was not written: %v", err)
			}
		})
	}
}

// A dangling symlink counts as an existing file, so the render never writes
// through it.
func TestTerraformScaffoldDoesNotWriteThroughSymlink(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	target := filepath.Join(t.TempDir(), "outside.json")
	if err := os.Symlink(target, "tsconfig.json"); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	terraformScaffoldRender(t, TerraformName, "demo-infra")

	if _, err := os.Stat(target); err == nil {
		t.Errorf("render wrote through the tsconfig.json symlink to %s", target)
	}
}

// The files are owned by Renovate, bun and the developer after the first run,
// so a second render leaves each one as it is.
func TestTerraformScaffoldKeepsExistingFiles(t *testing.T) {
	for _, template := range []string{TerraformName, TerraformModuleName} {
		t.Run(template, func(t *testing.T) {
			t.Chdir(t.TempDir())
			mine := map[string]string{
				"package.json":  "{\"name\": \"mine\"}\n",
				"bun.lock":      "",
				"tsconfig.json": "{}\n",
			}
			for name, content := range mine {
				if err := os.WriteFile(name, []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			terraformScaffoldRender(t, template, "demo-infra")

			for name, want := range mine {
				got, err := fs.ReadFile(os.DirFS("."), name)
				if err != nil {
					t.Fatal(err)
				}
				if string(got) != want {
					t.Errorf("%s = %q, want the existing %q", name, got, want)
				}
			}
			// The generated files are still rewritten.
			if _, err := os.Stat("src/vorpal.ts"); err != nil {
				t.Errorf("src/vorpal.ts was not written: %v", err)
			}
		})
	}
}

// A second render over the first changes nothing, so a lockfile bun or Renovate
// rewrote survives `build-configs generate`.
func TestTerraformScaffoldSecondRenderKeepsEdits(t *testing.T) {
	t.Chdir(t.TempDir())
	terraformScaffoldRender(t, TerraformName, "demo-infra")

	edited := "{\"edited\": true}\n"
	if err := os.WriteFile("bun.lock", []byte(edited), 0o600); err != nil {
		t.Fatal(err)
	}
	terraformScaffoldRender(t, TerraformName, "demo-infra")

	got, err := os.ReadFile("bun.lock")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != edited {
		t.Errorf("bun.lock = %q, want the edited %q", got, edited)
	}
}

// The name reaches package.json and bun.lock as a JSON string, so a quote or
// backslash cannot end it.
func TestTerraformScaffoldQuotesName(t *testing.T) {
	t.Chdir(t.TempDir())
	const name = `a"b\c`
	terraformScaffoldRender(t, TerraformName, name)

	var pkg terraformScaffoldPackage
	raw, err := os.ReadFile("package.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &pkg); err != nil {
		t.Fatalf("package.json is not JSON: %v\n%s", err, raw)
	}
	if pkg.Name != name {
		t.Errorf("package.json name = %q, want %q", pkg.Name, name)
	}
	if got := terraformScaffoldReadLock(t).Workspaces[""].Name; got != name {
		t.Errorf("bun.lock workspace name = %q, want %q", got, name)
	}
}

// A .bcignore entry opts a scaffolded file out like any other.
func TestTerraformScaffoldHonorsBcignore(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile(".bcignore", []byte("bun.lock\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	terraformScaffoldRender(t, TerraformName, "demo-infra")

	// The pair stays together, so ignoring one leaves the other unwritten too.
	for _, name := range []string{"bun.lock", "package.json"} {
		if _, err := os.Stat(name); err == nil {
			t.Errorf("%s was written although .bcignore lists bun.lock", name)
		}
	}
	if _, err := os.Stat("tsconfig.json"); err != nil {
		t.Errorf("tsconfig.json was not written: %v", err)
	}
}
