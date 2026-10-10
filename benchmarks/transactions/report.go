package main

import (
	"math"
	"time"
)

// Moments uses Welford's algorithm and retains constant memory.
type Moments struct {
	Count uint64  `json:"sample_count"`
	Mean  float64 `json:"mean"`
	M2    float64 `json:"-"`
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
}

func (m *Moments) Add(x float64) {
	m.Count++
	if m.Count == 1 {
		m.Min, m.Max = x, x
	} else {
		m.Min = math.Min(m.Min, x)
		m.Max = math.Max(m.Max, x)
	}
	delta := x - m.Mean
	m.Mean += delta / float64(m.Count)
	m.M2 += delta * (x - m.Mean)
}
func (m Moments) Stddev() float64 {
	if m.Count < 2 {
		return 0
	}
	return math.Sqrt(m.M2 / float64(m.Count-1))
}

type Summary struct {
	Count            uint64   `json:"sample_count"`
	Mean             float64  `json:"mean"`
	SampleStddev     float64  `json:"sample_stddev"`
	Min              float64  `json:"min"`
	Max              float64  `json:"max"`
	ConfidenceMethod string   `json:"confidence_method"`
	Confidence95Low  *float64 `json:"confidence_95_low"`
	Confidence95High *float64 `json:"confidence_95_high"`
}

func summarize(m Moments) Summary {
	return Summary{Count: m.Count, Mean: m.Mean, SampleStddev: m.Stddev(), Min: m.Min, Max: m.Max, ConfidenceMethod: "descriptive only; correlated within-run windows are not independent repetitions"}
}

// Histograms retain all observations in 48 logarithmic buckets, with exact
// count/min/max/mean/sample variance and explicitly upper-bound quantiles.
type Latency struct {
	Buckets [48]uint64 `json:"buckets_power_of_two_microseconds"`
	Moments Moments    `json:"-"`
}

func (h *Latency) Add(d time.Duration) {
	ns := d.Nanoseconds()
	if ns < 0 {
		ns = 0
	}
	h.Moments.Add(float64(ns) / 1e6)
	i := 0
	for i < 47 && ns > int64(time.Microsecond)*(int64(1)<<i) {
		i++
	}
	h.Buckets[i]++
}
func (h Latency) quantile(p float64) float64 {
	if h.Moments.Count == 0 {
		return 0
	}
	rank := uint64(math.Ceil(float64(h.Moments.Count) * p))
	if rank < 1 {
		rank = 1
	}
	var n uint64
	for i, c := range h.Buckets {
		n += c
		if n >= rank {
			return float64(uint64(1)<<i) / 1000
		}
	}
	return h.Moments.Max
}
func (h *Latency) Merge(v Latency) {
	for i, n := range v.Buckets {
		h.Buckets[i] += n
	}
	if v.Moments.Count == 0 {
		return
	}
	if h.Moments.Count == 0 {
		h.Moments = v.Moments
		return
	}
	a, b := h.Moments, v.Moments
	n := a.Count + b.Count
	d := b.Mean - a.Mean
	h.Moments = Moments{Count: n, Mean: a.Mean + d*float64(b.Count)/float64(n), M2: a.M2 + b.M2 + d*d*float64(a.Count)*float64(b.Count)/float64(n), Min: math.Min(a.Min, b.Min), Max: math.Max(a.Max, b.Max)}
}

type LatencyReport struct {
	Summary    Summary    `json:"milliseconds"`
	P50UpperMS float64    `json:"p50_bucket_upper_ms"`
	P95UpperMS float64    `json:"p95_bucket_upper_ms"`
	Buckets    [48]uint64 `json:"buckets_power_of_two_microseconds"`
}

func (h Latency) Report() LatencyReport {
	return LatencyReport{summarize(h.Moments), h.quantile(.5), h.quantile(.95), h.Buckets}
}

type Counts struct {
	Attempts          uint64 `json:"attempts"`
	Success           uint64 `json:"success"`
	Committed         uint64 `json:"committed"`
	RolledBack        uint64 `json:"intentional_rollbacks"`
	SavepointRollback uint64 `json:"savepoint_rollback_commits"`
	SavepointRelease  uint64 `json:"savepoint_release_commits"`
	Busy              uint64 `json:"busy_outcomes"`
	BusyEvents        uint64 `json:"busy_events_including_commit_retries"`
	Canceled          uint64 `json:"canceled_outcomes"`
	Failed            uint64 `json:"failed_outcomes"`
	SnapshotChecks    uint64 `json:"snapshot_checks"`
}

func (a *Counts) Merge(b Counts) {
	a.Attempts += b.Attempts
	a.Success += b.Success
	a.Committed += b.Committed
	a.RolledBack += b.RolledBack
	a.SavepointRollback += b.SavepointRollback
	a.SavepointRelease += b.SavepointRelease
	a.Busy += b.Busy
	a.BusyEvents += b.BusyEvents
	a.Canceled += b.Canceled
	a.Failed += b.Failed
	a.SnapshotChecks += b.SnapshotChecks
}
func (a Counts) Valid() bool { return a.Attempts == a.Success+a.Busy+a.Canceled+a.Failed }

type Window struct {
	FirstWriterOrdinal uint64  `json:"first_writer_ordinal"`
	WriterOutcomes     string  `json:"writer_outcomes"`
	Index              int     `json:"index"`
	StartedUTC         string  `json:"started_utc"`
	FinishedUTC        string  `json:"finished_utc"`
	ActiveSeconds      float64 `json:"active_seconds"`
	Writer             Counts  `json:"writer"`
	Readers            Counts  `json:"readers"`
	CommittedPerSecond float64 `json:"committed_transactions_per_second"`
}
