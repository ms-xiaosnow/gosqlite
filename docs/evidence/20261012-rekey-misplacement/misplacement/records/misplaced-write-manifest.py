from pathlib import Path
import re,json,hashlib,difflib,subprocess
R=Path('/workspace/scratch/5e06a74e5f65/restart-20261012T2033');O=R/'misplaced-write-overlay';W=R/'gosqlite';E=R/'misplaced-write-source-evidence';E.mkdir(exist_ok=True)
sha=lambda x:hashlib.sha256(x).hexdigest()
def block(s,start):
 a=s.index(start);b=s.index('\n}',a)+2;return s[a:b]
files=sorted(str(p.relative_to(O)) for p in O.rglob('*') if p.is_file());assert len(files)==13
manifest={'schema':'rollback-misplaced-write-source-only-v1','base_commit':'01c63e6f8fb3d9e196afe2f42a163eb3eac4ad5a','case':'index-misplace-minus64-error','source_status':'SOURCE_ONLY_REVIEW_PENDING','format_status':'NOT_RUN','model_guard_compile_status':'NOT_RUN','native_miswrite_recovery_status':'NOT_RUN','files':{},'unchanged_functions':{},'unchanged_types':{},'excluded_files':{},'source_only_roots':{}}
patch=[]
for f in files:
 before=(W/f).read_bytes();after=(O/f).read_bytes();assert before!=after,f
 manifest['files'][f]={'base_sha256':sha(before),'overlay_sha256':sha(after),'base_bytes':len(before),'overlay_bytes':len(after)}
 patch.extend(difflib.unified_diff(before.decode().splitlines(True),after.decode().splitlines(True),'a/'+f,'b/'+f))
 if f.endswith('.go'):
  prior=set(re.findall(r'^func (Test\w+)\(',before.decode(),re.M));new=set(re.findall(r'^func (Test\w+)\(',after.decode(),re.M))
  assert prior<=new,(f,'old roots removed');manifest['source_only_roots'][f]={'preserved':sorted(prior),'added':sorted(new-prior)}
for f,names in {'internal/pager/rollback_io_manual_linux.go':['rollbackFaultBefore','rollbackFaultBeforeCore','rollbackFaultAfter','rollbackFaultAfterCore'],'sqlite/rollback_fault_sql_manual_linux_test.go':['manualRollbackSQLRecoverCopy','manualRollbackSQLBaseline','manualRollbackSQLQuery','manualRollbackSQLReadback']}.items():
 for name in names:
  prefix='func '+name+'(';a=block((W/f).read_text(),prefix);b=block((O/f).read_text(),prefix);assert a==b,(f,name)
  manifest['unchanged_functions'][f+'::'+name]=sha(a.encode())
for name in ['Call','Result','Event','CommitResult']:
 f='internal/testkit/rollbackfault/contract.go';prefix='type '+name+' struct {';a=block((W/f).read_text(),prefix);b=block((O/f).read_text(),prefix);assert a==b,name;manifest['unchanged_types'][name]=sha(a.encode())
for f in ['internal/pager/commit.go','internal/pager/recovery.go','internal/pager/rollback_io_default.go','internal/pager/rollback_io_points.go','internal/pager/rollback_io_contract_test.go','internal/pager/rollback_restart_manual_linux.go','internal/testscope/rollback_restart.go','internal/testscope/main.go','goal.md']:
 assert not (O/f).exists(),f
 manifest['excluded_files'][f]=sha((W/f).read_bytes())
# Seven source snapshots and hashes must match the exact overlay bytes.
for guard,table,items in [
 ('internal/testscope/rollback_fault.go','rollbackFaultOwnedShapes',{'rollbackFaultManual':'internal/pager/rollback_io_manual_linux.go','rollbackFaultContract':'internal/testkit/rollbackfault/contract.go','rollbackFaultContractTest':'internal/testkit/rollbackfault/contract_test.go'}),
 ('internal/testscope/rollback_fault_sql.go','rollbackFaultSQLShapes',{'rollbackFaultSQLBridge':'internal/pager/rollback_fault_sql_manual_linux.go','rollbackFaultSQLNative':'sqlite/rollback_fault_sql_manual_linux_test.go','rollbackFaultSQLModel':'internal/testkit/rollbackfault/sql_ledger.go','rollbackFaultSQLModelTest':'internal/testkit/rollbackfault/sql_ledger_test.go'})]:
 s=(O/guard).read_text();prefix,tail=s.split('var '+table+' =',1)
 for key,f in items.items():
  pin=re.search(r'^\t'+key+r':\s+"([0-9a-f]{64})"',prefix,re.M);assert pin and pin[1]==sha((O/f).read_bytes()),key
  stored=re.search(r'^\t'+key+r':\s+(`[^`]*`|"(?:[^"\\]|\\.)*"),\n',tail,re.M);assert stored,key
  value=stored[1][1:-1] if stored[1][0]=='`' else json.loads(stored[1]);assert value==(O/f).read_text(),key
