package postgres

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"os"
	"sort"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

const (
	MaxImportDistanceRows                     = 10000
	MaxImportDistanceQueries                  = 16
	MaxImportDistancePairs                    = 100000
	MaxImportDistanceComponents         int64 = 8000000
	MaxImportDistanceRetainedComponents int64 = 1000000
	ImportDistanceTolerance                   = 64 * (1.0 / (1 << 52))
)

// ImportDistanceOptions is an opt-in, read-only verification contract for one
// explicitly mapped target VECTOR column. Zero budgets use bounded defaults.
// QueryPKs selects accepted, non-NULL source vectors; otherwise QueryCount
// (default four) evenly spaced signed-PK ranks select queries deterministically.
// Every query uses ALL accepted non-NULL candidates, never a candidate sample.
// No caller tolerance, ANN mode, SQL predicate or external connection is allowed.
type ImportDistanceOptions struct {
	Column           string              `json:"column"`
	Metrics          []vector.MetricKind `json:"metrics,omitempty"`
	QueryCount       int                 `json:"query_count,omitempty"`
	QueryPKs         []int64             `json:"query_pks,omitempty"`
	TopK             int                 `json:"top_k,omitempty"`
	MaxRows          int                 `json:"max_rows,omitempty"`
	MaxPairs         int                 `json:"max_pairs,omitempty"`
	MaxComponentWork int64               `json:"max_component_work,omitempty"`
}

// ImportDistanceVerification deliberately wraps, rather than extends, the
// existing positional ImportResult/ImportOptions public struct layouts.
type ImportDistanceVerification struct {
	Import    ImportResult         `json:"import"`
	Distances ImportDistanceReport `json:"distance_verification"`
}
type ImportDistanceReport struct {
	Version                   int                         `json:"version"`
	Status                    string                      `json:"status"`
	Options                   ImportDistanceOptions       `json:"options"`
	SourceField               string                      `json:"source_field"`
	PrimaryKeyField           string                      `json:"primary_key_field"`
	Dimensions                int                         `json:"dimensions"`
	AcceptedRows              int64                       `json:"accepted_rows"`
	CandidateRows             int                         `json:"candidate_rows"`
	NullRows                  int                         `json:"null_rows"`
	ZeroNormRows              int                         `json:"zero_norm_rows"`
	NormsChecked              int                         `json:"norms_checked"`
	NativeRowsChecked         int                         `json:"native_rows_checked"`
	QuerySelection            string                      `json:"query_selection"`
	QueryCoverage             string                      `json:"query_coverage"`
	QueryPKs                  []int64                     `json:"query_pks"`
	UnselectedVectorColumns   []string                    `json:"unselected_vector_columns"`
	Reference                 string                      `json:"reference"`
	DistanceRelativeTolerance float64                     `json:"distance_relative_tolerance"`
	CosineAbsoluteTolerance   float64                     `json:"cosine_absolute_tolerance"`
	ErrorBounds               string                      `json:"error_bounds"`
	NormRelativeTolerance     float64                     `json:"norm_relative_tolerance"`
	Ordering                  string                      `json:"ordering"`
	RetainedComponents        int64                       `json:"retained_components"`
	MaxRetainedComponents     int64                       `json:"max_retained_components"`
	PlannedComparisons        int                         `json:"planned_comparisons"`
	PlannedComponentWork      int64                       `json:"planned_component_work"`
	DistancesChecked          int                         `json:"distances_checked"`
	TopKChecked               int                         `json:"top_k_checked"`
	Queries                   []ImportDistanceQueryReport `json:"queries"`
	ANNRecall                 string                      `json:"ann_recall"`
}
type ImportDistanceQueryReport struct {
	QueryPK      int64               `json:"query_pk"`
	Metric       vector.MetricKind   `json:"metric"`
	Status       string              `json:"status"`
	Reason       string              `json:"reason,omitempty"`
	Exact        bool                `json:"native_exact"`
	TopKBoundary string              `json:"top_k_boundary"`
	All          []ImportDistanceHit `json:"all"`
	TopK         []ImportDistanceHit `json:"top_k"`
}
type ImportDistanceHit struct {
	PK        int64   `json:"pk"`
	Native    float64 `json:"native"`
	Reference float64 `json:"reference"`
	Tolerance float64 `json:"tolerance"`
}

