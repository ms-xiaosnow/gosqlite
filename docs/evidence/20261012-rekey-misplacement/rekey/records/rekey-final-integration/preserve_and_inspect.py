import pathlib,hashlib,json,re,difflib
R=pathlib.Path('/workspace/scratch/5e06a74e5f65/restart-20261012T2033'); W=R/'gosqlite'; E=R/'rekey-final-integration'; sha=lambda b:hashlib.sha256(b).hexdigest()
manifest=json.loads((E/'initial-manifest.json').read_text())
# Lexical comparison only. No Go parser/scanner, source selection or test execution.
lex=re.compile(r'//[^\n]*|/\*.*?\*/|"(?:[^"\\]|\\.)*"|`[^`]*`|\'(?:[^\'\\]|\\.)*\'|[A-Za-z_][A-Za-z_0-9]*|[0-9][A-Za-z_0-9.]*|<<=|>>=|&\^=|\.\.\.|:=|==|!=|<=|>=|&&|\|\||<-|\+\+|--|<<|>>|&\^|[+*/%&|^\-]=|[^\s;]',re.S)
def tokens(s):return [m.group() for m in lex.finditer(s) if not m.group().startswith(('//','/*'))]
format_diffs=[]; proofs=[]
for p in manifest['inputs']:
 old=(E/'before-format'/p).read_bytes(); new=(W/p).read_bytes()
 if p.endswith('.go'):assert tokens(old.decode())==tokens(new.decode()),p
 else:assert old==new,p
 after=E/'after-format'/p;after.parent.mkdir(parents=True,exist_ok=True);after.write_bytes(new)
 diff=''.join(difflib.unified_diff(old.decode().splitlines(True),new.decode().splitlines(True),fromfile='reviewed/'+p,tofile='formatted/'+p))
 format_diffs.append(diff)
 proofs.append({'path':p,'reviewed_sha256':sha(old),'formatted_sha256':sha(new),'changed':old!=new,'noncomment_nonseparator_token_spelling_equal':True})
for p,digest in manifest['models_preserved'].items():assert sha((W/p).read_bytes())==digest,p
(E/'format-only.diff').write_text(''.join(format_diffs))
(E/'format-proof.json').write_text(json.dumps(proofs,indent=2)+'\n')
guard=(W/'internal/testscope/rekey_interruption.go').read_text(); test=(W/'internal/testscope/rekey_interruption_test.go').read_text()
# The generated snapshot entries are JSON-compatible quoted Go string literals.
strpat=r'"(?:[^"\\]|\\.)*"'; shapesblock=guard.split('var rekeyGuardShapes = map[string]string{',1)[1].split('\n}',1)[0]
shapes={json.loads(m[0]):json.loads(m[1]) for m in re.findall('('+strpat+r'):\s*('+strpat+r'),',shapesblock)}
assert len(shapes)==9
for p,raw in shapes.items():
 group='rekey-guard' if p in ['internal/testscope/main.go','internal/testscope/oracle_startup.go'] else 'rekey-interruption'
 assert raw==(R/(group+'-overlay')/p).read_text(),p
 assert tokens(raw)==tokens((W/p).read_text()),p
lit=r'(?:`[^`]*`|"(?:[^"\\]|\\.)*")'
def decode(x):return x[1:-1] if x.startswith('`') else json.loads(x)
constants={k:json.loads(v) for k,v in re.findall(r'const (rekeyGuard\w+) = ('+strpat+r')',guard)}
anchor_calls=re.findall(r'rekeyGuardMutant\(t,\s*(rekeyGuard\w+),\s*('+lit+r'|rekeyGuard\w+),\s*('+lit+r')\)',test,re.S)
assert len(anchor_calls)==43,len(anchor_calls)
anchors=[]
for category,a,b in anchor_calls:
 p=constants[category]; old=decode(a) if a.startswith(('"','`')) else constants[a]
 assert old in shapes[p],(p,old)
 anchors.append({'path':p,'from':old,'to':decode(b),'before_count':shapes[p].count(old),'formatted_count':(W/p).read_text().count(old)})
(E/'anchors-before-sync.json').write_text(json.dumps(anchors,indent=2)+'\n')
print(json.dumps({'formatted_files':sum(p['changed'] for p in proofs),'format_diff_bytes':(E/'format-only.diff').stat().st_size,'all_nine_reviewed_snapshot_inputs_match':True,'stale_anchors':[a for a in anchors if not a['formatted_count']]},indent=2))
# All independent body strings remain unchanged and token equivalent to the
# frozen bodies within their final source. Do not repin/rewrite semantics.
bodies=[]
for name,p in [('rekeyGuardCriticalBodies',constants['rekeyGuardNative']),('rekeyGuardBridgeBodies',constants['rekeyGuardBridge'])]:
 block=guard.split('var '+name+' = map[string]string{',1)[1].split('\n}',1)[0]
 for k,v in re.findall('('+strpat+r'):\s*('+strpat+r'),',block):
  key,body=json.loads(k),json.loads(v)
  assert body in shapes[p],(name,key)
  needle=tokens(body); hay=tokens((W/p).read_text())
  matches=sum(hay[i:i+len(needle)]==needle for i in range(len(hay)-len(needle)+1))
  assert matches==1,(name,key,matches)
  bodies.append({'map':name,'function':key,'reviewed_body_sha256':sha(body.encode()),'same_token_body_in_formatted_source':True,'body_literal_unchanged':True})
assert len(bodies)==36
(E/'independent-body-proof.json').write_text(json.dumps(bodies,indent=2)+'\n')
