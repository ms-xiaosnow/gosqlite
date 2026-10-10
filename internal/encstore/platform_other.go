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