func (o ImportDistanceOptions) normalized(m ImportManifest) (ImportDistanceOptions, int, error) {
	bad := func() (ImportDistanceOptions, int, error) {
		return o, -1, fmt.Errorf("import distances: invalid column, metric, query selection or resource limits")
	}
	field := -1
	for i, f := range m.Fields {
		if f.Target == o.Column && f.Type == "vector" {
			field = i
		}
	}
	if field < 0 {
		return bad()
	}
	if len(o.Metrics) == 0 {
		o.Metrics = []vector.MetricKind{vector.L2, vector.L1, vector.NegativeIP, vector.Cosine}
	}
	if len(o.Metrics) > 4 {
		return bad()
	}
	seen := map[vector.MetricKind]bool{}
	for _, metric := range o.Metrics {
		switch metric {
		case vector.L2, vector.L1, vector.NegativeIP, vector.Cosine:
		default:
			return o, -1, fmt.Errorf("import distances: unsupported metric %q", metric)
		}
		if seen[metric] {
			return bad()
		}
		seen[metric] = true
	}
	if len(o.QueryPKs) > 0 {
		if o.QueryCount != 0 || len(o.QueryPKs) > MaxImportDistanceQueries {
			return bad()
		}
		seen := map[int64]bool{}
		for _, pk := range o.QueryPKs {
			if seen[pk] {
				return bad()
			}
			seen[pk] = true
		}
		o.QueryPKs = append([]int64(nil), o.QueryPKs...)
	} else {
		if o.QueryCount == 0 {
			o.QueryCount = 4
		}
		if o.QueryCount < 1 || o.QueryCount > MaxImportDistanceQueries {
			return bad()
		}
	}
	if o.TopK == 0 {
		o.TopK = 10
	}
	if o.MaxRows == 0 {
		o.MaxRows = MaxImportDistanceRows
	}
	if o.MaxPairs == 0 {
		o.MaxPairs = MaxImportDistancePairs
	}
	if o.MaxComponentWork == 0 {
		o.MaxComponentWork = MaxImportDistanceComponents
	}
	if o.TopK < 1 || o.TopK > MaxImportDistanceRows || o.MaxRows < 1 || o.MaxRows > MaxImportDistanceRows || o.MaxPairs < 1 || o.MaxPairs > MaxImportDistancePairs || o.MaxComponentWork < 1 || o.MaxComponentWork > MaxImportDistanceComponents {
		return bad()
	}
	o.Metrics = append([]vector.MetricKind(nil), o.Metrics...)
	return o, field, nil
}

// DecodeImportDistanceOptions reads at most 64 KiB of strict options JSON.
// Unknown/duplicate fields and trailing JSON are rejected. Manifest-dependent
// validation is performed by VerifyImportDistances before opening either file.
func DecodeImportDistanceOptions(r io.Reader) (ImportDistanceOptions, error) {
	var o ImportDistanceOptions
	if r == nil {
		return o, fmt.Errorf("import distances: nil options reader")
	}
	raw, err := io.ReadAll(io.LimitReader(r, MaxImportManifestBytes+1))
	if err != nil {
		return o, err
	}
	if len(raw) > MaxImportManifestBytes {
		return o, fmt.Errorf("import distances: options byte limit")
	}
	l := DefaultLimits()
	l.MaxBytes = MaxImportManifestBytes
	l.MaxTokens = 1024
	l.MaxDepth = 4
	if err = strictJSON(raw, &o, l); err != nil {
		return o, fmt.Errorf("import distances: %w", err)
	}
	return o, nil
}

