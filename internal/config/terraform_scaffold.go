package config

import (
	"errors"
	"fmt"
	"os"
	"path"
	"slices"

	"github.com/ALT-F4-LLC/build-configs/internal/templates"
)

// terraformBunProjectFiles are written together or not at all: bun install
// --frozen-lockfile, which Vorpal runs, fails when the lock does not match the
// manifest, so a template lock beside a manifest the developer wrote, or the
// reverse, is worse than neither.
var terraformBunProjectFiles = []string{"package.json", "bun.lock"}

// terraformForeignLockFiles are other package managers' lockfiles. Bun reads
// one of them when no bun.lock exists, and prefers a text bun.lock over it.
var terraformForeignLockFiles = []string{"bun.lockb", "package-lock.json", "yarn.lock", "pnpm-lock.yaml"}

// terraformTSConfig is created on its own, since tsconfig.json is optional for
// the build and independent of the manifest.
const terraformTSConfig = "tsconfig.json"

// addTerraformScaffold adds the Bun project files the Vorpal config program
// builds from to files. Renovate, bun and the developer rewrite them after the
// first run, so a file that exists, even empty or as a dangling symlink, is
// never replaced. package.json and bun.lock are added only when neither exists,
// neither is in .bcignore and no other package manager's lockfile exists.
func addTerraformScaffold(files map[string]string, data any) error {
	ignored, err := templates.GetIgnoredFiles(".")
	if err != nil {
		return fmt.Errorf("failed to get ignored files: %w", err)
	}

	pair := true
	for _, name := range terraformBunProjectFiles {
		if slices.Contains(ignored, path.Clean(name)) {
			pair = false
		}
	}
	for _, name := range append(slices.Clone(terraformBunProjectFiles), terraformForeignLockFiles...) {
		exists, err := terraformScaffoldExists(name)
		if err != nil {
			return err
		}
		if exists {
			pair = false
		}
	}

	names := []string{}
	if pair {
		names = append(names, terraformBunProjectFiles...)
	}
	exists, err := terraformScaffoldExists(terraformTSConfig)
	if err != nil {
		return err
	}
	if !exists {
		names = append(names, terraformTSConfig)
	}

	for _, name := range names {
		out, err := templates.RenderTemplate(templates.TerraformCommonTemplates, name, data)
		if err != nil {
			return err
		}
		files["./"+name] = out
	}

	return nil
}

// terraformScaffoldExists reports whether name exists in the working
// directory, without following a symlink.
func terraformScaffoldExists(name string) (bool, error) {
	if _, err := os.Lstat(name); err == nil {
		return true, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("checking %s: %w", name, err)
	}
	return false, nil
}
