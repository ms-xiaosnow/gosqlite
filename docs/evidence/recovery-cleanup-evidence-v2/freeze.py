from pathlib import Path
import hashlib,json,os,subprocess,stat
root=Path('/workspace/scratch/5e06a74e5f65/durable-restart')
repo=root/'gosqlite'; work=root/'recovery-cleanup-work-v1'; out=root/'recovery-cleanup-evidence-v2'; frozen=root/'recovery-cleanup-freeze-v2'
base='7d3184f87165e11e12560884ec6e4f92c9bfd039'
env=os.environ.copy();env['GIT_INDEX_FILE']=str(out/'freeze.index')
def git(*args, input=None):
 return subprocess.check_output(['git',*args],cwd=repo,env=env,input=input)
def digest(data): return hashlib.sha256(data).hexdigest()
entries={}
for line in git('ls-tree','-r',base).decode().splitlines():
 mode,kind,objpath=line.split(' ',2);obj,path=objpath.split('\t',1)
 assert kind=='blob', (path,kind)
 entries[path]={'mode':mode,'blob':obj,'sha256':digest(git('cat-file','blob',obj))}
final={}
for p in sorted(work.rglob('*')):
 if p.is_file():
  rel=p.relative_to(work).as_posix();assert not p.is_symlink()
  final[rel]={'mode':'100755' if p.stat().st_mode & stat.S_IXUSR else '100644','sha256':digest(p.read_bytes()),'bytes':p.stat().st_size}
owned=sorted(set(entries)^set(final)|{p for p in set(entries)&set(final) if entries[p]['sha256']!=final[p]['sha256'] or entries[p]['mode']!=final[p]['mode']})
expected=['docs/plaintext-recovery-cleanup.md','internal/pager/plain_recovery_cleanup.go','internal/pager/plain_recovery_cleanup_test.go','internal/pager/recovery.go']
assert owned==expected,(owned,expected)
git('read-tree',base)
for path in owned:
 assert path in final
 blob=git('hash-object','-w','--stdin',input=(work/path).read_bytes()).decode().strip()
 git('update-index','--add','--cacheinfo',final[path]['mode'],blob,path)
tree=git('write-tree').decode().strip()
patch=git('diff','--binary',base,tree)
(out/'source.patch').write_bytes(patch)
git('read-tree',base)
git('apply','--cached','--check',str(out/'source.patch'))
git('apply','--cached',str(out/'source.patch'))
assert git('write-tree').decode().strip()==tree
frozen.mkdir(exist_ok=False)
archive=git('archive',tree)
subprocess.run(['tar','-x','-C',str(frozen)],input=archive,check=True)
for path,record in final.items(): assert digest((frozen/path).read_bytes())==record['sha256'],path
assert sorted(p.relative_to(frozen).as_posix() for p in frozen.rglob('*') if p.is_file())==sorted(final)
(out/'all-files.json').write_text(json.dumps(final,indent=2)+'\n')
(out/'all-sha256.txt').write_text(''.join(v['sha256']+'  '+p+'\n' for p,v in final.items()))
manifest={'baseline_tree':base,'final_tree':tree,'file_count':len(final),'owned_paths':{p:{'baseline_sha256':entries.get(p,{}).get('sha256'),'final_sha256':final.get(p,{}).get('sha256')} for p in owned},'patch_sha256':digest(patch),'patch_check':'PASS: git apply --cached --check plus reconstructed tree equals final','freeze':str(frozen),'physical_recovery':'NOT_RUN'}
(out/'manifest.json').write_text(json.dumps(manifest,indent=2)+'\n')
print(json.dumps(manifest,indent=2))