// VerifyImportDistances retains every fidelity/count/provenance/index check of
// VerifyImport. It additionally checks native SQL dimensions/norms and full exact
// search plus an independent Top-K execution against source-export references,
// in the SAME native read snapshot. The source is rehashed after all checks.
// Returned reports are partial on error and must never be treated as success.
// The source parser is shared with Import; the mathematical reference is wholly
// separate from native vector metrics and uses 768-bit arithmetic. This tests
// supplied export fidelity, not the accuracy of an exporter's lost decimal bits.
func VerifyImportDistances(ctx context.Context, input, output string, m ImportManifest, options ImportDistanceOptions) (r ImportDistanceVerification, resultErr error) {
	r.Distances.Status = "not_run"
	if ctx == nil {
		return r, fmt.Errorf("import distances: nil context")
	}
	if err := m.Validate(); err != nil {
		return r, err
	}
	o, field, err := options.normalized(m)
	if err != nil {
		return r, err
	}
	r.Distances = ImportDistanceReport{Version: 1, Status: "not_run", Options: o, SourceField: m.Fields[field].Source, PrimaryKeyField: m.Fields[m.pkField()].Target, Dimensions: m.Fields[field].Dimensions, QueryPKs: []int64{}, Queries: []ImportDistanceQueryReport{}, UnselectedVectorColumns: []string{}, Reference: "source-export binary32 components; independent 768-bit arithmetic", DistanceRelativeTolerance: ImportDistanceTolerance, CosineAbsoluteTolerance: ImportDistanceTolerance, ErrorBounds: "epsilon=2^-52; L1/L2/norm:64*epsilon*abs(reference); cosine:64*epsilon; negative IP:outward-rounded 4*u*abs(reference)+(1+u)*gamma_n^2*S_upper; u=2^-53, gamma_n=n*u/(1-n*u), S_upper=upward-rounded sum(abs(source products))", NormRelativeTolerance: ImportDistanceTolerance, Ordering: "native distance ASC, signed PK ASC; reference interval ordering; top-K boundary overlap explicitly reported", MaxRetainedComponents: MaxImportDistanceRetainedComponents, ANNRecall: "not_run; exact verification does not measure ANN recall"}
	for i, f := range m.Fields {
		if f.Type == "vector" && i != field {
			r.Distances.UnselectedVectorColumns = append(r.Distances.UnselectedVectorColumns, f.Target)
		}
	}
	r.Import, err = verifyImport(ctx, input, output, m, func(src *os.File, db *sqlite.DB, base ImportResult) error {
		return verifyImportDistanceSnapshot(ctx, src, db, m, field, base, &r.Distances)
	})
	if err != nil {
		r.Distances.Status = "failed"
		r.Import.ExactDistances = "failed; see distance_verification"
		return r, err
	}
	r.Import.ExactDistances = r.Distances.Status + "; see distance_verification"
	return r, nil
}

type importDistanceRow struct {
	pk     int64
	values []float32
	norm   float64
}

