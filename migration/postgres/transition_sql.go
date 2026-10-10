package postgres

import (
	"fmt"
	"strings"
)

const transitionLocalSettings = "SET LOCAL search_path = pg_catalog;\nSET LOCAL lock_timeout = '5s';\nSET LOCAL statement_timeout = '60s';\n"

func (m *transitionModel) preflight() string {
	var b strings.Builder
	b.WriteString("-- READ-ONLY REFERENCE. Review before manual execution; this generator never connects.\n-- Exhaustive profiling may be expensive. Any timeout/error, missing row, unexpected identity or bad count means STOP.\n-- Compare a fresh complete collector snapshot separately; this query is not the canonical Go catalog fingerprint.\n")
	fmt.Fprintf(&b, "-- Expected catalog SHA-256: %s\n-- Expected dependencies SHA-256: %s\n", m.r.CatalogSHA256, m.r.DependencySHA256)
	b.WriteString("BEGIN TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY;\n" + transitionLocalSettings)
	b.WriteString("SELECT pg_catalog.current_setting('server_version_num') AS server_version_num,\n       pg_catalog.current_setting('standard_conforming_strings') AS standard_conforming_strings, current_user;\nSELECT e.oid, e.extname, e.extversion, n.nspname AS extension_schema\nFROM pg_catalog.pg_extension AS e JOIN pg_catalog.pg_namespace AS n ON n.oid=e.extnamespace\nWHERE e.extname IN ('embedding','vector') ORDER BY e.extname;\nSELECT name, version, installed, superuser, relocatable, schema\nFROM pg_catalog.pg_available_extension_versions WHERE name IN ('embedding','vector') ORDER BY name,version;\n-- hnsw is database-wide. Confirm exact extension ownership, not just the name.\nSELECT am.oid, am.amname, am.amhandler, e.extname, e.extversion\nFROM pg_catalog.pg_am AS am LEFT JOIN pg_catalog.pg_depend AS d\n ON d.classid='pg_catalog.pg_am'::pg_catalog.regclass AND d.objid=am.oid AND d.objsubid=0 AND d.deptype='e'\n LEFT JOIN pg_catalog.pg_extension AS e ON d.refclassid='pg_catalog.pg_extension'::pg_catalog.regclass AND d.refobjid=e.oid\nWHERE am.amname IN ('hnsw','ivfflat') ORDER BY am.amname;\n")
	fmt.Fprintf(&b, "-- Expected source OID/name/type must match exactly; no row is a failure.\nSELECT c.oid,n.nspname,c.relname,c.relkind,c.relpersistence,c.relrowsecurity,c.relforcerowsecurity,c.relowner,\n       a.attnum,a.attname,a.atttypid,a.atttypmod,a.attndims,a.attnotnull,a.attgenerated,a.attidentity,\n       pg_catalog.pg_get_expr(ad.adbin,ad.adrelid) AS default_expression,\n       pg_catalog.pg_total_relation_size(c.oid) AS total_relation_bytes,\n       pg_catalog.has_table_privilege(c.oid,'SELECT') AS can_select,\n       pg_catalog.has_table_privilege(c.oid,'UPDATE') AS can_update\nFROM pg_catalog.pg_class AS c JOIN pg_catalog.pg_namespace AS n ON n.oid=c.relnamespace\nJOIN pg_catalog.pg_attribute AS a ON a.attrelid=c.oid\nLEFT JOIN pg_catalog.pg_attrdef AS ad ON ad.adrelid=c.oid AND ad.adnum=a.attnum\nWHERE c.oid=%d AND a.attnum IN (%d,%d) ORDER BY a.attnum;\n", m.table.OID, m.column.AttNum, m.pk.AttNum)
	fmt.Fprintf(&b, "SELECT n.oid,n.nspname,n.nspowner,pg_catalog.has_schema_privilege(n.oid,'USAGE') AS can_use,\n pg_catalog.has_schema_privilege(n.oid,'CREATE') AS can_create\nFROM pg_catalog.pg_namespace AS n WHERE n.nspname IN (%s,%s,%s) ORDER BY n.nspname;\n", transitionLiteral(m.table.Schema), transitionLiteral(m.o.VectorSchema), transitionLiteral(m.o.StateSchema))
	b.WriteString("-- Complete transitive dependencies on embedding and the selected relation.\n-- UNION deduplicates object addresses; timeout bounds the server traversal. Definitions remain evidence, never commands.\nWITH RECURSIVE affected(classid,objid,objsubid) AS (\n")
	fmt.Fprintf(&b, " SELECT 'pg_catalog.pg_extension'::pg_catalog.regclass::pg_catalog.oid,%d::pg_catalog.oid,0\n UNION SELECT 'pg_catalog.pg_class'::pg_catalog.regclass::pg_catalog.oid,%d::pg_catalog.oid,0\n UNION SELECT d.classid,d.objid,d.objsubid FROM pg_catalog.pg_depend AS d JOIN affected AS a\n ON d.refclassid=a.classid AND d.refobjid=a.objid AND (a.objsubid=0 OR d.refobjsubid=a.objsubid)\n) SELECT a.classid,a.objid,a.objsubid,pg_catalog.pg_describe_object(a.classid,a.objid,a.objsubid) AS identity,\n d.refclassid,d.refobjid,d.refobjsubid,d.deptype\nFROM affected AS a LEFT JOIN pg_catalog.pg_depend AS d ON d.classid=a.classid AND d.objid=a.objid AND d.objsubid=a.objsubid\nORDER BY a.classid,a.objid,a.objsubid,d.refclassid,d.refobjid,d.refobjsubid;\n", m.old.OID, m.table.OID)
	fmt.Fprintf(&b, "SELECT i.indexrelid,n.nspname,c.relname,am.amname,i.indisvalid,i.indisready,i.indkey,i.indclass,i.indcollation,c.reloptions,\n pg_catalog.pg_get_indexdef(i.indexrelid) AS definition\nFROM pg_catalog.pg_index AS i JOIN pg_catalog.pg_class AS c ON c.oid=i.indexrelid\nJOIN pg_catalog.pg_namespace AS n ON n.oid=c.relnamespace JOIN pg_catalog.pg_am AS am ON am.oid=c.relam\nWHERE i.indrelid=%d ORDER BY i.indexrelid;\n", m.table.OID)
	fmt.Fprintf(&b, "-- Also inspect inheritance/partitioning, triggers, rules, constraints, policies and privileges; owner visibility is required.\nSELECT * FROM pg_catalog.pg_inherits WHERE inhrelid=%d OR inhparent=%d;\nSELECT oid,tgname,tgenabled,tgisinternal,pg_catalog.pg_get_triggerdef(oid) AS definition FROM pg_catalog.pg_trigger WHERE tgrelid=%d ORDER BY oid;\nSELECT oid,conname,contype,pg_catalog.pg_get_constraintdef(oid) AS definition FROM pg_catalog.pg_constraint WHERE conrelid=%d OR confrelid=%d ORDER BY oid;\nSELECT * FROM pg_catalog.pg_policy WHERE polrelid=%d ORDER BY oid;\nSELECT oid,rulename,pg_catalog.pg_get_ruledef(oid) AS definition FROM pg_catalog.pg_rewrite WHERE ev_class=%d ORDER BY oid;\n", m.table.OID, m.table.OID, m.table.OID, m.table.OID, m.table.OID, m.table.OID, m.table.OID)
	a := quoteIdent(m.column.Name)
	pk := quoteIdent(m.pk.Name)
	fmt.Fprintf(&b, "-- No unverified extension function is evaluated. CASE does not rely on AND evaluation order.\nSELECT pg_catalog.count(*) AS rows, pg_catalog.count(*) FILTER (WHERE %s IS NULL) AS null_rows,\n pg_catalog.count(*) FILTER (WHERE %s) AS bad_shape_rows,\n pg_catalog.count(*) FILTER (WHERE EXISTS (SELECT 1 FROM pg_catalog.unnest(%s) AS v(x) WHERE x IS NULL)) AS null_element_rows,\n pg_catalog.count(*) FILTER (WHERE EXISTS (SELECT 1 FROM pg_catalog.unnest(%s) AS v(x) WHERE x IN ('NaN'::pg_catalog.float4,'Infinity'::pg_catalog.float4,'-Infinity'::pg_catalog.float4))) AS nonfinite_rows,\n pg_catalog.count(*) FILTER (WHERE CASE WHEN %s IS NULL THEN false WHEN %s THEN false ELSE NOT EXISTS (SELECT 1 FROM pg_catalog.unnest(%s) AS v(x) WHERE x IS NULL OR x <> 0::pg_catalog.float4) END) AS zero_norm_rows,\n pg_catalog.count(DISTINCT %s) AS distinct_primary_keys, pg_catalog.min(%s) AS min_primary_key, pg_catalog.max(%s) AS max_primary_key\nFROM %s;\n", a, m.badShape(a), a, a, a, m.badShape(a), a, pk, pk, pk, m.tableSQL())
	b.WriteString("-- All bad counts must be zero; NULL policy and zero-norm handling need explicit review.\n-- Record source count, PK coverage, business fixtures and legacy distances before phase 10.\nCOMMIT;\n")
	return b.String()
}

