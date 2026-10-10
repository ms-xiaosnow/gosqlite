import pathlib,hashlib,json,subprocess,tarfile,io
root=pathlib.Path('/workspace/scratch/5e06a74e5f65/durable-restart')
repo=root/'gosqlite'; out=repo/'docs/evidence/public-cleanup-independent'
replay=root/'public-cleanup-independent-chain-replay-v1'; replay.mkdir()
base='508732922205872b7a8738b471b236fcf8ecb096'
blob=subprocess.check_output(['git','archive',base],cwd=repo)
with tarfile.open(fileobj=io.BytesIO(blob)) as tf: tf.extractall(replay,filter='data')
expected=[('plain-reader-v5-source.patch','b9f45b6ea035e254ba0b6b603150e2a158533b469e47eb45261f57eedadd0af7'),('plain-reader-v5-to-tx-v2.patch','688836b91343cb1c2c95c78adb2358e6485f7f72611254ede02ea2777df69e5a'),('plain-tx-v2-to-exclusive-v2.patch','c5c398e60c038e257382c3a4f32971e26ba596edcd46db8044719bf3c90d9b3b'),('plain-exclusive-v2-to-public-v1.patch','dd3919ba517674d47f395d6ae8ca399a378004c6166068dc4fe796b2da1b17d1')]
def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
for name,d in expected:
    p=root/name; assert sha(p)==d
    subprocess.run(['git','apply','--check',str(p)],cwd=replay,check=True)
    subprocess.run(['git','apply',str(p)],cwd=replay,check=True)
freeze=root/'plain-public-adoption-freeze-v1'
paths=[str(p.relative_to(freeze)) for p in freeze.rglob('*') if p.is_file() and (p.suffix=='.go' or p.name in ['go.mod','go.sum'])]
bad=[p for p in paths if not (replay/p).is_file() or sha(replay/p)!=sha(freeze/p)]
assert not bad,bad
result={'base_tree':base,'patches':[{'file':n,'sha256':d} for n,d in expected],'source_files_verified':len(paths),'differences':bad}
(out/'v1-chain-replay.json').write_text(json.dumps(result,indent=2)+'\n')
print(json.dumps(result,indent=2))
