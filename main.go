package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Metadata struct {
	Title          string `json:"title"`
	PhotoTakenTime struct {
		Timestamp string `json:"timestamp"`
	} `json:"photoTakenTime"`
}

type Stats struct {
	JSONFiles    int      // valid media JSON files found
	OutputFiles  int      // unique destination files written
	MissingMedia []string // JSON files whose referenced media file was not found
}

const metadataSuffix = ".supplemental-metadata.json"

var editedSuffixes = []string{"-edited", "-bewerkt"}

func main() {
	input := flag.String("i", "", "Input directory")
	output := flag.String("o", "", "Output directory")
	flat := flag.Bool("flat", false, "Dump all media into the output directory without preserving directory structure")

	flag.Parse()

	if len(*input) == 0 {
		fmt.Println("An input is required.")
		os.Exit(1)
	}

	if len(*output) == 0 {
		fmt.Println("An output is required.")
		os.Exit(1)
	}

	stats, err := processDirectory(*input, *output, *flat)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Done: %d / %d files written to output\n", stats.OutputFiles, stats.JSONFiles)
	if len(stats.MissingMedia) > 0 {
		fmt.Printf("\nNo media file found for %d JSON file(s):\n", len(stats.MissingMedia))
		for _, p := range stats.MissingMedia {
			fmt.Printf("  %s\n", p)
		}
	}
}

func processDirectory(inputDir, outputDir string, flat bool) (Stats, error) {
	stats := Stats{}
	err := filepath.WalkDir(inputDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(filepath.Base(path), ".json") {
			return nil
		}
		return processMetadataFile(path, inputDir, outputDir, flat, &stats)
	})
	return stats, err
}

func processMetadataFile(metaPath, inputDir, outputDir string, flat bool, stats *Stats) error {
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return fmt.Errorf("reading metadata %s: %w", metaPath, err)
	}

	var meta Metadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return fmt.Errorf("parsing metadata %s: %w", metaPath, err)
	}

	ts, err := strconv.ParseInt(meta.PhotoTakenTime.Timestamp, 10, 64)
	if err != nil || meta.Title == "" {
		// Not a media metadata file (e.g. album metadata)
		fmt.Fprintf(os.Stderr, "warning: metadata %s is skipped", metaPath)
		return nil
	}

	stats.JSONFiles++

	dir := filepath.Dir(metaPath)

	photoTime := time.Unix(ts, 0)
	mediaName := meta.Title
	ext := filepath.Ext(mediaName)
	base := strings.TrimSuffix(mediaName, ext)

	// If the metadata filename carries a "(N)" counter (e.g. "photo.jpg.supplemental-metadata(1).json")
	// but the title doesn't already include it, inject the counter into the media filename.
	metaBase := strings.TrimSuffix(filepath.Base(metaPath), ".json")
	if idx := strings.LastIndex(metaBase, "("); idx != -1 {
		if end := strings.Index(metaBase[idx:], ")"); end != -1 {
			counter := metaBase[idx : idx+end+1]
			if !strings.Contains(mediaName, counter) {
				base = base + counter
				mediaName = base + ext
			}
		}
	}

	// Prefer edited variants over the original
	sourceFile := ""
	for _, suffix := range editedSuffixes {
		if p, _ := findWithTruncation(dir, base, suffix+ext); p != "" {
			sourceFile = p
			break
		}
	}

	if sourceFile == "" {
		// The title in the metadata may be longer than the actual filename on disk
		// due to filesystem character limits. Try exact match first, then shorter stems.
		if p, n := findWithTruncation(dir, base, ext); p != "" {
			sourceFile = p
			mediaName = n
		} else {
			stats.MissingMedia = append(stats.MissingMedia, metaPath)
			return nil
		}
	}

	// For motion photos (motion_*), prefer the video version over the photo if it exists.
	// Use the current mediaName base (not the original title base) in case it was updated
	// by the truncation lookup above.
	currentBase := strings.TrimSuffix(mediaName, filepath.Ext(mediaName))
	if strings.HasPrefix(currentBase, "motion_") {
		for _, videoExt := range []string{".mp4", ".mov"} {
			if p, n := findWithTruncation(dir, currentBase, videoExt); p != "" {
				sourceFile = p
				mediaName = n
				break
			}
		}
	}

	outDir := outputDir
	if !flat {
		relDir, err := filepath.Rel(inputDir, dir)
		if err != nil {
			return fmt.Errorf("computing relative path for %s: %w", dir, err)
		}
		outDir = filepath.Join(outputDir, relDir)
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("creating output dir %s: %w", outDir, err)
	}

	destPath := filepath.Join(outDir, mediaName)
	overwriting := false
	if _, err := os.Stat(destPath); err == nil {
		fmt.Fprintf(os.Stderr, "warning: overwriting %s (already written this run)\n", destPath)
		overwriting = true
	}
	if err := copyFile(sourceFile, destPath); err != nil {
		return fmt.Errorf("copying %s to %s: %w", sourceFile, destPath, err)
	}

	if err := setFileTimes(destPath, photoTime); err != nil {
		return fmt.Errorf("setting timestamps on %s: %w", destPath, err)
	}

	if !overwriting {
		stats.OutputFiles++
	}
	return nil
}

// findWithTruncation looks for a file in dir whose name is base[:l]+suffix for some l,
// starting at the full len(base) and decreasing by one until at most len(metadataSuffix)
// characters have been removed. Returns the full path and matched filename, or both empty
// strings if no file is found.
func findWithTruncation(dir, base, suffix string) (path, name string) {
	for l := len(base); l >= len(base)-len(metadataSuffix) && l >= 1; l-- {
		n := base[:l] + suffix
		p := filepath.Join(dir, n)
		if _, err := os.Stat(p); err == nil {
			return p, n
		}
	}
	return "", ""
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