func verifyImportDistanceSnapshot(ctx context.Context, src *os.File, db *sqlite.DB, m ImportManifest, field int, base ImportResult, r *ImportDistanceReport) error {
	o := r.Options
	r.AcceptedRows = base.Accepted
	if base.Accepted > int64(o.MaxRows) {
		return fmt.Errorf("import distances: accepted-row cap exceeded; no candidate sampling")
	}
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return err
	}
	reader, err := newImportRecordReader(ctx, src, m, MaxImportRecordBytes, 0, nil)
	if err != nil {
		return err
	}
	rows := make(map[int64]importDistanceRow, int(base.Accepted))
	for n := int64(1); n <= base.SourceRecords; n++ {
		rec, parseErr := reader.Next(ctx)
		if parseErr == io.EOF {
			return fmt.Errorf("import distances: early source end")
		}
		var rejected importRejection
		skip, e := readImportJSON(ctx, db, importRejections, n, &rejected)
		if e != nil {
			return e
		}
		if skip {
			continue
		}
		if parseErr != nil {
			return parseErr
		}
		var provenance importAcceptedRow
		ok, e := readImportJSON(ctx, db, importAccepted, rec.PK, &provenance)
		if e != nil {
			return e
		}
		if !ok || provenance.Record != n {
			return fmt.Errorf("import distances: source PK provenance mismatch")
		}
		if _, exists := rows[rec.PK]; exists {
			return fmt.Errorf("import distances: duplicate accepted source PK")
		}
		row := importDistanceRow{pk: rec.PK}
		if rec.Values[field] == nil {
			r.NullRows++
		} else {
			v, ok := rec.Values[field].(vector.Vector)
			if !ok || v.Dims() != r.Dimensions {
				return fmt.Errorf("import distances: source dimensions")
			}
			r.RetainedComponents += int64(r.Dimensions)
			if r.RetainedComponents > MaxImportDistanceRetainedComponents {
				return fmt.Errorf("import distances: retained component cap exceeded")
			}
			row.values = v.Values()
			r.CandidateRows++
		}
		rows[rec.PK] = row
	}
	if len(rows) != int(base.Accepted) {
		return fmt.Errorf("import distances: accepted source count mismatch")
	}
	// Exhaust the source independently as well as checking the persisted cursor.
	if _, e := reader.Next(ctx); e != io.EOF {
		return errors.Join(e, fmt.Errorf("import distances: source end mismatch"))
	}
	candidates := make([]importDistanceRow, 0, r.CandidateRows)
	for _, row := range rows {
		if row.values != nil {
			candidates = append(candidates, row)
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].pk < candidates[j].pk })
	if len(o.QueryPKs) > 0 {
		r.QuerySelection = "supplied_source_pks"
		for _, pk := range o.QueryPKs {
			row, ok := rows[pk]
			if !ok || row.values == nil {
				return fmt.Errorf("import distances: query PK must identify an accepted non-NULL source vector")
			}
			r.QueryPKs = append(r.QueryPKs, pk)
		}
	} else {
		r.QuerySelection = "evenly_spaced_signed_pk_ranks"
		count := min(o.QueryCount, len(candidates))
		for i := 0; i < count; i++ {
			rank := 0
			if count > 1 {
				rank = i * (len(candidates) - 1) / (count - 1)
			}
			r.QueryPKs = append(r.QueryPKs, candidates[rank].pk)
		}
	}
	r.QueryCoverage = "sampled_queries_all_candidates"
	if len(r.QueryPKs) == len(candidates) {
		r.QueryCoverage = "all_source_vectors_as_queries_all_candidates"
	}
	queryMetrics := len(r.QueryPKs) * len(o.Metrics)
	pairs := queryMetrics * (r.CandidateRows + min(o.TopK, r.CandidateRows))
	r.PlannedComparisons = pairs
	// Conservative preflight charges even metrics subsequently skipped, one
	// reference + scale + two native scoring passes, and source/native norms.
	r.PlannedComponentWork = int64(r.Dimensions) * (2*int64(r.CandidateRows) + int64(queryMetrics)*(4*int64(r.CandidateRows)+2))
	if pairs > o.MaxPairs || r.PlannedComponentWork > o.MaxComponentWork {
		return fmt.Errorf("import distances: comparison or component-work cap exceeded")
	}
	for i := range candidates {
		if err = ctx.Err(); err != nil {
			return err
		}
		candidates[i].norm = importDistanceReferenceNorm(candidates[i].values)
		rows[candidates[i].pk] = candidates[i]
		if candidates[i].norm == 0 {
			r.ZeroNormRows++
		}
	}
	if err = verifyImportNativeNorms(ctx, db, m, field, rows, r); err != nil {
		return err
	}
	if len(candidates) == 0 {
		r.Status = "skipped_no_nonnull_vectors"
		r.QueryCoverage = "no_nonnull_vectors"
		return nil
	}
	limits := vector.SearchLimits{MaxCandidates: o.MaxRows, MaxResults: o.MaxRows, MaxComponents: uint64(r.Dimensions) * uint64(r.CandidateRows+1)}
	skipped := false
	for _, pk := range r.QueryPKs {
		query := rows[pk]
		for _, metric := range o.Metrics {
			if err = ctx.Err(); err != nil {
				return err
			}
			qr := ImportDistanceQueryReport{QueryPK: pk, Metric: metric, Status: "not_run", All: []ImportDistanceHit{}, TopK: []ImportDistanceHit{}}
			if metric == vector.Cosine && (r.ZeroNormRows > 0 || query.norm == 0) {
				qr.Status = "skipped_zero_norm"
				qr.Reason = "native cosine rejects zero norms; no candidate filtering or invented distance"
				r.Queries = append(r.Queries, qr)
				skipped = true
				continue
			}
			refs := make(map[int64]importDistanceReferenceValue, len(candidates))
			for _, row := range candidates {
				if err = ctx.Err(); err != nil {
					return err
				}
				ref := importDistanceReference(row.values, query.values, metric)
				refs[row.pk] = importDistanceReferenceValue{ref, importDistanceErrorBound(row.values, query.values, metric, ref)}
			}
			q, e := vector.New(query.values)
			if e != nil {
				return e
			}
			full, e := db.VectorSearch(ctx, m.Table, o.Column, q, sqlite.VectorSearchOptions{Mode: sqlite.VectorModeExact, Metric: metric, Limit: len(candidates), Limits: limits})
			if e != nil {
				return e
			}
			if e = checkImportDistancePlan(full, r, metric, len(candidates)); e != nil {
				return e
			}
			qr.All, e = checkImportDistanceHits(full.Hits, refs)
			if e != nil {
				return e
			}
			r.DistancesChecked += len(qr.All)
			k := min(o.TopK, len(candidates))
			top, e := db.VectorSearch(ctx, m.Table, o.Column, q, sqlite.VectorSearchOptions{Mode: sqlite.VectorModeExact, Metric: metric, Limit: k, Limits: limits})
			if e != nil {
				return e
			}
			if e = checkImportDistancePlan(top, r, metric, k); e != nil {
				return e
			}
			for i, hit := range top.Hits {
				if hit != full.Hits[i] {
					return fmt.Errorf("import distances: Top-K differs from complete exact result")
				}
				qr.TopK = append(qr.TopK, qr.All[i])
			}
			qr.TopKBoundary = importDistanceBoundary(qr.All, k)
			r.TopKChecked += len(qr.TopK)
			qr.Exact = true
			qr.Status = "passed"
			r.Queries = append(r.Queries, qr)
		}
	}
	r.Status = "passed"
	if skipped {
		r.Status = "partial_skipped_metrics"
		if r.DistancesChecked == 0 {
			r.Status = "skipped_no_defined_metrics"
		}
	}
	return nil
}

