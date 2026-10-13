"""Approved incremental syntax checks only; no manual/oracle/campaign execution."""
from pathlib import Path
import collections, datetime, fcntl, hashlib, json, os, subprocess, time
R=Path(__file__).resolve().parent;W=R/'gosqlite';E=R/'pgvector-ddl-validation'
GO=R/'toolchains/go1.27.2/bin/go';FMT=GO.with_name('gofmt')
ENV=dict(PATH=str(GO.parent)+':/usr/bin:/bin',GOMAXPROCS='2',GOENV='off',GO111MODULE='on',GOWORK='off',GOTOOLCHAIN='local',GOPROXY='off',GOSUMDB='off',GOAUTH='off',GOFLAGS='',GOEXPERIMENT='',GOFIPS140='off',GODEBUG='',GOCACHEPROG='',GOCOVERDIR='',GORACE='',GOTELEMETRY='off',GOROOT=str(GO.parent.parent),GOPATH=str(R/'toolchains/gopath'),GOMODCACHE=str(R/'toolchains/gopath/pkg/mod'),GOCACHE=str(R/'go-cache'),TMPDIR='/tmp',GOTMPDIR='',GOOS='linux',GOARCH='amd64',GOAMD64='v1',CGO_ENABLED='0',LC_ALL='C',LANG='C',TZ='UTC')
PARSERS=['TestPGVectorIndexParseAccepted','TestPGVectorIndexParseRejectsUnsupported','TestPGVectorIndexOriginalGrammarUnchanged','TestIndexBinaryCollation','TestExtensionGrammar','TestUniqueIndexParseAccepted','TestUniqueIndexParsePreservesOrdinary','TestUniqueIndexParseRejectsUnsupported']
HEALTHY=['TestPGVectorIndexHealthyEquivalenceAndReopen','TestPGVectorIndexExistingAdmissionAndTransaction']
FLAGS=['test','-json','-mod=readonly','-buildvcs=false','-p=1','-vet=off','-count=1','-parallel=1','-timeout=0']
def selector(xs):return '^('+'|'.join(xs)+')$'
COMMANDS=[('01_format',[str(FMT),'-l','internal/sql/parser.go','internal/sql/pgvector_index_parse_test.go','sqlite/pgvector_index_test.go'],ENV,None),('02_parser',[str(GO),*FLAGS,'-run',selector(PARSERS),'./internal/sql'],ENV,{'internal/sql':PARSERS}),('03_healthy',[str(GO),*FLAGS,'-run',selector(HEALTHY),'./sqlite'],ENV,{'sqlite':HEALTHY}),('04_race',[str(GO),*FLAGS,'-race','-run',selector(PARSERS+HEALTHY),'./internal/sql','./sqlite'],dict(ENV,CGO_ENABLED='1',CC='/usr/bin/gcc',CXX='/usr/bin/g++'),{'internal/sql':PARSERS,'sqlite':HEALTHY}),('05_sourceguard',[str(GO),'run','-mod=readonly','-buildvcs=false','-p=1','./internal/testscope'],ENV,None)]
def now():return datetime.datetime.now(datetime.timezone.utc).isoformat()
def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def save(n,v):(E/n).write_text(json.dumps(v,ensure_ascii=False,indent=2)+'\n')
def snapshot():
 names=subprocess.check_output(['git','ls-files','--cached','--others','--exclude-standard','-z'],cwd=W).decode().split('\0')
 return {p:sha(W/p) for p in sorted(set(names)) if p}
def resources():
 out={'utc':now(),'qualified':True,'paths':{}}
 for p,minimum in [(R,2*1024**3),(Path('/tmp'),1024**3)]:
  s=os.statvfs(p);b=s.f_bavail*s.f_frsize;out['paths'][str(p)]={'available_bytes':b,'available_inodes':s.f_favail,'minimum_bytes':minimum};out['qualified'] &= b>=minimum and s.f_favail>=10000
 return out
