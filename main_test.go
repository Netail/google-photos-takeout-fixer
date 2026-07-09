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

	if _, err := processDirectory(inputDir, outputDir, false); err != nil {
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

	if _, err := processDirectory(inputDir, outputDir, false); err != nil {
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

	if _, err := processDirectory(inputDir, outputDir, false); err != nil {
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

	if _, err := processDirectory(inputDir, outputDir, false); err != nil {
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

	if _, err := processDirectory(inputDir, outputDir, false); err != nil {
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

	if _, err := processDirectory(inputDir, outputDir, true); err != nil {
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

	if _, err := processDirectory(inputDir, outputDir, false); err != nil {
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

	if _, err := processDirectory(inputDir, outputDir, false); err != nil {
		t.Fatalf("processDirectory: %v", err)
	}

	dest := filepath.Join(outputDir, "photo.jpg")
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("output file missing: %v", err)
	}
}

// TestNumberedDuplicate verifies that Google Takeout's "(N)" counter pattern is handled:
// the media file is "photo(1).jpg" but the metadata file is "photo.jpg.supplemental-metadata(1).json".
func TestNumberedDuplicate(t *testing.T) {
	inputDir := t.TempDir()
	outputDir := t.TempDir()

	writeFile(t, filepath.Join(inputDir, "photo(1).jpg"), "img-data")
	writeFile(t, filepath.Join(inputDir, "photo.jpg.supplemental-metadata(1).json"), metadata("photo(1).jpg", "1108563029"))

	if _, err := processDirectory(inputDir, outputDir, false); err != nil {
		t.Fatalf("processDirectory: %v", err)
	}

	dest := filepath.Join(outputDir, "photo(1).jpg")
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("output file missing: %v", err)
	}

	want := time.Unix(1108563029, 0)
	if !info.ModTime().Equal(want) {
		t.Errorf("ModTime = %v, want %v", info.ModTime(), want)
	}
}

// TestNumberedInFilename verifies that a "(N)" counter that stays inside the media filename
// is handled: "photo (1).png" with "photo (1).png.supplemental-metadata.json".
func TestNumberedInFilename(t *testing.T) {
	inputDir := t.TempDir()
	outputDir := t.TempDir()

	writeFile(t, filepath.Join(inputDir, "photo (1).png"), "img-data")
	writeFile(t, filepath.Join(inputDir, "photo (1).png"+metadataSuffix), metadata("photo (1).png", "1108563029"))

	if _, err := processDirectory(inputDir, outputDir, false); err != nil {
		t.Fatalf("processDirectory: %v", err)
	}

	dest := filepath.Join(outputDir, "photo (1).png")
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("output file missing: %v", err)
	}

	want := time.Unix(1108563029, 0)
	if !info.ModTime().Equal(want) {
		t.Errorf("ModTime = %v, want %v", info.ModTime(), want)
	}
}

// TestPlainJsonMetadata verifies the plain "name.json" metadata format where the media file
// has a trailing underscore (e.g. "name_.jpg" with metadata in "name.json").
func TestPlainJsonMetadata(t *testing.T) {
	inputDir := t.TempDir()
	outputDir := t.TempDir()

	writeFile(t, filepath.Join(inputDir, "photo_.jpg"), "img-data")
	writeFile(t, filepath.Join(inputDir, "photo.json"), metadata("photo_.jpg", "1108563029"))

	if _, err := processDirectory(inputDir, outputDir, false); err != nil {
		t.Fatalf("processDirectory: %v", err)
	}

	dest := filepath.Join(outputDir, "photo_.jpg")
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("output file missing: %v", err)
	}

	want := time.Unix(1108563029, 0)
	if !info.ModTime().Equal(want) {
		t.Errorf("ModTime = %v, want %v", info.ModTime(), want)
	}
}

// TestTruncatedExtensionMetadata verifies that a metadata file whose name is the media filename
// with its extension partially truncated is handled (e.g. "name.mp4" → "name.mp.json").
func TestTruncatedExtensionMetadata(t *testing.T) {
	inputDir := t.TempDir()
	outputDir := t.TempDir()

	writeFile(t, filepath.Join(inputDir, "Screen_Recording_20201010-151432_Snapchat_1.mp4"), "video-data")
	writeFile(t, filepath.Join(inputDir, "Screen_Recording_20201010-151432_Snapchat_1.mp.json"),
		metadata("Screen_Recording_20201010-151432_Snapchat_1.mp4", "1602334472"))

	if _, err := processDirectory(inputDir, outputDir, false); err != nil {
		t.Fatalf("processDirectory: %v", err)
	}

	dest := filepath.Join(outputDir, "Screen_Recording_20201010-151432_Snapchat_1.mp4")
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("output file missing: %v", err)
	}

	want := time.Unix(1602334472, 0)
	if !info.ModTime().Equal(want) {
		t.Errorf("ModTime = %v, want %v", info.ModTime(), want)
	}
}

// TestMotionPhotoVideo verifies that for motion_ files the video (.mp4) is preferred over the photo.
func TestMotionPhotoVideo(t *testing.T) {
	inputDir := t.TempDir()
	outputDir := t.TempDir()

	writeFile(t, filepath.Join(inputDir, "motion_photo.jpg"), "photo-data")
	writeFile(t, filepath.Join(inputDir, "motion_photo.mp4"), "video-data")
	writeFile(t, filepath.Join(inputDir, "motion_photo.jpg"+metadataSuffix), metadata("motion_photo.jpg", "1108563029"))

	if _, err := processDirectory(inputDir, outputDir, false); err != nil {
		t.Fatalf("processDirectory: %v", err)
	}

	// The video should be in the output under its own name.
	dest := filepath.Join(outputDir, "motion_photo.mp4")
	content, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("output video missing: %v", err)
	}
	if string(content) != "video-data" {
		t.Errorf("content = %q, want %q", string(content), "video-data")
	}

	// The photo should not appear in the output.
	if _, err := os.Stat(filepath.Join(outputDir, "motion_photo.jpg")); err == nil {
		t.Error("motion_photo.jpg should not exist in output")
	}
}

