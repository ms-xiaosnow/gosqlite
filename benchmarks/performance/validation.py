"""Typed fixed-profile evidence contracts; pure parsing and deterministic models."""
import calendar
import datetime
import hashlib
import json
import math
import re
import statistics
from pathlib import PurePosixPath

class EvidenceError(Exception):
    def __init__(self, status, message):
        self.status = status
        super().__init__(message)

def require(condition, message, status="INADMISSIBLE"):
    if not condition:
        raise EvidenceError(status, message)

def canonical(value):
    return json.dumps(value, sort_keys=True, separators=(",", ":"), allow_nan=False).encode()

def sha(data):
    return hashlib.sha256(data).hexdigest()

def integer(v, minimum=0):
    require(type(v) is int and minimum <= v <= (1<<64)-1, "expected bounded nonnegative integer")
    return v

def number(v, minimum=0):
    require(type(v) in (int, float) and math.isfinite(v) and v >= minimum, "expected finite nonnegative number")
    return v

def finite(v):
    number(v)
    require(v > 0, "metric must be positive")
    return v

def digest(v):
    require(type(v) is str and re.fullmatch(r"[0-9a-f]{64}", v) is not None, "invalid SHA256 digest")
    return v

def relative(v):
    require(type(v) is str and v and "\\" not in v and not any(ord(x) < 32 for x in v), "invalid relative path")
    p = PurePosixPath(v)
    require(not p.is_absolute() and all(x not in (".", "..", "") for x in v.split("/")) and str(p) == v,
            "noncanonical relative path")
    return v

def timestamp(v):
    require(type(v) is str, "timestamp must be an RFC3339 UTC string")
    m = re.fullmatch(r"(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})(?:\.(\d{1,9}))?(?:Z|\+00:00)", v)
    require(m is not None, "timestamp must be an RFC3339 UTC string")
    try:
        dt = datetime.datetime.strptime(m[1], "%Y-%m-%dT%H:%M:%S")
    except ValueError as exc:
        raise EvidenceError("INADMISSIBLE", "invalid UTC date") from exc
    return calendar.timegm(dt.timetuple()) * 10**9 + int((m[2] or "").ljust(9, "0"))

def period(rep):
    a, b = timestamp(rep["started_utc"]), timestamp(rep["finished_utc"])
    require(a <= b, "reversed invocation chronology")
    return a, b

def close(a, b, label):
    number(a); number(b)
    require(math.isclose(a, b, rel_tol=1e-9, abs_tol=1e-9), label + " accounting mismatch")

def keys(obj, names):
    require(isinstance(obj, dict) and set(names) <= obj.keys(), "required report fields missing: " + ",".join(sorted(set(names)-set(obj) if isinstance(obj, dict) else set(names))))

def recursive_finite(v):
    if type(v) is float:
        require(math.isfinite(v), "nonfinite JSON value")
    elif isinstance(v, dict):
        for x in v.values(): recursive_finite(x)
    elif isinstance(v, list):
        for x in v: recursive_finite(x)

GRAMS_CONFIG = {"rows":64,"queries_per_gram_length":1,"independent_database_builds":2,
                "dataset_seed":2026101052,"page_size":4096,"max_transaction_pages":4096}
TRANSACTIONS_CONFIG = {"mode":"throughput","profile":"rollback","requested_active_duration_ns":10**9,
    "requested_writer_attempts":64,"rows":16,"readers":1,"transfer_pairs_per_transaction":1,
    "dataset_seed":20261010,"writer_pause_ns":0,"reader_pause_ns":10**6,"snapshot_hold_ns":10**6,
    "max_data_bytes":16777216}
GRAMS_ARGS = ["-rows","64","-queries","1","-repeats","2","-seed","2026101052","-page-size","4096","-max-pages","4096"]
TRANSACTIONS_ARGS = ["-mode","throughput","-profile","rollback","-transactions","64","-rows","16",
    "-readers","1","-batch","1","-seed","20261010","-duration","1s","-writer-pause","0",
    "-reader-pause","1ms","-snapshot-hold","1ms","-max-data-bytes","16777216"]
