from pathlib import Path
import hashlib,json
root=Path('/workspace/scratch/5e06a74e5f65/durable-restart'); out=root/'gosqlite/docs/evidence/public-cleanup-independent'
sha=lambda p:hashlib.sha256(p.read_bytes()).hexdigest()
mf=root/'plain-public-adoption-v1-all-sha256.txt'
assert sha(mf)=='ad05ae86ce81cd1eeca2603bc535f508ce5c773da4b59bc49c156480e384cc33'
entries=[line.split('  ',1) for line in mf.read_text().splitlines()]
reports=[]
for name in ['plain-public-adoption-freeze-v1','public-cleanup-independent-v1']:
    tree=root/name
    bad=[p for d,p in entries if not (tree/p).is_file() or sha(tree/p)!=d]
    assert not bad,bad
    expected={p for d,p in entries}
    allfiles={str(p.relative_to(tree)) for p in tree.rglob('*') if p.is_file()}
    extras=sorted(allfiles-expected)
    if name=='public-cleanup-independent-v1':
        assert extras==['internal/pager/public_cleanup_independent_test.go','sqlite/public_cleanup_independent_test.go'],extras
    else: assert not extras,extras
    reports.append({'tree':name,'verified_files':len(entries),'differences':bad,'declared_extras':{p:sha(tree/p) for p in extras}})
assert (out/'v1-checks.exit').read_text().strip()=='0'
assert (out/'v1-production-cgo.txt').stat().st_size==0
(out/'v1-final-identity.json').write_text(json.dumps({'manifest_sha256':sha(mf),'trees':reports,'checks_exit':0,'production_cgo_dependencies':[]},indent=2)+'\n')
print(json.dumps(reports,indent=2))
