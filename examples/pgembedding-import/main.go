// pgembedding-import creates synthetic CSV and NDJSON exports, validates them,
// deliberately pauses at a committed batch, resumes by the committed database
// cursor, and independently verifies real native SQLite output. No PostgreSQL
// server, credentials, external engine, or downloaded data is used.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/migration/postgres"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func writeNew(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	return errors.Join(err, f.Close())
}
func main() {
	dir := flag.String("dir", "", "new output directory, default a new temporary directory")
	flag.Parse()
	ctx := context.Background()
	if flag.NArg() != 0 {
		panic("no positional arguments")
	}
	var err error
	if *dir == "" {
		*dir, err = os.MkdirTemp("", "gosqlite-pgembedding-")
	} else {
		err = os.Mkdir(*dir, 0700)
	}
	if err != nil {
		panic(err)
	}
	type result struct {
		Format, Input, Manifest, Database  string
		DryRun, Paused, Complete, Verified postgres.ImportResult
		HNSWRecallAt4, IVFRecallAt4        float64
		HNSWPlan, IVFPlan                  string
	}
	results := []result{}
	for _, format := range []string{"csv", "ndjson"} {
		var data bytes.Buffer
		writer := csv.NewWriter(&data)
		if format == "csv" {
			if err = writer.Write([]string{"id", "embedding", "label"}); err != nil {
				panic(err)
			}
		}
		for i := 0; i < 64; i++ {
			id := int64(i)
			if i == 0 {
				id = math.MinInt64
			}
			if i == 63 {
				id = math.MaxInt64
			}
			center := float32(i/16) * 10
			d := float32(i%16) / 100
			values := []float32{center + d, center + 2*d}
			if i == 0 {
				values = []float32{math.Float32frombits(0x80000000), math.SmallestNonzeroFloat32}
			}
			v, _ := vector.New(values)
			label := fmt.Sprintf("synthetic row %d", i)
			if i == 1 {
				label = "synthetic\nmultiline label"
			}
			if format == "csv" {
				s := v.String()
				s = "{" + s[1:len(s)-1] + "}"
				if err = writer.Write([]string{strconv.FormatInt(id, 10), s, label}); err != nil {
					panic(err)
				}
			} else {
				raw, err := json.Marshal(struct {
					ID        int64           `json:"id"`
					Embedding json.RawMessage `json:"embedding"`
					Label     string          `json:"label"`
				}{id, json.RawMessage(v.String()), label})
				if err != nil {
					panic(err)
				}
				data.Write(raw)
				data.WriteByte('\n')
			}
		}
		if format == "csv" {
			writer.Flush()
			if err = writer.Error(); err != nil {
				panic(err)
			}
		}
		sum := sha256.Sum256(data.Bytes())
		count := int64(64)
		m := postgres.ImportManifest{FormatVersion: 1, SourceProfile: "pg_embedding-real-float4-v1", SourceID: "synthetic-64-v1", SourceSHA256: hex.EncodeToString(sum[:]), SourceSize: int64(data.Len()), ExpectedRows: &count, Format: format, RowOrder: "ascending", DecimalFidelity: "roundtrip_binary32", Table: "items", Fields: []postgres.ImportField{{Source: "id", Target: "id", Type: "int64", Encoding: "integer", PrimaryKey: true}, {Source: "embedding", Target: "embedding", Type: "vector", Encoding: "json_array", Dimensions: 2}, {Source: "label", Target: "label", Type: "text", Encoding: "utf8"}}, Indexes: []postgres.ImportIndex{{Name: "items_hnsw", Column: "embedding", Method: "hnsw", Metric: "l2", M: 4, EFConstruction: 32}, {Name: "items_ivf", Column: "embedding", Method: "ivfflat", Metric: "l2", Lists: 4, TrainingSamples: 64, TrainingIterations: 10}}}
		if format == "csv" {
			m.NullToken = `\N`
			m.Fields[1].Encoding = "pg_array"
		}
		r := result{Format: format, Input: filepath.Join(*dir, "source."+format), Manifest: filepath.Join(*dir, format+"-manifest.json"), Database: filepath.Join(*dir, format+".db")}
		raw, err := json.MarshalIndent(m, "", "  ")
		if err != nil {
			panic(err)
		}
		if err = writeNew(r.Input, data.Bytes()); err != nil {
			panic(err)
		}
		if err = writeNew(r.Manifest, append(raw, '\n')); err != nil {
			panic(err)
		}
		r.DryRun, err = postgres.Import(ctx, r.Input, r.Database, m, postgres.ImportOptions{})
		if err != nil {
			panic(err)
		}
		r.Paused, err = postgres.Import(ctx, r.Input, r.Database, m, postgres.ImportOptions{Apply: true, BatchRows: 8, StopAfterBatches: 1})
		if err != nil {
			panic(err)
		}
		r.Complete, err = postgres.Import(ctx, r.Input, r.Database, m, postgres.ImportOptions{Apply: true, Resume: true, BatchRows: 16})
		if err != nil {
			panic(err)
		}
		r.Verified, err = postgres.VerifyImport(ctx, r.Input, r.Database, m)
		if err != nil {
			panic(err)
		}
		db, err := sqlite.Open(r.Database, sqlite.Options{ReadOnly: true, NoCreate: true})
		if err != nil {
			panic(err)
		}
		q, _ := vector.New([]float32{20.055, 20.111})
		exact, err := db.VectorSearch(ctx, "items", "embedding", q, sqlite.VectorSearchOptions{Metric: vector.L2, Limit: 4})
		if err != nil {
			panic(err)
		}
		for _, method := range []string{"hnsw", "ivf"} {
			opts := sqlite.VectorSearchOptions{Mode: sqlite.VectorModeApproximate, Index: "items_" + method, Metric: vector.L2, Limit: 4}
			if method == "hnsw" {
				opts.EFSearch = 64
			} else {
				opts.Probes = 1
				opts.MaxProbes = 1
			}
			found, err := db.VectorSearch(ctx, "items", "embedding", q, opts)
			if err != nil {
				panic(err)
			}
			matches := 0
			for _, hit := range found.Hits {
				for _, want := range exact.Hits {
					if hit.RowID == want.RowID {
						matches++
					}
				}
			}
			recall := float64(matches) / float64(len(exact.Hits))
			if method == "hnsw" {
				r.HNSWRecallAt4 = recall
				r.HNSWPlan = found.Plan.Detail()
			} else {
				r.IVFRecallAt4 = recall
				r.IVFPlan = found.Plan.Detail()
			}
		}
		if err = db.Close(); err != nil {
			panic(err)
		}
		results = append(results, r)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err = enc.Encode(results); err != nil {
		panic(err)
	}
}
