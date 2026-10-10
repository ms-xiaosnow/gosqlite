//go:build oracle

package testkit_test

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

type liveErrorBuffer struct {
	mu   sync.Mutex
	data bytes.Buffer
}

func (b *liveErrorBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.data.Write(p)
}
func (b *liveErrorBuffer) take() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	s := b.data.String()
	b.data.Reset()
	return s
}

type liveOracle struct {
	t        *testing.T
	cmd      *exec.Cmd
	in       io.WriteCloser
	out      *bufio.Reader
	stderr   liveErrorBuffer
	sequence int
	closed   bool
}

// The test oracle stays alive for ordinary transactions and closes through
// normal .quit/EOF only. No Kill, signal, forced exit or fault injection is used.
func startLiveOracle(t *testing.T, path string) *liveOracle {
	t.Helper()
	p := &liveOracle{t: t}
	p.cmd = exec.Command(confOracle(t), "-batch", "-noheader", "-list", path)
	var err error
	p.in, err = p.cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	out, err := p.cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	p.out = bufio.NewReader(out)
	p.cmd.Stderr = &p.stderr
	if err = p.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.close() })
	p.run(".timeout 0\n.dbconfig no_ckpt_on_close 1\n.filectrl psow 0\nPRAGMA synchronous=FULL;\nPRAGMA wal_autocheckpoint=0;")
	return p
}
func (p *liveOracle) run(query string) []string {
	p.t.Helper()
	p.sequence++
	marker := fmt.Sprintf("__native_wal_done_%d__", p.sequence)
	if _, err := io.WriteString(p.in, query+"\n.print "+marker+"\n"); err != nil {
		p.t.Fatal(err)
	}
	var lines []string
	for {
		line, err := p.out.ReadString('\n')
		if err != nil {
			p.t.Fatalf("ordinary oracle ended: %v; %s", err, p.stderr.take())
		}
		line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		if line == marker {
			break
		}
		lines = append(lines, line)
	}
	if e := p.stderr.take(); e != "" {
		p.t.Fatalf("ordinary oracle SQL %q: %s", query, e)
	}
	return lines
}
func (p *liveOracle) close() {
	if p.closed {
		return
	}
	p.closed = true
	_, _ = io.WriteString(p.in, ".quit\n")
	_ = p.in.Close()
	if err := p.cmd.Wait(); err != nil {
		p.t.Errorf("normal oracle close: %v; %s", err, p.stderr.take())
	}
}
func sharedOracleValue(t *testing.T, db *sqlite.DB) string {
	t.Helper()
	r, err := db.Query(context.Background(), "SELECT text FROM t WHERE id=1")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if !r.Next() {
		t.Fatalf("missing value: %v", r.Err())
	}
	s := r.Values()[0].(string)
	if r.Next() || r.Err() != nil {
		t.Fatal(r.Err())
	}
	return s
}
func liveWant(t *testing.T, p *liveOracle, query, want string) {
	t.Helper()
	got := p.run(query)
	if len(got) != 1 || got[0] != want {
		t.Fatalf("%s: got%v want%q", query, got, want)
	}
}

func TestOracleLiveSharedWALReadersAndWriter(t *testing.T) {
	ctx := context.Background()
	for _, size := range []uint32{512, 4096, 65536} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "live.db")
			opts := sqlite.Options{WALShared: true, PageSize: size, KeepWALOnClose: true}
			a, err := sqlite.Open(path, opts)
			if err != nil {
				t.Fatal(err)
			}
			defer a.Close()
			writeExec(t, a, "CREATE TABLE t(id INTEGER PRIMARY KEY,text TEXT)")
			writeExec(t, a, "CREATE INDEX ix ON t(text)")
			writeExec(t, a, "INSERT INTO t VALUES(1,'old'),(2,'stable')")
			p := startLiveOracle(t, path)
			liveWant(t, p, "BEGIN; SELECT text FROM t WHERE id=1;", "old")
			writeExec(t, a, "UPDATE t SET text='native' WHERE id=1")
			liveWant(t, p, "SELECT text FROM t WHERE id=1;", "old")
			b, err := sqlite.Open(path, opts)
			if err != nil {
				t.Fatal(err)
			}
			defer b.Close()
			if got := sharedOracleValue(t, b); got != "native" {
				t.Fatal(got)
			}
			if err = a.Checkpoint(ctx); !errors.Is(err, sqlite.ErrBusy) {
				t.Fatalf("checkpoint ignores official reader: %v", err)
			}
			p.run("COMMIT;")
			if err = a.Checkpoint(ctx); err != nil {
				t.Fatal(err)
			}
			p.run("BEGIN IMMEDIATE; UPDATE t SET text='official' WHERE id=1;")
			if _, err = a.Exec(ctx, "UPDATE t SET text='must not win' WHERE id=1"); !errors.Is(err, sqlite.ErrBusy) {
				t.Fatalf("ignores official writer: %v", err)
			}
			if got := sharedOracleValue(t, b); got != "native" {
				t.Fatalf("uncommitted leak: %s", got)
			}
			p.run("COMMIT;")
			if got := sharedOracleValue(t, a); got != "official" {
				t.Fatal(got)
			}
			liveWant(t, p, "SELECT text FROM t INDEXED BY ix WHERE text='official';", "official")
			liveWant(t, p, "PRAGMA integrity_check;", "ok")
			p.run("PRAGMA wal_checkpoint(TRUNCATE);")
			writeExec(t, a, "INSERT INTO t VALUES(3,'after official reset')")
			liveWant(t, p, "SELECT text FROM t WHERE id=3;", "after official reset")
			p.close()
			if err = b.Close(); err != nil {
				t.Fatal(err)
			}
			if err = a.Checkpoint(ctx); err != nil {
				t.Fatal(err)
			}
			if err = a.Close(); err != nil {
				t.Fatal(err)
			}
			writeIntegrity(t, path)
		})
	}
}
