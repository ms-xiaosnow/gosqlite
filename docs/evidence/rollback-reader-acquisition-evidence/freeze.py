from pathlib import Path
import hashlib,json,os,shutil,stat,subprocess
root=Path('/workspace/scratch/5e06a74e5f65/durable-restart'); ev=root/'rollback-reader-acquisition-evidence'
def inventory(src):
 files={}
 def walk(d):
  entries=[]
  for p in sorted(d.iterdir(),key=lambda p:p.name+('/' if p.is_dir() else '')):
   if p.name=='.git':continue
   rel=str(p.relative_to(src)); st=p.lstat()
   if p.is_symlink(): content=os.readlink(p).encode(); mode='120000'; typ='blob'
   elif p.is_dir():
    digest=walk(p); entries.append(b'40000 '+p.name.encode()+b'\0'+bytes.fromhex(digest));continue
   else: content=p.read_bytes(); mode='100755' if st.st_mode & 0o111 else '100644'; typ='blob'
   digest=hashlib.sha1(f'{typ} {len(content)}\0'.encode()+content).hexdigest()
   entries.append(mode.encode()+b' '+p.name.encode()+b'\0'+bytes.fromhex(digest))
   files[rel]={'sha256':hashlib.sha256(content).hexdigest(),'size':len(content),'mode':oct(stat.S_IMODE(st.st_mode)),'git_mode':mode,'blob':digest}
  content=b''.join(entries);return hashlib.sha1(f'tree {len(content)}\0'.encode()+content).hexdigest()
 return {'tree':walk(src),'files':dict(sorted(files.items()))}
def save(name,src):
 inv=inventory(src);(ev/f'{name}-inventory.json').write_text(json.dumps(inv,indent=2)+'\n');(ev/f'{name}-sha256.txt').write_text(''.join(f'{v["sha256"]}  {k}\n' for k,v in inv['files'].items()));return inv
if __name__=='__main__':
 base=root/'grouped-counters-final-verify'; work=root/'rollback-reader-acquisition-work-v1'; frozen=root/'rollback-reader-acquisition-freeze-v1'
 b=save('base',base);assert b['tree']=='13f53e4cdadaaca2530e17e5f7a61b9dfe8e5c33',b['tree']
 assert not frozen.exists();shutil.copytree(work,frozen); f=save('freeze-v1',frozen)
 git=ev/'patch-objects.git';subprocess.run(['git','init','--bare','-q',str(git)],check=True)
 def gitrun(src,*args):return subprocess.check_output(['git',f'--git-dir={git}',f'--work-tree={src}',*args])
 gitrun(base,'add','-f','.');bt=gitrun(base,'write-tree').decode().strip();assert bt==b['tree']
 gitrun(frozen,'add','-A','-f','.');ft=gitrun(frozen,'write-tree').decode().strip();assert ft==f['tree']
 patch=ev/'source-v1.patch';patch.write_bytes(gitrun(frozen,'diff','--binary',bt,ft))
 replay=root/'rollback-reader-acquisition-patch-verify-v1';assert not replay.exists();shutil.copytree(base,replay)
 subprocess.run(['git','apply','--check',str(patch)],cwd=replay,check=True);subprocess.run(['git','apply',str(patch)],cwd=replay,check=True);assert inventory(replay)==f
 control=root/'rollback-reader-acquisition-control-v1';assert not control.exists();shutil.copytree(base,control);shutil.copy2(frozen/'internal/pager/reader_acquisition_test.go',control/'internal/pager/reader_acquisition_test.go');c=save('control-v1',control)
 hp=root/'transaction-soak-evidence/source-v2.patch';assert hashlib.sha256(hp.read_bytes()).hexdigest()=='71b64b771631b39978f635aadb3348c0e42d6f7c652f25e7f21b719f1d973606'
 compositions={}
 for name,source in [('candidate',frozen),('baseline',base)]:
  dest=root/f'rollback-reader-acquisition-{name}-soak-v1';assert not dest.exists();shutil.copytree(source,dest)
  subprocess.run(['git','apply','--check',str(hp)],cwd=dest,check=True);subprocess.run(['git','apply',str(hp)],cwd=dest,check=True)
  for p in (root/'transaction-soak-freeze-v2/benchmarks/transactions').iterdir():assert p.read_bytes()==(dest/'benchmarks/transactions'/p.name).read_bytes()
  compositions[name]=save(f'{name}-soak-v1',dest)['tree']
 changed=[k for k in sorted(set(b['files'])|set(f['files'])) if b['files'].get(k)!=f['files'].get(k)]
 report={'base_tree':bt,'candidate_tree':ft,'control_tree':c['tree'],'patch_sha256':hashlib.sha256(patch.read_bytes()).hexdigest(),'base_file_count':len(b['files']),'candidate_file_count':len(f['files']),'changed_files':changed,'patch_roundtrip_verified':True,'harness_patch_sha256':hashlib.sha256(hp.read_bytes()).hexdigest(),'compositions':compositions}
 (ev/'identity-v1.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report,indent=2))
