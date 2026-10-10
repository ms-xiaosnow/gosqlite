package main

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func TestMomentsAndHistogram(t *testing.T) {
	var m Moments
	for _, x := range []float64{1, 2, 3, 4, 5} {
		m.Add(x)
	}
	if m.Mean != 3 || math.Abs(m.Stddev()-math.Sqrt(2.5)) > 1e-12 {
		t.Fatal(m)
	}
	s := summarize(m)
	if s.Confidence95Low != nil || s.Confidence95High != nil {
		t.Fatal("correlated samples must not imply confidence")
	}
	var a, b, all Latency
	for i, d := range []time.Duration{0, time.Microsecond, 1500 * time.Nanosecond, 4 * time.Microsecond, 10 * time.Millisecond} {
		all.Add(d)
		if i < 2 {
			a.Add(d)
		} else {
			b.Add(d)
		}
	}
	a.Merge(b)
	if a.Buckets != all.Buckets || math.Abs(a.Moments.Mean-all.Moments.Mean) > 1e-12 || math.Abs(a.Moments.M2-all.Moments.M2) > 1e-12 {
		t.Fatal("merge mismatch", a, all)
	}
	if a.quantile(.5) != .002 || a.quantile(.95) != 16.384 {
		t.Fatal(a.Report())
	}
	if (Latency{}).quantile(.5) != 0 {
		t.Fatal("empty")
	}
}
func TestAccountingAndClassification(t *testing.T) {
	var c Counts
	for _, e := range []error{nil, sqlite.ErrBusy, context.Canceled, errors.New("model mismatch"), cleanupJoin(sqlite.ErrBusy, errors.New("cleanup"))} {
		c.Attempts++
		classify(&c, e)
	}
	if !c.Valid() || c.Success != 1 || c.Busy != 1 || c.Canceled != 1 || c.Failed != 2 {
		t.Fatal(c)
	}
	if ordinaryBusy(cleanupJoin(sqlite.ErrBusy, errors.New("cleanup"))) {
		t.Fatal("cleanup masked")
	}
	var copy Counts
	copy.Merge(c)
	if copy != c {
		t.Fatal(copy)
	}
}
func validConfig() Config {
	return Config{Mode: "throughput", Profile: "rollback", Duration: time.Second, Transactions: 16, Rows: 8, Readers: 1, Batch: 1, Seed: 20261010, ReaderPause: time.Millisecond, SnapshotHold: time.Millisecond, MaxDataBytes: 16 << 20, SourceID: "unit-test-source", HardwareID: "unit-test-host"}
}
func TestBoundsAndIdentity(t *testing.T) {
	c := validConfig()
	if e := c.Validate(); e != nil {
		t.Fatal(e)
	}
	for _, change := range []func(*Config){func(c *Config) { c.Readers = 0 }, func(c *Config) { c.Rows = 4097 }, func(c *Config) { c.Batch = 5 }, func(c *Config) { c.Profile = "wal-exclusive" }, func(c *Config) { c.Duration = 0 }, func(c *Config) { c.MaxDataBytes = 1 }, func(c *Config) { c.SourceID = "" }, func(c *Config) { c.WriterPause = -1 }} {
		bad := c
		change(&bad)
		if bad.Validate() == nil {
			t.Fatal("unbounded config accepted", bad)
		}
	}
	a, b := newModel(8, 7), newModel(8, 7)
	if digest(a.current) != digest(b.current) {
		t.Fatal("nondeterministic corpus")
	}
	if digest(a.current) == digest(newModel(8, 8).current) {
		t.Fatal("seed unused")
	}
	if equalState([]int64{1, 2}, []int64{2, 1}) == nil {
		t.Fatal("equal sum hid row mismatch")
	}
	if _, e := a.expected(1); e == nil {
		t.Fatal("missing generation accepted")
	}
}
func TestOrdinaryHarnessSharedLifecycle(t *testing.T) {
	c := validConfig()
	c.Profile = "wal-shared"
	c.Transactions = 32
	rep := run(context.Background(), c)
	if rep.Status == "skipped_unsupported" {
		t.Skip(rep.Errors)
	}
	if rep.Status != "passed" {
		t.Fatalf("status %s errors %v; retained data %s", rep.Status, rep.Errors, rep.DataDirectory)
	}
	if rep.Checkpoints < 1 || rep.Reopens < 1 || !rep.FinalModelVerified || rep.Shutdown != "workers_joined_normal_close" || !rep.Writer.Valid() || !rep.Readers.Valid() {
		t.Fatalf("incomplete report %+v", rep)
	}
	for _, window := range rep.Windows {
		if len(window.WriterOutcomes) != int(window.Writer.Attempts) || strings.Count(window.WriterOutcomes, "C") != int(window.Writer.Committed) || strings.Count(window.WriterOutcomes, "R") != int(window.Writer.RolledBack) {
			t.Fatal("trace/counters disagree", window)
		}
	}
	if rep.FinalModelGeneration != int64(rep.Writer.Committed) || rep.FinalModelSHA256 == "" {
		t.Fatal("model identity missing")
	}
	if rep.ElapsedSeconds <= 0 || rep.ActiveSeconds <= 0 || rep.Writer.Committed == 0 || rep.Readers.Success == 0 {
		t.Fatal("unmeasured work")
	}
}

func TestOrdinaryHarnessCooperativeCancellation(t *testing.T) {
	c := validConfig()
	c.Mode = "soak"
	c.Profile = "wal-shared"
	c.Duration = 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	rep := run(ctx, c)
	if rep.Status == "skipped_unsupported" {
		t.Skip(rep.Errors)
	}
	if rep.Status != "failed" || rep.Shutdown != "workers_joined_normal_close" || len(rep.Errors) == 0 || rep.ActiveSeconds >= c.Duration.Seconds() {
		t.Fatalf("canceled run incorrectly accepted or not closed: %+v", rep)
	}
	if !rep.Writer.Valid() || !rep.Readers.Valid() {
		t.Fatal("failed report lost outcomes")
	}
}

func TestRejectedProfileHasNoDatabase(t *testing.T) {
	c := validConfig()
	c.Profile = "encrypted-exclusive"
	rep := run(context.Background(), c)
	if rep.Status != "rejected" || rep.DataDirectory != "" || rep.Shutdown != "not_started" || len(rep.Errors) == 0 {
		t.Fatalf("unsupported profile fabricated coverage: %+v", rep)
	}
}

func TestOutcomeTraceCodes(t *testing.T) {
	tests := []struct {
		kind  int
		err   error
		fatal bool
		want  byte
	}{{0, nil, false, 'R'}, {1, nil, false, 'C'}, {2, nil, false, 'C'}, {3, sqlite.ErrBusy, false, 'B'}, {1, context.Canceled, false, 'X'}, {1, errors.New("model"), true, 'F'}}
	for _, tt := range tests {
		if got := outcomeCode(tt.kind, tt.err, tt.fatal); got != tt.want {
			t.Fatalf("outcome %q != %q", got, tt.want)
		}
	}
}
