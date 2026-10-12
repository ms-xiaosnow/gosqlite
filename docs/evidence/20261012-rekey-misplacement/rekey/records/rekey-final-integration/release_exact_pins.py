import pathlib,hashlib,json,re,difflib,subprocess
R=pathlib.Path('/workspace/scratch/5e06a74e5f65/restart-20261012T2033');W=R/'gosqlite';E=R/'rekey-final-integration';sha=lambda b:hashlib.sha256(b).hexdigest()
manifest=json.loads((E/'initial-manifest.json').read_text())
assert (E/'format-only.diff').is_file() and (E/'format-proof.json').is_file() and (E/'independent-body-proof.json').is_file()
guardpath='internal/testscope/rekey_interruption.go';testpath='internal/testscope/rekey_interruption_test.go'
raw=(W/guardpath).read_text();before=raw
strpat=r'"(?:[^"\\]|\\.)*"'
constants={k:json.loads(v) for k,v in re.findall(r'const (rekeyGuard\w+) = ('+strpat+r')',raw)}
shapematch=re.search(r'(var rekeyGuardShapes = map\[string\]string\{\n)(.*?)(\n\})',raw,re.S); assert shapematch
records=[]
def snapshot(m):
 p=json.loads(m[2]); prior=json.loads(m[4]); final=(W/p).read_text()
 group='rekey-guard' if p in ['internal/testscope/main.go','internal/testscope/oracle_startup.go'] else 'rekey-interruption'
 assert prior==(R/(group+'-overlay')/p).read_text(),p
 records.append({'path':p,'reviewed_source_sha256':sha(prior.encode()),'final_source_sha256':sha(final.encode()),'snapshot_exact_final_source':True})
 return m[1]+m[2]+m[3]+json.dumps(final,ensure_ascii=False)+m[5]
block=re.sub(r'(^\s*)('+strpat+r')(:\s*)('+strpat+r')(,)$',snapshot,shapematch[2],flags=re.M)
assert len(records)==9
raw=raw[:shapematch.start(2)]+block+raw[shapematch.end(2):]
start=raw.index('var rekeyGuardPins = map[string]string{');end=raw.index('\n}',start)
pins=[]
def pin(m):
 key=m[2];p=constants[key];value=sha((W/p).read_bytes());pins.append({'key':key,'path':p,'sha256':value})
 return m[1]+key+m[3]+'"'+value+'"'+m[4]
block=re.sub(r'(^\s*)(rekeyGuard\w+)(:\s*)rekeyGuardHold(,)$',pin,raw[start:end],flags=re.M)
assert len(pins)==9
raw=raw[:start]+block+raw[end:]
assert raw.count('const rekeyGuardHold = "HOLD_PENDING_INDEPENDENT_REKEY_FULL_SOURCE_REVIEW"')==1
(W/guardpath).write_text(raw)
# Exactly one stale anchor, caused only by gofmt alignment of adjacent methods.
test=(W/testpath).read_text();old='func (Result) Pass() bool { return false }';new='func (Result) Pass() bool              { return false }'
assert old not in (W/constants['rekeyGuardModel']).read_text() and new in (W/constants['rekeyGuardModel']).read_text()
assert test.count('`'+old+'`')==1 and test.count('`'+old.replace('false','true')+'`')==1
test=test.replace('`'+old+'`','`'+new+'`').replace('`'+old.replace('false','true')+'`','`'+new.replace('false','true')+'`')
(W/testpath).write_text(test)
main=(W/'internal/testscope/main.go').read_bytes();mainsha=sha(main)
oldmain='6dbab48ff374858bfafe578f0fca99f5176ced7b6bdc008bec04ce6a6d1d0eb2'
for p in manifest['pin_owners']:
 data=(W/p).read_bytes();prior=(E/'before-files'/p).read_bytes()
 assert data==prior and data.count(oldmain.encode())==1,p
 data=data.replace(oldmain.encode(),mainsha.encode());(W/p).write_bytes(data)
 assert data.replace(mainsha.encode(),oldmain.encode())==prior,p
# The ordinary source function is first; main is the next top-level function.
base=(E/'before-files/internal/testscope/main.go').read_bytes()
def ordinary(s):return s[s.index(b'func inspectSource('):s.index(b'\nfunc main()')].rstrip()
assert ordinary(base)==ordinary(main)
body=ordinary(main);body=body[body.index(b'{'):]
assert sha(body)=='98341345e43f39b39dbc78eaf49a916a0701f3d7f2f383d4949b9a3ed439d199',sha(body)
(E/'inspectSource-body.txt').write_bytes(body)
(E/'mainbody-unchanged-proof.json').write_text(json.dumps({'base_commit':'01c63e6f8fb3d9e196afe2f42a163eb3eac4ad5a','base_main_sha256':sha(base),'final_main_sha256':mainsha,'ordinary_function_byte_identical':True,'ordinary_body_sha256':sha(body),'old_whole_main_literal_replacements':manifest['pin_owners'],'R1_wal_model_pin_unchanged':'465813a5c6c4b1324ad6b4ed4850585c24d145e5e3a7a4f7e22ca1d178cd0902'},indent=2)+'\n')
for p,digest in manifest['models_preserved'].items():assert sha((W/p).read_bytes())==digest,p
syncdiff=[]
for p in [guardpath,testpath]+manifest['pin_owners']:
 old=(E/'after-format'/p).read_text() if (E/'after-format'/p).exists() else (E/'before-files'/p).read_text()
 diff=''.join(difflib.unified_diff(old.splitlines(True),(W/p).read_text().splitlines(True),fromfile='post-format/'+p,tofile='released/'+p));syncdiff.append(diff)
(E/'pin-snapshot-anchor-sync.diff').write_text(''.join(syncdiff))
(E/'pin-release.json').write_text(json.dumps({'nine_pins':pins,'nine_snapshots':records,'mutation_anchor_changes':[{'path':testpath,'from':old,'to':new,'reason':'gofmt alignment only; false and true mutation pair changed together'}] if False else [{'path':testpath,'before':'func (Result) Pass() bool { return false }','after':new,'matching_true_replacement_updated':True}],'critical_body_literals_changed':0,'hold_constant_retained':True,'negative_tests_removed_or_weakened':False},indent=2)+'\n')
print(json.dumps({'final_main_sha256':mainsha,'ordinary_body_sha256':sha(body),'nine_snapshots_and_pins_synchronized':True,'six_old_main_literals_replaced':True,'critical_body_literals_changed':0,'mutant_anchor_pairs_format_adjusted':1},indent=2))
