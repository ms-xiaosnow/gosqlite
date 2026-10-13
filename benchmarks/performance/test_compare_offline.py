"""Pure arithmetic/parser/model tests. No database executable is invoked."""
import copy
import json
import math
from pathlib import Path
import tempfile
import unittest

import compare as c


import datetime
import re
import statistics
from validation import GRAMS_CONFIG, TRANSACTIONS_CONFIG, GRAMS_ARGS, TRANSACTIONS_ARGS, CACHE, BUILD_ENV, model

TEMPLATES = json.loads((Path(__file__).parent/"fixtures.json").read_text())
OBSERVED = {"system":"Linux","release":"fixture","machine":"x86_64","cpu_model":"synthetic", "logical_cpus":9,"affinity":list(range(9)),"storage":"synthetic"}
SESSION = "00000000-0000-4000-8000-000000000001"

def file_ref(path, root):
    return {"path": str(path.relative_to(root)), "sha256": c.sha(path.read_bytes())}

def iso(seconds):
    return (datetime.datetime(2026,1,1,tzinfo=datetime.timezone.utc)+datetime.timedelta(seconds=seconds)).isoformat()

def sync_derived(rep,kind):
    if kind=="grams":
        values=[t["index_build_ms"] for t in rep["trials"]];d=rep["index_build_latency_across_repeats"]
        d.update(mean_ms=statistics.mean(values),sample_stddev_ms=statistics.stdev(values),min_ms=min(values),max_ms=max(values),p50_ms=min(values),p95_ms=max(values))
        for t in rep["trials"]:
            before=t["database_bytes_before_index"];extra=t["incremental_database_bytes"]
            t.update(database_bytes_with_index=before+extra,database_with_index_over_before=(before+extra)/before,incremental_bytes_over_utf8_text_bytes=extra/4288)
    else:
        values=[w["committed_transactions_per_second"] for w in rep["windows"]];d=rep["window_commit_rate_statistics"]
        d.update(sample_count=len(values),mean=statistics.mean(values),min=min(values),max=max(values),sample_stddev=statistics.stdev(values) if len(values)>1 else 0)

def fixture(kind, source, hardware, stamp, cost=1.0):
    rep=copy.deepcopy(TEMPLATES[kind])
    cfg=dict(GRAMS_CONFIG if kind=="grams" else TRANSACTIONS_CONFIG,source_id=source,hardware_id=hardware)
    rep["config"]=cfg
    match=re.search(r"(?:pair-)?(\d+)-(baseline|candidate)-(grams|transactions)",stamp)
    n=int(match[1]); first="baseline" if n%2 else "candidate"
    sec=(n-1)*400+(0 if match[2]==first else 200)+(0 if kind=="grams" else 100)
    elapsed=30 if kind=="grams" else cost+2
    rep.update(started_utc=iso(sec+10),finished_utc=iso(sec+10+elapsed),elapsed_seconds=elapsed)
    if kind=="grams":
        rep.update(source_id=source,hardware_id=hardware,cpus=9)
        for t in rep["trials"]:
            t.update(index_build_ms=100*cost,incremental_database_bytes=40960,database_bytes_before_index=4096,database_bytes_with_index=45056,
                     database_with_index_over_before=45056/4096,incremental_bytes_over_utf8_text_bytes=40960/4288)
    else:
        rep.update(harness_revision="ordinary-transactions-v4",active_seconds=cost,logical_cpus=9,gomaxprocs=2,
                   committed_transactions_per_active_second=48/cost,writer_attempts_per_active_second=64/cost,
                   committed_transactions_per_elapsed_second=48/elapsed)
        window=rep["windows"][0]
        window.update(started_utc=iso(sec+11),finished_utc=iso(sec+11+cost),active_seconds=cost,committed_transactions_per_second=48/cost)
    sync_derived(rep,kind)
    return rep

def latency_fixture(values):
    """Exact synthetic descriptive samples; no timings or native work are run."""
    n=len(values);buckets=[0]*48
    for x in values:
        i=0
        while i<47 and x>2**i/1000: i+=1
        buckets[i]+=1
    def quantile(q):
        total=0
        for i,count in enumerate(buckets):
            total+=count
            if n and total>=math.ceil(n*q): return 2**i/1000
        return 0
    return {"milliseconds":{"sample_count":n,"mean":statistics.mean(values) if n else 0,
            "sample_stddev":statistics.stdev(values) if n>1 else 0,"min":min(values) if n else 0,
            "max":max(values) if n else 0,"confidence_method":"synthetic descriptive fixture",
            "confidence_95_low":None,"confidence_95_high":None},
            "buckets_power_of_two_microseconds":buckets,"p50_bucket_upper_ms":quantile(.5),
            "p95_bucket_upper_ms":quantile(.95)}

def replace_trace(rep, trace):
    """Reconcile all affected current-schema fields, not merely a top-level count."""
    window=rep["windows"][0];window["writer_outcomes"]=trace
    w=rep["writer"]
    w.update(committed=trace.count("C"),intentional_rollbacks=trace.count("R"),
             success=trace.count("C")+trace.count("R"),busy_outcomes=trace.count("B"),
             canceled_outcomes=trace.count("X"),failed_outcomes=trace.count("F"),
             savepoint_rollback_commits=sum(x=="C" and i%4==1 for i,x in enumerate(trace)),
             savepoint_release_commits=sum(x=="C" and i%4==2 for i,x in enumerate(trace)))
    w["busy_events_including_commit_retries"]=max(w["busy_events_including_commit_retries"],w["busy_outcomes"])
    window["writer"]=copy.deepcopy(w)
    rep.update(final_model_sha256=model(trace)[1],final_model_generation=trace.count("C"),
               committed_transactions_per_active_second=w["committed"]/rep["active_seconds"],
               committed_transactions_per_elapsed_second=w["committed"]/rep["elapsed_seconds"])
    window["committed_transactions_per_second"]=w["committed"]/window["active_seconds"]


