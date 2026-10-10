-- READ-ONLY REFERENCE. Review before manual execution; this generator never connects.
-- Exhaustive profiling may be expensive. Any timeout/error, missing row, unexpected identity or bad count means STOP.
-- Compare a fresh complete collector snapshot separately; this query is not the canonical Go catalog fingerprint.
-- Expected catalog SHA-256: 9a28597fd6836c6f790a17e33b851d921be04ea78f36da145850255e9c982cf7
-- Expected dependencies SHA-256: 4fee66cf2d623d36210576acf859c4b358c66a7977d81907c10c7c410a0b54fd
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
FROM pg_catalog.pg_namespace AS n WHERE n.nspname IN (E'public',E'public',E'') ORDER BY n.nspname;
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

-- FULL-TABLE SUPPLEMENT COLLECTION REFERENCE. Still read-only. This is not a JSON collector or a completeness/authenticity proof.
-- Recollect every declared category under owner/admin visibility and compare canonical hashes before any reviewed mutation.
BEGIN TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY;
SET LOCAL search_path = pg_catalog;
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '60s';
SELECT c.oid,c.relowner,c.relnamespace,c.reltype,t.typarray,c.reltoastrelid,c.relam,c.reltablespace,c.reloptions,c.relreplident,c.relforcerowsecurity,c.relacl,pg_catalog.obj_description(c.oid,'pg_class') AS comment
FROM pg_catalog.pg_class AS c JOIN pg_catalog.pg_type AS t ON t.oid=c.reltype WHERE c.oid=30000;
SELECT a.attnum,a.attcollation,a.attstorage,a.attcompression,a.attstattarget,a.attoptions,a.attacl,pg_catalog.col_description(a.attrelid,a.attnum) AS comment
FROM pg_catalog.pg_attribute AS a WHERE a.attrelid=30000 AND a.attnum>0 ORDER BY a.attnum;
SELECT ad.oid,ad.adnum,pg_catalog.pg_get_expr(ad.adbin,ad.adrelid) AS expression FROM pg_catalog.pg_attrdef AS ad WHERE ad.adrelid=30000 ORDER BY ad.adnum;
SELECT c.oid,c.conname,c.contype,c.conkey,c.conindid,c.convalidated,c.condeferrable,c.condeferred,c.connoinherit,c.conrelid,c.confrelid,pg_catalog.pg_get_expr(c.conbin,c.conrelid) AS expression,pg_catalog.pg_get_constraintdef(c.oid) AS definition
FROM pg_catalog.pg_constraint AS c WHERE c.conrelid=30000 OR c.confrelid=30000 ORDER BY c.oid;
SELECT i.*,c.reltablespace,c.reloptions,pg_catalog.obj_description(c.oid,'pg_class') AS comment
FROM pg_catalog.pg_index AS i JOIN pg_catalog.pg_class AS c ON c.oid=i.indexrelid WHERE i.indrelid=30000 ORDER BY i.indexrelid;
SELECT d.*,s.relowner,n.nspname,s.relname,s.relpersistence,q.*
FROM pg_catalog.pg_depend AS d JOIN pg_catalog.pg_class AS s ON d.classid='pg_catalog.pg_class'::pg_catalog.regclass AND s.oid=d.objid AND s.relkind='S'
JOIN pg_catalog.pg_namespace AS n ON n.oid=s.relnamespace JOIN pg_catalog.pg_sequence AS q ON q.seqrelid=s.oid
WHERE d.refclassid='pg_catalog.pg_class'::pg_catalog.regclass AND d.refobjid=30000 ORDER BY s.oid;
SELECT oid,nspname FROM pg_catalog.pg_namespace ORDER BY oid;
SELECT oid,evtname,evtevent,evtowner,evtfoid,evtenabled,evttags FROM pg_catalog.pg_event_trigger ORDER BY oid;
SELECT oid,rolname FROM pg_catalog.pg_roles ORDER BY oid;
SELECT * FROM pg_catalog.pg_default_acl WHERE defaclrole=10 ORDER BY oid;
SELECT * FROM pg_catalog.pg_shdepend WHERE classid='pg_catalog.pg_class'::pg_catalog.regclass AND objid=30000;
SELECT * FROM pg_catalog.pg_publication_rel WHERE prrelid=30000;
SELECT * FROM pg_catalog.pg_publication WHERE puballtables;
SELECT * FROM pg_catalog.pg_publication_namespace WHERE pnnspid=2200;
SELECT * FROM pg_catalog.pg_seclabel WHERE classoid='pg_catalog.pg_class'::pg_catalog.regclass AND objoid=30000;
SELECT * FROM pg_catalog.pg_statistic_ext WHERE stxrelid=30000;
COMMIT;
-- Inspect transitive dependencies of the table, row types, indexes, defaults and sequences, including external consumers. Expand ACLs (including defaults) with exact grantor/grantee OIDs. Null ACL uses acldefault, not an empty grant list.
-- Database-wide/all-table and schema publications, rules/triggers/functions/views, security labels, extended statistics, RLS and inheritance require explicit inventory; unsupported presence blocks this profile.
