from pathlib import Path
import hashlib,subprocess,os,json,shutil,tempfile
root=Path('/workspace/scratch/5e06a74e5f65/durable-restart');base=root/'sql-connection-counters-freeze-v1';work=root/'sql-cached-preparation-work-v1';out=root/'sql-cached-preparation-freeze-v3';ev=root/'sql-cached-preparation-evidence'
def inventory(p):return {str(f.relative_to(p)):hashlib.sha256(f.read_bytes()).hexdigest() for f in sorted(p.rglob('*')) if f.is_file()}
expected={line.split('  ',1)[1]:line.split('  ',1)[0] for line in (root/'sql-connection-counters-evidence/all-sha256.txt').read_text().splitlines()}
bi=inventory(base);assert bi==expected,'baseline bytes changed'
shutil.copytree(work,out);fi=inventory(out)
owned={p:{'base':bi.get(p),'final':fi.get(p)} for p in sorted(bi.keys()|fi.keys()) if bi.get(p)!=fi.get(p)}
(ev/'all-sha256.txt').write_text(''.join(f'{sha}  {p}\n' for p,sha in fi.items()))
(ev/'owned-sha256.txt').write_text(''.join(f'{v["final"]}  {p}\n' for p,v in owned.items()))
(ev/'baseline-owned-sha256.txt').write_text(''.join(f'{v["base"]}  {p}\n' for p,v in owned.items() if v['base']))
tmp=Path(tempfile.mkdtemp(prefix='sql-cached-preparation-inventory-',dir=root));subprocess.run(['git','init','--bare',str(tmp/'objects.git')],check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
env=os.environ.copy();env['GIT_DIR']=str(tmp/'objects.git');env['GIT_INDEX_FILE']=str(tmp/'index')
def tree(path):
 subprocess.run(['git','read-tree','--empty'],env=env,check=True)
 files=sorted(f for f in path.rglob('*') if f.is_file());hashes=subprocess.check_output(['git','hash-object','-w','--no-filters','--stdin-paths'],input=''.join(str(f)+'\n' for f in files).encode(),env=env).decode().splitlines()
 data=''.join(f'{"100755" if f.stat().st_mode&0o111 else "100644"} {h}\t{f.relative_to(path)}\n' for f,h in zip(files,hashes));subprocess.run(['git','update-index','--index-info'],input=data.encode(),env=env,check=True)
 return subprocess.check_output(['git','write-tree'],env=env).decode().strip()
bt=tree(base);assert bt=='7c8f7a266a63a713b22709268fdd4d25f1b875d1',bt
ft=tree(out);patch=subprocess.check_output(['git','diff','--binary',bt,ft],env=env);(ev/'source.patch').write_bytes(patch)
subprocess.run(['git','diff','--check',bt,ft],env=env,check=True)
manifest={'baseline_tree':bt,'final_tree':ft,'frozen_path':str(out),'file_count':len(fi),'all_inventory_sha256':hashlib.sha256((ev/'all-sha256.txt').read_bytes()).hexdigest(),'patch_sha256':hashlib.sha256(patch).hexdigest(),'owned':owned,'object_store':str(tmp/'objects.git')};(ev/'manifest.json').write_text(json.dumps(manifest,indent=2)+'\n')
check=root/'sql-cached-preparation-apply-check-v3';shutil.copytree(base,check);subprocess.run(['git','apply','--check',str(ev/'source.patch')],cwd=check,check=True);subprocess.run(['git','apply',str(ev/'source.patch')],cwd=check,check=True)
assert inventory(check)==fi,'patch bytes differ';assert tree(check)==ft,'patch modes/tree differ';assert inventory(work)==fi,'worktree differs';assert inventory(base)==bi,'baseline changed'
(ev/'patch-apply.txt').write_text(f'PASS: exact baseline {bt} + checked source.patch reproduces all {len(fi)} paths and bytes, file modes, final tree {ft}; worktree and freeze identical; baseline unchanged.\n')
print(json.dumps(manifest,indent=2))
