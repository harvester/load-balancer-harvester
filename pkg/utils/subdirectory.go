package utils

import (
	"os"
	"path/filepath"
)

// GetSubdirectories returns the subdirectories of the given root directory.
func GetSubdirectories(root string) ([]string, error) {
	var subdirectories []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && path != root {
			subdirectories = append(subdirectories, info.Name())
			return filepath.SkipDir
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return subdirectories, nil
}
