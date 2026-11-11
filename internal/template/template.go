// Package template provides project template application.
package template

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// skipDirs contains directory names that should never be copied from templates.
var skipDirs = map[string]bool{
	".git": true,
	".jj":  true,
}

// Apply copies the contents of srcDir into dstDir recursively.
// It allows a symlinked root, but skips .git/.jj and rejects linked/special entries.
// dstDir must already exist.
func Apply(srcDir, dstDir string) error {
	srcDir, err := filepath.EvalSymlinks(srcDir)
	if err != nil {
		return fmt.Errorf("source directory: %w", err)
	}
	srcInfo, err := os.Stat(srcDir)
	if err != nil {
		return fmt.Errorf("source directory: %w", err)
	}
	if !srcInfo.IsDir() {
		return fmt.Errorf("source is not a directory: %s", srcDir)
	}

	return filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return fmt.Errorf("computing relative path: %w", err)
		}

		if rel == "." {
			return nil
		}

		if d.IsDir() && skipDirs[d.Name()] {
			return filepath.SkipDir
		}

		targetPath := filepath.Join(dstDir, rel)

		if d.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("unsupported template file type: %s", path)
		}
		return copyFile(path, targetPath)
	})
}

// copyFile copies a single file from src to dst, preserving permissions.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening source: %w", err)
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("stat source: %w", err)
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("creating parent directory: %w", err)
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("creating destination: %w", err)
	}
	_, copyErr := io.Copy(dstFile, srcFile)
	return errors.Join(copyErr, dstFile.Close())
}