patch=''.join(patch).encode();(E/'misplaced-write.patch').write_bytes(patch);manifest['patch_sha256']=sha(patch);manifest['patch_bytes']=len(patch)
raw=(json.dumps(manifest,indent=2,sort_keys=True)+'\n').encode();(E/'source-manifest.json').write_bytes(raw)
report=f'''# R2 source-only handoff

The overlay contains exactly eleven existing Go files and two existing Chinese manuals. It implements only index-misplace-minus64-error; original case IDs 1–9 remain intact and the appended case is 10. Current source, independent review, formatting and execution statuses are separate: source authored; independent review PENDING; Go/gofmt/parser/model/sourceguard/compile/nm/CI and all native miswrite/recovery NOT_RUN.

- Overlay: {O}
- Base commit: 01c63e6f8fb3d9e196afe2f42a163eb3eac4ad5a
- Patch: {E/'misplaced-write.patch'}
- Patch SHA-256: {sha(patch)}
- Manifest: {E/'source-manifest.json'}
- Manifest SHA-256: {sha(raw)}

## Implemented boundary

The new native action requests the unchanged full 4096-byte b from the original managed owner at the actual argument nativeOffset := off - 64, exactly once. Only native (4096,nil) becomes adapter (4096,existing-injected-error). Native errors and counts remain original. Actual incoming/native offsets and lengths, call count and pre-native payload hash are captured separately from the generic requested receipt. Exact actual three-page arming, strict original public-wrapper/poison/target-only cause, shifted range and untouched tail checks, complete original journal byte comparison and OLD-only one-fresh-copy recovery evidence bind this one case.

The byte-proof branch is gated by commitCheckErr == nil, which means the expected non-nil RecoveryRequired Commit and owner retirement have been validated. It never requires Commit itself to return nil. Original writer/cause evidence is captured regardless; dependent failure is reported only after commit-observation.json retains available observations. Native abnormal failures remain unfulfilled evidence and do not qualify.

## Text/byte checks actually performed

- Every prior test function name in each of the eleven files is retained.
- Four generic Call/Result/Event/CommitResult declarations are byte-identical.
- rollbackFaultBefore, rollbackFaultBeforeCore, rollbackFaultAfter, rollbackFaultAfterCore are byte-identical, including deferred unlock and restart observer order.
- Existing baseline constructor, SQL query/readback and manualRollbackSQLRecoverCopy bodies are byte-identical.
- Seven owning source pins and seven full source snapshots exactly match the overlay bytes by text/hash comparison.
- 270 finite source mutation anchors were found in the intended source text. This is an anchor audit only, not parser execution or a passing guard test.
- No output file was written into W by this worker. Concurrent R5 additions were visible in W and were not modified.

## Authored new tests

- Two generic model roots, two SQL evidence roots, and one generic double-repinned source-guard root; no new native runnable root.
- 24 new generic and 25 new SQL source counterexamples. Existing SQL counterexamples remain with equivalent renamed ten-case/five-recovery diagnostics. The old post-DB overwrite negative uses its existing lifecycle assignment as the updated anchor. Unknown SQLCase 10 tests move to 11 because case 10 is now explicitly admitted.
- Rejections include exact action/geometry, native short/invalid result, wrong requested/native argument, fabricated count/payload, masked native error, misplaced result exposed as success, source-only payload/tail/journal checks, wrong recovery partition, NEW/mixed results and old-nine nonzero placement fields.

## Next finite validation, requiring separate authorization

1. Independently review the eleven-file implementation/guard slice and two manuals; compare all preserved functions/types and default/Commit/A5 files using the manifest.
2. Authorized gofmt only on eleven files, then refresh only seven pins/snapshots with {R/'misplaced-write-repin-text.py'} against the intended tree; formatting the two guards afterward must not change the embedded source values. Whole-main/review pins remain the merge lead’s responsibility.
3. Fixed official Go 1.27.2 under the parent’s controlled environment/single Go queue, with CGO_ENABLED=0, -mod=readonly, -buildvcs=false, -p=1, -vet=off, -count=1, -parallel=1, -timeout=0: internal/testkit/rollbackfault, internal/testkit/rollbackrestart, internal/testscope.
4. Only TestRollbackWriteFullAtMemoryContract in internal/pager; ordinary `go run ./internal/testscope`.
5. Compile-only sqlite and pager with manual_fault_rollback; compile-only sqlite with manual_fault_rollback,manual_restart_rollback. Never execute those images, even with an empty run selector.
6. Hash and nm the three exact images and record root inventories. A finite race check, if separately approved, is restricted to the same three model/source packages.

Tool and gofmt hashes must be independently verified against the frozen proposal before execution. Source pins are proposed expectations pending review, not permission to release unrelated whole-main HOLD. All actual native miswrite, native recovery, media/power-loss, durability and external qualification remain NOT_RUN. Do not broaden to ANN/campaign/full repository work. docs/progress.md and acceptance-status.md are left to the parent’s result-aware integration update.
'''
(E/'handoff.md').write_text(report)
print(json.dumps({'overlay_files':len(files),'go_files':sum(f.endswith('.go') for f in files),'patch_sha256':sha(patch),'manifest_sha256':sha(raw),'manifest':str(E/'source-manifest.json'),'handoff':str(E/'handoff.md')},indent=2))
