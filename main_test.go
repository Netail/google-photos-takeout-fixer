package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeFile creates a file at path with the given content, creating parent dirs as needed.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func metadata(title, timestamp string) string {
	return `{"title":"` + title + `","photoTakenTime":{"timestamp":"` + timestamp + `"}}`
}

// TestOriginalFile verifies a plain media file is copied with the correct timestamp.
func TestOriginalFile(t *testing.T) {
	inputDir := t.TempDir()
	outputDir := t.TempDir()

	writeFile(t, filepath.Join(inputDir, "photo.jpg"), "img-data")
	writeFile(t, filepath.Join(inputDir, "photo.jpg"+metadataSuffix), metadata("photo.jpg", "1108563029"))

	if err := processDirectory(inputDir, outputDir, false); err != nil {
		t.Fatalf("processDirectory: %v", err)
	}

	dest := filepath.Join(outputDir, "photo.jpg")
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("output file missing: %v", err)
	}

	want := time.Unix(1108563029, 0)
	if !info.ModTime().Equal(want) {
		t.Errorf("ModTime = %v, want %v", info.ModTime(), want)
	}
}

// TestEditedSuffix verifies that a `-edited` variant is preferred and written under the original name.
func TestEditedSuffix(t *testing.T) {
	inputDir := t.TempDir()
	outputDir := t.TempDir()

	writeFile(t, filepath.Join(inputDir, "photo.jpg"), "original")
	writeFile(t, filepath.Join(inputDir, "photo-edited.jpg"), "edited")
	writeFile(t, filepath.Join(inputDir, "photo.jpg"+metadataSuffix), metadata("photo.jpg", "1500677462"))

	if err := processDirectory(inputDir, outputDir, false); err != nil {
		t.Fatalf("processDirectory: %v", err)
	}

	dest := filepath.Join(outputDir, "photo.jpg")
	content, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("output file missing: %v", err)
	}
	if string(content) != "edited" {
		t.Errorf("content = %q, want %q", string(content), "edited")
	}

	// The `-edited` variant must not appear in the output.
	if _, err := os.Stat(filepath.Join(outputDir, "photo-edited.jpg")); err == nil {
		t.Error("photo-edited.jpg should not exist in output")
	}
}

// TestBewerktSuffix verifies that a `-bewerkt` variant is preferred and written under the original name.
func TestBewerktSuffix(t *testing.T) {
	inputDir := t.TempDir()
	outputDir := t.TempDir()

	writeFile(t, filepath.Join(inputDir, "photo.jpg"), "original")
	writeFile(t, filepath.Join(inputDir, "photo-bewerkt.jpg"), "bewerkt")
	writeFile(t, filepath.Join(inputDir, "photo.jpg"+metadataSuffix), metadata("photo.jpg", "1500677462"))

	if err := processDirectory(inputDir, outputDir, false); err != nil {
		t.Fatalf("processDirectory: %v", err)
	}

	dest := filepath.Join(outputDir, "photo.jpg")
	content, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("output file missing: %v", err)
	}
	if string(content) != "bewerkt" {
		t.Errorf("content = %q, want %q", string(content), "bewerkt")
	}
}

// TestDirectoryStructurePreserved verifies nested directories are recreated in the output.
func TestDirectoryStructurePreserved(t *testing.T) {
	inputDir := t.TempDir()
	outputDir := t.TempDir()

	sub := filepath.Join(inputDir, "a", "b", "c")
	writeFile(t, filepath.Join(sub, "photo.jpg"), "img")
	writeFile(t, filepath.Join(sub, "photo.jpg"+metadataSuffix), metadata("photo.jpg", "1108563029"))

	if err := processDirectory(inputDir, outputDir, false); err != nil {
		t.Fatalf("processDirectory: %v", err)
	}

	dest := filepath.Join(outputDir, "a", "b", "c", "photo.jpg")
	if _, err := os.Stat(dest); err != nil {
		t.Errorf("expected output at %s: %v", dest, err)
	}
}

// TestMissingMediaFileSkipped verifies that a metadata file without a corresponding media file is skipped without error.
func TestMissingMediaFileSkipped(t *testing.T) {
	inputDir := t.TempDir()
	outputDir := t.TempDir()

	writeFile(t, filepath.Join(inputDir, "ghost.jpg"+metadataSuffix), metadata("ghost.jpg", "1108563029"))

	if err := processDirectory(inputDir, outputDir, false); err != nil {
		t.Fatalf("processDirectory: %v", err)
	}

	entries, err := os.ReadDir(outputDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("output dir should be empty, got %d entries", len(entries))
	}
}

// TestFlatMode verifies that -flat dumps all media directly into the output directory.
func TestFlatMode(t *testing.T) {
	inputDir := t.TempDir()
	outputDir := t.TempDir()

	sub := filepath.Join(inputDir, "a", "b", "c")
	writeFile(t, filepath.Join(sub, "photo.jpg"), "img")
	writeFile(t, filepath.Join(sub, "photo.jpg"+metadataSuffix), metadata("photo.jpg", "1108563029"))

	if err := processDirectory(inputDir, outputDir, true); err != nil {
		t.Fatalf("processDirectory: %v", err)
	}

	// File must be at the root of the output, not in a subdirectory.
	dest := filepath.Join(outputDir, "photo.jpg")
	if _, err := os.Stat(dest); err != nil {
		t.Errorf("expected output at %s: %v", dest, err)
	}

	// Subdirectory must not have been created.
	if _, err := os.Stat(filepath.Join(outputDir, "a")); err == nil {
		t.Error("subdirectory 'a' should not exist in flat output")
	}
}

// TestTruncatedMetadataSuffix verifies a file whose metadata suffix was truncated by Google Takeout is processed correctly.
func TestTruncatedMetadataSuffix(t *testing.T) {
	inputDir := t.TempDir()
	outputDir := t.TempDir()

	writeFile(t, filepath.Join(inputDir, "photo.jpg"), "img-data")
	// Simulate Google Takeout's 50-char filename truncation: ".supplemental-metadata" is cut to ".supplemental-metad"
	writeFile(t, filepath.Join(inputDir, "photo.jpg.supplemental-metad.json"), metadata("photo.jpg", "1108563029"))

	if err := processDirectory(inputDir, outputDir, false); err != nil {
		t.Fatalf("processDirectory: %v", err)
	}

	dest := filepath.Join(outputDir, "photo.jpg")
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("output file missing: %v", err)
	}

	want := time.Unix(1108563029, 0)
	if !info.ModTime().Equal(want) {
		t.Errorf("ModTime = %v, want %v", info.ModTime(), want)
	}
}

// TestHeavilyTruncatedMetadataSuffix verifies an even shorter truncation (e.g. ".supplemental-m") still matches.
func TestHeavilyTruncatedMetadataSuffix(t *testing.T) {
	inputDir := t.TempDir()
	outputDir := t.TempDir()

	writeFile(t, filepath.Join(inputDir, "photo.jpg"), "img-data")
	writeFile(t, filepath.Join(inputDir, "photo.jpg.suppl.json"), metadata("photo.jpg", "1108563029"))

	if err := processDirectory(inputDir, outputDir, false); err != nil {
		t.Fatalf("processDirectory: %v", err)
	}

	dest := filepath.Join(outputDir, "photo.jpg")
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("output file missing: %v", err)
	}
}
