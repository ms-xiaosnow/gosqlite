package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

// Published expected-output data is retained at the repository's existing
// pgvector v0.8.7 pin. These SQL tests do not run PostgreSQL or pgvector.
func TestHalfSQLPublishedPGVectorFixtures(t *testing.T) {
	data, e := os.ReadFile("../testdata/pgvector-v0.8.7/extended/golden.json")
	if e != nil {
		t.Fatal(e)
	}
	var f struct {
		Parse []struct {
			Kind, Input, Statement string
			Dimensions             *int
			Output, Error          *string
		}
		Metrics []struct {
			Kind, Operation, A, Policy, Statement string
			B, Output, Error                      *string
		}
		Quantize []struct {
			Kind, Input, Statement string
			Output, Error          *string
		}
		Subvector []struct {
			Kind, Input, Statement string
			Start, Count           int32
			Output, Error          *string
		}
	}
	if e = json.Unmarshal(data, &f); e != nil {
		t.Fatal(e)
	}
	db, _ := reviewVectorDB(t, Options{})
	ctx := context.Background()
	scalar := func(query string, args ...any) (any, error) {
		r, e := db.Query(ctx, query, args...)
		if e != nil {
			return nil, e
		}
		defer r.Close()
		if !r.Next() {
			return nil, r.Err()
		}
		v := r.Values()[0]
		if e = r.Close(); e != nil {
			return nil, e
		}
		return v, nil
	}
	counts := [4]int{}
	for i, c := range f.Parse {
		if c.Kind != "halfvec" {
			continue
		}
		counts[0]++
		t.Run(c.Statement, func(t *testing.T) {
			var got any
			var err error
			if c.Dimensions == nil {
				got, err = scalar("SELECT halfvec_to_text(halfvec(?))", c.Input)
			} else {
				name := fmt.Sprintf("fixture_%d", i)
				_, err = db.Exec(ctx, fmt.Sprintf("CREATE TABLE %s(h HALFVEC(%d))", name, *c.Dimensions))
				if err == nil {
					_, err = db.Exec(ctx, "INSERT INTO "+name+" VALUES(?)", c.Input)
				}
				if err == nil {
					got, err = scalar("SELECT halfvec_to_text(h) FROM " + name)
				}
			}
			if c.Error != nil {
				if err == nil {
					t.Fatal("published error accepted", got)
				}
				return
			}
			if err != nil || c.Output == nil {
				t.Fatal(got, err)
			}
			want := mustHalfSQL(t, *c.Output).String()
			if got != want {
				t.Fatal(got, want)
			}
		})
	}
	for _, c := range f.Metrics {
		if c.Kind != "halfvec" {
			continue
		}
		counts[1]++
		t.Run(c.Statement, func(t *testing.T) {
			query := "SELECT " + c.Operation + "(halfvec(?),halfvec(?))"
			args := []any{c.A}
			if c.Operation == "norm" {
				query = "SELECT halfvec_norm(halfvec(?))"
			} else {
				args = append(args, *c.B)
			}
			got, err := scalar(query, args...)
			if c.Error != nil {
				if !errors.Is(err, vector.ErrDimensions) {
					t.Fatal(got, err)
				}
				return
			}
			if c.Policy == "zero_norm_error" {
				if !errors.Is(err, vector.ErrZeroNorm) {
					t.Fatal(got, err)
				}
				return
			}
			if err != nil || c.Output == nil || c.Policy != "tolerance" {
				t.Fatal(got, err, c.Policy)
			}
			want, e := strconv.ParseFloat(*c.Output, 64)
			if e != nil {
				t.Fatal(e)
			}
			if math.Abs(got.(float64)-want) > 1e-6+1e-5*math.Abs(want) {
				t.Fatal(got, want)
			}
		})
	}
	for _, c := range f.Quantize {
		if c.Kind != "halfvec" {
			continue
		}
		counts[2]++
		t.Run(c.Statement, func(t *testing.T) {
			got, err := scalar("SELECT bit_to_text(binary_quantize(halfvec(?)))", c.Input)
			if c.Error != nil {
				if err == nil {
					t.Fatal(got)
				}
				return
			}
			if err != nil || c.Output == nil || got != *c.Output {
				t.Fatal(got, err)
			}
		})
	}
	for _, c := range f.Subvector {
		if c.Kind != "halfvec" {
			continue
		}
		counts[3]++
		t.Run(c.Statement, func(t *testing.T) {
			got, err := scalar("SELECT halfvec_to_text(subvector(halfvec(?),?,?))", c.Input, c.Start, c.Count)
			if c.Error != nil {
				if err == nil {
					t.Fatal(got)
				}
				return
			}
			if err != nil || c.Output == nil || got != mustHalfSQL(t, *c.Output).String() {
				t.Fatal(got, err)
			}
		})
	}
	if counts != [4]int{32, 31, 3, 10} {
		t.Fatal("fixture coverage changed", counts)
	}
}
