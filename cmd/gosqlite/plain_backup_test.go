package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func TestCLIPlainBackupMemoryFlags(t *testing.T) {
	for _, tail := range [][]string{{"--mode", "snapshot"}, {"--mode", "wal"}, {"--mode", "shared"}, {"--out-key-provider", "env-hex:KEY"}} {
		args := append([]string{"untouched", "--out", "untouched-copy"}, tail...)
		input := credentialInput{lookupEnv: func(string) (string, bool) { t.Fatal("credential access"); return "", false }, stdin: func() (io.Reader, error) { t.Fatal("stdin access"); return nil, nil }}
		var out bytes.Buffer
		if err := runMaintenanceWithInput(context.Background(), "backup", args, &out, input); !errors.Is(err, sqlite.ErrUnsupported) || out.Len() != 0 {
			t.Fatal(err)
		}
	}
}
