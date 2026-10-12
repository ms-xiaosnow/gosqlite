import hashlib, json, pathlib, shutil, subprocess, difflib
R=pathlib.Path('/workspace/scratch/5e06a74e5f65/restart-20261012T2033'); W=R/'gosqlite'; E=R/'rekey-final-integration'
assert not E.exists(), 'Evidence directory already exists; stop rather than overwrite'
sha=lambda b:hashlib.sha256(b).hexdigest()
def git(*args): return subprocess.check_output(['git','-C',str(W),*args])
assert git('rev-parse','HEAD').decode().strip()=='01c63e6f8fb3d9e196afe2f42a163eb3eac4ad5a'
models={'internal/testkit/rekeyinterruption/contract.go':'b9a179e2ca3f82678842f86a3e330b52cc916097408ccc1716fa74ebf08a0e02','internal/testkit/rekeyinterruption/contract_test.go':'ac32dd1977e8283aa66dec82f66f2635125ef0d16ef52c321303ee3d9e847159'}
expected={'rekey-interruption-independent-review.md':'1523e55299bbf9c609791e76aa63b43e1844e3166f4d9f7db37852c423fc8ada','rekey-guard-independent-review.md':'8aa0ce7092a95a7b08d498c697ae70741c1b168f792f98b86e185d82fb9e1029','rekey-interruption-source-evidence/manifest.json':'e78dcdf71c2a6cff108436f16c120982457ad810aa2d39bba487afbfb99f9b99','rekey-guard-source-evidence/manifest.json':'4ad67d8e663c869fce5622f2c710db2cb48e1b38cdde05e28be9f1e0fc04f42a','toolchains/go1.27.2/bin/gofmt':'5583dbb3339147fee7cdb067eb3cd4a50015b48c27084377f49d93833aa4ad77'}
for path,digest in expected.items(): assert sha((R/path).read_bytes())==digest,path
for path,digest in models.items():
 assert sha((W/path).read_bytes())==digest,path
 assert git('show',':'+path)==(W/path).read_bytes(),path
status=git('status','--porcelain=v1').decode()
assert status==''.join('A  '+p+'\n' for p in models),repr(status)
inputs={}
for group in ['rekey-interruption','rekey-guard']:
 manifest=json.loads((R/(group+'-source-evidence/manifest.json')).read_text())
 for f in manifest['overlay_files']:
  p=f['path']; data=(R/(group+'-overlay')/p).read_bytes()
  assert len(data)==f['bytes'] and sha(data)==f['sha256'],p
  if p in models: continue
  target=W/p
  if f['base_sha256'] is None: assert not target.exists(),p
  else: assert target.is_file() and sha(target.read_bytes())==f['base_sha256'],p
  assert p not in inputs
  inputs[p]={'overlay':str(R/(group+'-overlay')/p),**f}
assert len(inputs)==12
owners=['internal/testscope/'+p for p in ['wal_tail_publication.go','encryption_upgrade_manual.go','encrypted_root_negative.go','rollback_fault_test.go','rollback_fault_sql_test.go','rollback_restart_test.go']]
old='6dbab48ff374858bfafe578f0fca99f5176ced7b6bdc008bec04ce6a6d1d0eb2'
for p in owners:
 data=(W/p).read_bytes()
 assert data==git('show','HEAD:'+p),p
 assert data.count(old.encode())==1,p
assert b'"internal/vfs/wal_publication_model_test.go": "465813a5c6c4b1324ad6b4ed4850585c24d145e5e3a7a4f7e22ca1d178cd0902"' in (W/owners[0]).read_bytes()
E.mkdir()
(E/'initial-status.txt').write_text(status)
(E/'initial-index-tree.txt').write_bytes(git('write-tree'))
initial=[]
for p in list(inputs)+owners+list(models):
 target=W/p
 data=target.read_bytes() if target.exists() else None
 initial.append({'path':p,'full_path':str(target),'exists':data is not None,'sha256':sha(data) if data is not None else None,'bytes':len(data) if data is not None else None})
 if data is not None:
  backup=E/'before-files'/p; backup.parent.mkdir(parents=True,exist_ok=True); backup.write_bytes(data)
for p,f in inputs.items():
 data=pathlib.Path(f['overlay']).read_bytes()
 target=W/p; target.parent.mkdir(parents=True,exist_ok=True); target.write_bytes(data)
 before=E/'before-format'/p; before.parent.mkdir(parents=True,exist_ok=True); before.write_bytes(data)
for p in owners:
 before=E/'before-format'/p; before.parent.mkdir(parents=True,exist_ok=True); before.write_bytes((W/p).read_bytes())
(E/'initial-manifest.json').write_text(json.dumps({'inputs':inputs,'initial_files':initial,'models_preserved':models,'pin_owners':owners,'verified_inputs':expected},indent=2)+'\n')
format_paths=[p for p in inputs if p.endswith('.go')]
assert len(format_paths)==11
(E/'format-paths.txt').write_text(''.join(str(W/p)+'\n' for p in format_paths))
print(json.dumps({'integrated_files':list(inputs),'format_paths':format_paths,'evidence':str(E)},indent=2))
