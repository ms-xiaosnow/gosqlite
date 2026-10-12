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
E = R / 'rekey-guard-validation-v3'
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
IDENTITY_PATH = R/'rekey-model-name-fix/SOURCE-IDENTITY.json'
MODEL_ROOTS = ['TestRekeyInterruptionSixCases', 'TestRekeyInterruptionCutOrder', 'TestRekeyInterruptionCredentialPurpose', 'TestRekeyInterruptionSelectedGeneration', 'TestRekeyInterruptionSQLIndexLedger', 'TestRekeyInterruptionAckCausality', 'TestRekeyInterruptionOwnersAndArtifacts', 'TestRekeyInterruptionNeverPhysicalPass']
GUARD_ROOTS = ['TestRekeyGuardInventoryGate', 'TestRekeyGuardPublicationOrder', 'TestRekeyGuardObserverIsolation', 'TestRekeyGuardPublicReturnACK', 'TestRekeyGuardProcessOwnership', 'TestRekeyGuardProbeLedger', 'TestRekeyGuardIntegrationDefault', 'TestReadOnlyWALManualSourceContract', 'TestReadOnlyWALManualDefaultExclusion', 'TestReadOnlyWALManualWholeRootInventory', 'TestReadOnlyWALManualGateAndProfileMutations', 'TestReadOnlyWALManualEffectBoundaryMutations', 'TestReadOnlyWALManualOwnershipMutations', 'TestReadOnlyWALManualPureClosureMutations', 'TestRollbackRestartOwnedSourceContracts', 'TestRollbackRestartDoubleRepinnedSelectionAndStubCounterexamples', 'TestRollbackRestartOutsideConsumersAndAliasesRejected', 'TestRollbackRestartOldCoreAndMutexWrapperStayExact', 'TestRollbackRestartWiringPreservesAllPoliciesAndReviewHold', 'TestRollbackRestartDoubleRepinnedNativeProtocolCounterexamples', 'TestRollbackRestartDoubleRepinnedModelCausalCounterexamples', 'TestRollbackRestartOracleExceptionRequiresCompletePinnedNativeClosure', 'TestRollbackRestartDoubleRepinnedCommandCapabilityClosure', 'TestRollbackRestartDoubleRepinnedLaunchInputClosure', 'TestScopeSourceReferenceGuard', 'TestOracleSourceConstructorGuard', 'TestOracleSourceImportAndMutationGuard', 'TestOracleSourceFactoryLiteralGuard', 'TestOracleSourceNativeExceptionGuard', 'TestOracleSourcePathScope', 'TestWALPublicationGuardRejectsActivationReferences', 'TestWALPublicationGuardSafeControls', 'TestWALPublicationGuardPureFilesRejectNativeAliasesAndVariables', 'TestWALPublicationGuardExactCallBoundaries', 'TestWALPublicationGuardFixedSourceAndWiring', 'TestWALPublicationGuardFakeFactoryIsExactSource', 'TestWALPublicationGuardProductionBoundariesAreRequired', 'TestWALPublicationGuardRejectsControlFlowAndFactoryDrift', 'TestWALPublicationGuardManualSourceIsExactAndDoubleGated', 'TestWALPublicationGuardElapsedSourceIsInertAndPinned', 'TestWALPublicationGuardElapsedRejectsTimingAndQualificationDrift', 'TestFormatUpgradeManualStructuralCounterexamples', 'TestFormatUpgradePureClosureCounterexamples', 'TestFormatUpgradeMainKeepsExistingPolicies', 'TestEncryptedRootFiniteStructure', 'TestEncryptedRootFalsePassSourcesEvenWhenRepinned', 'TestEncryptedRootWholeSourceAndCapabilityBoundary', 'TestEncryptedRootPinsRequireIndependentRelease', 'TestRollbackFaultSourceContracts', 'TestRollbackFaultRepinnedSemanticCounterexamples', 'TestRollbackFaultOutsideConsumersRejected', 'TestRollbackFaultGuardWiringPreservesPriorPolicies', 'TestRollbackFaultSQLSourceContracts', 'TestRollbackFaultSQLRepinnedSemanticCounterexamples', 'TestRollbackFaultSQLOutsideConsumersAndOldEntriesRejected', 'TestRollbackFaultSQLOrdinaryRecoveryExceptionIsOneReference', 'TestRollbackFaultSQLWiringAndMainReviewHold', 'TestRollbackFaultSQLOrdinaryExampleFields', 'TestRollbackFaultSQLOrdinaryExamplesKeepModelBoundary', 'TestRollbackFaultSQLOrdinaryFieldBindingsReject', 'TestRollbackFaultSQLOrdinaryCompanionFailures']
HEALTHY_ROOTS = ['TestEncryptionWALPasswordOrdinaryReopenAndCancellation','TestEncryptionReviewFreshIdentityAndRetainedBytes']
IDENTITY = json.loads(IDENTITY_PATH.read_text())
SOURCES = ['internal/testkit/rekeyinterruption/contract.go','internal/testkit/rekeyinterruption/contract_test.go','internal/testscope/rekey_interruption.go','internal/testscope/rekey_interruption_test.go']
IMAGE = E/'sqlite-rekey-interruption-manual.test'
RACE_ENV = dict(ENV,CGO_ENABLED='1',CC='/usr/bin/gcc',CXX='/usr/bin/g++')
WINDOWS_ENV = dict(ENV,GOOS='windows')
TEST_FLAGS = ['test','-json','-mod=readonly','-buildvcs=false','-p=1','-vet=off','-count=1','-parallel=1','-timeout=0']
def selector(roots): return '^('+'|'.join(roots)+')$'
COMMANDS = [
 ('01_format',[str(FMT),'-l',*SOURCES],ENV),
 ('02_guard_normal',[str(GO),*TEST_FLAGS,'-run',selector(GUARD_ROOTS[:7]),'./internal/testscope'],ENV),
 ('03_guard_race',[str(GO),*TEST_FLAGS,'-race','-run',selector(GUARD_ROOTS[:7]),'./internal/testscope'],RACE_ENV),
 ('04_model_normal',[str(GO),*TEST_FLAGS,'-run',selector(MODEL_ROOTS),'./internal/testkit/rekeyinterruption'],ENV),
 ('04b_model_race',[str(GO),*TEST_FLAGS,'-race','-run',selector(MODEL_ROOTS),'./internal/testkit/rekeyinterruption'],RACE_ENV),
 ('06_sourceguard',[str(GO),'run','-mod=readonly','-buildvcs=false','-p=1','./internal/testscope'],ENV),
 ('07_list_default',[str(GO),'list','-json','-mod=readonly','-buildvcs=false','-p=1','./internal/encstore','./internal/testkit/rekeyinterruption','./sqlite'],ENV),
 ('08_list_manual',[str(GO),'list','-json','-mod=readonly','-buildvcs=false','-p=1','-tags=manual_rekey_interruption','./internal/encstore','./internal/testkit/rekeyinterruption','./sqlite'],ENV),
 ('09_list_windows_default',[str(GO),'list','-json','-mod=readonly','-buildvcs=false','-p=1','./internal/encstore','./internal/testkit/rekeyinterruption','./sqlite'],WINDOWS_ENV),
 ('10_manual_compile',[str(GO),'test','-mod=readonly','-buildvcs=false','-p=1','-vet=off','-c','-tags=manual_rekey_interruption','-o',str(IMAGE),'./sqlite'],ENV),
 ('11_nm',[str(GO),'tool','nm',str(IMAGE)],ENV),
]
TEST_SCOPES = {
 '02_guard_normal': ('./internal/testscope',GUARD_ROOTS[:7]),
 '03_guard_race': ('./internal/testscope',GUARD_ROOTS[:7]),
 '04_model_normal': ('./internal/testkit/rekeyinterruption',MODEL_ROOTS),
 '04b_model_race': ('./internal/testkit/rekeyinterruption',MODEL_ROOTS),
}
NATIVE_ROOTS = {'TestManualRekeyInterruptionController','TestManualRekeyInterruptionWriter','TestManualRekeyInterruptionOldProbe','TestManualRekeyInterruptionNewProbe','TestManualRekeyInterruptionArchive'}
NM_ROW = re.compile(r'(?:(?P<address>[0-9A-Fa-f]+)[ \t]+)?(?P<type>[A-Za-z?_])[ \t]+(?P<symbol>\S(?:[^\r\n]*\S)?)')
NM_ROOT = re.compile(r'github\.com/ms-xiaosnow/gosqlite/sqlite(?:_test)?\.(?:Test|Fuzz|Example)[A-Za-z0-9_]*')

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

