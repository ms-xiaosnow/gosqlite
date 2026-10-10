package postgres

const SourceProfile = "pg_embedding-0.3.6@5d48508aeaeb86b9e9c630468ada7a124b905795"
const TargetProfile = "pgvector-0.8.7@f37c13f68b57d2c3472b2214fbcff699d6d34876"
const CurrentNeonURL = "https://neon.com/docs/extensions/pgvector#migrate-from-pg_embedding-to-pgvector"
const HistoricalNeonURL = "https://github.com/neondatabase/website/blob/d4c1cf37f79acbf4c02880eddb97dd9a36cb2d20/content/docs/extensions/pg_embedding.md#migrate-from-pg_embedding-to-pgvector"

type metricProfile struct{ operator, function, target, metric string }

var legacyClasses = map[string]metricProfile{
	"ann_l2_ops":        {"<->", "l2_distance", "vector_l2_ops", "l2"},
	"ann_cos_ops":       {"<=>", "cosine_distance", "vector_cosine_ops", "cosine"},
	"ann_manhattan_ops": {"<~>", "manhattan_distance", "vector_l1_ops", "l1"},
}

type LegacyOptions struct {
	Dimensions     int `json:"dims"`
	M              int `json:"m"`
	EFConstruction int `json:"efconstruction"`
	EFSearch       int `json:"efsearch"`
}
type TargetOptions struct {
	Dimensions     int    `json:"dimensions"`
	M              int    `json:"m"`
	EFConstruction int    `json:"ef_construction"`
	OpClass        string `json:"opclass"`
	Metric         string `json:"metric"`
}
