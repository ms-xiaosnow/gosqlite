"""Read-only source-text audit; only writes evidence in this directory.
No Go tool, source parser/scanner/AST runner, test, compiler, or native execution.
The single authorized gofmt and reviewed seven-field repin ran before this audit.
"""
from pathlib import Path
import hashlib,json,re,difflib,subprocess
R=Path('/workspace/scratch/5e06a74e5f65/restart-20261012T2033'); E=R/'misplaced-write-format'; B=E/'before'; O=R/'misplaced-write-overlay'; W=R/'gosqlite'
sha=lambda b:hashlib.sha256(b).hexdigest()
m=json.loads((B/'source-manifest.json').read_text()); base=m['base_commit']
protected=re.compile(r'//[^\n]*|/\*[\s\S]*?\*/|"(?:[^"\\]|\\.)*"|`[^`]*`|\'(?:[^\'\\]|\\.)*\'')
def nonspace_text(s):
 out=[];end=0
 for match in protected.finditer(s):
  out.append(re.sub(r'\s+','',s[end:match.start()]));out.append(match.group());end=match.end()
 out.append(re.sub(r'\s+','',s[end:]));return ''.join(out)
items={'internal/testscope/rollback_fault.go':('rollbackFaultOwnedShapes',{'rollbackFaultManual':'internal/pager/rollback_io_manual_linux.go','rollbackFaultContract':'internal/testkit/rollbackfault/contract.go','rollbackFaultContractTest':'internal/testkit/rollbackfault/contract_test.go'}),'internal/testscope/rollback_fault_sql.go':('rollbackFaultSQLShapes',{'rollbackFaultSQLBridge':'internal/pager/rollback_fault_sql_manual_linux.go','rollbackFaultSQLNative':'sqlite/rollback_fault_sql_manual_linux_test.go','rollbackFaultSQLModel':'internal/testkit/rollbackfault/sql_ledger.go','rollbackFaultSQLModelTest':'internal/testkit/rollbackfault/sql_ledger_test.go'})}
def omit_seven_owned_values(p,s,verify=False):
 if p not in items:return s
 table,entries=items[p];head,tail=s.split('var '+table+' =',1)
 for key,source in entries.items():
  pin_pattern=r'(^\t'+key+r':\s+)"([0-9a-f]{64})"';snap_pattern=r'(^\t'+key+r':\s+)(`[^`]*`|"(?:[^"\\]|\\.)*")(,\n)'
  pin=re.search(pin_pattern,head,re.M);snap=re.search(snap_pattern,tail,re.M);assert pin and snap,(p,key)
  if verify:
   source_bytes=(O/source).read_bytes();value=snap[2][1:-1] if snap[2][0]=='`' else json.loads(snap[2]);assert pin[2]==sha(source_bytes);assert value.encode()==source_bytes
  head,n=re.subn(pin_pattern,lambda m:m[1]+'"<SOURCE-PIN>"',head,count=1,flags=re.M);assert n==1
  tail,n=re.subn(snap_pattern,lambda m:m[1]+'"<SOURCE-SNAPSHOT>"'+m[3],tail,count=1,flags=re.M);assert n==1
 return head+'var '+table+' ='+tail
checks={};formatdiff=[];basepatch=[];finalfiles={}
for p,d in sorted(m['files'].items()):
 before=(B/p).read_bytes();after=(O/p).read_bytes();assert sha(before)==d['overlay_sha256'],('frozen source changed',p)
 basebytes=subprocess.check_output(['git','-C',str(W),'show',base+':'+p]);assert sha(basebytes)==d['base_sha256'],('base manifest mismatch',p)
 if p.endswith('.go'):
  a=nonspace_text(omit_seven_owned_values(p,before.decode()));b=nonspace_text(omit_seven_owned_values(p,after.decode(),True));assert a==b,('non-format/non-pin change',p)
  checks[p]={'only_whitespace_outside_protected_literals_comments_and_seven_owned_values':True,'normalized_sha256':sha(a.encode())}
 else:assert before==after
 formatdiff.extend(difflib.unified_diff(before.decode().splitlines(True),after.decode().splitlines(True),'reviewed/'+p,'final-overlay/'+p))
 basepatch.extend(difflib.unified_diff(basebytes.decode().splitlines(True),after.decode().splitlines(True),'a/'+p,'b/'+p))
 finalfiles[p]={'sha256':sha(after),'bytes':len(after),'reviewed_sha256':sha(before),'base_sha256':sha(basebytes)}
(E/'final-format-and-repin.diff').write_text(''.join(formatdiff));(E/'final-overlay-from-01c.patch').write_text(''.join(basepatch));(E/'final-text-check.json').write_text(json.dumps(checks,indent=2)+'\n')
strings=r'(?:"(?:[^"\\]|\\.)*"|`[^`]*`)';pattern=re.compile(r'^\t\t\{('+strings+r'), (rollbackFault\w+), ('+strings+r'|rollbackFault\w+),',re.M)
def block(s,start):
 a=s.index(start);b=s.index('\n}',a)+2;return s[a:b]
def decode(s):return s[1:-1] if s.startswith('`') else json.loads(s)
constants={}
for p in items:
 for match in re.finditer(r'^const (rollbackFault\w+) = ('+strings+r')$',(O/p).read_text(),re.M):constants[match[1]]=decode(match[2])
