from pathlib import Path
import hashlib,json
r=Path('/workspace/scratch/5e06a74e5f65/durable-restart')
e=r/'gosqlite/docs/evidence/boolean-identifier-independent'
result={}
for label,name,manifest in [('baseline','affinity-substitution-freeze-v1','affinity-substitution-evidence/freeze-v1.sha256'),('candidate','boolean-identifier-freeze-v2','boolean-identifier-evidence/combined-v2-all-sha256.txt')]:
 root=r/name; raw=(r/manifest).read_bytes(); listed=set();failures=[]
 for line in raw.decode().splitlines():
  sha,rel=line.split('  ',1);listed.add(rel.removeprefix('./'));p=root/rel
  if not p.exists() or hashlib.sha256(p.read_bytes()).hexdigest()!=sha:failures.append(rel)
 actual={str(p.relative_to(root)) for p in root.rglob('*') if p.is_file() and '.git' not in p.relative_to(root).parts}
 result[label]={'manifest_sha256':hashlib.sha256(raw).hexdigest(),'entries':len(listed),'hash_mismatches':failures,'unlisted':sorted(actual-listed),'identical_to_before':raw==(e/f'{label}-before.sha256').read_bytes()}
 assert not failures and actual==listed and result[label]['identical_to_before']
 (e/f'{label}-after.sha256').write_bytes(raw)
b=r/'affinity-substitution-freeze-v1';c=r/'boolean-identifier-freeze-v2'
owned=json.loads((r/'boolean-identifier-evidence/combined-v2-owned-paths.json').read_text())
allpaths=set()
for root in [b,c]:allpaths|={str(p.relative_to(root)) for p in root.rglob('*') if p.is_file() and '.git' not in p.relative_to(root).parts}
changes=[]
for rel in sorted(allpaths):
 def h(root):
  p=root/rel;return hashlib.sha256(p.read_bytes()).hexdigest() if p.exists() else None
 if h(b)!=h(c):changes.append({'path':rel,'base_sha256':h(b),'candidate_sha256':h(c)})
result['ownership_exact']=changes==owned;assert changes==owned
result['delta_hashes']={p:hashlib.sha256((r/'boolean-identifier-evidence'/p).read_bytes()).hexdigest() for p in ['combined-v2-owned-paths.json','combined-v2.patch']}
(e/'hash-verification-after.json').write_text(json.dumps(result,indent=2)+'\n')
print(json.dumps(result,indent=2))