def decode_stream(raw):
    decoder=json.JSONDecoder();offset=0;out=[]
    while offset<len(raw):
        while offset<len(raw) and raw[offset].isspace(): offset+=1
        if offset==len(raw): break
        item,end=decoder.raw_decode(raw,offset);out.append(item);offset=end
    return out

def selection(stdout,manual):
    packages={p['ImportPath']:p for p in decode_stream(stdout)}
    prefix='github.com/ms-xiaosnow/gosqlite/'
    assert set(packages)=={prefix+x for x in ('internal/encstore','internal/testkit/rekeyinterruption','sqlite')}
    enc=packages[prefix+'internal/encstore'];sql=packages[prefix+'sqlite'];model=packages[prefix+'internal/testkit/rekeyinterruption']
    assert ('rekey_interruption_manual_linux.go' in enc.get('GoFiles',[]))==manual
    assert ('rekey_observation_default.go' in enc.get('GoFiles',[]))== (not manual)
    assert 'rekey_observation.go' in enc.get('GoFiles',[])
    target='rekey_interruption_manual_linux_test.go'
    assert (target in sql.get('XTestGoFiles',[]))==manual
    assert (target in sql.get('IgnoredGoFiles',[]))== (not manual)
    assert target not in sql.get('TestGoFiles',[]) and target not in sql.get('GoFiles',[])
    assert model.get('GoFiles')==['contract.go'] and model.get('TestGoFiles')==['contract_test.go']
    assert not model.get('CgoFiles')
    roots=set()
    for path in sql.get('TestGoFiles',[])+sql.get('XTestGoFiles',[]):
        text=(W/'sqlite'/path).read_text()
        package=re.search(r'^package (sqlite(?:_test)?)$',text,re.M).group(1)
        for root in re.findall(r'^func ((?:Test|Fuzz|Example)[A-Za-z0-9_]*)\(',text,re.M): roots.add(prefix+package+'.'+root)
    wanted_native={prefix+'sqlite_test.'+n for n in NATIVE_ROOTS}
    selected_manual={n for n in roots if '.TestManual' in n}
    assert selected_manual==(wanted_native if manual else set()), ('unexpected manual roots',selected_manual)
    return {'manual_selected':manual,'roots':sorted(roots),'native_executed':False,'packages':packages}