// TestTruncatedMotionPhotoVideo verifies that a motion_ photo whose base was truncated on disk
// still finds its video companion. Title "motion_photo_x.jpg", photo on disk "motion_photo_.jpg",
// video on disk "motion_photo_.mp4".
func TestTruncatedMotionPhotoVideo(t *testing.T) {
	inputDir := t.TempDir()
	outputDir := t.TempDir()

	writeFile(t, filepath.Join(inputDir, "motion_photo_.jpg"), "photo-data")
	writeFile(t, filepath.Join(inputDir, "motion_photo_.mp4"), "video-data")
	writeFile(t, filepath.Join(inputDir, "motion_photo_.jpg"+metadataSuffix), metadata("motion_photo_x.jpg", "1108563029"))

	if _, err := processDirectory(inputDir, outputDir, false); err != nil {
		t.Fatalf("processDirectory: %v", err)
	}

	dest := filepath.Join(outputDir, "motion_photo_.mp4")
	content, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("output video missing: %v", err)
	}
	if string(content) != "video-data" {
		t.Errorf("content = %q, want %q", string(content), "video-data")
	}
}

// TestTruncatedMediaFilename verifies that when meta.Title references a filename longer than
// what is on disk (due to filesystem limits), the truncated file is found and used.
// Example: title "photo_o.jpg" but actual file is "photo_.jpg" (one char truncated).
func TestTruncatedMediaFilename(t *testing.T) {
	inputDir := t.TempDir()
	outputDir := t.TempDir()

	writeFile(t, filepath.Join(inputDir, "photo_.jpg"), "img-data")
	writeFile(t, filepath.Join(inputDir, "photo_.jpg"+metadataSuffix), metadata("photo_o.jpg", "1108563029"))

	if _, err := processDirectory(inputDir, outputDir, false); err != nil {
		t.Fatalf("processDirectory: %v", err)
	}

	dest := filepath.Join(outputDir, "photo_.jpg")
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("output file missing: %v", err)
	}

	want := time.Unix(1108563029, 0)
	if !info.ModTime().Equal(want) {
		t.Errorf("ModTime = %v, want %v", info.ModTime(), want)
	}
}

// TestTruncatedEditedFilename verifies that an edited variant whose base was truncated on disk
// is still preferred over the original. Example: title "photo_o.jpg", edited file "photo_-edited.jpg"
// (base truncated from "photo_o" to "photo_").
func TestTruncatedEditedFilename(t *testing.T) {
	inputDir := t.TempDir()
	outputDir := t.TempDir()

	writeFile(t, filepath.Join(inputDir, "photo_o.jpg"), "original")
	writeFile(t, filepath.Join(inputDir, "photo_-edited.jpg"), "edited")
	writeFile(t, filepath.Join(inputDir, "photo_o.jpg"+metadataSuffix), metadata("photo_o.jpg", "1500677462"))

	if _, err := processDirectory(inputDir, outputDir, false); err != nil {
		t.Fatalf("processDirectory: %v", err)
	}

	dest := filepath.Join(outputDir, "photo_o.jpg")
	content, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("output file missing: %v", err)
	}
	if string(content) != "edited" {
		t.Errorf("content = %q, want %q", string(content), "edited")
	}
}

func TestDuplicates(t *testing.T) {
	inputDir := t.TempDir()
	outputDir := t.TempDir()

	writeFile(t, filepath.Join(inputDir, "photo.jpg"), "edited")
	writeFile(t, filepath.Join(inputDir, "photo(1).jpg"), "original")
	writeFile(t, filepath.Join(inputDir, "photo.jpg"+metadataSuffix), metadata("photo.jpg", "1500677462"))
	writeFile(t, filepath.Join(inputDir, "photo.jpg.supplemental-metadata(1).json"), metadata("photo.jpg", "1500677462"))

	if _, err := processDirectory(inputDir, outputDir, false); err != nil {
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

	dest1 := filepath.Join(outputDir, "photo(1).jpg")
	content1, err := os.ReadFile(dest1)
	if err != nil {
		t.Fatalf("output file missing: %v", err)
	}
	if string(content1) != "original" {
		t.Errorf("content = %q, want %q", string(content1), "original")
	}
}
