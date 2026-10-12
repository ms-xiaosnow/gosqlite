import pathlib,hashlib,json,re,difflib,subprocess
R=pathlib.Path('/workspace/scratch/5e06a74e5f65/restart-20261012T2033');W=R/'gosqlite';E=R/'rekey-final-integration';sha=lambda b:hashlib.sha256(b).hexdigest()
M=json.loads((E/'initial-manifest.json').read_text()); strpat=r'"(?:[^"\\]|\\.)*"'; lit=r'(?:`[^`]*`|"(?:[^"\\]|\\.)*")'
def git(*args):return subprocess.check_output(['git','-C',str(W),*args])
def decode(s):return s[1:-1] if s.startswith('`') else json.loads(s)
def block(s,name):return re.search(r'(var '+name+r' = map\[string\]string\{\n)(.*?)(\n\})',s,re.S)
guardpath='internal/testscope/rekey_interruption.go';testpath='internal/testscope/rekey_interruption_test.go';raw=(W/guardpath).read_text();before=(E/'after-format'/guardpath).read_text()
constants={k:json.loads(v) for k,v in re.findall(r'const (rekeyGuard\w+) = ('+strpat+r')',raw)}
shapes={json.loads(k):json.loads(v) for k,v in re.findall('('+strpat+r'):\s*('+strpat+r'),',block(raw,'rekeyGuardShapes')[2])}
pins={k:json.loads(v) for k,v in re.findall(r'(rekeyGuard\w+):\s*('+strpat+r'),',block(raw,'rekeyGuardPins')[2])}
assert len(shapes)==len(pins)==9
for p,s in shapes.items():assert s==(W/p).read_text(),p
for k,v in pins.items():assert v==sha((W/constants[k]).read_bytes()),k
# Mask only authorized nine pin values and nine snapshot values. Everything
# outside those two supplemental maps must remain exact after formatting.
def masked(s):
 for name in ['rekeyGuardPins','rekeyGuardShapes']:
  m=block(s,name);s=s[:m.start(2)]+'<authorized supplemental identity values>'+s[m.end(2):]
 return s
assert masked(raw)==masked(before)
assert raw.count('const rekeyGuardHold = "HOLD_PENDING_INDEPENDENT_REKEY_FULL_SOURCE_REVIEW"')==1
oldanchor='func (Result) Pass() bool { return false }';newanchor='func (Result) Pass() bool              { return false }'
test=(W/testpath).read_text();oldtest=(E/'after-format'/testpath).read_text()
assert test==oldtest.replace('`'+oldanchor+'`','`'+newanchor+'`').replace('`'+oldanchor.replace('false','true')+'`','`'+newanchor.replace('false','true')+'`')
anchors=re.findall(r'rekeyGuardMutant\(t,\s*(rekeyGuard\w+),\s*('+lit+r'|rekeyGuard\w+),\s*('+lit+r')\)',test,re.S);assert len(anchors)==43
for k,a,b in anchors:assert (decode(a) if a.startswith(('"','`')) else constants[a]) in shapes[constants[k]],(k,a)
main=(W/'internal/testscope/main.go').read_bytes();mainsha=sha(main);base=(E/'before-files/internal/testscope/main.go').read_bytes()
def ordinary(s):return s[s.index(b'func inspectSource('):s.index(b'\nfunc main()')].rstrip()
body=ordinary(main);assert body==ordinary(base);assert sha(body)=='98341345e43f39b39dbc78eaf49a916a0701f3d7f2f383d4949b9a3ed439d199'
(E/'inspectSource-function.txt').write_bytes(body)
oldmain=b'6dbab48ff374858bfafe578f0fca99f5176ced7b6bdc008bec04ce6a6d1d0eb2'
for p in M['pin_owners']:
 prior=(E/'before-files'/p).read_bytes();data=(W/p).read_bytes();assert prior.count(oldmain)==1
 assert data==prior.replace(oldmain,mainsha.encode()),p
assert b'"internal/vfs/wal_publication_model_test.go": "465813a5c6c4b1324ad6b4ed4850585c24d145e5e3a7a4f7e22ca1d178cd0902"' in (W/M['pin_owners'][0]).read_bytes()
for p,digest in M['models_preserved'].items():assert sha((W/p).read_bytes())==digest,p
for p in M['inputs']:
 if p not in [guardpath,testpath]:assert (W/p).read_bytes()==(E/'after-format'/p).read_bytes(),p
