from pathlib import Path
import datetime,hashlib,json,os,subprocess,sys
from freeze import inventory
root=Path('/workspace/scratch/5e06a74e5f65/durable-restart');ev=root/'rollback-reader-acquisition-evidence'
env=os.environ.copy();env.update(PATH=str(root/'toolchains/go1.27.2/bin')+':'+env['PATH'],GOTOOLCHAIN='local',GOPROXY='off',GOSUMDB='off',GOCACHE=str(root/'go-cache'),GOPATH=str(root/'go-path'),GOFLAGS='-buildvcs=false',CGO_ENABLED='0')
identity=json.loads((ev/'identity-v1.json').read_text()); records=[]
def run(name,cwd,args,cgo='0',expected=0):
 print(datetime.datetime.now(datetime.timezone.utc).isoformat(),name,flush=True)
 e=env.copy();e['CGO_ENABLED']=cgo
 with (ev/(name+'.txt')).open('w') as log:
  log.write('cwd='+str(cwd)+'\nCGO_ENABLED='+cgo+'\ncommand='+repr(args)+'\n');log.flush()
  p=subprocess.run(args,cwd=cwd,env=e,stdout=log,stderr=subprocess.STDOUT)
 (ev/(name+'.exit')).write_text(str(p.returncode)+'\n');records.append({'name':name,'exit':p.returncode,'expected_exit':expected});(ev/'checks-v1.json').write_text(json.dumps(records,indent=2)+'\n')
 if p.returncode!=expected:print((ev/(name+'.txt')).read_text(),flush=True);raise SystemExit(f'{name} unexpected status {p.returncode}')
 print(name,'exit',p.returncode,flush=True)
def verify(name,path):
 expected=json.loads((ev/(name+'-inventory.json')).read_text());assert inventory(path)==expected,name
f=root/'rollback-reader-acquisition-freeze-v1';control=root/'rollback-reader-acquisition-control-v1'
for n,p in [('freeze-v1',f),('control-v1',control),('candidate-soak-v1',root/'rollback-reader-acquisition-candidate-soak-v1'),('baseline-soak-v1',root/'rollback-reader-acquisition-baseline-soak-v1')]:verify(n,p)
run('frozen-scope-v1',f,['go','run','./internal/testscope'])
run('frozen-normal-v1',f,['go','test','./internal/pager','-run','^Test(ReaderAcquisition|ReadOnlyPageAccess$|TxCreatePageSizesAndReadSnapshot$)','-count=20','-v'])
run('frozen-sql-normal-v1',f,['go','test','./sqlite','-run','^TestTransactionDeferredSnapshotTiming$','-count=10','-v'])
run('frozen-vet-v1',f,['go','vet','./internal/pager','./sqlite'])
run('frozen-build-v1',f,['go','build','./internal/pager','./sqlite'])
run('frozen-race-v1',f,['go','test','-race','./internal/pager','-run','^Test(ReaderAcquisition|ReadOnlyPageAccess$|TxCreatePageSizesAndReadSnapshot$)','-count=5','-v'],cgo='1')
run('frozen-sql-race-v1',f,['go','test','-race','./sqlite','-run','^TestTransactionDeferredSnapshotTiming$','-count=5','-v'],cgo='1')
run('control-scope-v1',control,['go','run','./internal/testscope'])
run('control-source-regression-v1',control,['go','test','./internal/pager','-run','^TestReaderAcquisitionSourceSnapshotAfterShared$','-count=1','-v'],expected=1)
for kind in ['baseline','candidate']:
 src=root/f'rollback-reader-acquisition-{kind}-soak-v1'
 run(f'{kind}-composition-scope-v1',src,['go','run','./internal/testscope'])
 run(f'{kind}-composition-build-v1',src,['go','build','-trimpath','-o',str(ev/f'transactions-{kind}-v1'),'./benchmarks/transactions'])
run('candidate-harness-normal-v1',root/'rollback-reader-acquisition-candidate-soak-v1',['go','test','./benchmarks/transactions','-count=1','-v'])
run('candidate-harness-race-v1',root/'rollback-reader-acquisition-candidate-soak-v1',['go','test','-race','./benchmarks/transactions','-count=1','-v'],cgo='1')
for kind,i in [('baseline',1),('candidate',1),('baseline',2),('candidate',2),('candidate',3),('candidate',4)]:
 name=f'soak-v2-{kind}-{i}';args=[str(ev/f'transactions-{kind}-v1'),'-mode','soak','-profile','rollback','-duration','20s','-rows','64','-readers','2','-batch','2','-seed','20261010','-writer-pause','0','-reader-pause','1ms','-snapshot-hold','1ms','-source-id',identity['compositions'][kind],'-hardware-id','shared-cloud-linux-amd64-epyc-9v74']
 print(datetime.datetime.now(datetime.timezone.utc).isoformat(),name,flush=True)
 (ev/(name+'-command.json')).write_text(json.dumps(args)+'\n')
 with (ev/(name+'.json')).open('w') as out, (ev/(name+'.stderr')).open('w') as err:p=subprocess.run(args,cwd=root,env=env,stdout=out,stderr=err)
 (ev/(name+'.exit')).write_text(str(p.returncode)+'\n');r=json.loads((ev/(name+'.json')).read_text());brief={'name':name,'status':r['status'],'exit':p.returncode,'active_seconds':r['active_seconds'],'commits':r['writer']['committed'],'reads':r['readers']['success'],'errors':r['errors'],'final_model_verified':r['final_model_verified'],'shutdown':r['shutdown']};records.append(brief);(ev/'checks-v1.json').write_text(json.dumps(records,indent=2)+'\n');print(json.dumps(brief),flush=True)
 if kind=='candidate' and (p.returncode!=0 or r['status']!='passed'):raise SystemExit('candidate failed; acceptance blocked')
 if kind=='baseline' and p.returncode!=0 and not all('database changed during read' in e for e in r['errors']):raise SystemExit('new baseline failure requires investigation')
for n,p in [('freeze-v1',f),('control-v1',control),('candidate-soak-v1',root/'rollback-reader-acquisition-candidate-soak-v1'),('baseline-soak-v1',root/'rollback-reader-acquisition-baseline-soak-v1')]:verify(n,p)
original=root/'transaction-soak-evidence/soak-v1-rollback.json';assert hashlib.sha256(original.read_bytes()).hexdigest()=='a62f4bfed390582d1125d88a6dfbe06f06f3a33500a55fedacea934424a1e101'
(ev/'final-verification-v1.json').write_text(json.dumps({'all_source_inventories_unchanged':True,'original_failed_report_unchanged':True,'completed_utc':datetime.datetime.now(datetime.timezone.utc).isoformat()},indent=2)+'\n');print('all bounded checks finished; source inventories and original failure unchanged',flush=True)
