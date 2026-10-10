from pathlib import Path
import subprocess,os,hashlib,json
r=Path('/workspace/scratch/5e06a74e5f65/durable-restart/gosqlite');p=r.parent;base='ddb957e2a1d804b236a295f7fe6ce383954b852c';name='runtime-tools-combined'
patches=[('transaction-soak-evidence-v4/source-v4.patch','1e5d86e5b5f1643f3e20245048deda7c068630fade57d1d5f36d90e3ca33df4f'),('native-check-evidence/source-v2.patch','819b54f6af690a2ede21e78165dc5a07da4f5e608d70e5d398579c703487dafd')]
env=os.environ.copy();env['GIT_INDEX_FILE']=str(p/(name+'-index'))
def git(*a,**kw):return subprocess.check_output(['git',*a],cwd=r,env=env,**kw)
git('read-tree',base)
for rel,want in patches:
 f=p/rel;assert hashlib.sha256(f.read_bytes()).hexdigest()==want;opts=['apply','--cached','--exclude=docs/progress.md','--exclude=docs/compatibility.md','--exclude=README.md','--exclude=docs/manual-verification.md','--exclude=docs/checkpoints.json'];git(*opts,'--check',str(f));git(*opts,str(f))
tree=git('write-tree').decode().strip();dst=p/(name+'-verify');dst.mkdir(exist_ok=False)
a=subprocess.Popen(['git','archive',tree],cwd=r,stdout=subprocess.PIPE);subprocess.run(['tar','-x','-C',str(dst)],stdin=a.stdout,check=True);assert a.wait()==0
owned=git('diff','--cached','--name-only',base).decode().splitlines();(p/(name+'-tree.txt')).write_text(tree+'\n');(p/(name+'-owned.json')).write_text(json.dumps({'base':base,'tree':tree,'owned':{rel:hashlib.sha256((dst/rel).read_bytes()).hexdigest() for rel in owned},'patches':patches,'method':'Checked Git hunks over exact accepted-tools base; no stale unowned overwrite'},indent=2)+'\n');(p/(name+'-all-sha256.txt')).write_text(''.join(hashlib.sha256(f.read_bytes()).hexdigest()+'  '+str(f.relative_to(dst))+'\n' for f in sorted(dst.rglob('*')) if f.is_file()));print(name,tree,len(owned),'paths')
