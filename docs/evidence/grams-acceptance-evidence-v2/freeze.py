import hashlib, json, os, pathlib, stat, subprocess
base=pathlib.Path('/workspace/scratch/5e06a74e5f65/durable-restart')
e=base/'grams-acceptance-evidence-v2'
git=e/'inventory.git'
subprocess.run(['git','init','--bare',str(git)],check=True,stdout=subprocess.PIPE)
env=os.environ.copy();env['GIT_DIR']=str(git)
inventories={}
for label,name in [('baseline','plain-final-verify'),('final','grams-acceptance-freeze-v2')]:
 root=base/name
 env['GIT_WORK_TREE']=str(root);env['GIT_INDEX_FILE']=str(e/(label+'.index'))
 subprocess.run(['git','add','--all','--force','.'],cwd=root,env=env,check=True)
 tree=subprocess.check_output(['git','write-tree'],cwd=root,env=env,text=True).strip()
 (e/(label+'-tree.txt')).write_text(tree+'\n')
 entries=[]
 for path in sorted(root.rglob('*')):
  if path.is_file():
   data=path.read_bytes();entries.append({'path':str(path.relative_to(root)),'mode':oct(stat.S_IMODE(path.stat().st_mode)),'size':len(data),'sha256':hashlib.sha256(data).hexdigest()})
 (e/(label+'-inventory.json')).write_text(json.dumps({'root':str(root),'git_tree':tree,'files':entries},indent=2)+'\n')
 (e/(label+'-sha256.txt')).write_text(''.join(f"{f['sha256']}  {f['path']}\n" for f in entries))
 inventories[label]={f['path']:f for f in entries}
 print(label,tree,'files',len(entries))
 if label=='baseline':assert tree=='7d3184f87165e11e12560884ec6e4f92c9bfd039',tree
owned=[]
for p in sorted(set(inventories['baseline'])|set(inventories['final'])):
 before=inventories['baseline'].get(p);after=inventories['final'].get(p)
 if before!=after:owned.append({'path':p,'before':before,'after':after})
(e/'owned-files.json').write_text(json.dumps(owned,indent=2)+'\n')
a=(e/'baseline-tree.txt').read_text().strip();b=(e/'final-tree.txt').read_text().strip()
patch=subprocess.check_output(['git','diff','--binary','--full-index',a,b],env=env)
(e/'changes.patch').write_bytes(patch)
print('owned',*[x['path'] for x in owned],sep='\n')
