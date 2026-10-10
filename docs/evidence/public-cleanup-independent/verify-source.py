import hashlib,json, pathlib,shutil, subprocess
root=pathlib.Path('/workspace/scratch/5e06a74e5f65/durable-restart')
out=root/'gosqlite/docs/evidence/public-cleanup-independent'
def sha(p): return hashlib.sha256(p.read_bytes()).hexdigest()
def verify(tree,name,expected):
    mf=root/name
    assert sha(mf)==expected,(name,sha(mf))
    lines=mf.read_text().splitlines()
    bad=[]
    for line in lines:
        digest,p=line.split('  ',1)
        f=tree/p
        if not f.is_file() or sha(f)!=digest: bad.append(p)
    assert not bad,(tree,bad)
    return {'tree':str(tree),'manifest':name,'sha256':sha(mf),'verified_files':len(lines),'differences':bad}
records=[]
for tree,name,expected in [
('plain-reader-freeze-v5','plain-reader-v5-all-sha256.txt','12bb816edaf4b88af31c243bd409fbf2262d7ef1a5f0c1c7a3b5d46eeb725f50'),
('plain-tx-freeze-v2','plain-tx-v2-all-sha256.txt','bf450dcdde4293bbd303750d9e14d65d969b8a63fad66249f7c6ba1f8d6eaaa2'),
('plain-exclusive-freeze-v2','plain-exclusive-v2-all-sha256.txt','6180d33214507b003d4bf78f4d0da45477b946307ec00a0665fbe3d24fa6a11d'),
('plain-public-adoption-freeze-v1','plain-public-adoption-v1-all-sha256.txt','ad05ae86ce81cd1eeca2603bc535f508ce5c773da4b59bc49c156480e384cc33')]:
    records.append(verify(root/tree,name,expected))
candidate=root/'plain-public-adoption-freeze-v1'
copy=root/'public-cleanup-independent-v1'
shutil.copytree(candidate,copy)
records.append(verify(copy,'plain-public-adoption-v1-all-sha256.txt','ad05ae86ce81cd1eeca2603bc535f508ce5c773da4b59bc49c156480e384cc33'))
owned=(candidate/'docs/evidence/public-open-adoption/owned-source-paths.txt').read_text().splitlines()
ownedmf=candidate/'docs/evidence/public-open-adoption/owned-source-sha256.txt'
assert sha(ownedmf)=='9c91e9f4ead5924ee7ff664803001017d91ee621888b5db74a9bd99efa6fe601'
for line in ownedmf.read_text().splitlines():
    d,p=line.split('  ',1); assert sha(candidate/p)==d,p
patch=root/'plain-exclusive-v2-to-public-v1.patch'
assert sha(patch)=='dd3919ba517674d47f395d6ae8ca399a378004c6166068dc4fe796b2da1b17d1'
replay=root/'public-cleanup-independent-base-replay-v1'
shutil.copytree(root/'plain-exclusive-freeze-v2',replay)
subprocess.run(['git','apply','--check',str(patch)],cwd=replay,check=True)
subprocess.run(['git','apply',str(patch)],cwd=replay,check=True)
assert all(sha(replay/p)==sha(candidate/p) for p in owned)
def sourcepaths(tree): return set(str(p.relative_to(tree)) for p in tree.rglob('*') if p.is_file() and (p.suffix=='.go' or p.name in ('go.mod','go.sum')))
base=root/'plain-exclusive-freeze-v2'
diff=[]
for p in sourcepaths(candidate)|sourcepaths(base):
    if not (base/p).exists() or not (candidate/p).exists() or sha(base/p)!=sha(candidate/p): diff.append(p)
assert set(diff).issubset(owned),diff
report={'manifests':records,'owned_manifest_sha256':sha(ownedmf),'owned_paths':owned,'source_delta_sha256':sha(patch),'owned_replay_matches':len(owned),'source_changes':sorted(diff),'unowned_source_changes':[]}
(out/'v1-identity.json').write_text(json.dumps(report,indent=2)+'\n')
shutil.copyfile(out/'v1-pager-probes.go.txt',copy/'internal/pager/public_cleanup_independent_test.go')
shutil.copyfile(out/'v1-sqlite-probes.go.txt',copy/'sqlite/public_cleanup_independent_test.go')
print(json.dumps(report,indent=2))