def symbols(stdout):
    contract=json.loads((E/'08_list_manual.result.json').read_text())['selection']
    wanted=set(contract['roots']);names=[]
    for line in stdout.splitlines():
        m=NM_ROW.fullmatch(line.lstrip(' \t'));assert m,('malformed nm row',line)
        assert (m['address'] is None)==(m['type']=='U'),('nm address/type',line)
        names.append(m['symbol'])
    assert all(names.count(n)==1 for n in wanted), 'missing or duplicate test symbols'
    actual={n for n in names if NM_ROOT.fullmatch(n)}
    assert actual==wanted, ('unexpected test symbol set',sorted(actual^wanted))
    absent=set()
    prefix='github.com/ms-xiaosnow/gosqlite/'
    for path in (W/'sqlite').glob('*_test.go'):
        body=path.read_text();package=re.search(r'^package (sqlite(?:_test)?)$',body,re.M).group(1)
        for name in re.findall(r'^func (TestManual[A-Za-z0-9_]*)\(',body,re.M):
            full=prefix+package+'.'+name
            if full not in wanted: absent.add(full)
    assert not absent&set(names)
    return {'required':len(wanted),'absent':len(absent),'nm_rows':len(names),'image_sha256':sha(IMAGE),'image_executed':False}

def main():
    identity=IDENTITY
    assert identity['base_commit']=='01c63e6f8fb3d9e196afe2f42a163eb3eac4ad5a'
    assert all(sha(W/path)==digest for path,digest in identity['files'].items())
    release=json.loads((R/'rekey-model-name-fix/EXECUTION-RELEASE.json').read_text())
    assert release['identity_sha256']==sha(IDENTITY_PATH) and release['status']=='RELEASED'
    assert sha(W/'internal/testkit/rekeyinterruption/contract.go')=='e834e098a978d1715189e3cee293aab62ea2b806d27f892a60b3d6f4d89f888e'
    assert sha(W/'internal/testkit/rekeyinterruption/contract_test.go')=='41f7343636c45b56f081875db9ab0b81273bbef212282a02a46f6ae1e5978cc6'
    E.mkdir(exist_ok=False)
    (R/'go-cache').mkdir(exist_ok=True)
    lease=(R/'go-queue.lock').open('a+')
    fcntl.flock(lease,fcntl.LOCK_EX|fcntl.LOCK_NB)
    assert sha(GO)==GO_HASH and sha(FMT)==FMT_HASH
    source=snapshot()
    save('source-before.json',source)
    save('commands.json',[{'name':n,'argv':a,'cwd':str(W),'environment':e} for n,a,e in COMMANDS])
    save('start.json',{'utc':now(),'pid':os.getpid(),'base_commit':subprocess.check_output(['git','rev-parse','HEAD'],cwd=W).decode().strip(),
        'scope':'7 affectednewASTnormal/race,8puremodelnormal/race; healthydefault2V2PASSreused; sourceguard/3list/manualtest-c/nm; no manual image execution',
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
        elif name in TEST_SCOPES:
            package,expected=TEST_SCOPES[name]
            result['tests']=tests(stdout,[package])
            measured=result['tests']['github.com/ms-xiaosnow/gosqlite/'+package[2:]]
            assert set(measured['roots'])==set(expected),'exact selector roots'
        elif name.startswith(('07_list','08_list','09_list')): result['selection']=selection(stdout,name=='08_list_manual')
        elif name=='11_nm': result['symbols']=symbols(stdout)
        result['status']='PASS';save(name+'.result.json',result);finished.append(result)
        print(json.dumps({'utc':now(),'name':name,'status':'PASS'}),flush=True)
    save('completion.json',{'utc':now(),'status':'PASS','commands':finished,'source_unchanged':snapshot()==source,
        'manual_image_executed':False,'unrelated_tests':'REUSED, not rerun', 'prior_tests':'Prior R5models normal/race historical oldtypename; newly affectedmodels rerun. V2healthy2 and54legacyASTnormal/race reused; unrelated01c CI reused','rerun_reason':'Observed reservedSQLPlan typename collision; newtype renamedRekeyQueryPlan,2snapshots/pins andcrossguardpositive;7R5guards+8models affected; whole sourceguardFAIL rerun, last5gatesNOT_RUN','lease':'RELEASED_ON_RETURN'})
    fcntl.flock(lease,fcntl.LOCK_UN)

if __name__=='__main__':
    try: main()
    except BaseException as error:
        if E.exists(): save('STOP.json',{'utc':now(),'error_type':type(error).__name__,'error':str(error),
            'remaining_commands':'NOT_RUN','physical_manual_executed':False})
        raise
