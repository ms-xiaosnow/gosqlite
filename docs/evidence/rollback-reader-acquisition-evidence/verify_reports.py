"""Read-only independent reconstruction of deterministic accepted writer histories."""
import hashlib,json,pathlib,sys
root=pathlib.Path(__file__).parent
mask=(1<<64)-1
def mix(x):
 x=(x+0x9e3779b97f4a7c15)&mask
 x=((x^(x>>30))*0xbf58476d1ce4e5b9)&mask
 x=((x^(x>>27))*0x94d049bb133111eb)&mask
 return x^(x>>31)
def digest(v):return hashlib.sha256(json.dumps(v,separators=(',',':')).encode()).hexdigest()
checked=[]
for p in sorted(root.glob('soak-v2-*.json')):
 if '-command' in p.name:continue
 try:d=json.loads(p.read_text())
 except json.JSONDecodeError:continue
 if d.get('benchmark')!='native-transactions-and-soak' or d['status']=='rejected':continue
 c=d['config'];seed=c['dataset_seed'];n=c['rows'];values=[1000+mix((seed+i)&mask)%100 for i in range(n)]
 assert digest(values)==d['dataset_sha256'],p
 total=sum(values);ordinal=0;commits=0
 for window in d['windows']:
  assert window['first_writer_ordinal']==ordinal,(p,'ordinal')
  trace=window['writer_outcomes'];counts=window['writer']
  assert len(trace)==counts['attempts']<=64,(p,'attempts')
  for code,key in [('C','committed'),('R','intentional_rollbacks'),('B','busy_outcomes'),('X','canceled_outcomes'),('F','failed_outcomes')]:assert trace.count(code)==counts[key],(p,key)
  assert counts['attempts']==counts['success']+counts['busy_outcomes']+counts['canceled_outcomes']+counts['failed_outcomes'],p
  for code in trace:
   kind=ordinal%4
   if code=='C':
    assert kind!=0,p
    for salt in ([1,1000001] if kind==2 else [1]):
     for j in range(c['transfer_pairs_per_transaction']):
      x=mix(seed^mix((ordinal*67+j*2+salt)&mask));a=x%n;b=(a+1+mix(x)%(n-1))%n;delta=1+mix(x^17)%7
      values[a]+=delta;values[b]-=delta
    commits+=1
   ordinal+=1
 assert sum(values)==total,(p,'conservation')
 assert digest(values)==d['final_model_sha256'],(p,'final model hash')
 assert commits==d['final_model_generation']==d['writer']['committed'],(p,'commit generation')
 assert ordinal==d['writer']['attempts'],(p,'global attempts')
 checked.append({'report':p.name,'status':d['status'],'reconstructed_attempts':ordinal,'reconstructed_commits':commits,'final_model_sha256':digest(values),'retained_failure':d['errors']})
print(json.dumps(checked,indent=2))
