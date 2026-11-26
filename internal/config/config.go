package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Name       string                 `json:"name" yaml:"name"`
	Template   string                 `json:"template" yaml:"template"`
	Visibility string                 `json:"visibility,omitempty" yaml:"visibility,omitempty"`
	Parameters map[string]interface{} `json:"parameters" yaml:"parameters"`
}

// Repository visibility values for the top-level visibility key. An unset key
// means private.
const (
	VisibilityPrivate = "private"
	VisibilityPublic  = "public"
)

const (
	// PrivateRunner is the runner group private repositories run on. It carries
	// Vorpal already.
	PrivateRunner = "altf4llc"
	// PublicRunner is the GitHub-hosted runner public repositories run on.
	PublicRunner = "ubuntu-latest"
	// setupVorpalAction installs and starts Vorpal on a runner that lacks it.
	setupVorpalAction = "ALT-F4-LLC/setup-vorpal-action@0.2.1"
	// hostedVorpalVersion is the Vorpal release the setup action installs. The
	// templates are written and tested against it.
	hostedVorpalVersion = "0.6.0"
	// hostedVorpalRegistryBucket is the S3 registry that serves the tool
	// aliases the development environment fetches. The local registry a
	// fresh runner starts has none, so every ALT-F4-LLC workflow on a hosted
	// runner selects this bucket.
	hostedVorpalRegistryBucket = "altf4llc-vorpal-registry"
	// configureAWSAction trades the job's GitHub OIDC token for the
	// short-lived AWS credentials the S3 registry needs, so no AWS key is
	// stored in the repository.
	configureAWSAction = "aws-actions/configure-aws-credentials@v6"
	// hostedVorpalRegion is the region of the S3 registry, which every
	// ALT-F4-LLC role session uses.
	hostedVorpalRegion = "us-west-2"
	// awsAccountID owns the registry bucket and the altf4llc-gha-* roles.
	awsAccountID = "677459762413"
)

// Runner returns the CI runner the workflows use: the altf4llc runner group
// for a private repository and a GitHub-hosted runner for a public one.
func (c Config) Runner() string {
	if c.Visibility == VisibilityPublic {
		return PublicRunner
	}
	return PrivateRunner
}

// NeedsVorpalSetup reports whether the workflows must install Vorpal in every
// job, which every runner but the altf4llc group requires.
func (c Config) NeedsVorpalSetup() bool {
	return c.Runner() != PrivateRunner
}

// VorpalRoleARN returns the IAM role a job on a hosted runner assumes through
// GitHub OIDC to reach the Vorpal registry bucket. The role follows the
// altf4llc-gha-<repository> convention and lives in terraform-aws-gha-iam.
func (c Config) VorpalRoleARN() string {
	return fmt.Sprintf("arn:aws:iam::%s:role/altf4llc-gha-%s-vorpal", awsAccountID, c.Name)
}

// VorpalSetupStep returns the workflow steps that install Vorpal, indented by
// indent spaces. Templates call it inside an if on NeedsVorpalSetup, so a
// render on the altf4llc runner stays unchanged. Every job that calls it needs
// `id-token: write`, which the templates add under the same condition.
func (c Config) VorpalSetupStep(indent int) string {
	return vorpalSetupStep(indent, c.VorpalRoleARN())
}

// vorpalSetupStep assumes the registry role first. The credentials step
// exports AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_SESSION_TOKEN and
// AWS_DEFAULT_REGION to the job, and the setup action passes them to the
// Vorpal services it starts, so the setup step sets no env of its own. The
// services hold the credentials for the job's lifetime, which the role
// session (one hour unless the role says otherwise) caps.
func vorpalSetupStep(indent int, roleARN string) string {
	pad := strings.Repeat(" ", indent)
	return pad + "- uses: " + configureAWSAction + "\n" +
		pad + "  with:\n" +
		pad + "    audience: sts.amazonaws.com\n" +
		pad + "    aws-region: " + hostedVorpalRegion + "\n" +
		pad + "    role-to-assume: " + roleARN + "\n" +
		pad + "- uses: " + setupVorpalAction + "\n" +
		pad + "  with:\n" +
		pad + "    registry-backend: s3\n" +
		pad + "    registry-backend-s3-bucket: " + hostedVorpalRegistryBucket + "\n" +
		pad + "    version: " + hostedVorpalVersion
}

// validateVisibility rejects an unknown visibility. The key is top-level only:
// a parameters.visibility would be promoted onto the embedded Config by the
// parameter unmarshal and silently win, so it is an error too.
func (c Config) validateVisibility() error {
	// encoding/json matches field names case-insensitively, so every
	// spelling of the key reaches the embedded Config.
	for key := range c.Parameters {
		if strings.EqualFold(key, "visibility") {
			return errors.New("visibility is a top-level key beside name and template, not a parameter")
		}
	}
	switch c.Visibility {
	case "", VisibilityPrivate:
		return nil
	case VisibilityPublic:
		// The plan job uploads terraform.tfplan, which holds variable and
		// state values in cleartext, and a public repository's artifacts
		// download for any signed-in GitHub user.
		if c.Template == TerraformName {
			return fmt.Errorf("the %s template does not support visibility %q: its plan artifact holds cleartext variable and state values", TerraformName, VisibilityPublic)
		}
		return nil
	}
	return fmt.Errorf("unsupported visibility %q: use %q or %q", c.Visibility, VisibilityPrivate, VisibilityPublic)
}

