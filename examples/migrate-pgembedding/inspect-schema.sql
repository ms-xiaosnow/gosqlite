-- REVIEWED READ-ONLY EXPORT QUERY, collector v1. No extension code is executed.
-- Run manually on the source server in this repeatable-read/read-only snapshot.
-- Change only the trusted scope schema array below. Save the single JSON result
-- as UTF-8 catalog.json. This query is source-reviewed, NOT live-PG-tested.
-- It inventories HNSW indexes and their selected strategy/support-1 signatures;
-- other index methods and arbitrary application SQL remain explicit coverage gaps.
-- Profiles require a separate reviewed all-row read and are absent by default.
BEGIN TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY;
SET LOCAL statement_timeout = '60s';
SET LOCAL lock_timeout = '5s';
SET LOCAL search_path = pg_catalog;
WITH
scope AS (SELECT ARRAY['public']::pg_catalog.text[] AS schemas),
rels AS (
 SELECT c.*, n.nspname FROM pg_catalog.pg_class c
 JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
 WHERE n.nspname = ANY ((SELECT schemas FROM scope)::pg_catalog.text[])
 AND c.relkind IN ('r','p','f','m','v')
),
exts AS (SELECT e.*, n.nspname FROM pg_catalog.pg_extension e JOIN pg_catalog.pg_namespace n ON n.oid=e.extnamespace),
members AS (
 SELECT d.classid, d.objid, d.refobjid AS extension_oid
 FROM pg_catalog.pg_depend d
 WHERE d.refclassid='pg_catalog.pg_extension'::pg_catalog.regclass AND d.deptype='e'
),
idxs AS (
 SELECT i.*, x.relname, x.reloptions, n.nspname, x.relam
 FROM pg_catalog.pg_index i JOIN pg_catalog.pg_class x ON x.oid=i.indexrelid
 JOIN pg_catalog.pg_namespace n ON n.oid=x.relnamespace
 JOIN pg_catalog.pg_am a ON a.oid=x.relam
 WHERE i.indrelid IN (SELECT oid FROM rels) AND a.amname='hnsw'
),
classes AS (
 SELECT o.*,
 (SELECT am.amopopr FROM pg_catalog.pg_amop am WHERE am.amopfamily=o.opcfamily
   AND am.amoplefttype=o.opcintype AND am.amoprighttype=o.opcintype AND am.amopstrategy=1 LIMIT 1) AS operator_oid,
 (SELECT ap.amproc FROM pg_catalog.pg_amproc ap WHERE ap.amprocfamily=o.opcfamily
   AND ap.amproclefttype=o.opcintype AND ap.amprocrighttype=o.opcintype AND ap.amprocnum=1 LIMIT 1) AS function_oid
 FROM pg_catalog.pg_opclass o
 WHERE o.oid IN (SELECT pg_catalog.unnest(indclass::pg_catalog.oid[]) FROM idxs)
),
methods AS (SELECT * FROM pg_catalog.pg_am WHERE oid IN (SELECT relam FROM idxs)
 OR amname='hnsw'),
