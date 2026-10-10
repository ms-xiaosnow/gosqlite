from pathlib import Path
import hashlib,os,stat,json,shutil,subprocess
root=Path('/workspace/scratch/5e06a74e5f65/durable-restart');ev=root/'native-check-independent-evidence'
def inv(src):
 out={}
 def tree(p):
  entries=[]
  for q in sorted(p.iterdir(),key=lambda x:x.name+('/' if x.is_dir() else '')):
   if q.name=='.git': continue
   if q.is_symlink(): b=os.readlink(q).encode(); m='120000'
   elif q.is_dir(): entries.append(b'40000 '+q.name.encode()+b'\0'+tree(q)); continue
   else: b=q.read_bytes(); m='100755' if q.stat().st_mode&0o111 else '100644'
   blob=hashlib.sha1(b'blob '+str(len(b)).encode()+b'\0'+b).digest()
   out[str(q.relative_to(src))]={'sha256':hashlib.sha256(b).hexdigest(),'mode':stat.S_IMODE(q.lstat().st_mode),'git_mode':m,'size':len(b)}
   entries.append(m.encode()+b' '+q.name.encode()+b'\0'+blob)
  b=b''.join(entries); return hashlib.sha1(b'tree '+str(len(b)).encode()+b'\0'+b).digest()
 return {'tree':tree(src).hex(),'files':dict(sorted(out.items()))}
base=root/'native-check-base-v1';freeze=root/'native-check-freeze-v2';replay=root/'native-check-independent-replay-v2';patch=root/'native-check-evidence/source-v2.patch'
b,f=inv(base),inv(freeze)
assert b['tree']=='0374382bdef1c61a7f5b36092a42109046b100dd'
assert f['tree']=='99ee54afe1900ac0e9b814ac8f57772f6b76e493'
assert hashlib.sha256(patch.read_bytes()).hexdigest()=='819b54f6af690a2ede21e78165dc5a07da4f5e608d70e5d398579c703487dafd'
assert not replay.exists();shutil.copytree(base,replay)
subprocess.run(['git','apply','--check','--whitespace=error',str(patch)],cwd=replay,check=True)
subprocess.run(['git','apply','--whitespace=error',str(patch)],cwd=replay,check=True)
r=inv(replay);assert r==f
changed=[k for k in sorted(set(b['files'])|set(f['files'])) if b['files'].get(k)!=f['files'].get(k)]
assert changed==(root/'native-check-evidence/owned-v2-paths.txt').read_text().splitlines()
for n,o in [('baseline',b),('candidate',f),('replay',r)]: (ev/(n+'-v2-inventory.json')).write_text(json.dumps(o,indent=2)+'\n')
report={'baseline_tree':b['tree'],'candidate_tree':f['tree'],'baseline_count':len(b['files']),'candidate_count':len(f['files']),'changed':changed,'complete_bytes_modes_replay_equal':True}
(ev/'identity-v2.json').write_text(json.dumps(report,indent=2)+'\n'); print(json.dumps(report,indent=2))
