package provider

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetPlatformSourceFiles(t *testing.T) {
	tempDir := t.TempDir()

	// Initially no files exist
	files := GetPlatformSourceFiles(tempDir, "instagram")
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %v", files)
	}

	// Create public handles file
	pubFile := filepath.Join(tempDir, "instagram.txt")
	if err := os.WriteFile(pubFile, []byte("handle1\n"), 0644); err != nil {
		t.Fatalf("failed to write pubFile: %v", err)
	}

	files = GetPlatformSourceFiles(tempDir, "instagram")
	if len(files) != 1 || files[0] != pubFile {
		t.Errorf("expected [%s], got %v", pubFile, files)
	}

	// Create local handles file
	localFile := filepath.Join(tempDir, "instagram.local.txt")
	if err := os.WriteFile(localFile, []byte("handle2\n"), 0644); err != nil {
		t.Fatalf("failed to write localFile: %v", err)
	}

	files = GetPlatformSourceFiles(tempDir, "instagram")
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d: %v", len(files), files)
	}
	if files[0] != pubFile || files[1] != localFile {
		t.Errorf("expected [%s, %s], got %v", pubFile, localFile, files)
	}

	// Ensure private folder / unsupported formats are ignored
	privateDir := filepath.Join(tempDir, "private")
	_ = os.MkdirAll(privateDir, 0755)
	_ = os.WriteFile(filepath.Join(privateDir, "instagram.txt"), []byte("ignored\n"), 0644)

	files = GetPlatformSourceFiles(tempDir, "instagram")
	if len(files) != 2 {
		t.Errorf("expected private subfolder to be ignored, but got %d files: %v", len(files), files)
	}
}