paths=sorted(set(M['inputs'])|set(M['pin_owners'])|set(M['models_preserved']));assert len(paths)==20
beforepaths=git('diff','--name-only','HEAD').decode().splitlines()+git('ls-files','--others','--exclude-standard').decode().splitlines();assert set(beforepaths)==set(paths)
syncdiff=[]
for p in [guardpath,testpath]+M['pin_owners']:
 old=(E/'after-format'/p).read_text() if (E/'after-format'/p).exists() else (E/'before-files'/p).read_text()
 syncdiff.append(''.join(difflib.unified_diff(old.splitlines(True),(W/p).read_text().splitlines(True),fromfile='post-format/'+p,tofile='released/'+p)))
(E/'pin-snapshot-anchor-sync.diff').write_text(''.join(syncdiff))
(E/'mainbody-unchanged-proof.json').write_text(json.dumps({'base_commit':'01c63e6f8fb3d9e196afe2f42a163eb3eac4ad5a','base_main_sha256':sha(base),'final_main_sha256':mainsha,'ordinary_function_byte_identical':True,'ordinary_function_sha256':sha(body),'ordinary_pin_definition':'complete inspectSource declaration, from func through closing brace','six_old_main_literal_replacements':M['pin_owners'],'R1_wal_model_pin_unchanged':'465813a5c6c4b1324ad6b4ed4850585c24d145e5e3a7a4f7e22ca1d178cd0902'},indent=2)+'\n')
(E/'pin-release.json').write_text(json.dumps({'nine_pins':{constants[k]:v for k,v in pins.items()},'nine_snapshots_match_actual_sources':True,'43_mutant_anchors_present':True,'mutant_anchor_pair_changes':[{'path':testpath,'before':oldanchor,'after':newanchor,'matching_true_replacement_updated':True}],'critical_body_literals_changed':0,'hold_constant_retained':True,'negative_tests_removed_or_weakened':False,'only_two_supplemental_maps_changed_in_formatted_guard':True,'prior_scope_source_state':'Both independent reviews SOURCE_REVIEW_CLEAR_WITH_LIMITS; narrow final-format/pin review PENDING'},indent=2)+'\n')
# Only this finite 20-file integration is staged. No commit or remote action.
git('add','--',*paths);assert git('diff','--name-only').decode()==''
assert set(git('diff','--cached','--name-only').decode().splitlines())==set(paths)
tree=git('write-tree').decode().strip()
full=[]
for p in paths:
 data=(W/p).read_bytes();assert data==git('show',':'+p)
 full.append({'path':p,'full_path':str(W/p),'bytes':len(data),'sha256':sha(data)})
receipt={'base_commit':'01c63e6f8fb3d9e196afe2f42a163eb3eac4ad5a','candidate_tree':tree,'files':{x['path']:x['sha256'] for x in full},'file_details':full,'models_preserved':M['models_preserved'],'final_main_sha256':mainsha,'ordinary_function_sha256':sha(body),'scoped_files':20,'new_source_docs_files_integrated':12,'exact_old_main_pin_owners_updated':6,'supplemental_R5_pins_released':9,'snapshot_sources_synchronized':9,'gofmt':{'path':str(R/'toolchains/go1.27.2/bin/gofmt'),'sha256':'5583dbb3339147fee7cdb067eb3cd4a50015b48c27084377f49d93833aa4ad77','invocations':1,'files':11,'exit_code':0,'serialization_lock':str(R/'go-queue.lock')},'validation_this_integration':{'Go_tests':'NOT_RUN','Go_parser_or_scanner_execution':'NOT_RUN_EXCEPT_AUTHORIZED_GOFMT','compiler':'NOT_RUN','source_selection':'NOT_RUN','source_guard':'NOT_RUN','native_fixture_wrongkey_process_exit_archive_restore_physical':'NOT_RUN'},'independent_narrow_format_pin_review':'PENDING','git_commit':'NOT_CREATED','remote_action':'NONE'}
(E/'SOURCE-IDENTITY.json').write_text(json.dumps(receipt,indent=2)+'\n')
(E/'fullpath-manifest.json').write_text(json.dumps(full,indent=2)+'\n')
(E/'final-integration.diff').write_bytes(git('diff','--cached','--binary','HEAD','--',*paths))
(E/'final-status.txt').write_bytes(git('status','--short'))
(E/'final-diff-check.txt').write_bytes(git('diff','--cached','--check'))
print(json.dumps({'candidate_tree':tree,'receipt':str(E/'SOURCE-IDENTITY.json'),'scoped_files':20,'final_main_sha256':mainsha,'ordinary_function_sha256':sha(body),'two_models_unchanged':True,'all_43_final_mutant_anchors_present':True,'no_semantic_body_literal_changes':True},indent=2))