func (m *transitionModel) badShape(a string) string {
	return fmt.Sprintf("CASE WHEN %s IS NULL THEN false WHEN pg_catalog.array_ndims(%s) IS DISTINCT FROM 1 THEN true WHEN pg_catalog.array_lower(%s,1) IS DISTINCT FROM 1 THEN true WHEN pg_catalog.array_length(%s,1) IS DISTINCT FROM %d THEN true ELSE false END", a, a, a, a, m.o.Dimensions)
}

func (m *transitionModel) targetChecks() string {
	var b strings.Builder
	b.WriteString("-- STOP unless exact installed version/schema, method ownership, type/casts, operators, functions and opclasses match pinned source.\n")
	fmt.Fprintf(&b, "SELECT e.oid,e.extversion,n.nspname FROM pg_catalog.pg_extension AS e JOIN pg_catalog.pg_namespace AS n ON n.oid=e.extnamespace\nWHERE e.extname='vector' AND e.extversion='0.8.7' AND n.nspname=%s;\n", transitionLiteral(m.o.VectorSchema))
	b.WriteString("SELECT am.amname,am.amhandler,e.extname FROM pg_catalog.pg_am AS am JOIN pg_catalog.pg_depend AS d\n ON d.classid='pg_catalog.pg_am'::pg_catalog.regclass AND d.objid=am.oid AND d.deptype='e'\nJOIN pg_catalog.pg_extension AS e ON d.refclassid='pg_catalog.pg_extension'::pg_catalog.regclass AND d.refobjid=e.oid\nWHERE am.amname IN ('hnsw','ivfflat') ORDER BY am.amname;\n")
	fmt.Fprintf(&b, "SELECT t.oid,n.nspname,t.typname,t.typtype,d.refobjid AS owning_extension FROM pg_catalog.pg_type AS t\nJOIN pg_catalog.pg_namespace AS n ON n.oid=t.typnamespace LEFT JOIN pg_catalog.pg_depend AS d\n ON d.classid='pg_catalog.pg_type'::pg_catalog.regclass AND d.objid=t.oid AND d.deptype='e'\nWHERE n.nspname=%s AND t.typname='vector';\n", transitionLiteral(m.o.VectorSchema))
	fmt.Fprintf(&b, "SELECT c.castsource,c.casttarget,c.castfunc,c.castcontext,c.castmethod\nFROM pg_catalog.pg_cast AS c JOIN pg_catalog.pg_type AS t ON t.oid=c.casttarget OR t.oid=c.castsource\nJOIN pg_catalog.pg_namespace AS n ON n.oid=t.typnamespace\nWHERE n.nspname=%s AND t.typname='vector' AND (c.castsource=1021 OR c.casttarget=1021) ORDER BY c.castsource,c.casttarget;\n", transitionLiteral(m.o.VectorSchema))
	fmt.Fprintf(&b, "SELECT oc.oid,n.nspname,oc.opcname,am.amname,oc.opcintype,e.extname,ao.amopstrategy,ao.amopopr,ap.amprocnum,ap.amproc\nFROM pg_catalog.pg_opclass AS oc JOIN pg_catalog.pg_namespace AS n ON n.oid=oc.opcnamespace\nJOIN pg_catalog.pg_am AS am ON am.oid=oc.opcmethod LEFT JOIN pg_catalog.pg_depend AS d\n ON d.classid='pg_catalog.pg_opclass'::pg_catalog.regclass AND d.objid=oc.oid AND d.deptype='e'\nLEFT JOIN pg_catalog.pg_extension AS e ON d.refclassid='pg_catalog.pg_extension'::pg_catalog.regclass AND d.refobjid=e.oid\nLEFT JOIN pg_catalog.pg_amop AS ao ON ao.amopfamily=oc.opcfamily\nLEFT JOIN pg_catalog.pg_amproc AS ap ON ap.amprocfamily=oc.opcfamily\nWHERE n.nspname=%s AND oc.opcname IN ('vector_l2_ops','vector_cosine_ops','vector_l1_ops')\nORDER BY am.amname,oc.opcname,ao.amopstrategy,ap.amprocnum;\n", transitionLiteral(m.o.VectorSchema))
	fmt.Fprintf(&b, "SELECT o.oid,o.oprname,o.oprleft,o.oprright,o.oprcode,o.oprresult,e.extname\nFROM pg_catalog.pg_operator AS o JOIN pg_catalog.pg_namespace AS n ON n.oid=o.oprnamespace\nLEFT JOIN pg_catalog.pg_depend AS d ON d.classid='pg_catalog.pg_operator'::pg_catalog.regclass AND d.objid=o.oid AND d.deptype='e'\nLEFT JOIN pg_catalog.pg_extension AS e ON d.refclassid='pg_catalog.pg_extension'::pg_catalog.regclass AND d.refobjid=e.oid\nWHERE n.nspname=%s AND o.oprname IN ('<->','<=>','<+>') ORDER BY o.oid;\n", transitionLiteral(m.o.VectorSchema))
	fmt.Fprintf(&b, "SELECT p.oid,p.proname,p.proargtypes,p.prorettype,p.provolatile,p.proisstrict,e.extname\nFROM pg_catalog.pg_proc AS p JOIN pg_catalog.pg_namespace AS n ON n.oid=p.pronamespace\nLEFT JOIN pg_catalog.pg_depend AS d ON d.classid='pg_catalog.pg_proc'::pg_catalog.regclass AND d.objid=p.oid AND d.deptype='e'\nLEFT JOIN pg_catalog.pg_extension AS e ON d.refclassid='pg_catalog.pg_extension'::pg_catalog.regclass AND d.refobjid=e.oid\nWHERE n.nspname=%s AND p.proname IN ('vector','array_to_vector','vector_to_float4','vector_dims','vector_norm','l2_distance','cosine_distance','l1_distance') ORDER BY p.oid;\n", transitionLiteral(m.o.VectorSchema))
	return b.String()
}