def summarize(path,expected):
 roots=collections.defaultdict(set);counts=collections.defaultdict(collections.Counter);passed=set()
 for line in path.read_text().splitlines():
  e=json.loads(line);p=e.get('Package','').removeprefix('github.com/ms-xiaosnow/gosqlite/');n=e.get('Test');a=e.get('Action')
  if a=='run' and n and '/' not in n:roots[p].add(n)
  if a in ('pass','fail','skip') and n:counts[p][a]+=1
  if a=='pass' and not n:passed.add(p)
 assert passed==set(expected) and set(roots)==set(expected),(passed,roots)
 for p,ns in expected.items():assert roots[p]==set(ns) and not counts[p]['fail'] and not counts[p]['skip'],(p,roots[p],counts[p])
 return {p:{'roots':sorted(roots[p]),'root_count':len(roots[p]),'named_pass':counts[p]['pass'],'child_pass':counts[p]['pass']-len(roots[p]),'fail':counts[p]['fail'],'skip':counts[p]['skip']} for p in expected}
def main():
 E.mkdir(exist_ok=False)
 with (R/'go-queue.lock').open('a+') as lease:
  fcntl.flock(lease,fcntl.LOCK_EX|fcntl.LOCK_NB)
  assert sha(GO)=='548d3c32f83fa81152f543eb1526be7884cfe0526eb5e9ce29d3eca14786e7e0'
  assert sha(FMT)=='5583dbb3339147fee7cdb067eb3cd4a50015b48c27084377f49d93833aa4ad77'
  assert subprocess.check_output(['git','rev-parse','HEAD'],cwd=W).decode().strip()=='fd069d7a30b2e0ca8bdff67ca54c926aa118fc51'
  m=json.loads((R/'pgvector-ddl-overlay/manifest.json').read_text())
  for f in m['files']:assert sha(W/f['path'])==f['sha256']
  before=snapshot();save('source-before.json',before);save('commands.json',[{'name':n,'argv':a,'cwd':str(W),'environment':e,'expected':x} for n,a,e,x in COMMANDS]);save('start.json',{'utc':now(),'recorder_sha256':sha(Path(__file__)),'base':m['base_commit'],'scope':'8 pure parser roots + 2 healthy database roots normal/race, sourceguard; no manual/oracle/campaign','tool_go_sha256':sha(GO),'tool_gofmt_sha256':sha(FMT),'process_scope':'single coordinated Go lease; no global namespace claim'})
  results=[]
  for name,argv,env,expected in COMMANDS:
   assert snapshot()==before;initial=resources();save(name+'.resources-before.json',initial);assert initial['qualified']
   started=now();latched=False
   with (E/(name+'.stdout')).open('wb') as out,(E/(name+'.stderr')).open('wb') as err,(E/(name+'.resources.jsonl')).open('w') as samples:
    p=subprocess.Popen(argv,cwd=W,env=env,stdout=out,stderr=err)
    try:
     while True:
      s=resources();samples.write(json.dumps(s)+'\n');samples.flush();latched|=not s['qualified'];code=p.poll()
      if code is not None:break
      time.sleep(2)
    except BaseException:
     p.wait();raise
   result={'name':name,'start_utc':started,'end_utc':now(),'exit_code':code,'resource_breach_latched':latched,'stdout_sha256':sha(E/(name+'.stdout')),'stderr_sha256':sha(E/(name+'.stderr'))};save(name+'.result.json',result)
   assert code==0 and not latched,result
   assert snapshot()==before
   if name=='01_format':assert not (E/(name+'.stdout')).read_bytes()
   if expected:result['tests']=summarize(E/(name+'.stdout'),expected)
   result['status']='PASS';save(name+'.result.json',result);results.append(result);print(json.dumps({'utc':now(),'name':name,'status':'PASS','tests':result.get('tests')}),flush=True)
  save('completion.json',{'utc':now(),'status':'PASS','commands':results,'source_unchanged':snapshot()==before,'manual_oracle_campaign':'NOT_RUN','unrelated_tests':'NOT_RERUN; prior c5/fd evidence applicability preserved','lease':'RELEASED_ON_RETURN'})
if __name__=='__main__':
 try:main()
 except BaseException as e:
  if E.exists():save('STOP.json',{'utc':now(),'type':type(e).__name__,'error':str(e),'remaining_commands':'NOT_RUN'})
  raise