BUILD_ENV = {"CGO_ENABLED":"0","GOFLAGS":"-buildvcs=false","GOTOOLCHAIN":"local","GOMAXPROCS":"2","GOENV":"off","GOWORK":"off"}
CACHE = {"grams":"cold_connection and warm_process; uncontrolled OS caches; no cache flush",
         "transactions":"warm OS cache, bootstrap excluded, no OS cache flush; new DB each invocation"}
# V4 reads FinishedUTC immediately before ActiveSeconds. Allow at most 2ms
# for the adjacent reads/formatting; wall-clock discontinuities fail admission.
WINDOW_CLOCK_TOLERANCE_SECONDS = .002
TRANSACTION_HEAP_LIMIT_BYTES = 256 << 20

# This is accounting tolerance, not a statistical confidence calculation.
def time_bound(measured, available, label):
    number(measured); number(available)
    require(measured <= available + max(1e-9, available*1e-9), label)

COUNTS = "attempts success committed intentional_rollbacks savepoint_rollback_commits savepoint_release_commits busy_outcomes busy_events_including_commit_retries canceled_outcomes failed_outcomes snapshot_checks".split()

def config(rep, kind):
    expected = GRAMS_CONFIG if kind == "grams" else TRANSACTIONS_CONFIG
    cfg = rep["config"]
    require(isinstance(cfg, dict) and set(cfg) == set(expected)|{"source_id","hardware_id"}, "missing or unknown fixed-profile config")
    for k, v in expected.items():
        require(type(cfg[k]) is type(v) and cfg[k] == v, "invalid fixed-profile config: " + k)
    for k in ("source_id", "hardware_id"):
        require(type(cfg[k]) is str and cfg[k].startswith("sha256:"), "computed source/hardware identity required")
        digest(cfg[k][7:])

def counts(c, writer):
    require(isinstance(c, dict) and set(c) == set(COUNTS), "incomplete outcome counters")
    for v in c.values(): integer(v)
    require(c["failed_outcomes"] == 0, "failed outcomes invalidate performance", "FAILED")
    require(c["attempts"] == sum(c[k] for k in ("success","busy_outcomes","canceled_outcomes","failed_outcomes")), "outcome totals contradict attempts")
    require(c["failed_outcomes"] == 0, "failed outcomes invalidate performance", "FAILED")
    require(c["busy_events_including_commit_retries"] >= c["busy_outcomes"], "BUSY event count below outcomes")
    if writer:
        require(c["success"] == c["committed"] + c["intentional_rollbacks"] and
                c["savepoint_rollback_commits"] + c["savepoint_release_commits"] <= c["committed"] and c["snapshot_checks"] == 0,
                "writer commit/rollback accounting impossible")
    else:
        require(all(c[k] == 0 for k in ("committed","intentional_rollbacks","savepoint_rollback_commits","savepoint_release_commits")), "reader has writer outcomes")
        require(2*c["success"] <= c["snapshot_checks"] <= 2*c["attempts"], "reader snapshot accounting impossible")

def mix(x):
    mask=(1<<64)-1
    x=(x+0x9e3779b97f4a7c15)&mask; x=((x^(x>>30))*0xbf58476d1ce4e5b9)&mask
    x=((x^(x>>27))*0x94d049bb133111eb)&mask
    return x^(x>>31)

def model(trace, seed=20261010, n=16):
    mask=(1<<64)-1
    values=[1000+mix((seed+i)&mask)%100 for i in range(n)]
    initial=sha(json.dumps(values,separators=(",", ":")).encode())
    for ordinal, code in enumerate(trace):
        if code != "C": continue
        kind=ordinal%4
        require(kind != 0, "rollback ordinal cannot commit")
        for salt in ([1,1000001] if kind==2 else [1]):
            x=mix(seed ^ mix((ordinal*67+salt)&mask)); a=x%n; b=(a+1+mix(x)%(n-1))%n; delta=1+mix(x^17)%7
            values[a]+=delta; values[b]-=delta
    return initial, sha(json.dumps(values,separators=(",", ":")).encode())

