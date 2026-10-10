package similarity_test

import (
	"context"
	"fmt"

	sim "github.com/ms-xiaosnow/gosqlite/extension/similarity"
)

func ExampleEngine_Compare() {
	config := sim.DefaultConfig()
	config.MinGram, config.MaxGram = 2, 2
	e, err := sim.New(config)
	if err != nil {
		panic(err)
	}
	score, err := e.Compare(context.Background(), "abcd", "abce", sim.Jaccard)
	if err != nil {
		panic(err)
	}
	fmt.Println(score.Value, score.LeftTokens, score.RightTokens, score.Intersection)
	// Output: 0.5 3 3 2
}

func ExampleEngine_Levenshtein() {
	e, err := sim.New(sim.DefaultConfig())
	if err != nil {
		panic(err)
	}
	maximum := 2
	distance, err := e.Levenshtein(context.Background(), "kitten", "sitting", &maximum)
	if err != nil {
		panic(err)
	}
	fmt.Println(distance.Value, distance.Exact)
	// False means only that the actual distance exceeds 2; 3 is the sentinel.
	// Output: 3 false
}

func ExamplePlanCandidates() {
	positive, _ := sim.PlanCandidates(3, .5)
	unrestrictedTopK, _ := sim.PlanCandidates(3, 0)
	emptyQuery, _ := sim.PlanCandidates(0, 1)
	fmt.Println(positive.Mode)
	fmt.Println(unrestrictedTopK.Mode)
	fmt.Println(emptyQuery.Mode)
	// Output:
	// union-all-query-gram-postings
	// full-scan
	// full-scan
}
