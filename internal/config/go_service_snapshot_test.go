package config

import (
	"os"
	"path/filepath"
	"testing"
)

// The go-service snapshots pin every rendered file: the defaults, and the
// shape of quirk-backend-discord-bot (two clients, an image pushed from main,
// and the default private module). They reuse the -update flag and the
// snapshot helpers from terraform_module_snapshot_test.go.
func TestGoServiceSnapshot(t *testing.T) {
	for _, tc := range []struct {
		name   string
		golden string
		config func() GoServiceConfig
	}{
		{
			name:   "default",
			golden: "go-service-default.golden",
			config: func() GoServiceConfig {
				return NewGoServiceConfig(Config{Name: "example", Template: GoServiceName})
			},
		},
		{
			name:   "quirk-backend-discord-bot",
			golden: "go-service-bot.golden",
			config: goServiceBotConfig,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			golden, err := filepath.Abs(filepath.Join("testdata", tc.golden))
			if err != nil {
				t.Fatalf("resolve golden path: %v", err)
			}

			root := t.TempDir()
			out := filepath.Join(root, "out")
			if err := os.Mkdir(out, 0o750); err != nil {
				t.Fatalf("create render dir: %v", err)
			}
			t.Chdir(out)

			if err := tc.config().Render(); err != nil {
				t.Fatalf("Render: %v", err)
			}

			terraformModuleCompareGolden(t, golden, terraformModuleSnapshot(t, root))
		})
	}
}