def command_fixture(root,label,argv,source,session,start,end,stdout,env=None,rc=0):
    stderr=root/(label+".stderr"); stderr.write_bytes(b"")
    rec={"session_id":session,"source_id":source["id"],"cwd":source["root"],"command":argv,"exit_code":rc,
         "started_utc":start,"finished_utc":end,"build_environment":dict(BUILD_ENV),
         "executed_binary_sha256":c.sha(Path(argv[0]).read_bytes()) if argv[0]!="go" else None,
         "stdout":file_ref(stdout,root),"stderr":file_ref(stderr,root),"load_before":None,"load_after":None}
    path=root/(label+".command.json");path.write_bytes(c.canonical(rec))
    return file_ref(path,root)


class MathTests(unittest.TestCase):
    def test_exact_binomial_coverage(self):
        for n in range(1, 33):
            for target in (.90, .95, .99):
                interval = c.median_interval(list(range(1, n + 1)), target)
                valid = []
                for k in range(1, n // 2 + 1):
                    coverage = sum(math.comb(n, j) for j in range(k, n-k+1)) / 2**n
                    if coverage >= target:
                        valid.append((k, coverage))
                self.assertEqual(bool(interval), bool(valid))
                if interval:
                    self.assertEqual(interval["order_statistic_k"], valid[-1][0])
                    self.assertEqual(interval["coverage_at_least"], valid[-1][1])
    def test_known_eight_pair_interval(self):
        self.assertEqual(c.median_interval([1] * 8, 1-.05/3),
                         {"lower": 1, "upper": 1, "coverage_at_least": .9921875, "order_statistic_k": 1})
    def test_reject_nonpositive_and_nonfinite(self):
        for x in (0, -1, math.inf, math.nan, True, "1"):
            with self.subTest(x=x), self.assertRaises(c.EvidenceError):
                c.median_interval([x], .95)

class HistogramMomentTests(unittest.TestCase):
    def test_feasible_exact_moments_for_small_sample_sets(self):
        import itertools
        from validation import histogram
        for n in range(1,7):
            for values in itertools.combinations_with_replacement((0,.001,1,2),n):
                histogram(latency_fixture(values),n)
    def test_near_constant_nanosecond_welford_rounding(self):
        from validation import histogram
        for base in (1.,100.,1000.,5000.):
            for n in (2,3,64,128):
                values=[base+i%3*1e-6 for i in range(n)]
                h=latency_fixture(values);mean=m2=0
                for count,x in enumerate(values,1):
                    delta=x-mean;mean+=delta/count;m2+=delta*(x-mean)
                h["milliseconds"].update(mean=mean,sample_stddev=math.sqrt(m2/(n-1)))
                histogram(h,n)
    def test_one_and_constant_samples_require_zero_deviation(self):
        from validation import histogram
        for values in ([1],[1,1],[0,0,0]):
            h=latency_fixture(values);h["milliseconds"]["sample_stddev"]=1e-12
            with self.assertRaises(c.EvidenceError):histogram(h,len(values))

class ComparisonTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
    def bundle(self, n=8, controlled=True, cost=1):
        files={p:{"sha256":c.sha(p.encode()),"mode":"100644"} for p in
               ("go.mod","go.sum","benchmarks/grams/main.go","benchmarks/transactions/main.go","internal/testscope/main.go")}
        sources={}
        for arm in ("baseline","candidate"):
            m=copy.deepcopy(files);m["engine.go"]={"sha256":c.sha(arm.encode()),"mode":"100644"}
            sid="sha256:"+c.sha(c.canonical(m))
            sources[arm]={"id":sid,"manifest":m,"root":"/synthetic/"+arm,"verified_after":sid}
        hw="sha256:"+c.sha(c.canonical(OBSERVED))
        harness=c.sha(c.canonical({p:v for p,v in files.items() if p.startswith("benchmarks/")}))
        b={"schema_version":2,"policy":c.POLICY,"status":"complete","session_id":SESSION,
           "started_utc":iso(-100),"finished_utc":iso(15000),"recorded_evidence_root":str(self.root),
           "requested_pairs":n,"profile":{"grams":GRAMS_ARGS,"transactions":TRANSACTIONS_ARGS},
           "sources":sources,"harnesses":{a:harness for a in sources},"binaries":{},"build_commands":{},
           "environment":{"controlled":controlled,"control_reason":"pure model fixture, not evidence","hardware_id":hw,
                          "runner_id":"fixture","observed":copy.deepcopy(OBSERVED)},"pairs":[]}
        (self.root/"bin").mkdir(exist_ok=True)
        for arm in sources:
            b["binaries"][arm]={};b["build_commands"][arm]={}
            for j,kind in enumerate(("guard","grams","transactions")):
                label=arm+"-build-"+kind
                stdout=self.root/(label+".stdout");stdout.write_text("synthetic build fixture")
                if kind=="guard":argv=["go","run","./internal/testscope"]
                else:
                    binary=self.root/"bin"/(arm+"-"+kind);binary.write_bytes((arm+kind).encode())
                    b["binaries"][arm][kind]=file_ref(binary,self.root)
                    argv=["go","build","-trimpath","-o",str(binary),"./benchmarks/"+kind]
                b["build_commands"][arm][kind]=command_fixture(self.root,label,argv,sources[arm],SESSION,iso(-90+j*10),iso(-89+j*10),stdout)
        for i in range(1,n+1):
            pair={"pair":i,"order":["baseline","candidate"] if i%2 else ["candidate","baseline"]}
            for arm in sources:
                pair[arm]={}
                for kind,flags in (("grams",GRAMS_ARGS),("transactions",TRANSACTIONS_ARGS)):
                    label=f"{i}-{arm}-{kind}";value=fixture(kind,sources[arm]["id"],hw,label,cost if arm=="candidate" else 1)
                    path=self.root/(label+".json");path.write_bytes(c.canonical(value))
                    ref=file_ref(path,self.root);ref["exit_code"]=0
                    argv=[str(self.root/b["binaries"][arm][kind]["path"])]+flags+["-source-id",sources[arm]["id"],"-environment" if kind=="grams" else "-hardware-id",hw]
                    ref["command"]=command_fixture(self.root,label,argv,sources[arm],SESSION,value["started_utc"],value["finished_utc"],path)
                    pair[arm][kind]=ref
            b["pairs"].append(pair)
        return b
    def evaluate(self,b):
        p=self.root/"bundle.json";p.write_bytes(c.canonical(b));return c.evaluate(p)
    def change_raw(self,b,callback,arm="candidate",kind="grams"):
        ref=b["pairs"][0][arm][kind];path=self.root/ref["path"];rep=json.loads(path.read_text());callback(rep)
        if kind=="grams" and isinstance(rep.get("trials"),list) and len(rep["trials"])>1 and all(type(t.get("index_build_ms")) in (int,float) and math.isfinite(t["index_build_ms"]) for t in rep["trials"]):
            sync_derived(rep,kind)
        elif kind=="transactions" and "windows" in rep:
            sync_derived(rep,kind)
        path.write_bytes(c.canonical(rep));ref["sha256"]=c.sha(path.read_bytes())
        command=self.root/ref["command"]["path"];record=json.loads(command.read_text())
        record["stdout"]={"path":ref["path"],"sha256":ref["sha256"]};command.write_bytes(c.canonical(record))
        ref["command"]["sha256"]=c.sha(command.read_bytes())
    def reject_report(self, change, kind="transactions", arm="candidate"):
        b=self.bundle(n=1);self.change_raw(b,change,kind=kind,arm=arm)
        result=self.evaluate(b)
        self.assertIn(result["status"],("INADMISSIBLE","FAILED"), result)
        return result
    def admit_report(self, change, kind="transactions"):
        b=self.bundle(n=1);self.change_raw(b,change,kind=kind)
        result=self.evaluate(b)
        self.assertEqual(result["status"],"INCONCLUSIVE",result)
    def test_window_time_must_match_utc_with_bounded_tolerance(self):
        for arm in ("baseline","candidate"):
            for delta in (-.003,.003):
                def mutate(r):
                    w=r["windows"][0]
                    dt=datetime.datetime.fromisoformat(w["finished_utc"])
                    w["finished_utc"]=(dt+datetime.timedelta(seconds=delta)).isoformat()
                self.reject_report(mutate,arm=arm)
        for delta in (-.0019,.0019):
            def mutate(r):
                w=r["windows"][0];dt=datetime.datetime.fromisoformat(w["finished_utc"])
                w["finished_utc"]=(dt+datetime.timedelta(seconds=delta)).isoformat()
            self.admit_report(mutate)
    def test_short_consistent_window_cannot_hide_long_serial_samples(self):
        def mutate(r):
            r.update(active_seconds=.001,committed_transactions_per_active_second=48000,writer_attempts_per_active_second=64000)
            w=r["windows"][0];w.update(active_seconds=.001,committed_transactions_per_second=48000)
            w["finished_utc"]=(datetime.datetime.fromisoformat(w["started_utc"])+datetime.timedelta(milliseconds=1)).isoformat()
        result=self.reject_report(mutate)
        self.assertIn("writer samples exceed",result["reason"])
    def test_each_sequential_worker_sum_is_bounded_independently(self):
        for key in ("writer_attempt_latency","reader_attempt_latency"):
            def mutate(r):
                n=r[key]["milliseconds"]["sample_count"]
                r[key]=latency_fixture([1001/n]*n)
            self.reject_report(mutate)
        def overlap(r):
            for key in ("writer_attempt_latency","reader_attempt_latency"):
                n=r[key]["milliseconds"]["sample_count"]
                r[key]=latency_fixture([1000/n]*n)
        self.admit_report(overlap) # Each sum equals active; their sum is twice active.
    def test_reopen_is_bounded_by_nonactive_time_not_worker_time(self):
        self.admit_report(lambda r:r.update(ordinary_reopen_latency=latency_fixture([1500])))
        self.reject_report(lambda r:r.update(ordinary_reopen_latency=latency_fixture([2001])))
        self.reject_report(lambda r:r.update(ordinary_checkpoint_latency=latency_fixture([.1])))
    def test_histogram_finite_sample_moment_feasibility(self):
        for key in ("writer_attempt_latency","reader_attempt_latency","ordinary_reopen_latency"):
            self.reject_report(lambda r:r[key]["milliseconds"].update(sample_stddev=1e100))
        self.reject_report(lambda r:r["ordinary_reopen_latency"]["milliseconds"].update(mean=.069))
        self.reject_report(lambda r:r["writer_attempt_latency"]["milliseconds"].update(sample_stddev=0))
        self.reject_report(lambda r:r["writer_attempt_latency"]["milliseconds"].update(mean=12.246524))
    def test_complete_required_v4_coverage_on_both_arms(self):
        def no_readers(r):
            r["readers"]={k:0 for k in r["readers"]}
            r["windows"][0]["readers"]=copy.deepcopy(r["readers"])
            r["reader_attempt_latency"]=latency_fixture([])
        for arm in ("baseline","candidate"):
            self.reject_report(no_readers,arm=arm)
            for kind in range(4):
                def missing(r):
                    old=r["windows"][0]["writer_outcomes"]
                    replace_trace(r,"".join("B" if (i%4==kind if kind<3 else x=="C") else x for i,x in enumerate(old)))
                self.reject_report(missing,arm=arm)
    def test_counted_busy_cancellation_keep_required_coverage(self):
        def mutate(r):
            trace=list(r["windows"][0]["writer_outcomes"]);trace[0]="X";trace[1]="B"
            replace_trace(r,"".join(trace))
        self.admit_report(mutate)
    def test_multiwindow_coverage_and_latency_are_aggregate(self):
        def split(r):
            old=r["windows"][0];first=copy.deepcopy(old);second=copy.deepcopy(old)
            a=datetime.datetime.fromisoformat(old["started_utc"])
            for index,w in enumerate((first,second)):
                w.update(index=index,first_writer_ordinal=32*index,writer_outcomes=old["writer_outcomes"][index*32:(index+1)*32],
                         started_utc=(a+datetime.timedelta(seconds=index)).isoformat(),
                         finished_utc=(a+datetime.timedelta(seconds=index+.5)).isoformat(),active_seconds=.5)
                # The fixture's writer has 64 successes and no terminal errors.
                for k in ("attempts","success","committed","intentional_rollbacks","savepoint_rollback_commits","savepoint_release_commits"):
                    w["writer"][k]=old["writer"][k]//2
                events=old["writer"]["busy_events_including_commit_retries"]
                w["writer"]["busy_events_including_commit_retries"]=events//2 if index==0 else events-events//2
                w["committed_transactions_per_second"]=w["writer"]["committed"]/.5
            first["readers"]={k:0 for k in old["readers"]}
            r.update(windows=[first,second],verified_reopens=2,ordinary_reopen_latency=latency_fixture([.1,.2]))
        self.admit_report(split)
    def test_heap_guard_exact_boundary(self):
        self.admit_report(lambda r:r.update(max_observed_heap_bytes=256<<20))
        for arm in ("baseline","candidate"):
            self.reject_report(lambda r:r.update(max_observed_heap_bytes=(256<<20)+1),arm=arm)
    def test_grams_search_work_bounds_and_nonempty_selectivity(self):
        changes=[{"frequency_lookups":0},{"posting_keys_read":0},{"candidates":0},
                 {"base_row_fetches":3},{"candidates":64,"base_row_fetches":64},
                 {"returned_rows":3},{"frequency_lookups":129}]
        for arm in ("baseline","candidate"):
            for fields in changes:
                self.reject_report(lambda r:r["trials"][0]["samples"][0]["observations"][0]["automatic_work"].update(fields),kind="grams",arm=arm)
        def empty(r):
            row=r["trials"][0]["samples"][0]["observations"][0]
            for k in ("automatic_work","scan_work"):row[k]["returned_rows"]=0
        self.reject_report(empty,kind="grams")
    def test_grams_scan_and_returned_row_work(self):
        for key in ("frequency_lookups","posting_keys_read","candidates"):
            self.reject_report(lambda r:r["trials"][0]["samples"][0]["observations"][0]["scan_work"].update({key:1}),kind="grams")
        def impossible(r):
            row=r["trials"][0]["samples"][0]["observations"][0]
            for k in ("automatic_work","scan_work"):row[k]["returned_rows"]=65
        self.reject_report(impossible,kind="grams")
    def test_grams_target_method_field_types(self):
        for fields in ({"Table":"other_table"},{"Indexes":[]},{"Fields":[]},{"MethodVersion":True},
                       {"MethodVersion":1.0},{"GramLengths":[True]}):
            self.reject_report(lambda r:r["trials"][0]["samples"][0]["observations"][0]["automatic_plan"].update(fields),kind="grams")
        self.reject_report(lambda r:r["trials"][0]["samples"][0]["observations"][0]["scan_plan"].update(Table="other_table"),kind="grams")
    def test_grams_fallback_reason_and_full_scan_with_retained_probe_work(self):
        for reason in ("",None,False):
            self.reject_report(lambda r:r["trials"][0]["samples"][3]["observations"][0]["automatic_plan"].update(Reason=reason),kind="grams")
        self.reject_report(lambda r:r["trials"][0]["samples"][3]["observations"][1]["automatic_plan"].update(Reason="unexplained generic scan"),kind="grams")
        self.reject_report(lambda r:r["trials"][0]["samples"][3]["observations"][0]["automatic_work"].update(base_row_fetches=0),kind="grams")
        # The frequency fallback legitimately has planning probes. An automatic
        # fallback may retain abandoned postings; only NOT INDEXED must have none.
        def retained(r):
            row=r["trials"][0]["samples"][3]["observations"][0]
            row["automatic_plan"]["Reason"]="posting/candidate memory budget exceeded"
            row["automatic_work"].update(frequency_lookups=1,posting_keys_read=2)
        self.admit_report(retained,kind="grams")
    def test_pass_and_significant_regression(self):
        self.assertEqual(self.evaluate(self.bundle())["status"], "PASS")
        self.assertEqual(self.evaluate(self.bundle(cost=2))["status"], "REGRESSION")
    def test_shared_host_never_passes_or_claims_confidence(self):
        for cost in (.5, 1, 2):
            r = self.evaluate(self.bundle(controlled=False, cost=cost))
            self.assertEqual(r["status"], "INCONCLUSIVE")
            self.assertFalse(r["metrics"]["grams_build_ms"]["confidence_applicable"])
    def test_few_pairs_do_not_use_inner_builds(self):
        r = self.evaluate(self.bundle(n=4))
        self.assertEqual(r["status"], "INCONCLUSIVE")
        self.assertEqual(len(r["metrics"]["grams_build_ms"]["paired_ratios_candidate_over_baseline_cost"]), 4)
    def test_overlap_is_inconclusive(self):
        b = self.bundle()
        self.change_raw(b, lambda r: r["trials"][0].update(index_build_ms=300))
        self.assertEqual(self.evaluate(b)["status"], "INCONCLUSIVE")
    def test_statuses(self):
        for status, expected in (("failed", "FAILED"), ("bootstrap", "UNSUPPORTED"), ("incomplete", "MISSING")):
            b = self.bundle(); b["status"] = status
            if status == "bootstrap":
                b["pairs"] = [{"pair":1,"order":["candidate"],"candidate":b["pairs"][0]["candidate"]}]
            self.assertEqual(self.evaluate(b)["status"], expected)
        self.assertEqual(c.evaluate(self.root/"absent.json")["status"], "MISSING")
    def test_report_failure_wins_over_good_timings(self):
        changes = [lambda r: r.update(status="failed"), lambda r: r.update(errors=["failure"]),
                   lambda r: r["trials"][0].update(extension_consistency_after_reopen=False),
                   lambda r: r["trials"][0]["samples"][0]["observations"][0].update(full_ordered_results_equal=False)]
        for change in changes:
            b = self.bundle(); self.change_raw(b, change)
            self.assertEqual(self.evaluate(b)["status"], "FAILED")
    def test_rejects_identity_mismatch(self):
        changes = [lambda r: r.update(cache_condition="different"), lambda r: r.update(workload_sha256="e"*64),
                   lambda r: r["config"].update(rows=128), lambda r: r["config"].update(source_id="wrong"),
                   lambda r: r.update(gomaxprocs=4), lambda r: r.update(hardware_id="other")]
        for change in changes:
            b = self.bundle(); self.change_raw(b, change)
            self.assertEqual(self.evaluate(b)["status"], "INADMISSIBLE")
    def test_rejects_partial_duplicate_and_reordered_evidence(self):
        for mutator, want in ((lambda b: b["pairs"].pop(), "MISSING"),
             (lambda b: b["pairs"][1].update(order=["baseline", "candidate"]), "INADMISSIBLE"),
             (lambda b: b["pairs"][1]["baseline"].update(grams=b["pairs"][0]["baseline"]["grams"]), "INADMISSIBLE"),
             (lambda b: b["pairs"][0]["baseline"]["grams"].update(exit_code=1), "FAILED"),
             (lambda b: b["pairs"][0]["baseline"]["grams"].update(sha256="bad"), "INADMISSIBLE"),
             (lambda b: b["pairs"][0]["baseline"]["grams"].update(path="../escape"), "INADMISSIBLE")):
            b = self.bundle(); mutator(b)
            self.assertEqual(self.evaluate(b)["status"], want)
    def test_manifest_harness_host_attestation(self):
        changes = [lambda b: b["sources"]["baseline"].update(id="fake"),
                   lambda b: b["harnesses"].update(candidate="fake"),
                   lambda b: b["environment"].update(hardware_id="fake"),
                   lambda b: b["environment"].update(control_reason="")]
        for change in changes:
            b=self.bundle(); change(b)
            self.assertEqual(self.evaluate(b)["status"], "INADMISSIBLE")
    def test_nonfinite_json_and_nonpositive_metrics(self):
        p=self.root/"bad.json"; p.write_text('{"x":NaN}')
        self.assertEqual(c.evaluate(p)["status"], "INADMISSIBLE")
        b=self.bundle(); self.change_raw(b, lambda r: r["trials"][0].update(index_build_ms=0))
        self.assertEqual(self.evaluate(b)["status"], "INADMISSIBLE")
    def test_transaction_validation(self):
        changes = [lambda r: r.update(final_model_verified=False), lambda r: r.update(shutdown="unknown"),
                   lambda r: r["writer"].update(failed_outcomes=1), lambda r: r.update(verified_reopens=0)]
        for change in changes:
            b=self.bundle(); self.change_raw(b, change, kind="transactions")
            self.assertEqual(self.evaluate(b)["status"], "FAILED")
        b=self.bundle(); self.change_raw(b, lambda r: r.update(committed_transactions_per_active_second=2), kind="transactions")
        self.assertEqual(self.evaluate(b)["status"], "INADMISSIBLE")
    def test_strict_transaction_closure_version_and_latch(self):
        b=self.bundle(); self.change_raw(b, lambda r: r.pop("close_failures"), kind="transactions")
        self.assertEqual(self.evaluate(b)["status"], "INADMISSIBLE")
        for key, value in (("close_failures",1),("unverified_close_handles",1),("unverified_retirement",True)):
            b=self.bundle(); self.change_raw(b, lambda r: r.update({key:value}),kind="transactions")
            self.assertEqual(self.evaluate(b)["status"], "FAILED")
    def test_unsupported_schema_and_profile(self):
        b=self.bundle(); self.change_raw(b, lambda r: r.update(schema_version=9))
        self.assertEqual(self.evaluate(b)["status"], "UNSUPPORTED")
        b=self.bundle(); self.change_raw(b, lambda r: r["config"].update(mode="soak"),kind="transactions")
        self.assertEqual(self.evaluate(b)["status"], "INADMISSIBLE")
    def test_duplicate_keys_and_nonobject_json(self):
        p=self.root/"invalid.json"
        for raw in ('{"schema_version":1,"schema_version":2}', '[]', 'null', 'false'):
            p.write_text(raw)
            self.assertEqual(c.evaluate(p)["status"], "INADMISSIBLE")
    def test_missing_expected_report_is_missing(self):
        b=self.bundle(); (self.root/b["pairs"][0]["baseline"]["grams"]["path"]).unlink()
        self.assertEqual(self.evaluate(b)["status"], "MISSING")
    def test_dependency_change_requires_new_baseline(self):
        b=self.bundle(); m=b["sources"]["baseline"]["manifest"]
        m["go.mod"]["sha256"]="f"*64
        b["sources"]["baseline"]["id"]="sha256:"+c.sha(c.canonical(m))
        b["sources"]["baseline"]["verified_after"]=b["sources"]["baseline"]["id"]
        self.assertEqual(self.evaluate(b)["status"], "UNSUPPORTED")
    def test_parser_missing_bucket(self):
        b=self.bundle(); self.change_raw(b, lambda r: r["trials"][0]["samples"].pop())
        self.assertEqual(self.evaluate(b)["status"], "INADMISSIBLE")
    def test_unsupported_baseline_harness(self):
        b=self.bundle(); m=b["sources"]["baseline"]["manifest"]
        m["benchmarks/grams/main.go"]["sha256"]="e"*64
        b["sources"]["baseline"]["id"]="sha256:"+c.sha(c.canonical(m))
        b["sources"]["baseline"]["verified_after"]=b["sources"]["baseline"]["id"]
        b["harnesses"]["baseline"]=c.sha(c.canonical({p:v for p,v in m.items() if p.startswith("benchmarks/")}))
        self.assertEqual(self.evaluate(b)["status"], "UNSUPPORTED")

class AdmissionRepairs(unittest.TestCase):
    setUp=ComparisonTests.setUp
    bundle=ComparisonTests.bundle
    evaluate=ComparisonTests.evaluate
    change_raw=ComparisonTests.change_raw
    def test_known_defective_transaction_revisions_rejected(self):
        for revision in (None,"ordinary-transactions-v1","ordinary-transactions-v2","ordinary-transactions-v3",True):
            b=self.bundle();self.change_raw(b,lambda r:r.update(harness_revision=revision),kind="transactions")
            self.assertEqual(self.evaluate(b)["status"],"UNSUPPORTED")
    def test_impossible_accounting_and_corrupted_trace(self):
        changes=[lambda r:r["writer"].update(committed=4800),lambda r:r["writer"].update(canceled_outcomes=64),
                 lambda r:r["writer"].update(success=True),lambda r:r["windows"][0].update(writer_outcomes="C"*64),
                 lambda r:r.update(final_model_sha256="a"*64),lambda r:r["windows"][0]["writer"].update(attempts=65),
                 lambda r:r["writer_attempt_latency"]["buckets_power_of_two_microseconds"].__setitem__(0,100)]
        for change in changes:
            b=self.bundle();self.change_raw(b,change,kind="transactions")
            self.assertIn(self.evaluate(b)["status"],("INADMISSIBLE","FAILED"))
    def test_fixed_config_fields_types_values(self):
        for kind,cfg in (("grams",GRAMS_CONFIG),("transactions",TRANSACTIONS_CONFIG)):
            for key in cfg:
                b=self.bundle();self.change_raw(b,lambda r:r["config"].pop(key),kind=kind)
                self.assertEqual(self.evaluate(b)["status"],"INADMISSIBLE")
            for value in (-100,True,"64"):
                b=self.bundle();self.change_raw(b,lambda r:r["config"].update(rows=value),kind=kind)
                self.assertEqual(self.evaluate(b)["status"],"INADMISSIBLE")
    def test_manifest_path_and_digest_rejected(self):
        for path,value in (("../outside.go","Z"*64),("inside.go","Z"*64),("a//b.go","a"*64),("/absolute","a"*64)):
            b=self.bundle();m=b["sources"]["baseline"]["manifest"];m[path]={"sha256":value,"mode":"100644"}
            sid="sha256:"+c.sha(c.canonical(m));b["sources"]["baseline"].update(id=sid,verified_after=sid)
            self.assertEqual(self.evaluate(b)["status"],"INADMISSIBLE")
    def test_boolean_schema_and_real_chronology(self):
        b=self.bundle();b["schema_version"]=True;self.assertEqual(self.evaluate(b)["status"],"UNSUPPORTED")
        for value in (True,-1,"2026-02-30T00:00:00Z","2026-01-01T99:00:00Z"):
            b=self.bundle();self.change_raw(b,lambda r:r.update(started_utc=value))
            self.assertEqual(self.evaluate(b)["status"],"INADMISSIBLE")
        b=self.bundle();b["session_id"]="not-a-session";self.assertEqual(self.evaluate(b)["status"],"INADMISSIBLE")
    def test_binary_commands_and_workspaces_are_bound(self):
        for change in (lambda b:b.pop("binaries"),lambda b:b["binaries"]["baseline"]["grams"].update(sha256="Z"*64),
                       lambda b:b["sources"]["candidate"].update(verified_after="other")):
            b=self.bundle();change(b);self.assertEqual(self.evaluate(b)["status"],"INADMISSIBLE")
        for mutation in (lambda r:r["build_environment"].update(GOWORK="/outside/go.work"),
                         lambda r:r.update(executed_binary_sha256="a"*64),lambda r:r.update(source_id="wrong"),
                         lambda r:r.update(session_id="wrong"),lambda r:r.update(started_utc=iso(14000))):
            b=self.bundle();ref=b["pairs"][0]["baseline"]["grams"]["command"];p=self.root/ref["path"]
            rec=json.loads(p.read_text());mutation(rec);p.write_bytes(c.canonical(rec));ref["sha256"]=c.sha(p.read_bytes())
            self.assertEqual(self.evaluate(b)["status"],"INADMISSIBLE")
    def test_nonfinite_exponent_anywhere_rejected(self):
        b=self.bundle();ref=b["pairs"][0]["baseline"]["transactions"];p=self.root/ref["path"]
        p.write_bytes(p.read_bytes().replace(b'{',b'{"ignored":1e309,',1));ref["sha256"]=c.sha(p.read_bytes())
        self.assertEqual(self.evaluate(b)["status"],"INADMISSIBLE")
    def test_ordinary_busy_history_remains_admissible(self):
        def change(r):
            w=r["windows"][0];trace=w["writer_outcomes"];trace=trace[:1]+"B"+trace[2:];w["writer_outcomes"]=trace
            for counts in (r["writer"],w["writer"]):
                counts["success"]-=1;counts["committed"]-=1;counts["savepoint_rollback_commits"]-=1;counts["busy_outcomes"]+=1
                counts["busy_events_including_commit_retries"]+=1
            r["final_model_sha256"]=model(trace)[1];r["final_model_generation"]=47
            w["committed_transactions_per_second"]=47/w["active_seconds"]
            r["committed_transactions_per_active_second"]=47/r["active_seconds"]
            r["committed_transactions_per_elapsed_second"]=47/r["elapsed_seconds"]
        b=self.bundle();self.change_raw(b,change,kind="transactions")
        self.assertEqual(self.evaluate(b)["status"],"PASS")

class HistogramBucketTests(unittest.TestCase):
    def test_constant_moments_cannot_contradict_buckets(self):
        from validation import histogram
        h=latency_fixture([1.]*64)
        h["buckets_power_of_two_microseconds"]=[64]+[0]*47
        h.update(p50_bucket_upper_ms=.001,p95_bucket_upper_ms=.001)
        with self.assertRaisesRegex(c.EvidenceError,"minimum contradicts"):
            histogram(h,64)
    def test_both_extrema_must_match_first_and_last_occupied_buckets(self):
        from validation import histogram
        for samples, replacement, label in (([.0005,.003],[.003,.003],"minimum"),
                                             ([.0005,.003],[.0005,.0005],"maximum")):
            h=latency_fixture(samples);other=latency_fixture(replacement)
            for key in ("buckets_power_of_two_microseconds","p50_bucket_upper_ms","p95_bucket_upper_ms"):
                h[key]=other[key]
            with self.assertRaisesRegex(c.EvidenceError,label+" contradicts"):
                histogram(h,2)
    def test_weighted_bucket_mean_bound(self):
        from validation import histogram
        h=latency_fixture([.001]*90+[1.024]*10)
        h["milliseconds"].update(mean=.5,sample_stddev=.3)
        with self.assertRaisesRegex(c.EvidenceError,"mean contradicts bucket counts"):
            histogram(h,100)
    def test_weighted_bucket_variance_bound(self):
        from validation import histogram
        h=latency_fixture([0]*50+[1.024]*50)
        h["milliseconds"]["sample_stddev"]=.1
        with self.assertRaisesRegex(c.EvidenceError,"variance contradicts bucket counts"):
            histogram(h,100)
    def test_valid_bucket_edges_empty_and_overflow(self):
        from validation import histogram
        for values in ([],[0],[.001],[.002],[.001]*64,
                       [0,.001,.001000001,.002,.002000001,1.024],
                       [2**46/1000,2**47/1000,2**49/1000],
                       [2**49/1000]*3):
            with self.subTest(values=values):
                histogram(latency_fixture(values),len(values))


class InputProbe:
    """Small in-memory regular-file double; it never opens an OS file."""
    def __init__(self, snapshots=(b'{}',), mode=c.stat.S_IFREG, error=None, response=None):
        self.snapshots=snapshots;self.mode=mode;self.error=error;self.response=response
        self.opens=0;self.requests=[];self.closed=0
        self.path=Path("/synthetic/evidence/record.json")
    def __str__(self):return str(self.path)
    def stat(self):
        if self.error is not None:raise self.error
        return type("FileStat",(),{"st_mode":self.mode})()
    def is_symlink(self):return False
    def resolve(self):return self.path
    def open(self,mode):
        if mode!="rb":raise AssertionError("expected binary snapshot read")
        self.data=self.snapshots[min(self.opens,len(self.snapshots)-1)]
        self.opens+=1;self.position=0
        return self
    def __enter__(self):return self
    def __exit__(self,*args):self.closed+=1
    def read(self,size):
        self.requests.append(size)
        if self.response is not None:return self.response
        chunk=self.data[self.position:self.position+size];self.position+=len(chunk)
        return chunk
    def read_bytes(self):raise AssertionError("unbounded read forbidden")
    def read_text(self):raise AssertionError("second text read forbidden")


class ProbeRoot:
    def __init__(self,probe):self.probe=probe
    def __truediv__(self,name):
        if name!="record.json":raise AssertionError("unexpected fixture path")
        return self.probe
    def resolve(self):return self.probe.path.parent


class BoundedInputTests(unittest.TestCase):
    def test_json_reads_at_most_limit_plus_one_once(self):
        probe=InputProbe()
        self.assertEqual(c.read_json(probe),{})
        self.assertEqual((probe.opens,probe.requests,probe.closed),(1,[c.MAX_JSON_BYTES+1],1))
        # A sized sentinel exercises the oversize branch without allocating a
        # 32 MiB fixture; parsing/hash access would fail if the guard were absent.
        oversized=type("Oversized",(),{"__len__":lambda self:c.MAX_JSON_BYTES+1})()
        probe=InputProbe(response=oversized)
        with self.assertRaisesRegex(c.EvidenceError,"exceeds bounded 32MiB") as caught:
            c.read_json(probe)
        self.assertEqual(caught.exception.status,"INADMISSIBLE")
        self.assertEqual((probe.opens,probe.requests,probe.closed),(1,[c.MAX_JSON_BYTES+1],1))
    def test_exact_json_size_boundary_is_allowed(self):
        # This small bytes subclass models the length boundary without a large
        # allocation; real byte decoding still sees a complete JSON object.
        class AtLimit(bytes):
            def __len__(self):return c.MAX_JSON_BYTES
        probe=InputProbe(response=AtLimit(b'{}'))
        self.assertEqual(c.read_json(probe),{})
        self.assertEqual(probe.requests,[c.MAX_JSON_BYTES+1])
    def test_report_hash_and_parse_share_one_snapshot(self):
        first=b'{"version":1}';probe=InputProbe((first,b'{"version":2}'))
        ref={"path":"record.json","sha256":c.sha(first),"exit_code":0}
        self.assertEqual(c.reference(ProbeRoot(probe),ref,set()),{"version":1})
        self.assertEqual((probe.opens,probe.requests),(1,[c.MAX_JSON_BYTES+1]))
        ref["sha256"]="0"*64
        with self.assertRaisesRegex(c.EvidenceError,"digest mismatch"):
            c.reference(ProbeRoot(InputProbe((first,))),ref,set())
    def test_command_artifact_hash_and_parse_share_one_snapshot(self):
        first=b'{"version":1}';probe=InputProbe((first,b'{"version":2}'))
        ref={"path":"record.json","sha256":c.sha(first)}
        self.assertEqual(c.artifact(ProbeRoot(probe),ref,json_record=True),{"version":1})
        self.assertEqual((probe.opens,probe.requests),(1,[c.MAX_JSON_BYTES+1]))
        ref["sha256"]="0"*64
        with self.assertRaisesRegex(c.EvidenceError,"digest mismatch"):
            c.artifact(ProbeRoot(InputProbe((first,))),ref,json_record=True)
    def test_missing_nonregular_and_io_errors_have_explicit_status(self):
        for reader in (c.read_json_bytes,c.hash_artifact):
            for probe,status in ((InputProbe(error=FileNotFoundError("missing")),"MISSING"),
                                 (InputProbe(mode=c.stat.S_IFDIR),"INADMISSIBLE"),
                                 (InputProbe(mode=c.stat.S_IFIFO),"INADMISSIBLE"),
                                 (InputProbe(error=PermissionError("denied")),"INADMISSIBLE")):
                with self.subTest(reader=reader.__name__,status=status):
                    with self.assertRaises(c.EvidenceError) as caught:reader(probe)
                    self.assertEqual(caught.exception.status,status)
                    self.assertEqual(probe.opens,0)
        with tempfile.TemporaryDirectory() as directory:
            self.assertEqual(c.evaluate(Path(directory))["status"],"INADMISSIBLE")
            self.assertEqual(c.evaluate(Path(directory)/"missing.json")["status"],"MISSING")
    def test_artifact_hashing_uses_bounded_chunks(self):
        raw=b"synthetic artifact\n"*(c.HASH_CHUNK_BYTES//8)
        probe=InputProbe((raw,))
        self.assertEqual(c.hash_artifact(probe),c.sha(raw))
        self.assertGreater(len(probe.requests),2)
        self.assertEqual(set(probe.requests),{c.HASH_CHUNK_BYTES})
        self.assertEqual((probe.opens,probe.closed),(1,1))
    def test_bounded_snapshot_keeps_strict_json_errors(self):
        for raw in (b'\xff',b'{"x":1,"x":2}',b'{"x":NaN}',b'{"x":1e309}',b'{'):
            with self.subTest(raw=raw),self.assertRaises(c.EvidenceError) as caught:
                c.read_json(InputProbe((raw,)))
            self.assertEqual(caught.exception.status,"INADMISSIBLE")


if __name__ == "__main__":
    unittest.main()