func (m *transitionModel) extensionSQL() string {
	var b strings.Builder
	b.WriteString("GATES: fresh_catalog, maintenance_window, snapshot_and_old_service, privileges_capacity_locks, extension_transition.\nCapture original definitions and legacy query results BEFORE this phase.\nBEGIN;\n" + transitionLocalSettings)
	fmt.Fprintf(&b, "LOCK TABLE %s IN ACCESS EXCLUSIVE MODE;\n", m.tableSQL())
	for _, x := range m.indexes {
		fmt.Fprintf(&b, "DROP INDEX %s RESTRICT;\n", objectKey(x.source.Schema, x.source.Name))
	}
	b.WriteString("-- Every other inventoried dependency needs a reviewed plan first. Unexpected dependencies must make RESTRICT fail.\nDROP EXTENSION \"embedding\" RESTRICT;\n")
	fmt.Fprintf(&b, "CREATE EXTENSION \"vector\" WITH SCHEMA %s VERSION '0.8.7';\n", quoteIdent(m.o.VectorSchema))
	b.WriteString(m.targetChecks())
	if m.o.Mode == TransitionSingle {
		b.WriteString("-- Keep this transaction and connection open for phases 20, 40, 50 and reviewed commit in 60.\n")
	} else {
		b.WriteString("-- Only after target_capabilities checks pass and the extension transition commit is explicitly approved:\nCOMMIT;\n-- Old extension/index service is now gone; a later ROLLBACK cannot restore it.\n")
	}
	return b.String()
}

