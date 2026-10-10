import collections, hashlib, json, pathlib, statistics
root=pathlib.Path(__file__).parent
out={}
for profile in ('smoke','medium'):
 p=root/(profile+'.json')
 if not p.exists() or p.stat().st_size==0: continue
 r=json.loads(p.read_text());assert r['status']=='pass',r.get('errors')
 trials=r['trials'];groups=collections.defaultdict(list); projections=[]; correct=search=fallback=memberships=0
 for t in trials:
  assert t['extension_consistency_before_close'] and t['extension_consistency_after_reopen']
  projection={'database_bytes_before_index':t['database_bytes_before_index'],'database_bytes_with_index':t['database_bytes_with_index'],'samples':[]}
  for s in sorted(t['samples'],key=lambda x:(x['cache_condition'],x['gram_length_0_is_fallback_control'])):
   key=(s['cache_condition'],s['gram_length_0_is_fallback_control']);groups[key].append(s)
   search+=s['actual_grams_search_queries'];fallback+=s['actual_scan_fallback_queries'];memberships+=s['query_text_membership_comparisons']
   obs=[]
   for o in s['observations']:
    assert o['full_ordered_results_equal'];correct+=1
    assert o['scan_plan']['Access']=='SCAN' and o['scan_plan']['Reason']=='NOT INDEXED requested'
    assert o['scan_work']['base_row_fetches']==r['config']['rows'] and o['scan_work']['posting_keys_read']==0
    if o['query']['require_real_search']:
     assert o['automatic_plan']['Access']=='SEARCH' and o['automatic_plan']['Method']=='grams13'
     assert o['automatic_plan']['Namespace']=='LIKE_RAW_V1' and o['automatic_plan']['GramLengths']==[o['query']['gram_length']]
     assert o['automatic_work']['posting_keys_read']>0 and o['automatic_work']['base_row_fetches']<r['config']['rows']
    else: assert o['automatic_plan']['Access']=='SCAN'
    obs.append({k:v for k,v in o.items() if k not in ('automatic_ms','real_scan_ms')})
   projection['samples'].append({'cache':key[0],'length':key[1],'observations':obs})
  projections.append(hashlib.sha256(json.dumps(projection,sort_keys=True,ensure_ascii=False,separators=(',',':')).encode()).hexdigest())
 summary={'source_id':r['source_id'],'workload_sha256':r['workload_sha256'],'elapsed_seconds':r['elapsed_seconds'],'rows':r['config']['rows'],'queries_per_length':r['config']['queries_per_gram_length'],'repeats':len(trials),'full_query_pairs_equal':correct,'actual_search_query_pairs':search,'truthful_fallback_query_pairs':fallback,'membership_comparisons':memberships,'build_ms':[t['index_build_ms'] for t in trials],'before_bytes':[t['database_bytes_before_index'] for t in trials],'after_bytes':[t['database_bytes_with_index'] for t in trials],'incremental_bytes':[t['incremental_database_bytes'] for t in trials],'database_amplification':[t['database_with_index_over_before'] for t in trials],'index_over_utf8_text':[t['incremental_bytes_over_utf8_text_bytes'] for t in trials],'database_sha256':[t['database_sha256_after_build_close'] for t in trials],'deterministic_result_work_projection_sha256':projections,'projection_identical_across_repeats':len(set(projections))==1,'query_summary':[]}
 assert summary['projection_identical_across_repeats']
 for (cache,n),samples in sorted(groups.items()):
  summary['query_summary'].append({'cache':cache,'gram_length':n,'automatic_p50_ms_by_repeat':[s['automatic_query_latency']['p50_ms'] for s in samples],'automatic_p95_ms_by_repeat':[s['automatic_query_latency']['p95_ms'] for s in samples],'scan_p50_ms_by_repeat':[s['real_scan_query_latency']['p50_ms'] for s in samples],'scan_p95_ms_by_repeat':[s['real_scan_query_latency']['p95_ms'] for s in samples],'mean_automatic_source_fetches':statistics.mean(o['automatic_work']['base_row_fetches'] for s in samples for o in s['observations']),'mean_posting_keys_read':statistics.mean(o['automatic_work']['posting_keys_read'] for s in samples for o in s['observations']),'mean_scan_source_fetches':statistics.mean(o['scan_work']['base_row_fetches'] for s in samples for o in s['observations'])})
 out[profile]=summary
(root/'measurements-summary.json').write_text(json.dumps(out,indent=2)+'\n')
print(json.dumps(out,indent=2))
