import hashlib,json,math,pathlib,statistics,sys,unicodedata
report_path,corpus_path=map(pathlib.Path,sys.argv[1:])
r=json.loads(report_path.read_text());c=json.loads(corpus_path.read_text())
assert r['status']=='pass' and not r['errors'] and not r.get('error')
assert r['source_id']=='tree:762a9e9dd8105cf620c68a8398922ddf254a9d5f'
assert r['go_version']=='go1.27.2' and r['goos']=='linux' and r['goarch']=='amd64'
assert r['dataset_seed']==c['dataset_seed']==2026101052
assert r['page_size']==r['config']['page_size']==4096
assert r['config']['rows']==len(c['texts'])
assert all(unicodedata.normalize('NFC',x)==x for x in c['texts'])
compact=json.dumps(c,ensure_ascii=False,separators=(',',':')).encode()
assert r['workload_sha256']==hashlib.sha256(compact).hexdigest()
assert r['source_utf8_text_bytes']==sum(len(x.encode()) for x in c['texts'])
assert any(s['Key']=='CGO_ENABLED' and s['Value']=='0' for s in r['go_build_settings'])
assert len(r['trials'])==r['config']['independent_database_builds']

def close(a,b): return math.isclose(a,b,rel_tol=1e-12,abs_tol=1e-12)
def dist(d,vs):
 s=sorted(vs);n=len(s)
 assert d['sample_count']==n
 assert d['p50_ms']==s[math.ceil(.50*n)-1] and d['p95_ms']==s[math.ceil(.95*n)-1]
 assert d['min_ms']==min(s) and d['max_ms']==max(s)
 assert close(d['mean_ms'],statistics.mean(s))
 assert close(d['sample_stddev_ms'],statistics.stdev(s) if n>1 else 0)
 assert d['ci_low_ms'] is None and d['ci_high_ms'] is None and 'no inferential CI' in d['confidence_method']

observations=memberships=searches=fallbacks=0
for t in r['trials']:
 assert t['extension_consistency_before_close'] and t['extension_consistency_after_reopen']
 assert not t.get('retained_temporary_directory_on_error')
 assert len(t['database_sha256_after_build_close'])==64
 before,after=t['database_bytes_before_index'],t['database_bytes_with_index']
 assert after>before>0 and t['incremental_database_bytes']==after-before
 assert close(t['database_with_index_over_before'],after/before)
 assert close(t['incremental_bytes_over_utf8_text_bytes'],(after-before)/r['source_utf8_text_bytes'])
 assert len(t['samples'])==8
 assert {(s['cache_condition'],s['gram_length_0_is_fallback_control']) for s in t['samples']}=={(cache,n) for cache in ['cold_connection','warm_process'] for n in [0,1,2,3]}
 for s in t['samples']:
  length=s['gram_length_0_is_fallback_control'];obs=s['observations'];n=len(obs)
  assert n==(2 if length==0 else r['config']['queries_per_gram_length'])
  assert [o['query'] for o in obs]==[q for q in c['queries'] if q['gram_length']==length]
  assert s['query_text_membership_comparisons']==n*len(c['texts'])
  assert s['actual_grams_search_queries']==(n if length else 0)
  assert s['actual_scan_fallback_queries']==(0 if length else n)
  dist(s['automatic_query_latency'],[o['automatic_ms'] for o in obs]);dist(s['real_scan_query_latency'],[o['real_scan_ms'] for o in obs])
  for o in obs:
   q=o['query'];pattern=q['pattern'];needle=pattern[1:-1] if pattern!='%' else ''
   ids=[i+1 for i,text in enumerate(c['texts']) if needle in text]
   assert o['full_ordered_results_equal']
   assert o['result_sha256']==hashlib.sha256(json.dumps(ids,separators=(',',':')).encode()).hexdigest()
   sp,ap,sw,aw=o['scan_plan'],o['automatic_plan'],o['scan_work'],o['automatic_work']
   assert sp['Access']=='SCAN' and sp['Reason']=='NOT INDEXED requested'
   assert sw==dict(frequency_lookups=0,posting_keys_read=0,candidates=0,base_row_fetches=len(c['texts']),returned_rows=len(ids))
   if length:
    assert ap['Access']=='SEARCH' and ap['Method']=='grams13' and ap['Namespace']=='LIKE_RAW_V1' and ap['Recheck'] and ap['GramLengths']==[length]
    assert aw==dict(frequency_lookups=1,posting_keys_read=len(ids),candidates=len(ids),base_row_fetches=len(ids),returned_rows=len(ids))
    assert len(needle)==length and len(ids)>0
    searches+=1
   else:
    assert ap['Access']=='SCAN' and ap['Reason']
    assert aw==dict(frequency_lookups=(4 if needle else 0),posting_keys_read=0,candidates=0,base_row_fetches=len(c['texts']),returned_rows=len(ids))
    fallbacks+=1
   observations+=1;memberships+=len(c['texts'])
dist(r['index_build_latency_across_repeats'],[t['index_build_ms'] for t in r['trials']])
print(json.dumps({'report':str(report_path),'corpus':str(corpus_path),'status':'PASS','independent_substring_result_hash_checks':observations,'memberships':memberships,'actual_searches':searches,'actual_fallbacks':fallbacks,'quantiles_space_and_exact_logical_counters':'recomputed independently','workload_sha256':r['workload_sha256'],'elapsed_seconds':r['elapsed_seconds'],'builds_ms':[t['index_build_ms'] for t in r['trials']]},indent=2))
