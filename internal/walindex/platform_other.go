//go:build !linux

package walindex

import "os"

func openFile(string) (*os.File, error)                 { return nil, ErrUnsupported }
func validateLinks(os.FileInfo) error                   { return ErrUnsupported }
func attachDMS(*os.File) error                          { return ErrUnsupported }
func setRangeLock(*os.File, int, int, bool, bool) error { return ErrUnsupported }
func mapFile(*os.File, int64, int) ([]byte, error)      { return nil, ErrUnsupported }
func unmapFile([]byte) error                            { return ErrUnsupported }
