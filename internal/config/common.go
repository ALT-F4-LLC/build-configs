package config

import (
	"fmt"
	"os"
	"slices"

	"github.com/ALT-F4-LLC/build-configs/internal/templates"
)

// vorpalShell is the CI step shell every template renders from
// templates/common/vorpal.
const vorpalShell = ".github/vorpal-shell.sh"

// vorpalCommonFiles lists the files templates/common/vorpal renders for every
// template type. Each type's render map lists them here and nowhere else, so a
// file is never written by two template sets.
func vorpalCommonFiles() []string {
	return []string{".envrc", vorpalShell}
}

// makeVorpalShellExecutable sets the CI step shell to 0755, since
// templates.WriteFiles writes every file 0644. A path listed in .bcignore is
// left untouched, including any copy the repository keeps.
func makeVorpalShellExecutable() error {
	ignored, err := templates.GetIgnoredFiles(".")
	if err != nil {
		return fmt.Errorf("failed to get ignored files: %w", err)
	}
	if slices.Contains(ignored, vorpalShell) {
		return nil
	}
	return os.Chmod(vorpalShell, 0o755) //nolint:gosec // CI runs this script directly, so it must be executable
}
