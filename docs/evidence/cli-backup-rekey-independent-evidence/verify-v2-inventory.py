from pathlib import Path
import json, hashlib, stat, subprocess, os, shutil
r=Path('/workspace/scratch/5e06a74e5f65/durable-restart')
e=r/'cli-backup-rekey-independent-evidence'
b=r/'plain-final-verify'; f=r/'cli-backup-rekey-freeze-v2'; c=r/'cli-backup-rekey-independent-v2'
def inv(d):
 out={}
 for p in sorted(d.rglob('*')):
  if p.is_symlink(): raise RuntimeError(f'symlink: {p}')
  if p.is_file(): out[p.relative_to(d).as_posix()]={'sha256':hashlib.sha256(p.read_bytes()).hexdigest(),'mode':oct(stat.S_IMODE(p.stat().st_mode)),'bytes':p.stat().st_size}
 return out
bi,fi,ci=inv(b),inv(f),inv(c)
assert ci==fi
assert bi==json.loads((r/'cli-backup-rekey-evidence/baseline-inventory.json').read_text())
assert fi==json.loads((r/'cli-backup-rekey-evidence/v2-final-inventory.json').read_text())
changes=sorted(p for p in set(bi)|set(fi) if bi.get(p)!=fi.get(p))
assert changes==(r/'cli-backup-rekey-evidence/owned-paths.txt').read_text().splitlines()
patch=r/'cli-backup-rekey-evidence/source-v2.patch'
assert hashlib.sha256(patch.read_bytes()).hexdigest()=='c7893f8a1c6f686cf137cca15dbfa4cdf670e986625f2bf863e01caf80e8c359'
replay=r/'cli-backup-rekey-independent-replay-v2'; shutil.copytree(b,replay)
p=subprocess.run(['git','apply','--check',str(patch)],cwd=replay,capture_output=True,text=True); assert p.returncode==0,p.stderr
p=subprocess.run(['git','apply',str(patch)],cwd=replay,capture_output=True,text=True); assert p.returncode==0,p.stderr
assert inv(replay)==fi
repo=e/'trees.git'
env=dict(os.environ,GIT_DIR=str(repo),GIT_WORK_TREE=str(c),GIT_INDEX_FILE=str(e/'v2-final.index'))
subprocess.run(['git','add','-f','--all','.'],cwd=c,env=env,check=True,capture_output=True)
tree=subprocess.check_output(['git','write-tree'],cwd=c,env=env,text=True).strip()
assert tree=='d9969679165e42e22d8de5fe12ab57ee873b6c52',tree
(e/'v2-final-tree.txt').write_text(tree+'\n')
(e/'v2-frozen-inventory.json').write_text(json.dumps(fi,indent=2)+'\n')
result={'baseline_count':len(bi),'final_count':len(fi),'changed':changes,'patch_sha256':hashlib.sha256(patch.read_bytes()).hexdigest(),'patch_reconstruction':'PASS, all bytes/modes and exact paths equal freeze','baseline_tree':'7d3184f87165e11e12560884ec6e4f92c9bfd039','final_tree':tree}
(e/'v2-inventory-proof.json').write_text(json.dumps(result,indent=2)+'\n'); print(json.dumps(result,indent=2))
