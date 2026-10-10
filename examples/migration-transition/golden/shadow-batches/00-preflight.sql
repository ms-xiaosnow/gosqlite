-- READ-ONLY REFERENCE. Review before manual execution; this generator never connects.
-- Exhaustive profiling may be expensive. Any timeout/error, missing row, unexpected identity or bad count means STOP.
-- Compare a fresh complete collector snapshot separately; this query is not the canonical Go catalog fingerprint.
-- Expected catalog SHA-256: bdb6e93d8e95112231c892ad5a6653ca72ccd66297efa99446ac0f2610b8f804
-- Expected dependencies SHA-256: 4f53cda18c2baa0c0354bb5f9a3ecbe5ed12ab4d8e11ba873c2f11161202b945
BEGIN TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY;
SET LOCAL search_path = pg_catalog;
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '60s';
SELECT pg_catalog.current_setting('server_version_num') AS server_version_num,
       pg_catalog.current_setting('standard_conforming_strings') AS standard_conforming_strings, current_user;
SELECT e.oid, e.extname, e.extversion, n.nspname AS extension_schema
FROM pg_catalog.pg_extension AS e JOIN pg_catalog.pg_namespace AS n ON n.oid=e.extnamespace
WHERE e.extname IN ('embedding','vector') ORDER BY e.extname;
SELECT name, version, installed, superuser, relocatable, schema
FROM pg_catalog.pg_available_extension_versions WHERE name IN ('embedding','vector') ORDER BY name,version;
-- hnsw is database-wide. Confirm exact extension ownership, not just the name.
SELECT am.oid, am.amname, am.amhandler, e.extname, e.extversion
FROM pg_catalog.pg_am AS am LEFT JOIN pg_catalog.pg_depend AS d
 ON d.classid='pg_catalog.pg_am'::pg_catalog.regclass AND d.objid=am.oid AND d.objsubid=0 AND d.deptype='e'
 LEFT JOIN pg_catalog.pg_extension AS e ON d.refclassid='pg_catalog.pg_extension'::pg_catalog.regclass AND d.refobjid=e.oid
WHERE am.amname IN ('hnsw','ivfflat') ORDER BY am.amname;
-- Expected source OID/name/type must match exactly; no row is a failure.
SELECT c.oid,n.nspname,c.relname,c.relkind,c.relpersistence,c.relrowsecurity,c.relforcerowsecurity,c.relowner,
       a.attnum,a.attname,a.atttypid,a.atttypmod,a.attndims,a.attnotnull,a.attgenerated,a.attidentity,
       pg_catalog.pg_get_expr(ad.adbin,ad.adrelid) AS default_expression,
       pg_catalog.pg_total_relation_size(c.oid) AS total_relation_bytes,
       pg_catalog.has_table_privilege(c.oid,'SELECT') AS can_select,
       pg_catalog.has_table_privilege(c.oid,'UPDATE') AS can_update
FROM pg_catalog.pg_class AS c JOIN pg_catalog.pg_namespace AS n ON n.oid=c.relnamespace
JOIN pg_catalog.pg_attribute AS a ON a.attrelid=c.oid
LEFT JOIN pg_catalog.pg_attrdef AS ad ON ad.adrelid=c.oid AND ad.adnum=a.attnum
WHERE c.oid=30000 AND a.attnum IN (3,1) ORDER BY a.attnum;
SELECT n.oid,n.nspname,n.nspowner,pg_catalog.has_schema_privilege(n.oid,'USAGE') AS can_use,
 pg_catalog.has_schema_privilege(n.oid,'CREATE') AS can_create
FROM pg_catalog.pg_namespace AS n WHERE n.nspname IN (E'public',E'public',E'public') ORDER BY n.nspname;
-- Complete transitive dependencies on embedding and the selected relation.
-- UNION deduplicates object addresses; timeout bounds the server traversal. Definitions remain evidence, never commands.
WITH RECURSIVE affected(classid,objid,objsubid) AS (
 SELECT 'pg_catalog.pg_extension'::pg_catalog.regclass::pg_catalog.oid,20000::pg_catalog.oid,0
 UNION SELECT 'pg_catalog.pg_class'::pg_catalog.regclass::pg_catalog.oid,30000::pg_catalog.oid,0
 UNION SELECT d.classid,d.objid,d.objsubid FROM pg_catalog.pg_depend AS d JOIN affected AS a
 ON d.refclassid=a.classid AND d.refobjid=a.objid AND (a.objsubid=0 OR d.refobjsubid=a.objsubid)
) SELECT a.classid,a.objid,a.objsubid,pg_catalog.pg_describe_object(a.classid,a.objid,a.objsubid) AS identity,
 d.refclassid,d.refobjid,d.refobjsubid,d.deptype
