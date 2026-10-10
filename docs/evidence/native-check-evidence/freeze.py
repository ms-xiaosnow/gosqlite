from pathlib import Path
import hashlib,importlib.util,json,shutil,subprocess
root=Path('/workspace/scratch/5e06a74e5f65/durable-restart');ev=root/'native-check-evidence'
spec=importlib.util.spec_from_file_location('inventory',root/'rollback-reader-acquisition-evidence/freeze.py');inv=importlib.util.module_from_spec(spec);spec.loader.exec_module(inv)
base=root/'native-check-base-v1';work=root/'native-check-work-v1';freeze=root/'native-check-freeze-v1'
b=inv.inventory(base);assert b['tree']=='0374382bdef1c61a7f5b36092a42109046b100dd'
assert not freeze.exists();shutil.copytree(work,freeze);f=inv.inventory(freeze)
for name,value in [('base',b),('freeze-v1',f)]:
 (ev/f'{name}-inventory.json').write_text(json.dumps(value,indent=2)+'\n');(ev/f'{name}-sha256.txt').write_text(''.join(f'{v["sha256"]}  {k}\n' for k,v in value['files'].items()))
git=ev/'patch-objects.git';subprocess.run(['git','init','--bare','-q',str(git)],check=True)
def run(src,*args):return subprocess.check_output(['git',f'--git-dir={git}',f'--work-tree={src}',*args])
run(base,'add','-f','.');bt=run(base,'write-tree').decode().strip();assert bt==b['tree']
run(freeze,'add','-A','-f','.');ft=run(freeze,'write-tree').decode().strip();assert ft==f['tree']
patch=ev/'source-v1.patch';patch.write_bytes(run(freeze,'diff','--binary',bt,ft))
subprocess.run(['git','apply','--check','--whitespace=error',str(patch)],cwd=base,check=True)
replay=root/'native-check-patch-verify-v1';assert not replay.exists();shutil.copytree(base,replay);subprocess.run(['git','apply','--check',str(patch)],cwd=replay,check=True);subprocess.run(['git','apply',str(patch)],cwd=replay,check=True);assert inv.inventory(replay)==f
owned=[k for k in sorted(set(b['files'])|set(f['files'])) if b['files'].get(k)!=f['files'].get(k)]
assert owned==['cmd/gosqlite/check.go','cmd/gosqlite/check_test.go','cmd/gosqlite/main.go','docs/native-check.md','examples/native-check/main.go','sqlite/check.go','sqlite/check_internal_test.go','sqlite/check_test.go'],owned
(ev/'owned-paths.txt').write_text('\n'.join(owned)+'\n');(ev/'owned-inventory.json').write_text(json.dumps({k:{'before':b['files'].get(k),'after':f['files'].get(k)} for k in owned},indent=2)+'\n')
report={'baseline_tree':bt,'frozen_tree':ft,'baseline_files':len(b['files']),'frozen_files':len(f['files']),'patch_sha256':hashlib.sha256(patch.read_bytes()).hexdigest(),'owned_paths':owned,'all_unowned_modes_and_content_unchanged':True,'patch_roundtrip_verified':True,'composed_from_tree':'ad066d04d676b8368ca7d02d83664e50dfa47216','dependency_patch_sha256':'c7893f8a1c6f686cf137cca15dbfa4cdf670e986625f2bf863e01caf80e8c359'}
(ev/'identity-v1.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report,indent=2))
