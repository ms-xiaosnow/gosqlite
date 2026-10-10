package main

import (
	"fmt"
	"github.com/ms-xiaosnow/gosqlite/internal/buildinfo"
	"io"
	"os"
)

func run(args []string, out io.Writer) error {
	if len(args) == 1 && (args[0] == "version" || args[0] == "--version") {
		_, err := fmt.Fprintf(out, "gosqlite %s; Go %s; SQLite behavior baseline %s\n", buildinfo.Version, buildinfo.GoBaseline, buildinfo.SQLiteBaseline)
		return err
	}
	if len(args) == 0 || (len(args) == 1 && args[0] == "help") {
		_, err := fmt.Fprintln(out, "usage: gosqlite version\nDevelopment scaffold. Database commands are not implemented yet.")
		return err
	}
	return fmt.Errorf("unsupported command %q", args[0])
}
func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "gosqlite:", err)
		os.Exit(1)
	}
}
