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
	PhotoTakenTime struct {
		Timestamp string `json:"timestamp"`
	} `json:"photoTakenTime"`
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

	if err := processDirectory(*input, *output, *flat); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func processDirectory(inputDir, outputDir string, flat bool) error {
	return filepath.WalkDir(inputDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, metadataSuffix) {
			return nil
		}
		return processMetadataFile(path, inputDir, outputDir, flat)
	})
}

func processMetadataFile(metaPath, inputDir, outputDir string, flat bool) error {
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return fmt.Errorf("reading metadata %s: %w", metaPath, err)
	}

	var meta Metadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return fmt.Errorf("parsing metadata %s: %w", metaPath, err)
	}

	ts, err := strconv.ParseInt(meta.PhotoTakenTime.Timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("parsing timestamp in %s: %w", metaPath, err)
	}
	photoTime := time.Unix(ts, 0)

	dir := filepath.Dir(metaPath)
	mediaName := strings.TrimSuffix(filepath.Base(metaPath), metadataSuffix)

	ext := filepath.Ext(mediaName)
	base := strings.TrimSuffix(mediaName, ext)

	// Prefer edited variants over the original
	sourceFile := ""
	for _, suffix := range editedSuffixes {
		candidate := filepath.Join(dir, base+suffix+ext)
		if _, statErr := os.Stat(candidate); statErr == nil {
			sourceFile = candidate
			break
		}
	}

	if sourceFile == "" {
		original := filepath.Join(dir, mediaName)
		if _, statErr := os.Stat(original); statErr != nil {
			fmt.Printf("Warning: no media file found for %s, skipping\n", metaPath)
			return nil
		}
		sourceFile = original
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
	if err := copyFile(sourceFile, destPath); err != nil {
		return fmt.Errorf("copying %s to %s: %w", sourceFile, destPath, err)
	}

	if err := setFileTimes(destPath, photoTime); err != nil {
		return fmt.Errorf("setting timestamps on %s: %w", destPath, err)
	}

	return nil
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
