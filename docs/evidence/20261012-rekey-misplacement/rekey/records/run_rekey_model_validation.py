"""Bounded affected validation. Does not execute the manual test image."""
import collections
import datetime
import fcntl
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import time

R = Path(__file__).resolve().parent
W = R / 'gosqlite'
E = R / 'rekey-model-validation'
GO = R / 'toolchains/go1.27.2/bin/go'
FMT = GO.with_name('gofmt')
GO_HASH = '548d3c32f83fa81152f543eb1526be7884cfe0526eb5e9ce29d3eca14786e7e0'
FMT_HASH = '5583dbb3339147fee7cdb067eb3cd4a50015b48c27084377f49d93833aa4ad77'
GOPATH = R / 'toolchains/gopath'
ENV = dict(PATH=str(GO.parent)+':/usr/bin:/bin', GOMAXPROCS='2', GOENV='off',
    GO111MODULE='on', GOWORK='off', GOTOOLCHAIN='local', GOPROXY='off',
    GOSUMDB='off', GOAUTH='off', GOFLAGS='', GOEXPERIMENT='', GOFIPS140='off',
    GODEBUG='', GOCACHEPROG='', GOCOVERDIR='', GORACE='', GOTELEMETRY='off',
    GOROOT=str(GO.parent.parent), GOPATH=str(GOPATH), GOMODCACHE=str(GOPATH/'pkg/mod'),
    GOCACHE=str(R/'go-cache'), TMPDIR='/tmp', GOTMPDIR='', GOOS='linux',
    GOARCH='amd64', GOAMD64='v1', CGO_ENABLED='0', LC_ALL='C', LANG='C', TZ='UTC')
PACKAGES = ['./internal/testkit/rekeyinterruption']
SOURCES = ['internal/testkit/rekeyinterruption/contract.go', 'internal/testkit/rekeyinterruption/contract_test.go']
EXPECTED_ROOTS = {'github.com/ms-xiaosnow/gosqlite/internal/testkit/rekeyinterruption': ['TestRekeyInterruptionSixCases', 'TestRekeyInterruptionCutOrder', 'TestRekeyInterruptionCredentialPurpose', 'TestRekeyInterruptionSelectedGeneration', 'TestRekeyInterruptionSQLIndexLedger', 'TestRekeyInterruptionAckCausality', 'TestRekeyInterruptionOwnersAndArtifacts', 'TestRekeyInterruptionNeverPhysicalPass']}
SELECTOR = '^(TestRekeyInterruptionSixCases|TestRekeyInterruptionCutOrder|TestRekeyInterruptionCredentialPurpose|TestRekeyInterruptionSelectedGeneration|TestRekeyInterruptionSQLIndexLedger|TestRekeyInterruptionAckCausality|TestRekeyInterruptionOwnersAndArtifacts|TestRekeyInterruptionNeverPhysicalPass)$'
TEST_FLAGS = ['test','-json','-mod=readonly','-buildvcs=false','-p=1','-vet=off',
    '-count=1','-parallel=1','-timeout=0']
COMMANDS = [
    ('01_format', [str(FMT), '-l', *SOURCES], ENV),
    ('02_normal', [str(GO), *TEST_FLAGS, '-run', SELECTOR, *PACKAGES], ENV),
]

def now():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()

def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()

def save(name, value):
    (E/name).write_text(json.dumps(value, ensure_ascii=False, indent=2)+'\n')

def snapshot():
    names = subprocess.check_output(['git','ls-files','-z'],cwd=W).decode().split('\0')
    return {name:{'bytes':(W/name).stat().st_size,'sha256':sha(W/name),
        'git_mode':'100755' if (W/name).stat().st_mode & 0o111 else '100644'}
        for name in names if name}

def resources():
    out = {'utc':now(),'paths':{},'qualified':True}
    for path, minimum in [(R,2*1024**3),(Path('/tmp'),1024**3)]:
        s = os.statvfs(path)
        entry = {'available_bytes':s.f_bavail*s.f_frsize,'available_inodes':s.f_favail,'minimum_bytes':minimum}
        out['paths'][str(path)] = entry
        out['qualified'] &= entry['available_bytes'] >= minimum and entry['available_inodes'] >= 10000
    return out

def tests(stdout, expected_packages):
    roots = collections.defaultdict(set)
    terminals = collections.Counter()
    outcomes = collections.Counter()
    package_pass = set()
    for line in stdout.splitlines():
        event = json.loads(line)
        action, package, name = event.get('Action'),event.get('Package'),event.get('Test')
        if action == 'run' and name and '/' not in name: roots[package].add(name)
        if action in ('pass','fail','skip'):
            if name:
                terminals[(package,action)] += 1
                outcomes[(package,name)] += 1
            elif action == 'pass': package_pass.add(package)
    assert package_pass == {'github.com/ms-xiaosnow/gosqlite/'+x[2:] for x in expected_packages}
    assert all(n == 1 for n in outcomes.values()), 'duplicate test terminal events'
    assert all(action == 'pass' for (_,action) in terminals), 'test FAIL or SKIP'
    assert all(roots.values()) and len(roots)==len(expected_packages), 'empty package selection'
    return {p:{'roots':sorted(names),'root_count':len(names),'named_pass':terminals[(p,'pass')],
        'child_pass':terminals[(p,'pass')]-len(names),'fail':terminals[(p,'fail')],'skip':terminals[(p,'skip')]}
        for p,names in roots.items()}

