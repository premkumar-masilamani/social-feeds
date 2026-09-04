package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnv(t *testing.T) {
	tempDir := t.TempDir()
	envFile := filepath.Join(tempDir, ".env")
	content := `
# Comment line
TEST_KEY_ONE=alpha
TEST_KEY_TWO="beta"
TEST_KEY_THREE='gamma'
`
	if err := os.WriteFile(envFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test .env: %v", err)
	}

	LoadDotEnv(envFile)

	if val := os.Getenv("TEST_KEY_ONE"); val != "alpha" {
		t.Errorf("expected alpha, got %q", val)
	}
	if val := os.Getenv("TEST_KEY_TWO"); val != "beta" {
		t.Errorf("expected beta, got %q", val)
	}
	if val := os.Getenv("TEST_KEY_THREE"); val != "gamma" {
		t.Errorf("expected gamma, got %q", val)
	}
}
