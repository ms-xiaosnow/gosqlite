package vector_test

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

func TestBinaryQuantizePinned(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"[1,0,-1]", "100"}, {"[0,0.1,-0.2,-0.3,0.4,0.5,0.6,-0.7,0.8,-0.9,1]", "01001110101"},
		{"[1,2,3,-4,5,6,-7,8,1,-2,-3,4,5,-6,7,8,-1,2,3]", "1110110110011011011"},
		{"[-0,0,-1,1]", "0001"},
	} {
		d := must(t, c.in)
		h := mustHalfM9(t, c.in)
		bd, ed := vector.BinaryQuantize(d)
		bh, eh := vector.HalfBinaryQuantize(h)
		if ed != nil || eh != nil || bd.String() != c.want || bh.String() != c.want || bd.Dims() != d.Dims() {
			t.Fatal(c, bd, bh, ed, eh)
		}
	}
	d := must(t, "[1e-8,-1e-8]")
	h := mustHalfM9(t, d.String())
	bd, _ := vector.BinaryQuantize(d)
	bh, _ := vector.HalfBinaryQuantize(h)
	if bd.String() != "10" || bh.String() != "00" {
		t.Fatal("quantize after half rounding", bd, bh)
	}
	if _, err := vector.BinaryQuantize(vector.Vector{}); !errors.Is(err, vector.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := vector.HalfBinaryQuantize(vector.HalfVector{}); !errors.Is(err, vector.ErrInvalid) {
		t.Fatal(err)
	}
}

// This example uses one immutable in-memory snapshot, applies a business filter
// before candidate scoring, keeps row identities, and reranks original dense
// vectors. It uses exact transformed candidates, not a typed ANN index. Cutting
// candidates can lose a true dense neighbor; reranking does not restore recall.
func ExampleBinaryQuantize() {
	query, _ := vector.Parse("[1,0.1]")
	qbit, _ := vector.BinaryQuantize(query)
	type item struct {
		row    vector.Row
		active bool
	}
	data := []item{}
	for i, text := range []string{"[100,100]", "[1,0.2]", "[1,-0.1]", "[1,0.1]"} {
		v, _ := vector.Parse(text)
		data = append(data, item{vector.Row{RowID: int64(i + 1), Vector: v}, i != 3})
	}
	type candidate struct {
		row      vector.Row
		distance float64
	}
	candidates := []candidate{}
	for _, x := range data {
		if !x.active {
			continue
		}
		bits, _ := vector.BinaryQuantize(x.row.Vector)
		distance, _ := vector.HammingDistance(qbit, bits)
		candidates = append(candidates, candidate{x.row, distance})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].distance != candidates[j].distance {
			return candidates[i].distance < candidates[j].distance
		}
		return candidates[i].row.RowID < candidates[j].row.RowID
	})
	rows := []vector.Row{candidates[0].row, candidates[1].row}
	result, err := vector.TopK(context.Background(), query, rows, vector.SearchOptions{Metric: vector.L2, Limit: 2})
	if err != nil {
		panic(err)
	}
	fmt.Println("candidate metric: hamming; final metric: l2")
	for _, hit := range result.Hits {
		fmt.Println("row", hit.RowID)
	}
	fmt.Println("dense recall is not guaranteed by candidate reranking")
	// Output:
	// candidate metric: hamming; final metric: l2
	// row 2
	// row 1
	// dense recall is not guaranteed by candidate reranking
}
