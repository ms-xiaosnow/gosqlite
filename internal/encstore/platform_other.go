//go:build !linux

package encstore

import "os"

func supported() error                       { return ErrUnsupported }
func openFile(string, int) (*os.File, error) { return nil, ErrUnsupported }
func validateFile(os.FileInfo) error         { return ErrUnsupported }
func validateLinks(*os.File) error           { return ErrUnsupported }
func lockFile(*os.File, bool) error          { return ErrUnsupported }
func renameAbsent(string, string) error      { return ErrUnsupported }

func openDirectory(string) (*os.File, error) { return nil, ErrUnsupported }

func openFileAt(*os.File, string, int) (*os.File, error)         { return nil, ErrUnsupported }
func stableFileAt(*os.File, string, *os.File, os.FileInfo) error { return ErrUnsupported }
func renameFileAt(*os.File, string, string, bool) error          { return ErrUnsupported }

func rangeLock(*os.File, int64, bool, bool) error { return ErrUnsupported }
