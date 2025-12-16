package utils

import (
	"os"
	"os/user"
	"path"
)

func HomeDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	if usr, err := user.Current(); err == nil {
		return usr.HomeDir
	}
	return ""
}

// FindProjectRoot finds the project root directory
func FindProjectRoot(startDir string) string {
	// Start from the current directory and move up to find go.mod file
	dir := startDir
	for {
		// Check if go.mod file exists
		if _, err := os.Stat(path.Join(dir, "go.mod")); err == nil {
			return dir
		}

		// Move to parent directory
		parentDir := path.Dir(dir)
		if parentDir == dir {
			// Reached root but couldn't find go.mod
			// Return current directory
			return startDir
		}
		dir = parentDir
	}
}