FROM affected AS a LEFT JOIN pg_catalog.pg_depend AS d ON d.classid=a.classid AND d.objid=a.objid AND d.objsubid=a.objsubid
ORDER BY a.classid,a.objid,a.objsubid,d.refclassid,d.refobjid,d.refobjsubid;
SELECT i.indexrelid,n.nspname,c.relname,am.amname,i.indisvalid,i.indisready,i.indkey,i.indclass,i.indcollation,c.reloptions,
 pg_catalog.pg_get_indexdef(i.indexrelid) AS definition
FROM pg_catalog.pg_index AS i JOIN pg_catalog.pg_class AS c ON c.oid=i.indexrelid
JOIN pg_catalog.pg_namespace AS n ON n.oid=c.relnamespace JOIN pg_catalog.pg_am AS am ON am.oid=c.relam
WHERE i.indrelid=30000 ORDER BY i.indexrelid;
-- Also inspect inheritance/partitioning, triggers, rules, constraints, policies and privileges; owner visibility is required.
SELECT * FROM pg_catalog.pg_inherits WHERE inhrelid=30000 OR inhparent=30000;
SELECT oid,tgname,tgenabled,tgisinternal,pg_catalog.pg_get_triggerdef(oid) AS definition FROM pg_catalog.pg_trigger WHERE tgrelid=30000 ORDER BY oid;
SELECT oid,conname,contype,pg_catalog.pg_get_constraintdef(oid) AS definition FROM pg_catalog.pg_constraint WHERE conrelid=30000 OR confrelid=30000 ORDER BY oid;
SELECT * FROM pg_catalog.pg_policy WHERE polrelid=30000 ORDER BY oid;
SELECT oid,rulename,pg_catalog.pg_get_ruledef(oid) AS definition FROM pg_catalog.pg_rewrite WHERE ev_class=30000 ORDER BY oid;
-- No unverified extension function is evaluated. CASE does not rely on AND evaluation order.
SELECT pg_catalog.count(*) AS rows, pg_catalog.count(*) FILTER (WHERE "embedding" IS NULL) AS null_rows,
 pg_catalog.count(*) FILTER (WHERE CASE WHEN "embedding" IS NULL THEN false WHEN pg_catalog.array_ndims("embedding") IS DISTINCT FROM 1 THEN true WHEN pg_catalog.array_lower("embedding",1) IS DISTINCT FROM 1 THEN true WHEN pg_catalog.array_length("embedding",1) IS DISTINCT FROM 3 THEN true ELSE false END) AS bad_shape_rows,
 pg_catalog.count(*) FILTER (WHERE EXISTS (SELECT 1 FROM pg_catalog.unnest("embedding") AS v(x) WHERE x IS NULL)) AS null_element_rows,
 pg_catalog.count(*) FILTER (WHERE EXISTS (SELECT 1 FROM pg_catalog.unnest("embedding") AS v(x) WHERE x IN ('NaN'::pg_catalog.float4,'Infinity'::pg_catalog.float4,'-Infinity'::pg_catalog.float4))) AS nonfinite_rows,
 pg_catalog.count(*) FILTER (WHERE CASE WHEN "embedding" IS NULL THEN false WHEN CASE WHEN "embedding" IS NULL THEN false WHEN pg_catalog.array_ndims("embedding") IS DISTINCT FROM 1 THEN true WHEN pg_catalog.array_lower("embedding",1) IS DISTINCT FROM 1 THEN true WHEN pg_catalog.array_length("embedding",1) IS DISTINCT FROM 3 THEN true ELSE false END THEN false ELSE NOT EXISTS (SELECT 1 FROM pg_catalog.unnest("embedding") AS v(x) WHERE x IS NULL OR x <> 0::pg_catalog.float4) END) AS zero_norm_rows,
 pg_catalog.count(DISTINCT "id") AS distinct_primary_keys, pg_catalog.min("id") AS min_primary_key, pg_catalog.max("id") AS max_primary_key
FROM "public"."documents";
-- All bad counts must be zero; NULL policy and zero-norm handling need explicit review.
-- Record source count, PK coverage, business fixtures and legacy distances before phase 10.
COMMIT;
