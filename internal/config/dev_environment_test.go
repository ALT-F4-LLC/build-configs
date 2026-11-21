package config

import "testing"

func TestConfigDevEnvironmentUsesName(t *testing.T) {
	c := Config{Name: "demo"}

	if got, want := c.DevEnvironment(), "demo-dev"; got != want {
		t.Errorf("DevEnvironment() = %q, want %q", got, want)
	}
	if got, want := c.ShellQuotedDevEnvironment(), `'demo-dev'`; got != want {
		t.Errorf("ShellQuotedDevEnvironment() = %q, want %q", got, want)
	}
}

func TestConfigShellQuotedDevEnvironmentEscapesSingleQuotes(t *testing.T) {
	c := Config{Name: "a'$(id)'b"}

	if got, want := c.ShellQuotedDevEnvironment(), `'a'\''$(id)'\''b-dev'`; got != want {
		t.Errorf("ShellQuotedDevEnvironment() = %q, want %q", got, want)
	}
}