ops AS (SELECT * FROM pg_catalog.pg_operator WHERE oid IN (SELECT operator_oid FROM classes)),
funcs AS (
 SELECT * FROM pg_catalog.pg_proc WHERE oid IN (
 SELECT function_oid FROM classes UNION SELECT oprcode::pg_catalog.oid FROM ops
 UNION SELECT amhandler::pg_catalog.oid FROM methods)
),
deps AS (
 SELECT d.* FROM pg_catalog.pg_depend d
 WHERE (d.refclassid='pg_catalog.pg_class'::pg_catalog.regclass AND d.refobjid IN (SELECT oid FROM rels))
 OR (d.refclassid='pg_catalog.pg_extension'::pg_catalog.regclass AND d.refobjid IN (SELECT oid FROM exts WHERE extname='embedding'))
)
SELECT pg_catalog.jsonb_build_object(
 'format_version',1,'collector','gosqlite-pgembedding-catalog-v1',
 'extracted_at',pg_catalog.to_char(pg_catalog.statement_timestamp() AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
 'server_version_num',pg_catalog.current_setting('server_version_num')::pg_catalog.int4,
 'scope',pg_catalog.jsonb_build_object('schemas',(SELECT schemas FROM scope),'coverage',pg_catalog.jsonb_build_array(
  pg_catalog.jsonb_build_object('category','relations','complete',false,'reason','Named schemas only; privileges and RLS require review.'),
  pg_catalog.jsonb_build_object('category','columns','complete',true,'reason','Catalog columns of the selected relations.'),
  pg_catalog.jsonb_build_object('category','indexes','complete',false,'reason','Only HNSW methods; other indexes and all opfamily strategies require review.'),
  pg_catalog.jsonb_build_object('category','types','complete',true,'reason','All visible pg_type rows.'),
  pg_catalog.jsonb_build_object('category','extension_membership','complete',true,'reason','Direct pg_depend extension memberships.'),
  pg_catalog.jsonb_build_object('category','dependencies','complete',false,'reason','Direct dependents only; transitive executable dependencies require separate review.'),
  pg_catalog.jsonb_build_object('category','policies','complete',false,'reason','RLS flag and dependency identity only; full policy definitions require review.')
 )),
 'extensions',COALESCE((SELECT pg_catalog.jsonb_agg(pg_catalog.jsonb_build_object('oid',oid,'name',extname,'version',extversion,'schema',nspname) ORDER BY oid) FROM exts),'[]'::pg_catalog.jsonb),
 'types',COALESCE((SELECT pg_catalog.jsonb_agg(pg_catalog.jsonb_build_object('oid',t.oid,'schema',n.nspname,'name',t.typname,'kind',t.typtype,'element_oid',t.typelem,'base_oid',t.typbasetype) ORDER BY t.oid) FROM pg_catalog.pg_type t JOIN pg_catalog.pg_namespace n ON n.oid=t.typnamespace),'[]'::pg_catalog.jsonb),
 'access_methods',COALESCE((SELECT pg_catalog.jsonb_agg(pg_catalog.jsonb_build_object('oid',a.oid,'name',a.amname,'extension_oid',COALESCE(m.extension_oid,0),'handler_oid',a.amhandler::pg_catalog.oid) ORDER BY a.oid) FROM methods a LEFT JOIN members m ON m.classid='pg_catalog.pg_am'::pg_catalog.regclass AND m.objid=a.oid),'[]'::pg_catalog.jsonb),
 'functions',COALESCE((SELECT pg_catalog.jsonb_agg(pg_catalog.jsonb_build_object('oid',p.oid,'schema',n.nspname,'name',p.proname,'argument_oids',p.proargtypes::pg_catalog.oid[],'return_oid',p.prorettype,'extension_oid',COALESCE(m.extension_oid,0)) ORDER BY p.oid) FROM funcs p JOIN pg_catalog.pg_namespace n ON n.oid=p.pronamespace LEFT JOIN members m ON m.classid='pg_catalog.pg_proc'::pg_catalog.regclass AND m.objid=p.oid),'[]'::pg_catalog.jsonb),
 'operators',COALESCE((SELECT pg_catalog.jsonb_agg(pg_catalog.jsonb_build_object('oid',o.oid,'schema',n.nspname,'name',o.oprname,'left_oid',o.oprleft,'right_oid',o.oprright,'function_oid',o.oprcode::pg_catalog.oid,'extension_oid',COALESCE(m.extension_oid,0)) ORDER BY o.oid) FROM ops o JOIN pg_catalog.pg_namespace n ON n.oid=o.oprnamespace LEFT JOIN members m ON m.classid='pg_catalog.pg_operator'::pg_catalog.regclass AND m.objid=o.oid),'[]'::pg_catalog.jsonb),
 'opclasses',COALESCE((SELECT pg_catalog.jsonb_agg(pg_catalog.jsonb_build_object('oid',o.oid,'schema',n.nspname,'name',o.opcname,'method_oid',o.opcmethod,'input_oid',o.opcintype,'default',o.opcdefault,'extension_oid',COALESCE(m.extension_oid,0),'operator_oid',o.operator_oid,'function_oid',o.function_oid) ORDER BY o.oid) FROM classes o JOIN pg_catalog.pg_namespace n ON n.oid=o.opcnamespace LEFT JOIN members m ON m.classid='pg_catalog.pg_opclass'::pg_catalog.regclass AND m.objid=o.oid),'[]'::pg_catalog.jsonb),
 'relations',COALESCE((SELECT pg_catalog.jsonb_agg(pg_catalog.jsonb_build_object(
  'oid',r.oid,'schema',r.nspname,'name',r.relname,'kind',r.relkind,'persistence',r.relpersistence,'rls',r.relrowsecurity,
  'parents',COALESCE((SELECT pg_catalog.jsonb_agg(i.inhparent ORDER BY i.inhseqno) FROM pg_catalog.pg_inherits i WHERE i.inhrelid=r.oid),'[]'::pg_catalog.jsonb),
  'primary_key',COALESCE((SELECT pg_catalog.to_jsonb(i.indkey::pg_catalog.int2[]) FROM pg_catalog.pg_index i WHERE i.indrelid=r.oid AND i.indisprimary),'[]'::pg_catalog.jsonb),
  'columns',COALESCE((SELECT pg_catalog.jsonb_agg(pg_catalog.jsonb_build_object('attnum',a.attnum,'name',a.attname,'type_oid',a.atttypid,'typmod',a.atttypmod,'array_dimensions',a.attndims,'not_null',a.attnotnull,'default',CASE WHEN a.attgenerated='' THEN COALESCE(pg_catalog.pg_get_expr(ad.adbin,ad.adrelid),'') ELSE '' END,'generated',CASE WHEN a.attgenerated<>'' THEN COALESCE(pg_catalog.pg_get_expr(ad.adbin,ad.adrelid),'') ELSE '' END,'identity',a.attidentity,'dropped',a.attisdropped) ORDER BY a.attnum) FROM pg_catalog.pg_attribute a LEFT JOIN pg_catalog.pg_attrdef ad ON ad.adrelid=a.attrelid AND ad.adnum=a.attnum WHERE a.attrelid=r.oid AND a.attnum>0),'[]'::pg_catalog.jsonb)
 ) ORDER BY r.oid) FROM rels r),'[]'::pg_catalog.jsonb),
 'indexes',COALESCE((SELECT pg_catalog.jsonb_agg(pg_catalog.jsonb_build_object('oid',i.indexrelid,'schema',i.nspname,'name',i.relname,'table_oid',i.indrelid,'method_oid',i.relam,'keys',i.indkey::pg_catalog.int2[],'opclasses',i.indclass::pg_catalog.oid[],'collations',i.indcollation::pg_catalog.oid[],'options',COALESCE(i.reloptions,ARRAY[]::pg_catalog.text[]),'expression',COALESCE(pg_catalog.pg_get_expr(i.indexprs,i.indrelid),''),'predicate',COALESCE(pg_catalog.pg_get_expr(i.indpred,i.indrelid),''),'valid',i.indisvalid,'ready',i.indisready,'definition',pg_catalog.pg_get_indexdef(i.indexrelid)) ORDER BY i.indexrelid) FROM idxs i),'[]'::pg_catalog.jsonb),
 'dependencies',COALESCE((SELECT pg_catalog.jsonb_agg(pg_catalog.jsonb_build_object('class',(SELECT c.relname FROM pg_catalog.pg_class c WHERE c.oid=d.classid),'oid',d.objid,'sub_id',d.objsubid,'identity',pg_catalog.pg_describe_object(d.classid,d.objid,d.objsubid),'referenced_class',(SELECT c.relname FROM pg_catalog.pg_class c WHERE c.oid=d.refclassid),'referenced_oid',d.refobjid,'referenced_sub_id',d.refobjsubid,'kind',CASE WHEN d.deptype='e' THEN 'extension_member' ELSE 'pg_depend_' || d.deptype END,'definition','') ORDER BY d.classid,d.objid,d.objsubid,d.refclassid,d.refobjid,d.refobjsubid,d.deptype) FROM deps d),'[]'::pg_catalog.jsonb),
 'profiles','[]'::pg_catalog.jsonb,
 'sql_corpus','[]'::pg_catalog.jsonb
);
COMMIT;
