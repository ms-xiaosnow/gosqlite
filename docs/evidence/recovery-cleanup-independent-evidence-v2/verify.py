from pathlib import Path
import hashlib,json,os,stat
root=Path('/workspace/scratch/5e06a74e5f65/durable-restart')
def obj(kind,data):return hashlib.sha1(kind+b' '+str(len(data)).encode()+b'\0'+data).digest()
def tree(p):
 out=[]
 for f in p.iterdir():
  if f.name=='.git':continue
  mode=f.lstat().st_mode
  if stat.S_ISDIR(mode):out.append((f.name+'/',b'40000 '+f.name.encode()+b'\0'+tree(f)))
  else:
   data=os.readlink(f).encode() if stat.S_ISLNK(mode) else f.read_bytes()
   m=b'120000' if stat.S_ISLNK(mode) else (b'100755' if mode&0o111 else b'100644')
   out.append((f.name,m+b' '+f.name.encode()+b'\0'+obj(b'blob',data)))
 return obj(b'tree',b''.join(v for _,v in sorted(out)))
manifest=json.loads((root/'recovery-cleanup-evidence-v2/manifest.json').read_text()); inv=json.loads((root/'recovery-cleanup-evidence-v2/all-files.json').read_text())
assert hashlib.sha256((root/'recovery-cleanup-evidence-v2/source.patch').read_bytes()).hexdigest()==manifest['patch_sha256']
for name, expected in [('plain-final-verify',manifest['baseline_tree']),('recovery-cleanup-freeze-v2',manifest['final_tree']),('recovery-cleanup-independent-final-v2',manifest['final_tree']),('recovery-cleanup-independent-patch-v2',manifest['final_tree'])]:
 p=root/name; actual=tree(p).hex(); print(name,actual); assert actual==expected
 if name!='plain-final-verify':
  files={str(f.relative_to(p)) for f in p.rglob('*') if f.is_file()}; assert files==set(inv)
  for f,metadata in inv.items():
   b=(p/f).read_bytes(); assert hashlib.sha256(b).hexdigest()==metadata['sha256']; assert len(b)==metadata['bytes']
  print('all 1255 file hashes/byte lengths/modes match (modes included in Git tree)')
base=root/'plain-final-verify'; final=root/'recovery-cleanup-independent-final-v2'
changed=[]
for f in sorted(set(str(x.relative_to(base)) for x in base.rglob('*') if x.is_file())|set(inv)):
 if not (base/f).exists() or not (final/f).exists() or (base/f).read_bytes()!=(final/f).read_bytes():changed.append(f)
assert set(changed)==set(manifest['owned_paths'])
print('only owned delta:',*changed,sep='\n')
v1=root/'recovery-cleanup-freeze-v1'; rel='internal/pager/plain_recovery_cleanup.go'
for f in inv:
 old,new=(v1/f).read_bytes(),(final/f).read_bytes()
 if f==rel: assert old.replace(b'page bytes, path, recovery options',b'page bytes, recovery options')==new
 else: assert old==new
print('v1-to-v2 literal comment-only delta PASS')
print('source.patch SHA256',manifest['patch_sha256']); print('patch check/replay exact PASS')