func (m *transitionModel) copySQL() string {
	if m.o.Approach == RetainArrayCast {
		return "No copy: source real[] retained. Queries/indexes use explicit vector(n) casts after validated target installation.\n"
	}
	var b strings.Builder
	b.WriteString("GATES: target_capabilities, data_and_training, maintenance_window. New shadow remains nullable; source is unchanged.\n")
	if m.o.Mode == TransitionBatches {
		b.WriteString("BEGIN;\n" + transitionLocalSettings)
	}
	fmt.Fprintf(&b, "ALTER TABLE %s ADD COLUMN %s %s;\n", m.tableSQL(), quoteIdent(m.o.ShadowColumn), m.vectorType())
	if m.o.Mode == TransitionSingle {
		fmt.Fprintf(&b, "UPDATE %s SET %s = %s::%s;\n", m.tableSQL(), quoteIdent(m.o.ShadowColumn), quoteIdent(m.column.Name), m.vectorType())
	} else {
		var expected int64
		for _, p := range m.c.Profiles {
			if p.RelationOID == m.o.RelationOID && p.AttNum == m.o.AttNum {
				expected = p.Rows
			}
		}
		fmt.Fprintf(&b, "CREATE TABLE %s (\n plan_sha256 pg_catalog.text PRIMARY KEY CHECK (plan_sha256=%s),\n last_id pg_catalog.int8 NULL,\n rows_done pg_catalog.int8 NOT NULL CHECK (rows_done>=0 AND rows_done<=expected_rows),\n expected_rows pg_catalog.int8 NOT NULL CHECK (expected_rows>=0),\n batches pg_catalog.int8 NOT NULL CHECK (batches>=0),\n complete pg_catalog.bool NOT NULL\n);\nINSERT INTO %s (plan_sha256,last_id,rows_done,expected_rows,batches,complete)\nVALUES (%s,NULL,0,%d,0,false);\nCOMMIT;\n", m.stateSQL(), transitionLiteral(m.r.PlanSHA256), m.stateSQL(), transitionLiteral(m.r.PlanSHA256), expected)
	}
	return b.String()
}

