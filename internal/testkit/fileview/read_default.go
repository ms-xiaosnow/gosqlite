//go:build !darwin

// Package fileview supplies test-only inspection through platform-appropriate
// ownership. It does not implement storage or database recovery.
package fileview

import "os"

// ReadFile preserves the existing non-Darwin fixture-inspection behavior.
func ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }
