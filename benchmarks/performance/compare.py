#!/usr/bin/env python3
"""Strict, offline comparison of paired ordinary native benchmark invocations."""
import argparse
import hashlib
import json
import math
from pathlib import Path
import stat
import statistics

POLICY = "ordinary-performance-v1"
# Engineering alert margins, NOT original goal SLOs. Larger ratios are worse.
MARGINS = {"grams_build_ms": 1.30, "grams_index_bytes": 1.10,
           "transaction_seconds_per_commit": 1.0 / 0.75}
FAMILY_CONFIDENCE = 0.95
MIN_PAIRS = 8
MAX_PAIRS = 32
MAX_JSON_BYTES = 32 * 1024 * 1024
HASH_CHUNK_BYTES = 64 * 1024
EXITS = {"PASS": 0, "REGRESSION": 2, "FAILED": 3, "INADMISSIBLE": 3,
         "MISSING": 4, "UNSUPPORTED": 4, "INCONCLUSIVE": 4}

from validation import (EvidenceError, require, canonical, sha, finite, integer, number, digest,
                        relative, timestamp, period, recursive_finite, validate_report,
                        GRAMS_CONFIG, TRANSACTIONS_CONFIG, GRAMS_ARGS, TRANSACTIONS_ARGS, BUILD_ENV)
import uuid

def unique_object(pairs):
    obj = {}
    for key, value in pairs:
        if key in obj:
            raise ValueError("duplicate JSON object key: " + key)
        obj[key] = value
    return obj

def strict_float(text):
    value = float(text)
    if not math.isfinite(value):
        raise ValueError("nonfinite JSON exponent")
    return value

def read_json_bytes(path):
    """Read one bounded snapshot of an operator-supplied regular JSON file."""
    try:
        require(stat.S_ISREG(path.stat().st_mode), "JSON evidence must be a regular file")
        with path.open("rb") as stream:
            data = stream.read(MAX_JSON_BYTES + 1)
        require(len(data) <= MAX_JSON_BYTES, "JSON evidence exceeds bounded 32MiB input")
        return data
    except FileNotFoundError as exc:
        raise EvidenceError("MISSING", str(exc)) from exc
    except OSError as exc:
        raise EvidenceError("INADMISSIBLE", f"cannot read JSON evidence: {path}: {exc}") from exc

def parse_json(data, path):
    try:
        return json.loads(data.decode("utf-8"), object_pairs_hook=unique_object, parse_float=strict_float, parse_constant=lambda s: (_ for _ in ()).throw(ValueError(s)))
    except (ValueError, UnicodeError, RecursionError) as exc:
        raise EvidenceError("INADMISSIBLE", f"invalid JSON: {path}: {exc}") from exc

def read_json(path, expected_sha256=None):
    data = read_json_bytes(path)
    if expected_sha256 is not None:
        require(sha(data) == expected_sha256, "JSON artifact digest mismatch")
    return parse_json(data, path)

def hash_artifact(path):
    """Hash regular artifact bytes with bounded allocation, not bounded total I/O."""
    try:
        require(stat.S_ISREG(path.stat().st_mode), "artifact must be a regular file")
        hasher = hashlib.sha256()
        with path.open("rb") as stream:
            while chunk := stream.read(HASH_CHUNK_BYTES):
                hasher.update(chunk)
        return hasher.hexdigest()
    except FileNotFoundError as exc:
        raise EvidenceError("MISSING", str(exc)) from exc
    except OSError as exc:
        raise EvidenceError("INADMISSIBLE", f"cannot read artifact: {path}: {exc}") from exc