func (m *transitionModel) batchSQL() string {
	if m.o.Mode != TransitionBatches || m.o.Approach != TypedCopyShadow {
		return "Not used by this approach/mode. No batch cursor or synthetic MinInt64 sentinel is needed.\n"
	}
	var b strings.Builder
	b.WriteString("Single-worker batch. Application writes and PK changes MUST remain paused between all commits.\nInspect state before retrying after an ambiguous client result. Reuse only this exact plan, never regenerate over in-progress state.\nBEGIN;\n" + transitionLocalSettings)
	pk := quoteIdent(m.pk.Name)
	fmt.Fprintf(&b, "WITH prior AS MATERIALIZED (\n SELECT plan_sha256,last_id,rows_done,expected_rows,batches FROM %s\n WHERE plan_sha256=%s AND NOT complete FOR UPDATE\n), batch AS MATERIALIZED (\n SELECT src.%s AS id FROM %s AS src CROSS JOIN prior AS p\n WHERE (p.last_id IS NULL OR src.%s > p.last_id)\n ORDER BY src.%s LIMIT %d FOR UPDATE OF src\n), changed AS (\n UPDATE %s AS dst SET %s=dst.%s::%s\n FROM batch AS b WHERE dst.%s=b.id RETURNING dst.%s AS id\n), progress AS (\n SELECT pg_catalog.count(*) AS n,pg_catalog.max(id) AS hi FROM changed\n)\nUPDATE %s AS state\nSET last_id=CASE WHEN progress.n=0 THEN state.last_id ELSE progress.hi END,\n rows_done=state.rows_done+progress.n,batches=state.batches+1,\n complete=(progress.n=0 AND state.rows_done=state.expected_rows)\nFROM progress,prior\nWHERE state.plan_sha256=prior.plan_sha256 AND state.last_id IS NOT DISTINCT FROM prior.last_id\n AND state.rows_done=prior.rows_done AND state.batches=prior.batches AND NOT state.complete\nRETURNING state.plan_sha256,state.last_id,state.rows_done,state.expected_rows,state.batches,state.complete,progress.n AS batch_rows;\n-- Require exactly one returned state row, then COMMIT success. Zero rows means inspect state; never assume data is complete.\n-- An empty batch without complete=true is a count mismatch and must stop. Source NULL rows are included in changed/count.\nCOMMIT;\n", m.stateSQL(), transitionLiteral(m.r.PlanSHA256), pk, m.tableSQL(), pk, pk, m.o.BatchSize, m.tableSQL(), quoteIdent(m.o.ShadowColumn), quoteIdent(m.column.Name), m.vectorType(), pk, pk, m.stateSQL())
	return b.String()
}

