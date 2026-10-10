from pathlib import Path
import subprocess,os,hashlib,json
r=Path('/workspace/scratch/5e06a74e5f65/durable-restart/gosqlite');p=r.parent
base='e825d3096dd521a730caaaca2f5c85af0e9d76de'
layers=[('recovery-cleanup-evidence-v2/source.patch','e05bcffffc0388e9033a5e9cff212fed8b5eac370157d7284f13ad8e719e3c5c')]
def freeze(name,base,patches):
 env=os.environ.copy();env['GIT_INDEX_FILE']=str(p/(name+'-index'))
 def git(*a):return subprocess.check_output(['git',*a],cwd=r,env=env)
 git('read-tree',base)
 for path,want in patches:
  f=p/path;assert hashlib.sha256(f.read_bytes()).hexdigest()==want,path
  opts=['apply','--cached','--exclude=docs/progress.md','--exclude=docs/compatibility.md','--exclude=README.md','--exclude=docs/manual-verification.md','--exclude=docs/checkpoints.json','--exclude=docs/plans/README.md']
  git(*opts,'--check',str(f));git(*opts,str(f))
 tree=git('write-tree').decode().strip();dst=p/(name+'-verify');dst.mkdir(exist_ok=False)
 a=subprocess.Popen(['git','archive',tree],cwd=r,stdout=subprocess.PIPE);subprocess.run(['tar','-x','-C',str(dst)],stdin=a.stdout,check=True);assert a.wait()==0
 owned=git('diff','--cached','--name-only',base).decode().splitlines();sha={rel:hashlib.sha256((dst/rel).read_bytes()).hexdigest() for rel in owned}
 (p/(name+'-tree.txt')).write_text(tree+'\n');(p/(name+'-owned.json')).write_text(json.dumps({'base':base,'tree':tree,'owned':sha,'patches':patches,'method':'Every layer checked and applied by exact Git hunks; no whole-file stale overwrite'},indent=2)+'\n')
 (p/(name+'-all-sha256.txt')).write_text(''.join(hashlib.sha256(f.read_bytes()).hexdigest()+'  '+str(f.relative_to(dst))+'\n' for f in sorted(dst.rglob('*')) if f.is_file()))
 print(name,tree,len(owned),'checked paths');return tree
recovery=freeze('recovery-combined',base,layers)
freeze('grouped-counters-combined',recovery,[('grouped-aggregate-evidence/grouped-aggregate-v2.patch','d456e8caef731c0c3ee897e774e4c98e5d5c59a364111276a45eca925b521577'),('sql-connection-counters-evidence/source.patch','7a09d38be5ccb2fd18b11e3f74a1d57ba8b98e04ef7bd6a56491dd5b8ec0099c'),('sql-cached-preparation-evidence/source.patch','c96429d2427037bfc6c2286ff2a744300484f8f0834c419e4ace6c1bafcd50e0'),('sql-counters-independent-final-v1/docs/evidence/connection-counters-independent/independent-tests.patch','90db24227f5809592b3c2ac7aef3cdaed528e8bf98e6385b6e0def8d6be40bdf')])