func verifyImportNativeNorms(ctx context.Context, db *sqlite.DB, m ImportManifest, field int, want map[int64]importDistanceRow, r *ImportDistanceReport) error {
	col := importQuote(m.Fields[field].Target)
	result, err := db.Query(ctx, "SELECT "+importQuote(m.Fields[m.pkField()].Target)+", vector_dims("+col+"), vector_norm("+col+") FROM "+importQuote(m.Table))
	if err != nil {
		return err
	}
	defer result.Close()
	seen := map[int64]bool{}
	for result.Next() {
		v := result.Values()
		pk, ok := v[0].(int64)
		row, exists := want[pk]
		if !ok || !exists || seen[pk] {
			return fmt.Errorf("import distances: native mapped PK mismatch")
		}
		seen[pk] = true
		r.NativeRowsChecked++
		if row.values == nil {
			if v[1] != nil || v[2] != nil {
				return fmt.Errorf("import distances: native NULL mismatch")
			}
			continue
		}
		dims, ok := v[1].(int64)
		norm, normOK := v[2].(float64)
		if !ok || dims != int64(r.Dimensions) || !normOK || !importDistanceClose(norm, row.norm, ImportDistanceTolerance*math.Abs(row.norm)) {
			return fmt.Errorf("import distances: native dimension or norm mismatch at PK %d", pk)
		}
		r.NormsChecked++
	}
	if err = errors.Join(result.Err(), result.Close()); err != nil {
		return err
	}
	if len(seen) != len(want) {
		return fmt.Errorf("import distances: native norm row count mismatch")
	}
	return nil
}
func checkImportDistancePlan(got sqlite.VectorResult, r *ImportDistanceReport, metric vector.MetricKind, count int) error {
	if !got.Plan.Exact || got.Plan.Metric != metric || got.Plan.Field != r.Options.Column || got.Plan.Method != "exact_vector" || got.Plan.Dimensions != r.Dimensions || len(got.Hits) != count || got.Stats.BaseRowFetches != r.AcceptedRows || got.Stats.ScoredRows != int64(r.CandidateRows) || got.Stats.NullRows != int64(r.NullRows) {
		return fmt.Errorf("import distances: incomplete native exact plan/count coverage")
	}
	return nil
}
func importDistanceClose(got, want, tolerance float64) bool {
	return !math.IsNaN(got) && !math.IsInf(got, 0) && !math.IsNaN(want) && !math.IsInf(want, 0) && math.Abs(got-want) <= tolerance
}

