package postgres

import (
	"fmt"
	"io"
)

const MaxTransitionOptionsBytes = 64 << 10

// DecodeTransitionOptions reads strict bounded JSON. It rejects duplicate and
// unknown keys; GenerateTransition then validates catalog-dependent semantics.
// Options describe proposed artifacts and never grant execution permission.
func DecodeTransitionOptions(r io.Reader) (TransitionOptions, error) {
	var o TransitionOptions
	if r == nil {
		return o, fmt.Errorf("transition options: nil reader")
	}
	b, err := io.ReadAll(io.LimitReader(r, MaxTransitionOptionsBytes+1))
	if err != nil {
		return o, err
	}
	if len(b) > MaxTransitionOptionsBytes {
		return o, fmt.Errorf("transition options: byte limit")
	}
	l := DefaultLimits()
	l.MaxBytes, l.MaxTokens, l.MaxDepth = MaxTransitionOptionsBytes, 8192, 8
	if err = strictJSON(b, &o, l); err != nil {
		return o, fmt.Errorf("transition options: %w", err)
	}
	return o, nil
}