func (m *transitionModel) indexSQL() string {
	var b strings.Builder
	b.WriteString("GATES: data_and_training, target_capabilities, privileges_capacity_locks. Use populated, validated data.\nVerify intended retuning/IVF training/list/probe choices; no default option substitution.\n")
	if m.o.ConcurrentIndexes {
		b.WriteString("OUTSIDE EVERY EXPLICIT TRANSACTION. Run each CREATE INDEX CONCURRENTLY separately.\nA failed concurrent build can leave an invalid index. Inspect exact identity/definition, indisvalid/indisready before a separately reviewed retry; no IF NOT EXISTS.\n")
	} else if m.o.Mode == TransitionBatches {
		b.WriteString("BEGIN;\n" + transitionLocalSettings)
	}
	for _, x := range m.indexes {
		concurrently := ""
		if m.o.ConcurrentIndexes {
			concurrently = "CONCURRENTLY "
		}
		// CREATE INDEX takes an unqualified index name; PostgreSQL creates it in
		// the table's schema. Types and opclasses are explicitly qualified.
		fmt.Fprintf(&b, "CREATE INDEX %s%s ON %s USING %s (%s %s) WITH (", concurrently, quoteIdent(x.target.Name), m.tableSQL(), x.target.Method, m.expression(""), objectKey(m.o.VectorSchema, x.opclass))
		if x.target.Method == "hnsw" {
			fmt.Fprintf(&b, "m=%d, ef_construction=%d", x.target.M, x.target.EFConstruction)
		} else {
			fmt.Fprintf(&b, "lists=%d", x.target.Lists)
		}
		b.WriteString(");\n")
	}
	if !m.o.ConcurrentIndexes && m.o.Mode == TransitionBatches {
		b.WriteString("COMMIT;\n")
	}
	return b.String()
}

func (m *transitionModel) verifySQL() string {
	var b strings.Builder
	b.WriteString("GATES: data_and_training, target_capabilities. Keep writes paused. Source identity/profile must still match.\n")
	if m.o.Mode == TransitionBatches {
		b.WriteString("BEGIN TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY;\n" + transitionLocalSettings)
	}
	b.WriteString(m.targetChecks())
	a := quoteIdent(m.column.Name)
	// Table functions implicitly see preceding FROM items. Qualify the outer
	// values so source/shadow names x or n cannot bind to a(x,n) below.
	source := "src." + a
	v := m.expression("src")
	fmt.Fprintf(&b, "SELECT pg_catalog.count(*) AS rows,pg_catalog.count(*) FILTER (WHERE %s IS NULL) AS source_null_rows,\n pg_catalog.count(*) FILTER (WHERE (%s IS NULL) IS DISTINCT FROM (%s IS NULL)) AS null_mismatches,\n pg_catalog.count(*) FILTER (WHERE %s IS NOT NULL AND %s(%s) IS DISTINCT FROM %d) AS dimension_mismatches,\n pg_catalog.count(*) FILTER (WHERE CASE WHEN %s IS NULL THEN false ELSE EXISTS (\n  SELECT 1 FROM pg_catalog.unnest(%s) WITH ORDINALITY AS a(x,n)\n  FULL JOIN pg_catalog.unnest((%s)::pg_catalog.float4[]) WITH ORDINALITY AS b(x,n) USING(n)\n  WHERE pg_catalog.float4send(a.x) IS DISTINCT FROM pg_catalog.float4send(b.x)\n ) END) AS float32_bit_mismatches\nFROM %s AS src;\n", source, source, v, v, objectKey(m.o.VectorSchema, "vector_dims"), v, m.o.Dimensions, source, source, v, m.tableSQL())
	fmt.Fprintf(&b, "-- Norm tolerance is explicitly 1e-6 absolute plus 1e-6 relative. Not legacy float32 output-bit equivalence.\nSELECT pg_catalog.count(*) AS norm_mismatches FROM %s AS src\nWHERE CASE WHEN src.%s IS NULL THEN false ELSE\n pg_catalog.abs(%s(%s) - (SELECT pg_catalog.sqrt(pg_catalog.sum(x::pg_catalog.float8*x::pg_catalog.float8)) FROM pg_catalog.unnest(src.%s) AS v(x)))\n > 0.000001 + 0.000001 * pg_catalog.abs(%s(%s)) END;\n", m.tableSQL(), a, objectKey(m.o.VectorSchema, "vector_norm"), m.expression("src"), a, objectKey(m.o.VectorSchema, "vector_norm"), m.expression("src"))
	fmt.Fprintf(&b, "SELECT i.indexrelid,n.nspname,c.relname,i.indisvalid,i.indisready,pg_catalog.pg_get_indexdef(i.indexrelid) AS definition\nFROM pg_catalog.pg_index AS i JOIN pg_catalog.pg_class AS c ON c.oid=i.indexrelid JOIN pg_catalog.pg_namespace AS n ON n.oid=c.relnamespace\nWHERE i.indrelid=%d ORDER BY i.indexrelid;\n", m.table.OID)
	if m.o.Mode == TransitionBatches && m.o.Approach == TypedCopyShadow {
		fmt.Fprintf(&b, "SELECT * FROM %s WHERE plan_sha256=%s;\n-- Require complete=true and rows_done=expected_rows=the exhaustive source count.\n", m.stateSQL(), transitionLiteral(m.r.PlanSHA256))
	}
	b.WriteString("-- Require exact PK/row/NULL counts and zero mismatches, plus zero-policy approval.\n-- Compare captured legacy and target exact distance fixtures with declared tolerances (legacy float32 is architecture-sensitive).\n-- Measure ANN recall separately on a fixed query set against exact target scans; do not require identical ANN row order.\n-- Verify actual staging query plans, filters, NULL handling and tie behavior. No live PostgreSQL evidence is supplied by this file.\n")
	if m.o.Mode == TransitionBatches {
		b.WriteString("COMMIT;\n")
	}
	return b.String()
}