def histogram_bucket_moments(s, buckets, n):
    """Necessary bucket/moment bounds, not reconstruction of individual samples."""
    if not n:
        return
    occupied = [i for i, count in enumerate(buckets) if count]
    low, high = s["min"], s["max"]
    # The last bucket is overflow: the collector stops at index 47 even when
    # an observation exceeds its displayed quantile bound. Use the recorded
    # maximum as its upper bound, never invent a finite overflow-bucket cap.
    def bounds(i):
        return (0 if i == 0 else 2**(i-1)/1000,
                high if i == 47 else 2**i/1000)
    edge_tolerance = max(1e-9, high*1e-9, 8*n*math.ulp(high))
    for value, i, label in ((low, occupied[0], "minimum"),
                            (high, occupied[-1], "maximum")):
        a, b = bounds(i)
        require(a-edge_tolerance <= value <= b+edge_tolerance,
                "histogram " + label + " contradicts occupied buckets")
    span = high-low
    if not span:
        return
    # Work in [0,1] to avoid overflow while squaring durations. Each actual
    # observation lies in the intersection of its bucket and observed extrema.
    # Relax open lower edges and use the existing bounded Welford tolerance.
    intervals = []
    for i in occupied:
        a, b = bounds(i)
        a = max(0., min(1., (a-low)/span))
        b = max(0., min(1., (b-low)/span))
        intervals.append((buckets[i]/n, a, b))
    q = (s["mean"]-low)/span
    tolerance = max(1e-9, 8*n*math.ulp(high)/span)
    mean_low = math.fsum(weight*a for weight, a, b in intervals)
    mean_high = math.fsum(weight*b for weight, a, b in intervals)
    require(mean_low-tolerance <= q <= mean_high+tolerance,
            "histogram mean contradicts bucket counts")
    # Bound squared deviations around the reported mean in each interval.
    # These necessary weighted bounds do not assert joint moment feasibility.
    variance_low = math.fsum(weight*(0 if a <= q <= b else min((a-q)**2, (b-q)**2))
                             for weight, a, b in intervals)
    variance_high = math.fsum(weight*max((a-q)**2, (b-q)**2)
                              for weight, a, b in intervals)
    deviation = s["sample_stddev"]/span*math.sqrt((n-1)/n)
    require(math.sqrt(variance_low)-tolerance <= deviation <= math.sqrt(variance_high)+tolerance,
            "histogram variance contradicts bucket counts")

def histogram(h, n):
    keys(h, ["milliseconds","p50_bucket_upper_ms","p95_bucket_upper_ms","buckets_power_of_two_microseconds"])
    s=h["milliseconds"]; keys(s,["sample_count","mean","sample_stddev","min","max","confidence_method","confidence_95_low","confidence_95_high"])
    require(integer(s["sample_count"])==n and s["confidence_95_low"] is None and s["confidence_95_high"] is None, "invalid histogram count/confidence")
    for k in ("mean","sample_stddev","min","max"): number(s[k])
    require(s["min"] <= s["mean"] <= s["max"], "histogram moments inconsistent")
    if n < 2:
        require(s["sample_stddev"] == 0, "fewer than two samples cannot have deviation")
    if n == 1:
        require(s["min"] == s["mean"] == s["max"], "single-sample moments disagree")
    if n >= 2:
        span=s["max"]-s["min"]
        if span == 0:
            require(s["sample_stddev"] == 0, "constant samples cannot have deviation")
        else:
            # Normalize first to avoid overflow when squaring millisecond values.
            # Both observed extrema must occur; the other n-2 observations lie
            # between them. These are necessary finite-sample moment bounds,
            # not a reconstruction of the unretained individual observations.
            q=(s["mean"]-s["min"])/span
            # Welford updates/merges can lose low bits for nearly equal, large
            # durations. Allow bounded ulp accumulation, not unbounded variance.
            tolerance=max(1e-9, 8*n*math.ulp(s["max"])/span)
            require(1/n-tolerance <= q <= 1-1/n+tolerance, "mean cannot include both observed extrema")
            scaled=s["sample_stddev"]/span
            lower=math.sqrt((q*q+(1-q)*(1-q))/(n-1))
            upper=math.sqrt(max(0, q*(1-q))*n/(n-1))
            require(lower-tolerance <= scaled <= upper+tolerance, "histogram variance infeasible for count/mean/extrema")
    buckets=h["buckets_power_of_two_microseconds"]
    require(isinstance(buckets,list) and len(buckets)==48, "histogram buckets missing")
    require(sum(integer(x) for x in buckets)==n, "histogram does not account for all attempts")
    histogram_bucket_moments(s, buckets, n)
    for p,k in ((.5,"p50_bucket_upper_ms"),(.95,"p95_bucket_upper_ms")):
        rank=math.ceil(n*p); total=0; expected=0
        if n:
            for i,count in enumerate(buckets):
                total+=count
                if total>=rank: expected=2**i/1000; break
        close(h[k],expected,"histogram quantile")
    if not n: require(all(s[k]==0 for k in ("mean","min","max","sample_stddev")), "empty histogram moments")