def main():
    identity = json.loads((R/'rekey-model-integration/SOURCE-IDENTITY.json').read_text())
    assert identity['base_commit']=='01c63e6f8fb3d9e196afe2f42a163eb3eac4ad5a'
    assert identity['independent_source_review']=='SOURCE_REVIEW_CLEAR_WITH_LIMITS'
    assert set(identity['files']) == set(SOURCES)
    assert all(sha(W/path)==digest for path,digest in identity['files'].items())
    E.mkdir(exist_ok=False)
    (R/'go-cache').mkdir(exist_ok=True)
    lease=(R/'go-queue.lock').open('a+')
    fcntl.flock(lease,fcntl.LOCK_EX|fcntl.LOCK_NB)
    assert sha(GO)==GO_HASH and sha(FMT)==FMT_HASH
    source=snapshot()
    save('source-before.json',source)
    save('commands.json',[{'name':n,'argv':a,'cwd':str(W),'environment':e} for n,a,e in COMMANDS])
    save('start.json',{'utc':now(),'pid':os.getpid(),'base_commit':subprocess.check_output(['git','rev-parse','HEAD'],cwd=W).decode().strip(),
        'scope':'exact8 descriptor-free rekey model roots only; no encstore/sqlite/os/observer/native dependency or sourceguard execution',
        'source_manifest_sha256':sha(E/'source-before.json'),'tool_go_sha256':GO_HASH,'tool_gofmt_sha256':FMT_HASH,
        'process_scope':'single coordinated Go queue; no claim of global cross-namespace visibility'})
    finished=[]
    for name,argv,env in COMMANDS:
        assert snapshot()==source, 'source changed before '+name
        start=now(); initial=resources(); save(name+'.resources-before.json',initial)
        assert initial['qualified'], 'resource admission '+name
        latched=False
        observer_error=None
        with (E/(name+'.stdout')).open('wb') as out, (E/(name+'.stderr')).open('wb') as err, (E/(name+'.resources.jsonl')).open('w') as samples:
            p=subprocess.Popen(argv,cwd=W,env=env,stdout=out,stderr=err)
            while True:
                try:
                    sample=resources();samples.write(json.dumps(sample)+'\n');samples.flush()
                    latched |= not sample['qualified']
                except Exception as error:
                    latched=True
                    observer_error=type(error).__name__+': '+str(error)
                    code=p.wait()  # Observation failure must not orphan or kill the running gate.
                    break
                code=p.poll()
                if code is not None: break
                time.sleep(2)
        result={'name':name,'start_utc':start,'end_utc':now(),'exit_code':code,'resource_breach_latched':latched,'observer_error':observer_error,
            'stdout_sha256':sha(E/(name+'.stdout')),'stderr_sha256':sha(E/(name+'.stderr'))}
        save(name+'.result.json',result)
        assert snapshot()==source, 'source changed during '+name
        assert code==0 and not latched, ('gate failed',result)
        stdout=(E/(name+'.stdout')).read_text()
        if name=='01_format': assert stdout=='', 'unformatted sources'
        elif name in ('02_normal','03_race'):
            result['tests']=tests(stdout, PACKAGES)
            assert all(set(result['tests'][p]['roots'])==set(names) for p,names in EXPECTED_ROOTS.items() if p in result['tests']), 'exact selector roots'
        result['status']='PASS';save(name+'.result.json',result);finished.append(result)
        print(json.dumps({'utc':now(),'name':name,'status':'PASS'}),flush=True)
    save('completion.json',{'utc':now(),'status':'PASS','commands':finished,'source_unchanged':snapshot()==source,
        'manual_image_executed':False,'unrelated_tests':'REUSED, not rerun', 'prior_tests':'Unchanged unrelated source closures reuse their recorded applicable evidence','rerun_reason':'New independent value-only model package initial8rootnormal. Native/controller/guard not integrated or executed; no priorchecksrepeated','lease':'RELEASED_ON_RETURN'})
    fcntl.flock(lease,fcntl.LOCK_UN)

if __name__=='__main__':
    try: main()
    except BaseException as error:
        if E.exists(): save('STOP.json',{'utc':now(),'error_type':type(error).__name__,'error':str(error),
            'remaining_commands':'NOT_RUN','physical_manual_executed':False})
        raise