def median_interval(ratios, confidence):
    """Exact distribution-free two-sided order-statistic interval for median.

    Requires independent identically distributed pair ratios. Ties are conservative.
    No normality approximation, bootstrap RNG, or within-invocation pseudoreplication.
    """
    values = sorted(finite(x) for x in ratios)
    n = len(values)
    require(n > 0 and 0 < confidence < 1, "invalid interval input")
    best = None
    for k in range(1, n // 2 + 1):
        coverage = 1 - 2 * sum(math.comb(n, j) for j in range(k)) / (2 ** n)
        if coverage >= confidence:
            best = {"lower": values[k - 1], "upper": values[n - k],
                    "coverage_at_least": coverage, "order_statistic_k": k}
    return best

def reference(root, ref, seen):
    require(isinstance(ref, dict), "report reference must be an object")
    require(type(ref.get("exit_code")) is int and ref["exit_code"] == 0, "benchmark process did not complete successfully", "FAILED")
    name = ref["path"]
    require(type(name) is str and name and not Path(name).is_absolute(), "relative report path required")
    p = root / name
    require(".." not in Path(name).parts and not p.is_symlink() and p.resolve().is_relative_to(root.resolve()),
            "report path escapes evidence directory")
    require(str(p.resolve()) not in seen, "reused invocation report path")
    seen.add(str(p.resolve()))
    data = read_json_bytes(p)
    digest = sha(data)
    require(digest == ref["sha256"], "report digest mismatch")
    require(digest not in seen, "duplicate invocation bytes cannot be independent samples")
    seen.add(digest)
    return parse_json(data, p)

def normalize(rep, kind, source, hardware):
    validate_report(rep, kind)
    require(rep.get("schema_version") == 1, "unsupported benchmark schema", "UNSUPPORTED")
    require(rep.get("status") == ("pass" if kind == "grams" else "passed") and
            rep.get("errors") == [] and not rep.get("error"), "benchmark correctness/status failed", "FAILED")
    cfg = dict(rep["config"])
    require(cfg.pop("source_id") == source and cfg.pop("hardware_id") == hardware,
            "report source or hardware identity mismatch")
    require(rep["go_version"] == "go1.27.2", "unrecognized pinned Go version", "UNSUPPORTED")
    require(rep["page_size"] == 4096 and rep["gomaxprocs"] == 2, "unexpected page size or GOMAXPROCS")
    require(rep.get("started_utc") and rep.get("finished_utc"), "incomplete invocation timestamps")
    if kind == "grams":
        require(rep["benchmark"] == "grams" and rep["workload_version"] == "grams-nfc-selectivity-v1",
                "unknown grams workload", "UNSUPPORTED")
        require(rep["source_id"] == source and rep["hardware_id"] == hardware, "top-level grams identity mismatch")
        settings = {x["Key"]: x["Value"] for x in rep["go_build_settings"]}
        trials = rep["trials"]
        require(len(trials) == cfg["independent_database_builds"] and len(trials) >= 2, "incomplete grams builds")
        signatures = []
        for number, trial in enumerate(trials, 1):
            require(trial["repeat"] == number and trial["extension_consistency_before_close"] is True and
                    trial["extension_consistency_after_reopen"] is True, "grams integrity check failed", "FAILED")
            samples = trial["samples"]
            require(len(samples) == 8, "missing grams cache/length bucket")
            buckets = set()
            sig = []
            for sample in samples:
                cache, length = sample["cache_condition"], sample["gram_length_0_is_fallback_control"]
                require(cache in ("cold_connection", "warm_process") and length in (0, 1, 2, 3), "unknown grams bucket")
                require((cache, length) not in buckets, "duplicate grams bucket")
                buckets.add((cache, length))
                obs = sample["observations"]
                expected = 2 if length == 0 else cfg["queries_per_gram_length"]
                require(len(obs) == expected, "missing grams observations")
                require(sample["actual_grams_search_queries"] == (0 if length == 0 else expected) and
                        sample["actual_scan_fallback_queries"] == (expected if length == 0 else 0), "missing required grams search path", "FAILED")
                for row in obs:
                    require(row["full_ordered_results_equal"] is True, "grams result mismatch", "FAILED")
                    require(len(row["result_sha256"]) == 64, "missing grams result digest")
                sig.append((cache, length, [(r["query"], r["result_sha256"]) for r in obs]))
            signatures.append(sorted(sig))
        require(all(x == signatures[0] for x in signatures), "grams results differ across independent builds", "FAILED")
        metrics = {"grams_build_ms": statistics.mean(finite(t["index_build_ms"]) for t in trials),
                   "grams_index_bytes": statistics.mean(finite(t["incremental_database_bytes"]) for t in trials)}
        result_signature = sha(canonical(signatures[0]))
        cpus = rep["cpus"]
        dataset = rep["workload_sha256"]
        extra = {"source_utf8_text_bytes": rep["source_utf8_text_bytes"], "results": result_signature}
    else:
        require(rep["benchmark"] == "native-transactions-and-soak" and rep["workload_version"] == "ordinary-transfer-history-v1",
                "unknown transaction workload", "UNSUPPORTED")
        require(cfg["mode"] == "throughput" and cfg["profile"] == "rollback", "only fixed rollback throughput admitted", "UNSUPPORTED")
        require(all(k in rep for k in ("close_failures", "unverified_close_handles", "unverified_retirement")),
                "transaction report predates strict closure accounting", "UNSUPPORTED")
        require(type(rep["close_failures"]) is int and rep["close_failures"] == 0 and
                type(rep["unverified_close_handles"]) is int and rep["unverified_close_handles"] == 0 and
                rep["unverified_retirement"] is False, "transaction closure uncertainty is fatal", "FAILED")
        require(rep["final_model_verified"] is True and rep["shutdown"] == "workers_joined_normal_close",
                "transaction final model or normal closure failed", "FAILED")
        require(rep["writer"]["attempts"] == cfg["requested_writer_attempts"] and
                rep["writer"]["failed_outcomes"] == 0 and rep["readers"]["failed_outcomes"] == 0,
                "transaction attempts incomplete or failed", "FAILED")
        require(rep["verified_reopens"] > 0, "transaction reopen verification missing", "FAILED")
        rate = finite(rep["committed_transactions_per_active_second"])
        expected_rate = finite(rep["writer"]["committed"]) / finite(rep["active_seconds"])
        require(math.isclose(rate, expected_rate, rel_tol=1e-12), "transaction rate does not match raw counts")
        metrics = {"transaction_seconds_per_commit": 1 / rate}
        cpus, settings = rep["logical_cpus"], rep["build_settings"]
        dataset = rep["dataset_sha256"]
        extra = {"workload_sha256": rep["workload_sha256"]}
    require(settings.get("CGO_ENABLED") == "0" and settings.get("-trimpath") == "true" and
            settings.get("-race", "false") != "true", "benchmark build mode must be pure Go trimpath without race")
    require(type(dataset) is str and len(dataset) == 64, "missing dataset digest")
    identity = {"config": cfg, "workload_version": rep["workload_version"], "dataset": dataset,
                "go_version": rep["go_version"], "goos": rep["goos"], "goarch": rep["goarch"],
                "cpus": cpus, "gomaxprocs": rep["gomaxprocs"], "page_size": rep["page_size"],
                "cache": rep["cache_condition"], "settings": settings, "extra": extra}
    require(identity["cache"], "cache condition missing")
    return metrics, identity

def artifact(root, ref, *, json_record=False):
    require(isinstance(ref, dict) and set(ref) == {"path", "sha256"}, "malformed artifact reference")
    name = relative(ref["path"]); digest(ref["sha256"])
    p = root / name
    require(not p.is_symlink() and p.resolve().is_relative_to(root.resolve()), "artifact path escape/symlink")
    if json_record:
        return read_json(p, expected_sha256=ref["sha256"])
    require(hash_artifact(p) == ref["sha256"], "artifact digest mismatch: " + name)
    return p

def command_record(root, ref, session, source, expected_argv, bounds, binary=None):
    rec = artifact(root, ref, json_record=True)
    require(isinstance(rec, dict), "command record must be an object")
    require(rec["session_id"] == session and rec["cwd"] == source["root"] and
            rec["source_id"] == source["id"] and rec["command"] == expected_argv,
            "command/source/session/arguments mismatch")
    require(type(rec["exit_code"]) is int and rec["exit_code"] == 0, "command failed", "FAILED")
    require(rec["build_environment"] == BUILD_ENV, "uncontrolled build environment")
    a,b = period(rec); require(bounds[0] <= a <= b <= bounds[1], "command outside session chronology")
    if binary is not None:
        require(rec["executed_binary_sha256"] == binary["sha256"], "executed binary identity mismatch")
    for stream in ("stdout", "stderr"):
        artifact(root, rec[stream])
    return rec, (a,b)

def source_check(source):
    require(isinstance(source, dict) and set(source) == {"id","manifest","root","verified_after"}, "incomplete source identity")
    require(type(source["root"]) is str and Path(source["root"]).is_absolute(), "absolute recorded source root required")
    manifest = source["manifest"]
    require(isinstance(manifest, dict) and manifest, "missing source manifest")
    for p,v in manifest.items():
        relative(p)
        require(isinstance(v,dict) and set(v)=={"sha256","mode"} and v["mode"] in ("100644","100755"), "invalid source entry")
        digest(v["sha256"])
    sid="sha256:"+sha(canonical(manifest))
    require(source["id"]==source["verified_after"]==sid, "source manifest/post-run identity mismatch")
    require("go.mod" in manifest and "go.sum" in manifest, "module identities missing")
    return {p:v for p,v in manifest.items() if p.endswith(".go") and
            (p.startswith("benchmarks/grams/") or p.startswith("benchmarks/transactions/"))}

def compare(bundle, root):
    require(isinstance(bundle,dict), "comparison bundle must be an object")
    recursive_finite(bundle)
    require(type(bundle.get("schema_version")) is int and bundle["schema_version"] == 2 and bundle.get("policy") == POLICY,
            "unsupported comparison schema/policy", "UNSUPPORTED")
    require(bundle.get("status") != "failed", bundle.get("reason", "runner failed"), "FAILED")
    require(bundle.get("status") in ("complete", "bootstrap"), "unfinished runner evidence", "MISSING")
    bootstrap = bundle["status"] == "bootstrap"
    arms = ("candidate",) if bootstrap else ("baseline","candidate")
    session = bundle["session_id"]
    require(type(session) is str and str(uuid.UUID(session))==session, "invalid session UUID")
    bounds = period(bundle)
    recorded_root=Path(bundle["recorded_evidence_root"])
    require(recorded_root.is_absolute(),"absolute recorded evidence root required")
    require(bundle["profile"] == {"grams":GRAMS_ARGS,"transactions":TRANSACTIONS_ARGS}, "runner workload flags mismatch")
    env=bundle["environment"]
    require(isinstance(env,dict) and type(env["controlled"]) is bool and type(env["runner_id"]) is str and env["runner_id"], "host identity missing")
    require(type(env["control_reason"]) is str and (not env["controlled"] or env["control_reason"].strip()), "controlled environment needs explicit attestation")
    observed=env["observed"]
    require(isinstance(observed,dict) and set(observed)==set("system release machine cpu_model logical_cpus affinity storage".split()), "incomplete hardware description")
    integer(observed["logical_cpus"],1)
    for k in ("system","release","machine","cpu_model","storage"):
        require(type(observed[k]) is str and observed[k], "missing hardware description")
    require(observed["affinity"] is None or (isinstance(observed["affinity"],list) and observed["affinity"] and
            all(type(x) is int and x>=0 for x in observed["affinity"]) and len(set(observed["affinity"]))==len(observed["affinity"])), "invalid affinity")
    require(env["hardware_id"]=="sha256:"+sha(canonical(observed)), "hardware fingerprint mismatch")
    sources=bundle["sources"]; harnesses=bundle["harnesses"]
    for arm in arms:
        selected=source_check(sources[arm])
        require(selected and {"benchmarks/grams/main.go","benchmarks/transactions/main.go"}<=set(selected),"required harness missing")
        require(harnesses[arm]==sha(canonical(selected)),"harness digest does not match source manifest")
    if not bootstrap:
        require(harnesses["baseline"]==harnesses["candidate"],"harnesses differ","UNSUPPORTED")
        for name in ("go.mod","go.sum"):
            require(sources["baseline"]["manifest"][name]==sources["candidate"]["manifest"][name],"dependency identity changed","UNSUPPORTED")
    ready={}
    for arm in arms:
        source=sources[arm]; binaries=bundle["binaries"][arm]; commands=bundle["build_commands"][arm]
        require(set(binaries)=={"grams","transactions"} and set(commands)=={"guard","grams","transactions"},"incomplete binaries/build provenance")
        _,guard=command_record(root,commands["guard"],session,source,["go","run","./internal/testscope"],bounds)
        ready[arm]=guard[1]
        for kind in ("grams","transactions"):
            artifact(root,binaries[kind])
            binary_abs=str(recorded_root/binaries[kind]["path"])
            _,times=command_record(root,commands[kind],session,source,["go","build","-trimpath","-o",binary_abs,"./benchmarks/"+kind],bounds)
            require(ready[arm]<=times[0],"build precedes guard/earlier build")
            ready[arm]=times[1]
    pairs=bundle["pairs"]
    require(type(bundle["requested_pairs"]) is int and 1<=bundle["requested_pairs"]<=MAX_PAIRS and isinstance(pairs,list),"invalid requested pair count")
    require(len(pairs)==(1 if bootstrap else bundle["requested_pairs"]),"missing paired invocations","MISSING")
    seen,identities,ratios=set(),{}, {k:[] for k in MARGINS}
    last=max(ready.values())
    for i,pair in enumerate(pairs,1):
        order=["candidate"] if bootstrap else (["baseline","candidate"] if i%2 else ["candidate","baseline"])
        require(type(pair["pair"]) is int and pair["pair"]==i and pair["order"]==order and set(pair)=={"pair","order"}|set(order),"paired order/arms must alternate without dropped runs")
        values={}
        for arm in order:
            require(set(pair[arm])=={"grams","transactions"},"incomplete benchmark pair")
            values[arm]={}
            for kind,flags in (("grams",GRAMS_ARGS),("transactions",TRANSACTIONS_ARGS)):
                ref=pair[arm][kind]
                require(isinstance(ref,dict) and set(ref)=={"path","sha256","exit_code","command"},"incomplete invocation reference")
                rep=reference(root,ref,seen)
                binary=bundle["binaries"][arm][kind]
                argv=[str(recorded_root/binary["path"])]+flags+["-source-id",sources[arm]["id"],"-environment" if kind=="grams" else "-hardware-id",env["hardware_id"]]
                rec,times=command_record(root,ref["command"],session,sources[arm],argv,bounds,binary)
                require(rec["stdout"]=={"path":ref["path"],"sha256":ref["sha256"]} and last<=times[0],"invocation output/chronology mismatch")
                a,b=period(rep); require(times[0]<=a<=b<=times[1],"report outside its process interval")
                last=times[1]
                metrics,identity=normalize(rep,kind,sources[arm]["id"],env["hardware_id"])
                require(identity["cpus"]==(len(observed["affinity"]) if observed["affinity"] is not None else observed["logical_cpus"]),"runtime CPU identity differs from observed hardware affinity")
                require(identity["goos"]=={"Linux":"linux","Darwin":"darwin","Windows":"windows"}.get(observed["system"]) and
                        identity["goarch"]=={"x86_64":"amd64","AMD64":"amd64","aarch64":"arm64","arm64":"arm64"}.get(observed["machine"]),"runtime platform differs from observed hardware")
                key=canonical(identity)
                if kind in identities:require(identities[kind]==key,f"{kind} data/config/results/toolchain/cache/host identity mismatch")
                identities[kind]=key; values[arm].update(metrics)
        if not bootstrap:
            for name in MARGINS:ratios[name].append(values["candidate"][name]/values["baseline"][name])
    if bootstrap:
        return {"status":"UNSUPPORTED","reason":"validated candidate-only bootstrap; historical harness unavailable or incompatible; no performance PASS","policy":POLICY,"pairs":0,"bootstrap_validated":True}
    confidence = 1 - (1 - FAMILY_CONFIDENCE) / len(MARGINS)
    details = {}
    decisions = []
    for name, values in ratios.items():
        interval = median_interval(values, confidence)
        decision = "INCONCLUSIVE"
        if len(values) >= MIN_PAIRS and interval is not None and env["controlled"]:
            if interval["lower"] > MARGINS[name]:
                decision = "REGRESSION"
            elif interval["upper"] <= MARGINS[name]:
                decision = "PASS"
        decisions.append(decision)
        details[name] = {"paired_ratios_candidate_over_baseline_cost": values,
                         "median_ratio": statistics.median(values), "ratio_sample_stdev": statistics.stdev(values) if len(values) > 1 else None,
                         "engineering_limit": MARGINS[name], "decision": decision,
                         "descriptive_regression_signal": statistics.median(values) > MARGINS[name],
                         "conditional_median_interval": interval,
                         "confidence_applicable": env["controlled"] and len(values) >= MIN_PAIRS}
    status = "REGRESSION" if "REGRESSION" in decisions else ("PASS" if all(x == "PASS" for x in decisions) else "INCONCLUSIVE")
    reason = "all admitted median cost ratios bounded within engineering margins" if status == "PASS" else (
        "confidence-qualified engineering regression" if status == "REGRESSION" else
        "insufficient independent pairs, uncontrolled host, or interval overlaps a margin")
    return {"status": status, "reason": reason, "policy": POLICY, "pairs": len(pairs),
            "family_confidence_at_least": FAMILY_CONFIDENCE, "metrics": details,
            "same_source": sources["baseline"]["id"] == sources["candidate"]["id"],
            "limitations": ["Coverage assumes independent identically distributed whole-invocation pair ratios; temporal drift and co-tenants can invalidate it.",
                            "Conditional intervals on uncontrolled hosts are descriptive calculations, not calibrated confidence or controlled wins.",
                            "Within-invocation builds and query repetitions are aggregated, never counted as independent pairs.",
                            "PASS is bounded non-regression against declared engineering margins, not equivalence, a speedup, original SLO acceptance, or full release acceptance."]}

def evaluate(path):
    try:
        return compare(read_json(path), path.parent)
    except EvidenceError as exc:
        return {"status": exc.status, "reason": str(exc), "policy": POLICY}
    except (KeyError, TypeError, ValueError, IndexError, OSError, OverflowError, AttributeError) as exc:
        return {"status": "INADMISSIBLE", "reason": f"malformed evidence: {exc}", "policy": POLICY}

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("bundle", type=Path)
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    result = evaluate(args.bundle)
    raw = json.dumps(result, indent=2, allow_nan=False) + "\n"
    if args.output:
        args.output.write_text(raw)
    print(raw, end="")
    return EXITS[result["status"]]

if __name__ == "__main__":
    raise SystemExit(main())