// DevEnvironment returns the Vorpal development environment name, <name>-dev.
func (c Config) DevEnvironment() string {
	return c.Name + "-dev"
}

// JSString returns s as a JavaScript string literal. A JSON string is one.
func (c Config) JSString(s string) (string, error) {
	b, err := json.Marshal(s)
	return string(b), err
}

// ShellQuotedDevEnvironment returns DevEnvironment as one POSIX single-quoted
// shell word, so generated scripts pass it to vorpal verbatim whatever
// characters the name holds.
func (c Config) ShellQuotedDevEnvironment() string {
	return "'" + strings.ReplaceAll(c.DevEnvironment(), "'", `'\''`) + "'"
}

var (
	ErrUnsupportedFileType = errors.New("unsupported config file type")
	ErrUnsupportedTemplate = errors.New("unsupported template selected")
)

func New(configPath string) (Config, error) {
	b, err := os.ReadFile("./" + configPath) //nolint:gosec // config path is chosen by the operator on the command line
	if err != nil {
		return Config{}, err
	}

	// Detect config file type and load it depending on extension
	ext := path.Ext(configPath)
	switch ext {
	case ".json":
		return loadConfigJSON(b)
	case ".yaml", ".yml":
		return loadConfigYaml(b)
	}

	return Config{}, ErrUnsupportedFileType
}

func (c Config) GetTemplater() (Templater, error) {
	if err := c.validateVisibility(); err != nil {
		return nil, err
	}

	switch c.Template {
	case "go-cobra-cli":
		if Debug {
			fmt.Println("loading go-cobra-cli templater")
		}
		tpl := NewGoCobraCliConfig(c)

		// Convert the parameters (map type) to JSON
		b, err := json.Marshal(c.Parameters)
		if err != nil {
			return tpl, err
		}

		// Then convert them back into the type for the templater selected
		if err := json.Unmarshal(b, &tpl); err != nil {
			return tpl, err
		}
		return tpl, nil

	case "go-lambda":
		if Debug {
			fmt.Println("loading go-lambda templater")
		}
		tpl := NewGoLambdaConfig(c)

		// Convert the parameters (map type) to JSON
		b, err := json.Marshal(c.Parameters)
		if err != nil {
			return tpl, err
		}

		// Then convert them back into the type for the templater selected
		if err := json.Unmarshal(b, &tpl); err != nil {
			return tpl, err
		}
		return tpl, nil

	case "go-service":
		if Debug {
			fmt.Println("loading go-service templater")
		}
		tpl := NewGoServiceConfig(c)

		// Convert the parameters (map type) to JSON
		b, err := json.Marshal(c.Parameters)
		if err != nil {
			return tpl, err
		}

		// Then convert them back into the type for the templater selected
		if err := json.Unmarshal(b, &tpl); err != nil {
			return tpl, err
		}
		return tpl, nil

	case "go-library":
		if Debug {
			fmt.Println("loading go-library templater")
		}
		tpl := NewGoLibraryConfig(c)

		// Convert the parameters (map type) to JSON
		b, err := json.Marshal(c.Parameters)
		if err != nil {
			return tpl, err
		}

		// Then convert them back into the type for the templater selected
		if err := json.Unmarshal(b, &tpl); err != nil {
			return tpl, err
		}
		return tpl, nil

	case "terraform":
		if Debug {
			fmt.Println("loading terraform templater")
		}
		tpl := NewTerraformConfig(c)

		// Convert the parameters (map type) to JSON
		b, err := json.Marshal(c.Parameters)
		if err != nil {
			return tpl, err
		}

		// Then convert them back into the type for the templater selected
		if err := json.Unmarshal(b, &tpl); err != nil {
			return tpl, err
		}
		return tpl, nil

	case TypeScriptFrontendName:
		if Debug {
			fmt.Println("loading typescript-frontend templater")
		}
		tpl := NewTypeScriptFrontendConfig(c)

		// Convert the parameters (map type) to JSON
		b, err := json.Marshal(c.Parameters)
		if err != nil {
			return tpl, err
		}

		// Then convert them back into the type for the templater selected
		if err := json.Unmarshal(b, &tpl); err != nil {
			return tpl, err
		}
		return tpl, nil

	case "terraform-module":
		if Debug {
			fmt.Println("loading terraform module templater")
		}
		tpl := NewTerraformModuleConfig(c)

		// Convert the parameters (map type) to JSON
		b, err := json.Marshal(c.Parameters)
		if err != nil {
			return tpl, err
		}

		// Then convert them back into the type for the templater selected
		if err := json.Unmarshal(b, &tpl); err != nil {
			return tpl, err
		}
		return tpl, nil
	}

	return nil, ErrUnsupportedTemplate
}

func loadConfigJSON(b []byte) (Config, error) {
	var cfg Config

	if err := json.Unmarshal(b, &cfg); err != nil {
		return cfg, err
	}

	Cfg = cfg

	return cfg, nil
}

func loadConfigYaml(b []byte) (Config, error) {
	var cfg Config

	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return cfg, err
	}

	Cfg = cfg

	return cfg, nil
}