// Bounds follow widened binary32 inputs and native Neumaier accumulation.
// L1/L2/norm have no cancellation of nonnegative terms; 64 eps covers their
// input subtraction, accumulation, final rounding and sqrt. Cosine's bound
// is absolute. For signed IP, let u=2^-53, gamma=n*u/(1-n*u), and S=sum|a*b|.
// Widened binary32 products and FastTwoSum residuals are exact in binary64.
// Each partial sum is bounded by S/(1-n*u), so the absolute residual sum is
// at most gamma*S. Summing these residuals ordinarily costs <=gamma^2*S.
// Final native and reference rounding cost <=2*u/(1-u)*abs(ref), bounded by
// 4*u*abs(ref). Thus 4*u*abs(ref)+(1+u)*gamma^2*S is conservative. This retains
// the dimension-squared term lost by v1. All scale/bound operations round
// outward; neither cancellation nor downward scale rounding invalidates it.
// Native metrics are unchanged. See docs/import-distance-verification.md.
func importDistanceErrorBound(a, b []float32, metric vector.MetricKind, ref float64) float64 {
	if metric == vector.Cosine {
		return ImportDistanceTolerance
	}
	if metric != vector.NegativeIP {
		return ImportDistanceTolerance * math.Abs(ref)
	}
	const unit = 1.0 / (1 << 53)
	up := func(v float64) float64 { return math.Nextafter(v, math.Inf(1)) }
	down := func(v float64) float64 { return math.Nextafter(v, math.Inf(-1)) }
	scaleUpper := 0.0
	for i, x := range a {
		product := math.Abs(float64(x) * float64(b[i]))
		if product != 0 {
			scaleUpper = up(scaleUpper + product)
		}
	}
	if scaleUpper == 0 {
		return 0
	} // Every product, the sum and reference are exact zero.
	nu := float64(len(a)) * unit // Exact for the supported integer dimension range.
	gammaUpper := up(nu / down(1-nu))
	correctionUpper := up(up(gammaUpper*gammaUpper) * scaleUpper)
	finalUpper := up((4 * unit) * math.Abs(ref))
	return up(finalUpper + up(up(1+unit)*correctionUpper))
}

type importDistanceReferenceValue struct{ distance, tolerance float64 }