func (m *transitionModel) applicationSQL() string {
	var b strings.Builder
	b.WriteString("GATE: application_cutover. Review the concrete application deployment separately.\nNo column/table rename or app deployment is automated. Bind $1 as finite vector text with exact dimensions; $2 is a reviewed nonnegative bigint LIMIT.\nOld Manhattan <~> becomes dense <+>; do not apply this rule to bit Hamming or arbitrary text.\nNULL vectors preserve NULL; explicit ASC NULLS LAST and primary-key tie order are shown. Validate their actual plan effects.\n")
	if m.o.Mode == TransitionSingle {
		b.WriteString("-- Only after phases 10/20/40/50 pass and coordinated cutover is approved:\nCOMMIT;\n-- Everything below is a SEPARATE application transaction example after migration commit.\n")
	}
	for _, x := range m.indexes {
		fmt.Fprintf(&b, "-- Independent prepared-query example for source index OID %d; choose per-query setting, not a global overwrite.\nBEGIN;\nSET LOCAL search_path = pg_catalog;\n", x.source.OID)
		if x.target.Method == "hnsw" {
			fmt.Fprintf(&b, "SET LOCAL hnsw.ef_search = %d;\n", x.target.EFSearch)
		} else {
			fmt.Fprintf(&b, "SET LOCAL ivfflat.probes = %d;\n", x.target.Probes)
		}
		fmt.Fprintf(&b, "SELECT %s FROM %s ORDER BY %s OPERATOR(%s.%s) $1::%s ASC NULLS LAST, %s ASC LIMIT $2::pg_catalog.int8;\nCOMMIT;\n", quoteIdent(m.pk.Name), m.tableSQL(), m.expression(""), quoteIdent(m.o.VectorSchema), x.operator, m.vectorType(), quoteIdent(m.pk.Name))
	}
	b.WriteString("-- Update all application INSERT/UPDATE/parameter paths, not only SELECT. Keep source arrays current through the rollback window.\n-- The separate Convert API supports bounded AST/source-span transformations; arbitrary SQL/functions/triggers require manual review.\n")
	return b.String()
}

func (m *transitionModel) rollbackSQL() string {
	return "DATA ROLLBACK versus SERVICE ROLLBACK:\nBefore the still-open single transaction commits, the operator can issue ROLLBACK on that SAME connection.\nROLLBACK;\nAfter any earlier COMMIT, this statement cannot reverse that transaction, committed batches or the extension transition.\nBefore cutover, abandoning shadow reads leaves source arrays retained but does not restore old operators/HNSW.\nAfter cutover, pause writes and reconcile all new writes with retained arrays before reverting readers. A stale retained array is not a rollback guarantee.\nOld-ANN service restoration requires separately approved reverse migration or database/branch restore, available old binaries and dependency reconciliation. Modern vector owns hnsw and may be referenced by shadow columns/indexes.\nNo DROP of target extension, target column, old column, table, backup or state is generated. No CASCADE or automatic cleanup.\n"
}