anchors=[]
for p,funcs in [('internal/testscope/rollback_fault_test.go',['TestRollbackFaultRepinnedSemanticCounterexamples','TestRollbackFaultMisplacementDoubleRepinnedCounterexamples']),('internal/testscope/rollback_fault_sql_test.go',['TestRollbackFaultSQLRepinnedSemanticCounterexamples'])]:
 for name in funcs:
  b=list(pattern.finditer(block((B/p).read_text(),'func '+name+'(')));a=list(pattern.finditer(block((O/p).read_text(),'func '+name+'(')));assert [v.groups() for v in b]==[v.groups() for v in a]
  for match in a:
   label,key,expr=match.groups();label=decode(label);source=constants[key];needle=constants[expr] if expr.startswith('rollbackFault') else decode(expr)
   prior=(B/source).read_text() if (B/source).exists() else (W/source).read_text();current=(O/source).read_text() if (O/source).exists() else (W/source).read_text()
   assert needle in prior and needle in current,(p,label)
   anchors.append({'test_file':p,'function':name,'name':label,'source':source,'before_count':prior.count(needle),'after_count':current.count(needle),'anchor_sha256':sha(needle.encode())})
assert len(anchors)==290; (E/'mutation-anchor-check.json').write_text(json.dumps(anchors,indent=2)+'\n');(E/'03-anchor-adaptation.diff').write_text('')
# Keep the old reviewed main pins exact; W integration is a separate three-way merge.
for p,pin in json.loads((E/'old-main-pins.json').read_text()).items():assert 'const reviewedMainSHA256 = "'+pin+'"' in (O/p).read_text()
changed=subprocess.check_output(['git','-C',str(W),'diff','--name-only',base,'--']).decode().splitlines();overlaps=sorted(set(changed)&set(m['files']))
assert overlaps==['internal/testscope/rollback_fault_sql_test.go','internal/testscope/rollback_fault_test.go'],overlaps
(E/'known-w-overlap-paths.txt').write_text(''.join(p+'\n' for p in overlaps))
(E/'w-overlap-diff-from-01c.patch').write_bytes(subprocess.check_output(['git','-C',str(W),'diff',base,'--',*overlaps]))
w_before=json.loads((E/'w-overlay-paths-before.json').read_text()); w_final={p:sha((W/p).read_bytes()) for p in m['files']};assert w_before==w_final,'concurrent W source changed; review attribution'
(E/'w-overlay-paths-after.json').write_text(json.dumps(w_final,indent=2)+'\n')
excluded={p:{'reviewed_sha256':h,'current_w_sha256':sha((W/p).read_bytes())} for p,h in m['excluded_files'].items()}
for p,d in excluded.items():assert d['reviewed_sha256']==d['current_w_sha256'] or p=='internal/testscope/main.go',p
roots={}
for p in ['internal/testscope/rollback_fault_test.go','internal/testscope/rollback_fault_sql_test.go','internal/testscope/rollback_restart_test.go']:
 source=O/p if (O/p).exists() else W/p; roots[p]=re.findall(r'^func (Test\w+)\(t \*testing\.T\)',source.read_text(),re.M)
selected=[n for names in roots.values() for n in names];assert len(selected)==len(set(selected));selector='^('+'|'.join(selected)+')$'
(E/'selected-testscope-roots.json').write_text(json.dumps({'files':roots,'root_count':len(selected),'run_selector':selector,'status':'NOT_RUN'},indent=2)+'\n')
(E/'selected-testscope-run-selector.txt').write_text(selector+'\n')
manifest={'schema':'rollback-misplaced-write-format-overlay-v1','overlay':str(O),'base_commit':base,'review_sha256':sha((R/'misplaced-write-independent-review.md').read_bytes()),'reviewed_source_manifest_sha256':sha((B/'source-manifest.json').read_bytes()),'gofmt_sha256':sha((R/'toolchains/go1.27.2/bin/gofmt').read_bytes()),'repin_script_sha256':sha((R/'misplaced-write-repin-text.py').read_bytes()),'format_status':'COMPLETE_EXACT_ELEVEN_FILES_ONE_GO_FMT_INVOCATION','text_check_status':'WHITESPACE_ONLY_WITH_LITERAL_COMMENT_PRESERVATION_PLUS_SEVEN_OWNED_PIN_SNAPSHOTS','anchor_status':{'all_present':True,'count':290,'adaptations':0,'prior_handoff_count_270_is_stale':True},'main_pin_status':'OLD_REVIEWED_PIN_PRESERVED_FOR_SEPARATE_THREE_WAY_W_MERGE','w_overlay_paths_unchanged_during_worker':True,'known_w_overlap_paths':overlaps,'excluded_current_w_sources':excluded,'model_guard_compile_status':'NOT_RUN','go_parser_scanner_ast_execution_status':'NOT_RUN_EXCEPT_GOFMT_INTERNALS','native_execution_status':'NOT_RUN','files':finalfiles,'evidence_files':{}}
for name in ['01-gofmt.diff','02-repin.diff','03-anchor-adaptation.diff','final-format-and-repin.diff','final-overlay-from-01c.patch','gofmt-text-check.json','final-text-check.json','seven-pin-snapshot-check.json','mutation-anchor-check.json','old-main-pins.json','known-w-overlap-paths.txt','w-overlap-diff-from-01c.patch','selected-testscope-roots.json','selected-testscope-run-selector.txt']:
 raw=(E/name).read_bytes();manifest['evidence_files'][name]={'sha256':sha(raw),'bytes':len(raw)}
(E/'final-path-sha256-manifest.json').write_text(json.dumps(manifest,indent=2,sort_keys=True)+'\n')
print(json.dumps({'manifest':str(E/'final-path-sha256-manifest.json'),'manifest_sha256':sha((E/'final-path-sha256-manifest.json').read_bytes()),'selected_testscope_roots':roots,'root_count':len(selected),'known_w_overlap_paths':overlaps,'all_290_anchors_present':True,'no_anchor_adaptations':True,'all_13_w_overlay_paths_unchanged':True},indent=2))
