from pathlib import Path
import ast,datetime,fcntl,hashlib,json,os,subprocess
R=Path(__file__).resolve().parent;W=R/'gosqlite';E=R/'performance-offline-validation';P=Path('/opt/codex/runtimes/codex-primary-runtime/dependencies/python/bin/python3.12')
sha=lambda p:hashlib.sha256(p.read_bytes()).hexdigest()
def now():return datetime.datetime.now(datetime.timezone.utc).isoformat()
def save(n,x):(E/n).write_text(json.dumps(x,indent=2)+'\n')
def res():
 d={}
 for p,n in [(R,2*1024**3),(Path('/tmp'),1024**3)]:
  s=os.statvfs(p);d[str(p)]={'bytes':s.f_bavail*s.f_frsize,'inodes':s.f_favail};assert d[str(p)]['bytes']>=n and s.f_favail>=10000
 return d
E.mkdir(exist_ok=False)
with (R/'go-queue.lock').open('a+') as f:
 fcntl.flock(f,fcntl.LOCK_EX|fcntl.LOCK_NB)
 assert sha(P)=='fa67443527ed9647f760d807e2a38f26340757123e643c4639cf273ed15d5ea7'
 m=json.loads((R/'performance-offline-integration.json').read_text());before={x['path']:sha(W/x['path']) for x in m['files']};assert all(before[x['path']]==x['integrated_sha256'] for x in m['files'])
 t=ast.parse((W/'benchmarks/performance/test_compare_offline.py').read_text());names=[c.name+'.'+x.name for c in t.body if isinstance(c,ast.ClassDef) for x in c.body if isinstance(x,ast.FunctionDef) and x.name.startswith('test_')];assert len(names)==57
 env={'PATH':str(P.parent)+':/usr/bin:/bin','LC_ALL':'C','LANG':'C','TZ':'UTC','PYTHONDONTWRITEBYTECODE':'1','PYTHONNOUSERSITE':'1'}
 commands=[('01_fixtures',[str(P),'-m','unittest','discover','-s','benchmarks/performance','-p','test_compare_offline.py','-v']),('02_shell_syntax',['/bin/bash','--noprofile','--norc','-n','scripts/check.sh'])]
 save('source.json',before);save('start.json',{'utc':now(),'base':m['base'],'python_sha256':sha(P),'cwd':str(W),'env':env,'test_methods':names,'scope':'57 offline fixture methods; shell syntax only; no native runner or measurements','commands':commands});results=[]
 for name,argv in commands:
  b=res();start=now()
  with (E/(name+'.stdout')).open('wb') as out,(E/(name+'.stderr')).open('wb') as err:p=subprocess.run(argv,cwd=W,env=env,stdout=out,stderr=err)
  x={'name':name,'start':start,'end':now(),'exit':p.returncode,'resources_before':b,'resources_after':res(),'stdout_sha256':sha(E/(name+'.stdout')),'stderr_sha256':sha(E/(name+'.stderr'))};save(name+'.result.json',x)
  assert p.returncode==0,x
  if name=='01_fixtures':
   log=(E/(name+'.stderr')).read_text();assert 'Ran 57 tests' in log and log.rstrip().endswith('OK');assert sum(line.rstrip().endswith(' ... ok') for line in log.splitlines())==57
  x['status']='PASS';save(name+'.result.json',x);results.append(x);print(name,'PASS',flush=True)
 assert before=={k:sha(W/k) for k in before}
 save('completion.json',{'utc':now(),'status':'PASS','results':results,'methods':57,'source_unchanged':True,'real_baseline_measurements':'NOT_RUN/EXTERNAL_INPUT_REQUIRED','performance_regression_CI':'NOT_DEPLOYED; ordinary CI executes synthetic comparator unittest only'})
