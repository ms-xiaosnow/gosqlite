import hashlib,json,os,pathlib,shutil,stat,subprocess
root=pathlib.Path('/workspace/scratch/5e06a74e5f65/durable-restart')
base=root/'grouped-counters-final-verify'; frozen=root/'shared-absence-freeze-v1'; evidence=root/'shared-absence-evidence'
expected='13f53e4cdadaaca2530e17e5f7a61b9dfe8e5c33'
def digest(tag,data): return hashlib.sha1(tag.encode()+b' '+str(len(data)).encode()+b'\0'+data).hexdigest()
def inventory(directory):
 out={}
 for p in sorted(directory.rglob('*')):
  if p.is_dir(): continue
  data=os.readlink(p).encode() if p.is_symlink() else p.read_bytes()
  mode='120000' if p.is_symlink() else ('100755' if p.stat().st_mode&0o111 else '100644')
  out[p.relative_to(directory).as_posix()]={'mode':mode,'sha256':hashlib.sha256(data).hexdigest(),'blob':digest('blob',data),'size':len(data)}
 return out

def tree(inv,prefix=''):
 children={}
 for path,info in inv.items():
  if not path.startswith(prefix): continue
  rest=path[len(prefix):]
  if '/' in rest: children[rest.split('/')[0]]=None
  else: children[rest]=info
 records=[]
 for name,info in sorted(children.items(),key=lambda x: (x[0]+('/' if x[1] is None else '')).encode()):
  mode,oid=('40000',tree(inv,prefix+name+'/')) if info is None else (info['mode'],info['blob'])
  records.append(mode.encode()+b' '+name.encode()+b'\0'+bytes.fromhex(oid))
 return digest('tree',b''.join(records))
old=inventory(base); new=inventory(frozen)
assert tree(old)==expected,(tree(old),expected)
raw=subprocess.check_output(['git','-C',str(root/'gosqlite'),'ls-tree','-rz',expected])
objects={}
for line in raw.split(b'\0'):
 if line:
  meta,path=line.split(b'\t'); mode,kind,blob=meta.decode().split(); objects[path.decode()]={'mode':mode,'blob':blob}
assert objects=={p:{'mode':v['mode'],'blob':v['blob']} for p,v in old.items()}
owned=[p for p in sorted(set(old)|set(new)) if old.get(p)!=new.get(p)]
assert owned==['docs/progress.md','docs/shared-absence-provenance.md','internal/pager/plain_session_cleanup.go','internal/pager/shared_absence_test.go','internal/pager/shared_session.go','internal/vfs/shared_absence_test.go'],owned
for label,inv in [('base',old),('full',new)]:
 (evidence/(label+'-manifest.json')).write_text(json.dumps(inv,indent=2)+'\n')
 (evidence/(label+'-sha256.txt')).write_text(''.join(f"{v['sha256']}  {p}\n" for p,v in inv.items()))
 (evidence/(label+'-modes.txt')).write_text(''.join(f"{v['mode']} {p}\n" for p,v in inv.items()))
 (evidence/(label+'-tree.txt')).write_text(tree(inv)+'\n')
(evidence/'owned.json').write_text(json.dumps({p:{'base':old.get(p),'final':new.get(p)} for p in owned},indent=2)+'\n')
(evidence/'owned-paths.txt').write_text('\n'.join(owned)+'\n')
patch=[]
for p in owned:
 a='grouped-counters-final-verify/'+p if p in old else '/dev/null'; b='shared-absence-freeze-v1/'+p if p in new else '/dev/null'
 result=subprocess.run(['git','diff','--no-index','--binary','--',a,b],cwd=root,stdout=subprocess.PIPE,check=False)
 assert result.returncode in (0,1)
 patch.append(result.stdout.decode().replace('a/grouped-counters-final-verify/','a/').replace('b/shared-absence-freeze-v1/','b/').replace('a/shared-absence-freeze-v1/','a/'))
patch=''.join(patch); (evidence/'source.patch').write_text(patch)
replay=root/'shared-absence-replay-v1'; shutil.copytree(base,replay)
subprocess.run(['git','apply','--check',str(evidence/'source.patch')],cwd=replay,check=True)
subprocess.run(['git','apply',str(evidence/'source.patch')],cwd=replay,check=True)
assert inventory(replay)==new
unchanged=[p for p in old if old[p]==new.get(p)]
(evidence/'verification.json').write_text(json.dumps({'base_tree':tree(old),'final_tree':tree(new),'base_files':len(old),'final_files':len(new),'owned':owned,'unchanged_files':len(unchanged),'base_git_objects_exact':True,'base_patch_replay_exact_bytes_modes':True,'patch_sha256':hashlib.sha256(patch.encode()).hexdigest()},indent=2)+'\n')
negative=root/'shared-absence-negative-v1'; shutil.copytree(base,negative)
for p in ['internal/pager/shared_absence_test.go','internal/vfs/shared_absence_test.go']:
 shutil.copy2(frozen/p,negative/p)
print((evidence/'verification.json').read_text())