func checkImportDistanceHits(hits []vector.Hit, refs map[int64]importDistanceReferenceValue) ([]ImportDistanceHit, error) {
	if len(hits) != len(refs) {
		return nil, fmt.Errorf("import distances: incomplete result")
	}
	seen := map[int64]bool{}
	out := make([]ImportDistanceHit, 0, len(hits))
	priorLower := math.Inf(-1)
	for i, hit := range hits {
		ref, exists := refs[hit.RowID]
		if !exists || seen[hit.RowID] {
			return nil, fmt.Errorf("import distances: unknown/duplicate result PK")
		}
		seen[hit.RowID] = true
		if !importDistanceClose(hit.Distance, ref.distance, ref.tolerance) {
			return nil, fmt.Errorf("import distances: distance mismatch at PK %d", hit.RowID)
		}
		if i > 0 {
			prior := hits[i-1]
			if prior.Distance > hit.Distance || prior.Distance == hit.Distance && prior.RowID >= hit.RowID {
				return nil, fmt.Errorf("import distances: native distance/signed-PK ordering")
			}
		}
		if priorLower > ref.distance+ref.tolerance {
			return nil, fmt.Errorf("import distances: independent reference ordering outside error bounds")
		}
		priorLower = max(priorLower, ref.distance-ref.tolerance)
		out = append(out, ImportDistanceHit{PK: hit.RowID, Native: hit.Distance, Reference: ref.distance, Tolerance: ref.tolerance})
	}
	return out, nil
}
func importDistanceBoundary(hits []ImportDistanceHit, k int) string {
	if k == len(hits) {
		return "all_candidates"
	}
	highestIncluded := math.Inf(-1)
	lowestExcluded := math.Inf(1)
	for i, h := range hits {
		if i < k {
			highestIncluded = max(highestIncluded, h.Reference+h.Tolerance)
		} else {
			lowestExcluded = min(lowestExcluded, h.Reference-h.Tolerance)
		}
	}
	if highestIncluded >= lowestExcluded {
		return "ambiguous_within_error_bounds; native distance/signed-PK contract checked"
	}
	return "clear_under_reference_error_bounds"
}

// 768 bits exceed the complete exponent span of products/differences of
// binary32 values plus 16000-term accumulation. Component products and sums
// are exact here. Cosine's product of squared norms, sqrt and division may
// round at this independent high precision before the final float64 rounding.
// Neither reference helper calls native Norm, Metric.Distance, Scan or SQL.
func importDistanceReferenceNorm(a []float32) float64 {
	z := make([]float32, len(a))
	return importDistanceReference(a, z, vector.L2)
}
func importDistanceReference(a, b []float32, metric vector.MetricKind) float64 {
	fresh := func() *big.Float { return new(big.Float).SetPrec(768) }
	av, bv, d, tmp := fresh(), fresh(), fresh(), fresh()
	sum, dot, aa, bb := fresh(), fresh(), fresh(), fresh()
	for i, x := range a {
		av.SetFloat64(float64(x))
		bv.SetFloat64(float64(b[i]))
		d.Sub(av, bv)
		switch metric {
		case vector.L1:
			sum.Add(sum, tmp.Abs(d))
		case vector.L2:
			sum.Add(sum, tmp.Mul(d, d))
		case vector.NegativeIP:
			dot.Add(dot, tmp.Mul(av, bv))
		case vector.Cosine:
			dot.Add(dot, tmp.Mul(av, bv))
			aa.Add(aa, tmp.Mul(av, av))
			bb.Add(bb, tmp.Mul(bv, bv))
		}
	}
	var result *big.Float
	switch metric {
	case vector.L1:
		result = sum
	case vector.L2:
		result = fresh().Sqrt(sum)
	case vector.NegativeIP:
		result = fresh().Neg(dot)
	case vector.Cosine:
		den := fresh().Sqrt(fresh().Mul(aa, bb))
		result = fresh().Sub(fresh().SetInt64(1), fresh().Quo(dot, den))
	default:
		return math.NaN()
	}
	value, _ := result.Float64()
	if metric == vector.Cosine {
		value = max(0, min(2, value))
	}
	return value
}
