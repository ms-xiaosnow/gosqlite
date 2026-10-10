package vector_test

import (
	"context"
	"fmt"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

func ExampleTopK() {
	query, _ := vector.Parse("[1,2,3]")
	near, _ := vector.New([]float32{1, 2, 4})
	far, _ := vector.Parse("[4,6,3]")
	result, err := vector.TopK(context.Background(), query, []vector.Row{
		{RowID: 20, Vector: far},
		{RowID: 10, Vector: near},
		{RowID: 30, Null: true},
	}, vector.SearchOptions{Metric: vector.L2, Limit: 1})
	fmt.Println(err)
	fmt.Println(result.Hits[0].RowID, result.Hits[0].Distance)
	// Output:
	// <nil>
	// 10 1
}

func ExampleNullVector() {
	v, _ := vector.Parse("[1,-0]")
	blob, _ := v.Value() // pass v itself as a database/sql parameter
	var destination vector.NullVector
	_ = destination.Scan(blob)
	fmt.Println(destination.Valid, destination.Vector.String())
	_ = destination.Scan(nil)
	fmt.Println(destination.Valid)
	// Output:
	// true [1,-0]
	// false
}
