"""Independent read-only model, timing, trace and report verification."""
import hashlib,json,math,pathlib
ROOT=pathlib.Path('/workspace/scratch/5e06a74e5f65/durable-restart');OUT=pathlib.Path(__file__).parent
MASK=2**64-1
def mix(x):
 x=(x+0x9e3779b97f4a7c15)&MASK;x=((x^(x>>30))*0xbf58476d1ce4e5b9)&MASK;x=((x^(x>>27))*0x94d049bb133111eb)&MASK;return x^(x>>31)
def sha(x):return hashlib.sha256(json.dumps(x,separators=(',',':')).encode()).hexdigest()
def check(p):
 d=json.loads(p.read_text());c=d['config'];rows=c['rows'];seed=c['dataset_seed'];state=[1000+mix(seed+i)%100 for i in range(rows)];initial_total=sum(state)
 assert sha(state)==d['dataset_sha256'],(p,'dataset')
 ident=dict(c);ident['source_id']=ident['hardware_id']='';assert sha({'Version':d['workload_version'],'Config':ident})==d['workload_sha256'],(p,'workload')
 writer={k:0 for k in d['writer']};readers={k:0 for k in d['readers']};ordinal=0;commits=0;active=0;rate_samples=[]
 for i,w in enumerate(d['windows']):
  assert w['index']==i and w['first_writer_ordinal']==ordinal
  wc,rc=w['writer'],w['readers'];trace=w['writer_outcomes'];assert len(trace)==wc['attempts']<=64
  assert set(trace)<=set('CRBXF');assert len(trace)==wc['success']+wc['busy_outcomes']+wc['canceled_outcomes']+wc['failed_outcomes']
  assert wc['success']==wc['committed']+wc['intentional_rollbacks']
  for code,key in [('C','committed'),('R','intentional_rollbacks'),('B','busy_outcomes'),('X','canceled_outcomes'),('F','failed_outcomes')]:assert trace.count(code)==wc[key]
  sp={1:0,2:0}
  for code in trace:
   kind=ordinal%4
   if code=='R':assert kind==0
   if code=='C':
    assert kind!=0
    if kind in sp:sp[kind]+=1
    for salt in ([1,1000001] if kind==2 else [1]):
     for pair in range(c['transfer_pairs_per_transaction']):
      x=mix(seed^mix((ordinal*67+pair*2+salt)&MASK));a=x%rows;b=(a+1+mix(x)%(rows-1))%rows;delta=1+mix(x^17)%7
      assert a!=b;state[a]+=delta;state[b]-=delta
    commits+=1
   ordinal+=1
  assert wc['savepoint_rollback_commits']==sp[1] and wc['savepoint_release_commits']==sp[2]
  for counts,merged in [(wc,writer),(rc,readers)]:
   assert counts['attempts']==sum(counts[k] for k in ['success','busy_outcomes','canceled_outcomes','failed_outcomes'])
   for k,v in counts.items():merged[k]+=v
  assert 2*rc['success']<=rc['snapshot_checks']<=2*rc['attempts']
  active+=w['active_seconds'];rate=wc['committed']/w['active_seconds'];rate_samples.append(rate);assert math.isclose(rate,w['committed_transactions_per_second'],rel_tol=1e-12)
 assert writer==d['writer'] and readers==d['readers'];assert commits==d['final_model_generation']==writer['committed'];assert sha(state)==d['final_model_sha256'];assert sum(state)==initial_total
 assert math.isclose(active,d['active_seconds'],rel_tol=1e-12);assert d['elapsed_seconds']>=active
 for field,count in [('writer_attempt_latency',writer['attempts']),('reader_attempt_latency',readers['attempts']),('ordinary_checkpoint_latency',None),('ordinary_reopen_latency',None)]:
  h=d[field];n=h['milliseconds']['sample_count'];assert sum(h['buckets_power_of_two_microseconds'])==n
  if count is not None:assert n==count
  assert h['milliseconds']['confidence_95_low'] is None and h['milliseconds']['confidence_95_high'] is None
  for percent,name in [(50,'p50_bucket_upper_ms'),(95,'p95_bucket_upper_ms')]:
   rank=max(1,(n*percent+99)//100);total=0;quantile=0
   for j,v in enumerate(h['buckets_power_of_two_microseconds']):
    total+=v
    if n and total>=rank:quantile=2**j/1000;break
   assert quantile==h[name]
 assert math.isclose(writer['committed']/active,d['committed_transactions_per_active_second'],rel_tol=1e-12)
 assert math.isclose(writer['attempts']/active,d['writer_attempts_per_active_second'],rel_tol=1e-12)
 assert math.isclose(writer['committed']/d['elapsed_seconds'],d['committed_transactions_per_elapsed_second'],rel_tol=1e-12)
 if d['status']=='passed':
  assert not d['errors'] and d['final_model_verified'] and d['shutdown']=='workers_joined_normal_close'
  if c['mode']=='soak':assert active>=c['requested_active_duration_ns']/1e9
  else:assert ordinal==c['requested_writer_attempts']
 else:assert d['status']=='failed' and d['errors']
 return {'path':str(p.relative_to(ROOT)),'sha256':hashlib.sha256(p.read_bytes()).hexdigest(),'status':d['status'],'active_seconds':active,'attempts':ordinal,'commits':commits,'read_success':readers['success'],'reconstructed_model_sha256':sha(state),'errors':d['errors']}
reports=list((ROOT/'transaction-soak-evidence-v4').glob('throughput-v4-*.json'))
result=[check(p) for p in sorted(reports)];assert len(result)==3
for p in reports:
 d=json.loads(p.read_text());assert d['harness_revision']=='ordinary-transactions-v4' and not d['unverified_retirement'] and d['close_failures']==0 and d['unverified_close_handles']==0 and d['writer']['attempts']==64 and d['writer']['committed']==48
prior=json.loads((ROOT/'transaction-soak-independent-review-v3/report-verification.json').read_text())
for rec in prior['checked']:
 p=ROOT/rec['path'];assert hashlib.sha256(p.read_bytes()).hexdigest()==rec['sha256']
for rec in prior['wal_guard_failed_data_inventory']:
 p=pathlib.Path(rec['path']);assert p.stat().st_size==rec['bytes'] and hashlib.sha256(p.read_bytes()).hexdigest()==rec['sha256']
wal=ROOT/'transaction-soak-evidence-v3/soak-v3-wal-shared.json';d=json.loads(wal.read_text());assert d['status']=='failed' and d['active_seconds']==1.675996218 and d['active_seconds']<d['config']['requested_active_duration_ns']/1e9 and 'data guard exceeded' in d['errors'][0]
p=ROOT/'transaction-soak-evidence/soak-v1-rollback.json';d=json.loads(p.read_text());assert hashlib.sha256(p.read_bytes()).hexdigest()=='a62f4bfed390582d1125d88a6dfbe06f06f3a33500a55fedacea934424a1e101';assert d['status']=='failed' and d['active_seconds']==14.015854093 and not d['final_model_verified']
p=pathlib.Path('/tmp/gosqlite-ordinary-transactions-1157657524/benchmark.db');assert hashlib.sha256(p.read_bytes()).hexdigest()=='b996a714bd220924cd93370ca257c553f17ec775159a33382606b9b29ac773e9'
(OUT/'report-verification.json').write_text(json.dumps({'checked':result,'original_v1_failed_report_and_closed_data_unchanged':True,'v3_reports_and_failed_WAL_data_unchanged':True},indent=2)+'\n');print('Independent report/model/timing/trace/histogram checks PASS: 3 V4 reports; V3 reports and failed WAL data unchanged; original V1 failed report/data unchanged')
