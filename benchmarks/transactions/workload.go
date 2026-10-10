package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

var errCleanup = errors.New("normal resource cleanup failed")

type model struct {
	mu         sync.RWMutex
	generation int64
	current    []int64
	history    map[int64][]int64
}

func newModel(rows int, seed uint64) *model {
	v := make([]int64, rows)
	for i := range v {
		v[i] = 1000 + int64(mix(seed+uint64(i))%100)
	}
	return &model{current: v, history: map[int64][]int64{0: v}}
}
func mix(x uint64) uint64 {
	x += 0x9e3779b97f4a7c15
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	return x ^ (x >> 31)
}
func (m *model) prune() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.history = map[int64][]int64{m.generation: m.current}
}
func (m *model) expected(g int64) ([]int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.history[g]
	if !ok {
		return nil, fmt.Errorf("generation %d absent from retained committed model", g)
	}
	return v, nil
}
func equalState(got, want []int64) error {
	if len(got) != len(want) {
		return fmt.Errorf("row count %d != %d", len(got), len(want))
	}
	var gs, ws int64
	for i, v := range got {
		gs += v
		ws += want[i]
		if v != want[i] {
			return fmt.Errorf("row %d value %d != model %d", i+1, v, want[i])
		}
	}
	if gs != ws {
		return errors.New("conserved total mismatch")
	}
	return nil
}
func bootstrap(db *sqlite.DB, values []int64) (err error) {
	ctx := context.Background()
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { err = cleanupJoin(err, tx.Rollback()) }()
	for _, q := range []string{"CREATE TABLE accounts(id INTEGER PRIMARY KEY, value INTEGER NOT NULL)", "CREATE TABLE generation(id INTEGER PRIMARY KEY, version INTEGER NOT NULL)", "INSERT INTO generation VALUES(1,0)"} {
		if _, err = tx.Exec(ctx, q); err != nil {
			return err
		}
	}
	for i, v := range values {
		if _, err = tx.Exec(ctx, "INSERT INTO accounts VALUES(?,?)", int64(i+1), v); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
func cleanupJoin(err, cleanup error) error {
	if cleanup == nil {
		return err
	}
	return errors.Join(err, errCleanup, cleanup)
}
func version(ctx context.Context, tx *sqlite.Tx) (v int64, err error) {
	r, err := tx.Query(ctx, "SELECT version FROM generation WHERE id=1")
	if err != nil {
		return 0, err
	}
	defer func() { err = cleanupJoin(err, r.Close()) }()
	if !r.Next() {
		return 0, missingGenerationError(r.Err())
	}
	vals := r.Values()
	if len(vals) != 1 {
		return 0, errors.New("generation shape")
	}
	var ok bool
	v, ok = vals[0].(int64)
	if !ok {
		return 0, errors.New("generation type")
	}
	if r.Next() {
		return 0, errors.New("duplicate generation row")
	}
	return v, r.Err()
}
func state(ctx context.Context, tx *sqlite.Tx, n int) (v []int64, err error) {
	r, err := tx.Query(ctx, "SELECT id,value FROM accounts ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer func() { err = cleanupJoin(err, r.Close()) }()
	v = make([]int64, 0, n)
	for r.Next() {
		x := r.Values()
		if len(x) != 2 {
			return nil, errors.New("account row shape")
		}
		id, ok := x[0].(int64)
		if !ok || id != int64(len(v)+1) {
			return nil, errors.New("account id ordering/identity mismatch")
		}
		value, ok := x[1].(int64)
		if !ok {
			return nil, errors.New("account value type")
		}
		v = append(v, value)
		if len(v) > n {
			return nil, errors.New("account cardinality bound exceeded")
		}
	}
	if err = r.Err(); err != nil {
		return nil, err
	}
	return v, nil
}
func verifyCurrent(db *sqlite.DB, m *model) (err error) {
	ctx := context.Background()
	tx, err := db.Begin(ctx, sqlite.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer func() { err = cleanupJoin(err, tx.Rollback()) }()
	g, err := version(ctx, tx)
	if err != nil {
		return err
	}
	m.mu.RLock()
	want, current := m.current, m.generation
	m.mu.RUnlock()
	if g != current {
		return fmt.Errorf("reopened generation %d != %d", g, current)
	}
	got, err := state(ctx, tx, len(want))
	if err != nil {
		return err
	}
	return equalState(got, want)
}
func pause(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func readerAttempt(ctx context.Context, db *sqlite.DB, m *model, c Config) (checks uint64, err error) {
	tx, err := db.Begin(ctx, sqlite.TxOptions{ReadOnly: true})
	if err != nil {
		return 0, err
	}
	defer func() { err = cleanupJoin(err, tx.Rollback()) }()
	g, err := version(ctx, tx)
	if err != nil {
		return 0, fmt.Errorf("reader generation: %w", err)
	}
	want, err := m.expected(g)
	if err != nil {
		return 0, err
	}
	for i := 0; i < 2; i++ {
		got, e := state(ctx, tx, c.Rows)
		if e != nil {
			return checks, fmt.Errorf("reader scan %d generation %d: %w", i+1, g, e)
		}
		if e = equalState(got, want); e != nil {
			return checks, fmt.Errorf("reader model %d generation %d: %w", i+1, g, e)
		}
		checks++
		if i == 0 {
			if e = pause(ctx, c.SnapshotHold); e != nil {
				return checks, e
			}
		}
	}
	return checks, nil
}

// Attempt identity depends on seed+ordinal, not scheduling. BUSY attempts do
// not advance the committed model; exact observed outcomes remain in reports.
func writerAttempt(ctx context.Context, db *sqlite.DB, m *model, c Config, ordinal uint64) (kind int, busy uint64, err error) {
	kind = int(ordinal % 4)
	tx, err := db.Begin(ctx, sqlite.TxOptions{Immediate: true})
	if err != nil {
		return kind, 0, err
	}
	defer func() { err = cleanupJoin(err, tx.Rollback()) }()
	m.mu.RLock()
	next := append([]int64(nil), m.current...)
	g := m.generation
	m.mu.RUnlock()
	transfer := func(salt uint64) error {
		for j := 0; j < c.Batch; j++ {
			x := mix(c.Seed ^ mix(ordinal*67+uint64(j)*2+salt))
			a := int(x % uint64(c.Rows))
			b := (a + 1 + int(mix(x)%uint64(c.Rows-1))) % c.Rows
			delta := int64(1 + mix(x^17)%7)
			next[a] += delta
			next[b] -= delta
			for _, id := range []int{a, b} {
				if _, e := tx.Exec(ctx, "UPDATE accounts SET value=? WHERE id=?", next[id], int64(id+1)); e != nil {
					return e
				}
			}
		}
		return nil
	}
	if err = transfer(1); err != nil {
		return kind, busy, err
	}
	if kind == 1 || kind == 2 {
		if _, err = tx.Exec(ctx, "SAVEPOINT inner_transfer"); err != nil {
			return kind, busy, err
		}
		outer := append([]int64(nil), next...)
		if err = transfer(1000001); err != nil {
			return kind, busy, err
		}
		if kind == 1 {
			if _, err = tx.Exec(ctx, "ROLLBACK TO inner_transfer"); err != nil {
				return kind, busy, err
			}
			next = outer
		}
		if _, err = tx.Exec(ctx, "RELEASE inner_transfer"); err != nil {
			return kind, busy, err
		}
	}
	// Validate the writer's own overlay, including exact savepoint restoration.
	got, e := state(ctx, tx, c.Rows)
	if e != nil {
		return kind, busy, e
	}
	if e = equalState(got, next); e != nil {
		return kind, busy, e
	}
	if kind == 0 {
		if err = tx.Rollback(); err != nil {
			return kind, busy, cleanupJoin(nil, err)
		}
		return kind, busy, verifyCurrent(db, m)
	}
	if _, err = tx.Exec(ctx, "UPDATE generation SET version=? WHERE id=1", g+1); err != nil {
		return kind, busy, err
	}
	// Readers looking up a newly committed generation wait for its publication.
	// Never hold this mutex during BUSY retry waits: old readers need to finish.
	for retry := 0; retry <= 20; retry++ {
		m.mu.Lock()
		err = tx.Commit(ctx)
		if err == nil {
			m.generation = g + 1
			m.current = next
			m.history[g+1] = next
		}
		m.mu.Unlock()
		if err == nil {
			return kind, busy, nil
		}
		if !ordinaryBusy(err) {
			return kind, busy, err
		}
		busy++
		if retry == 20 {
			return kind, busy, err
		}
		if e = pause(ctx, time.Millisecond); e != nil {
			return kind, busy, e
		}
	}
	return kind, busy, err
}

type workerResult struct {
	outcomes []byte
	counts   Counts
	latency  Latency
	err      error
	writer   bool
	next     uint64
}

func executeEpoch(parent context.Context, writer *sqlite.DB, path string, o sqlite.Options, c Config, m *model, owners *closeLedger, index int, ordinal uint64, limit int, budget time.Duration) (w Window, wl, rl Latency, next uint64, err error) {
	handles := make([]*sqlite.DB, 0, c.Readers)
	readerOwners := make([]*ownedHandle, 0, c.Readers)
	// The main goroutine owns all DB handles and closes only after workers join.
	defer func() {
		for _, owner := range readerOwners {
			err = cleanupJoin(err, owners.close(owner))
		}
	}()
	for i := 0; i < c.Readers; i++ {
		db, e := sqlite.Open(path, o)
		if e != nil {
			return w, wl, rl, ordinal, e
		}
		handles = append(handles, db)
		readerOwners = append(readerOwners, owners.acquire(db, fmt.Sprintf("reader %d", i)))
	}
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	results := make(chan workerResult, c.Readers+1)
	start := make(chan struct{})
	var ready sync.WaitGroup
	ready.Add(c.Readers + 1)
	for _, db := range handles {
		go func(db *sqlite.DB) {
			r := workerResult{}
			ready.Done()
			<-start
			for ctx.Err() == nil {
				t := time.Now()
				checks, e := readerAttempt(ctx, db, m, c)
				r.latency.Add(time.Since(t))
				r.counts.Attempts++
				r.counts.SnapshotChecks += checks
				if classify(&r.counts, e) {
					r.err = e
					cancel()
					break
				}
				if ordinaryBusy(e) {
					r.counts.BusyEvents++
				}
				if pause(ctx, c.ReaderPause) != nil {
					break
				}
			}
			results <- r
		}(db)
	}
	go func() {
		r := workerResult{writer: true, next: ordinal, outcomes: make([]byte, 0, limit)}
		ready.Done()
		<-start
		for i := 0; i < limit && ctx.Err() == nil; i++ {
			t := time.Now()
			kind, busy, e := writerAttempt(ctx, writer, m, c, r.next)
			r.next++
			r.latency.Add(time.Since(t))
			r.counts.Attempts++
			r.counts.BusyEvents += busy
			if ordinaryBusy(e) && busy == 0 {
				r.counts.BusyEvents++
			}
			fatal := classify(&r.counts, e)
			r.outcomes = append(r.outcomes, outcomeCode(kind, e, fatal))
			if fatal {
				r.err = fmt.Errorf("writer attempt %d: %w", r.next-1, e)
				cancel()
				break
			}
			if e == nil {
				if kind == 0 {
					r.counts.RolledBack++
				} else {
					r.counts.Committed++
					if kind == 1 {
						r.counts.SavepointRollback++
					}
					if kind == 2 {
						r.counts.SavepointRelease++
					}
				}
			}
			if pause(ctx, c.WriterPause) != nil {
				break
			}
		}
		cancel()
		results <- r
	}()
	ready.Wait()
	t := time.Now()
	timer := time.AfterFunc(budget, cancel)
	close(start)
	for i := 0; i < c.Readers+1; i++ {
		r := <-results
		if r.writer {
			w.FirstWriterOrdinal = ordinal
			w.WriterOutcomes = string(r.outcomes)
			w.Writer = r.counts
			wl = r.latency
			next = r.next
		} else {
			w.Readers.Merge(r.counts)
			rl.Merge(r.latency)
		}
		if r.err != nil {
			err = errors.Join(err, r.err)
		}
	}
	timer.Stop()
	w.Index = index
	w.StartedUTC = t.UTC().Format(time.RFC3339Nano)
	w.FinishedUTC = time.Now().UTC().Format(time.RFC3339Nano)
	w.ActiveSeconds = time.Since(t).Seconds()
	w.CommittedPerSecond = float64(w.Writer.Committed) / w.ActiveSeconds
	return
}

// Trace codes describe terminal outcomes, not retry events. One byte per attempt.
func outcomeCode(kind int, err error, fatal bool) byte {
	if fatal {
		return 'F'
	}
	if err == nil {
		if kind == 0 {
			return 'R'
		}
		return 'C'
	}
	if ordinaryBusy(err) {
		return 'B'
	}
	return 'X'
}

// A canceled/failed iterator is not evidence that the generation row is absent.
// Preserve its entire diagnostic; only clean EOF creates the absence failure.
func missingGenerationError(iteratorErr error) error {
	if iteratorErr != nil {
		return iteratorErr
	}
	return errors.New("missing generation row")
}
