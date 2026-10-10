from pathlib import Path
import subprocess,os,hashlib,json
r=Path('/workspace/scratch/5e06a74e5f65/durable-restart/gosqlite');p=r.parent
# Prepared composition only. Run after all listed source reviews/required owner
# results are terminal and shared absence candidate, if included, is frozen.
base='13f53e4cdadaaca2530e17e5f7a61b9dfe8e5c33'
# Only independently accepted components; the separately held harness is excluded.
patches=[('shared-absence-evidence/source.patch','a4aa67ed7b2764726b059d0f18a6ce6fe2b8f2ea3d0663968b0137f6310c7770'),('grams-acceptance-evidence-v2/changes.patch','d9feb249e2678f76d58da6d0bd25a56d47c1dc8a9dfd51b8060210fc75d9cbfe'),('rollback-reader-acquisition-evidence/source-v1.patch','ccc9bf728b3df6b0daa65562955ae54cc84fcdf690a9d8985d17888d740c3659'),('cli-backup-rekey-evidence/source-v2.patch','c7893f8a1c6f686cf137cca15dbfa4cdf670e986625f2bf863e01caf80e8c359'),('import-distance-v2-source.patch','64697339b9b7a087a1c0ba813fad87effd9200489c3f5f4b227149933a07e37b')]
# No transaction/performance/native-check/planner/Windows candidate is included.
name='accepted-tools-combined';env=os.environ.copy();env['GIT_INDEX_FILE']=str(p/(name+'-index'))
def git(*a,**kw):return subprocess.check_output(['git',*a],cwd=r,env=env,**kw)
git('read-tree',base)
for rel,want in patches:
 f=p/rel; assert hashlib.sha256(f.read_bytes()).hexdigest()==want,rel
 opts=['apply','--cached','--exclude=docs/progress.md','--exclude=docs/compatibility.md','--exclude=README.md','--exclude=docs/manual-verification.md','--exclude=docs/checkpoints.json']
 git(*opts,'--check',str(f));git(*opts,str(f))
independent=p/'import-distance-independent-review-v2/regressions/migration/postgres/import_distance_bound_independent_v2_test.go'
sha=git('hash-object','-w','--stdin',input=independent.read_bytes()).decode().strip()
git('update-index','--add','--cacheinfo','100644,'+sha+',migration/postgres/import_distance_bound_independent_v2_test.go')
tree=git('write-tree').decode().strip();dst=p/(name+'-verify');dst.mkdir(exist_ok=False)
a=subprocess.Popen(['git','archive',tree],cwd=r,stdout=subprocess.PIPE);subprocess.run(['tar','-x','-C',str(dst)],stdin=a.stdout,check=True);assert a.wait()==0
owned=git('diff','--cached','--name-only',base).decode().splitlines()
(p/(name+'-tree.txt')).write_text(tree+'\n')
(p/(name+'-owned.json')).write_text(json.dumps({'base':base,'tree':tree,'owned':{rel:hashlib.sha256((dst/rel).read_bytes()).hexdigest() for rel in owned},'patches':patches,'independent_distance_regression_sha256':hashlib.sha256(independent.read_bytes()).hexdigest(),'method':'Checked Git hunks over exact integrated base; no stale unowned overwrite'},indent=2)+'\n')
(p/(name+'-all-sha256.txt')).write_text(''.join(hashlib.sha256(f.read_bytes()).hexdigest()+'  '+str(f.relative_to(dst))+'\n' for f in sorted(dst.rglob('*')) if f.is_file()))
print(name,tree,len(owned),'paths')
