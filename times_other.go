//go:build !windows

package main

import (
	"os"
	"time"
)

func setFileTimes(path string, t time.Time) error {
	return os.Chtimes(path, t, t)
}