def distribution(d, values, grams=False):
    n=len(values)
    count="sample_count"
    fields=["mean_ms","sample_stddev_ms","min_ms","max_ms"] if grams else ["mean","sample_stddev","min","max"]
    keys(d,[count,"confidence_method"]+fields)
    require(integer(d[count])==n and type(d["confidence_method"]) is str and d["confidence_method"],"invalid descriptive distribution")
    expected=[statistics.mean(values) if n else 0,statistics.stdev(values) if n>1 else 0,min(values) if n else 0,max(values) if n else 0]
    for key,value in zip(fields,expected):close(d[key],value,"distribution "+key)
    low,high=("ci_low_ms","ci_high_ms") if grams else ("confidence_95_low","confidence_95_high")
    require(d[low] is None and d[high] is None,"correlated samples cannot have inferential confidence")
    if grams:
        ordered=sorted(values)
        for key,q in (("p50_ms",.5),("p95_ms",.95)):
            close(d[key],ordered[max(0,math.ceil(q*n)-1)] if n else 0,"empirical percentile")

def validate_transactions(rep):
    require(rep.get("harness_revision") == "ordinary-transactions-v4", "unsupported transaction implementation revision", "UNSUPPORTED")
    keys(rep, "unverified_retirement unverified_close_handles close_failures final_model_generation final_model_sha256 max_observed_data_files final_encrypted_commit_sequence build_settings workload_sha256 dataset_sha256 active_seconds retained_data_directory max_observed_data_bytes max_observed_heap_bytes writer readers writer_attempt_latency reader_attempt_latency ordinary_checkpoint_latency ordinary_reopen_latency checkpoint_status successful_checkpoints verified_reopens final_model_verified shutdown committed_transactions_per_active_second writer_attempts_per_active_second committed_transactions_per_elapsed_second window_commit_rate_statistics windows harness_limits notes".split())
    require(rep["unverified_retirement"] is False and integer(rep["unverified_close_handles"])==0 and integer(rep["close_failures"])==0, "retirement uncertainty", "FAILED")
    counts(rep["writer"],True); counts(rep["readers"],False)
    require(rep["writer"]["attempts"]==64, "fixed attempts missing")
    require(rep["readers"]["success"] > 0 and all(rep["writer"][k] > 0 for k in
            ("committed", "intentional_rollbacks", "savepoint_rollback_commits", "savepoint_release_commits"))
            and rep["final_model_verified"] is True,
            "required V4 successful outcome coverage missing", "FAILED")
    ws=rep["windows"]; require(isinstance(ws,list) and 1<=len(ws)<=64, "invalid transaction windows")
    start,end=period(rep); ordinal=0; trace=""; seconds=0; previous=start
    totals={arm:{k:0 for k in COUNTS} for arm in ("writer","readers")}
    for i,w in enumerate(ws):
        keys(w,"first_writer_ordinal writer_outcomes index started_utc finished_utc active_seconds writer readers committed_transactions_per_second".split())
        require(type(w["index"]) is int and w["index"]==i and integer(w["first_writer_ordinal"])==ordinal,"window index/ordinal mismatch")
        a,b=period(w); require(previous<=a<=b<=end,"window chronology outside report"); previous=b
        for arm in totals:
            counts(w[arm],arm=="writer")
            for k in COUNTS: totals[arm][k]+=w[arm][k]
        t=w["writer_outcomes"]; require(type(t) is str and len(t)==w["writer"]["attempts"] and 1<=len(t)<=64 and set(t)<=set("CRBXF"),"invalid outcome trace")
        for code,key in (("C","committed"),("R","intentional_rollbacks"),("B","busy_outcomes"),("X","canceled_outcomes"),("F","failed_outcomes")):
            require(t.count(code)==w["writer"][key],"trace/outcome counters disagree")
        require(all((ordinal+j)%4==0 for j,x in enumerate(t) if x=="R"),"rollback trace ordinal mismatch")
        require(sum(x=="C" and (ordinal+j)%4==1 for j,x in enumerate(t))==w["writer"]["savepoint_rollback_commits"] and
                sum(x=="C" and (ordinal+j)%4==2 for j,x in enumerate(t))==w["writer"]["savepoint_release_commits"],"savepoint counters disagree with trace")
        active=finite(w["active_seconds"])
        require(abs(active-(b-a)/1e9) <= WINDOW_CLOCK_TOLERANCE_SECONDS,
                "window active time contradicts UTC duration")
        close(w["committed_transactions_per_second"],t.count("C")/active,"window rate")
        seconds+=active; ordinal+=len(t); trace+=t
    require(totals["writer"]==rep["writer"] and totals["readers"]==rep["readers"],"window totals disagree with report")
    close(rep["active_seconds"],seconds,"active time")
    time_bound(seconds, finite(rep["elapsed_seconds"]), "active time exceeds elapsed")
    for field,expected in (("committed_transactions_per_active_second",rep["writer"]["committed"]/seconds),
        ("writer_attempts_per_active_second",64/seconds),("committed_transactions_per_elapsed_second",rep["writer"]["committed"]/rep["elapsed_seconds"])):
        close(rep[field],expected,field)
    initial, final=model(trace)
    require(rep["dataset_sha256"]==initial and rep["final_model_sha256"]==final and integer(rep["final_model_generation"])==trace.count("C"),"deterministic transaction model mismatch","FAILED")
    require(integer(rep["verified_reopens"])==len(ws) and integer(rep["successful_checkpoints"])==0 and rep["checkpoint_status"]=="skipped: rollback API has no WAL checkpoint", "ordinary maintenance accounting mismatch", "FAILED")
    require(integer(rep["final_encrypted_commit_sequence"])==0,"rollback has encrypted commit sequence")
    require(integer(rep["max_observed_data_files"],1)>0 and 0<integer(rep["max_observed_data_bytes"])<=16777216,"storage bound invalid")
    require(integer(rep["max_observed_heap_bytes"]) <= TRANSACTION_HEAP_LIMIT_BYTES,
            "sampled heap exceeds V4 fatal guard", "FAILED")
    for name,n in (("writer_attempt_latency",64),("reader_attempt_latency",rep["readers"]["attempts"]),("ordinary_checkpoint_latency",0),("ordinary_reopen_latency",len(ws))):histogram(rep[name],n)
    # Per-worker samples are sequential and entirely inside joined active
    # windows. Writer and reader overlap: never sum the two worker arms.
    def sample_seconds(name):
        h=rep[name]["milliseconds"]
        return number(h["mean"]*h["sample_count"]/1000)
    time_bound(sample_seconds("writer_attempt_latency"), seconds,
               "sequential writer samples exceed active time")
    time_bound(sample_seconds("reader_attempt_latency"), seconds*TRANSACTIONS_CONFIG["readers"],
               "sequential reader samples exceed active time")
    # The fixed rollback profile has no checkpoint samples; its reopens occur
    # after workers join. Bound maintenance by the non-active report interval,
    # not by the active window. Histograms do not retain per-window latencies.
    maintenance=sample_seconds("ordinary_checkpoint_latency")+sample_seconds("ordinary_reopen_latency")
    time_bound(maintenance, max(0, rep["elapsed_seconds"]-seconds),
               "ordinary maintenance samples exceed non-active elapsed time")
    distribution(rep["window_commit_rate_statistics"],[w["committed_transactions_per_second"] for w in ws])
    cfg=dict(TRANSACTIONS_CONFIG,source_id="",hardware_id="")
    expected_workload=sha(json.dumps({"Version":"ordinary-transfer-history-v1","Config":cfg},separators=(",", ":")).encode())
    require(rep["workload_sha256"]==expected_workload,"transaction workload digest/config mismatch")

def validate_report(rep, kind):
    require(isinstance(rep,dict),"report must be an object"); recursive_finite(rep)
    require(type(rep.get("schema_version")) is int and rep["schema_version"]==1,"unsupported typed report schema","UNSUPPORTED")
    keys(rep,"benchmark workload_version status errors config started_utc finished_utc elapsed_seconds go_version goos goarch gomaxprocs page_size cache_condition workload_sha256 notes".split())
    require(rep["status"]==("pass" if kind=="grams" else "passed") and rep["errors"]==[] and not rep.get("error"),"report failed","FAILED")
    if kind=="transactions" and rep.get("harness_revision")!="ordinary-transactions-v4":
        raise EvidenceError("UNSUPPORTED","unsupported transaction implementation revision")
    config(rep,kind); a,b=period(rep)
    elapsed=finite(rep["elapsed_seconds"])
    require(abs((b-a)/1e9-elapsed)<=max(.002,elapsed*1e-4),"report timestamps and elapsed time disagree")
    require(type(rep["gomaxprocs"]) is int and rep["gomaxprocs"]==2 and type(rep["page_size"]) is int and rep["page_size"]==4096,"runtime/page profile mismatch")
    integer(rep["cpus" if kind=="grams" else "logical_cpus"],1)
    require(rep["cache_condition"]==CACHE[kind],"unsupported cache protocol")
    require(isinstance(rep["notes"],list) and all(type(x) is str for x in rep["notes"]),"invalid notes")
    for k in ("workload_sha256",):digest(rep[k])
    if kind=="transactions":validate_transactions(rep)
    else:
        keys(rep,"source_id hardware_id dataset_seed format source_utf8_text_bytes trials index_build_latency_across_repeats go_build_settings modules".split())
        require(type(rep["dataset_seed"]) is int and rep["dataset_seed"]==2026101052 and rep["format"]=="gosqlite-grams-benchmark-v1","invalid grams dataset/format")
        require(rep["workload_sha256"]=="004eef36b55aa98909f1dbe5f6eab0c074b0126eea4b270f0c1c22f7de96c63c" and integer(rep["source_utf8_text_bytes"])==4288,"fixed grams corpus mismatch")
        require(isinstance(rep["trials"],list) and len(rep["trials"])==2,"missing grams trials")
        distribution(rep["index_build_latency_across_repeats"],[t["index_build_ms"] for t in rep["trials"]],True)
        measured=sum(t["source_insert_ms"]+t["index_build_ms"]+sum(r["automatic_ms"]+r["real_scan_ms"] for sample in t["samples"] for r in sample["observations"]) for t in rep["trials"])
        require(measured <= rep["elapsed_seconds"]*1000+2,"sequential measured grams operations exceed elapsed time")
        for t in rep["trials"]:
            keys(t,"repeat source_insert_ms index_build_ms database_bytes_before_index database_bytes_with_index incremental_database_bytes database_with_index_over_before incremental_bytes_over_utf8_text_bytes database_sha256_after_build_close extension_consistency_before_close extension_consistency_after_reopen samples".split())
            integer(t["repeat"],1); finite(t["source_insert_ms"]); finite(t["index_build_ms"])
            before=integer(t["database_bytes_before_index"],1); after=integer(t["database_bytes_with_index"],1); extra=integer(t["incremental_database_bytes"],1)
            require(after-before==extra and before%4096==after%4096==extra%4096==0,"grams page-aligned space difference mismatch")
            close(t["database_with_index_over_before"],after/before,"grams amplification")
            close(t["incremental_bytes_over_utf8_text_bytes"],extra/4288,"grams source bytes amplification")
            digest(t["database_sha256_after_build_close"])
            for s in t["samples"]:
                keys(s,"cache_condition gram_length_0_is_fallback_control automatic_query_latency real_scan_query_latency actual_grams_search_queries actual_scan_fallback_queries query_text_membership_comparisons observations".split())
                integer(s["gram_length_0_is_fallback_control"]); integer(s["actual_grams_search_queries"]); integer(s["actual_scan_fallback_queries"])
                require(integer(s["query_text_membership_comparisons"])==64*len(s["observations"]),"grams membership count mismatch")
                distribution(s["automatic_query_latency"],[r["automatic_ms"] for r in s["observations"]],True)
                distribution(s["real_scan_query_latency"],[r["real_scan_ms"] for r in s["observations"]],True)
                for r in s["observations"]:
                    keys(r,"query automatic_ms real_scan_ms automatic_plan scan_plan automatic_work scan_work full_ordered_results_equal result_sha256".split())
                    finite(r["automatic_ms"]);finite(r["real_scan_ms"]);digest(r["result_sha256"])
                    keys(r["query"],["gram_length","pattern","require_real_search"])
                    require(type(r["query"]["pattern"]) is str and type(r["query"]["require_real_search"]) is bool,"invalid query shape")
                    length=s["gram_length_0_is_fallback_control"]
                    require(type(r["query"]["gram_length"]) is int and r["query"]["gram_length"]==length and
                            r["query"]["require_real_search"] is (length!=0),"query/bucket mismatch")
                    auto,scan=r["automatic_plan"],r["scan_plan"]
                    require(isinstance(auto,dict) and isinstance(scan,dict) and scan.get("Access")=="SCAN" and scan.get("Reason")=="NOT INDEXED requested", "real scan plan missing", "FAILED")
                    require(auto.get("Access")==("SCAN" if length==0 else "SEARCH"),"required actual search/fallback plan missing","FAILED")
                    require(auto.get("Table") == scan.get("Table") == "docs", "grams plan targets wrong table", "FAILED")
                    for plan in (auto,scan):
                        require(type(plan.get("MethodVersion")) is int, "grams method version must be an integer")
                        if plan["Access"] == "SCAN":
                            require(plan.get("Method") == "" and plan["MethodVersion"] == 0 and
                                    plan.get("Indexes") in (None, []) and plan.get("Fields") in (None, []) and
                                    plan.get("GramLengths") in (None, []) and plan.get("Namespace") == "" and
                                    plan.get("Recheck") is False, "SCAN advertises an indexed method", "FAILED")
                    if length:
                        require(auto.get("Method")=="grams13" and auto["MethodVersion"]==1 and
                                auto.get("Indexes")==["docs_grams"] and auto.get("Fields")==["title"] and
                                auto.get("GramLengths")==[length] and all(type(x) is int for x in auto["GramLengths"]) and
                                auto.get("Namespace")=="LIKE_RAW_V1" and auto.get("Recheck") is True,"actual grams path differs","FAILED")
                    aw,sw=r["automatic_work"],r["scan_work"]
                    for work in (aw,sw):
                        require(isinstance(work,dict) and set(work)==set("frequency_lookups posting_keys_read candidates base_row_fetches returned_rows".split()),"invalid work counter schema")
                        for v in work.values():integer(v)
                        # This harness consumes all rows from one docs.title LIKE
                        # SELECT with no join/LIMIT/OFFSET/aggregate. Not a general
                        # invariant for arbitrary SQL or early-stopped iterators.
                        require(work["returned_rows"] <= work["base_row_fetches"] <= GRAMS_CONFIG["rows"],
                                "grams returned/fetched row bounds impossible")
                    require(sw["base_row_fetches"]==64 and all(sw[k]==0 for k in
                            ("frequency_lookups","posting_keys_read","candidates")), "reference is not a real full scan", "FAILED")
                    require(sw["returned_rows"]==aw["returned_rows"],"scan/result work mismatch")
                    require(aw["frequency_lookups"] <= 128, "grams planner frequency probe budget exceeded")
                    if length:
                        require(aw["frequency_lookups"] > 0 and aw["posting_keys_read"] > 0 and
                                0 < aw["candidates"] == aw["base_row_fetches"] < sw["base_row_fetches"] and
                                aw["returned_rows"] > 0, "required selective SEARCH work missing", "FAILED")
                    else:
                        reason=auto.get("Reason")
                        require(type(reason) is str and bool(reason) and aw["base_row_fetches"]==sw["base_row_fetches"] and
                                (r["query"]["pattern"]!="%common%" or "frequency" in reason),
                                "fallback is not an explained full scan", "FAILED")
                        # A fallback can retain planning or abandoned posting work;
                        # do not require zero automatic frequency/posting counters.
