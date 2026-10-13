#!/usr/bin/env python3
"""Offline, fixed-root admission for a manually dispatched evidence comparison.

Only reads stable operator-supplied files; never collects or executes evidence.
The origin declaration is an assertion, not authentication of measurements.
"""
import argparse
import json
import os
from pathlib import Path
import re
import stat
import sys

import compare as comparator

EVIDENCE_ROOT = Path("benchmarks/performance/evidence")
MAX_ENTRIES = 1024  # Files and directories combined, including origin.json.
MAX_FILE_BYTES = 64 * 1024 * 1024
MAX_TOTAL_BYTES = 256 * 1024 * 1024
MAX_ORIGIN_BYTES = 4096
OUTPUT_NAME = "ordinary-performance-comparison.json"


def evidence_path(repository, directory):
    """Admit one canonical relative directory, without following symlinks."""
    comparator.require(type(directory) is str and 0 < len(directory) <= 1024,
                       "expected a bounded relative evidence directory")
    parts = directory.split("/")
    comparator.require(len(parts) <= 16 and all(
        part not in ("", ".", "..") and len(part) <= 255 and
        re.fullmatch(r"[A-Za-z0-9_.-]+", part) for part in parts),
        "evidence directory must use relative ASCII path components")
    root = Path(repository)
    for part in (*EVIDENCE_ROOT.parts, *parts):
        root /= part
        comparator.require(stat.S_ISDIR(root.lstat().st_mode),
                           "evidence directory component must be a directory, not a symlink")
    return root


def inspect_tree(root):
    """Bound a static input tree before the existing comparator reads it.

    lstat/scandir checks are not a hostile concurrent-filesystem protocol.
    """
    pending = [root]
    entries = total = 0
    while pending:
        with os.scandir(pending.pop()) as children:
            for child in children:
                entries += 1
                comparator.require(entries <= MAX_ENTRIES, "evidence tree has too many entries")
                info = child.stat(follow_symlinks=False)
                if stat.S_ISDIR(info.st_mode):
                    pending.append(Path(child.path))
                    continue
                comparator.require(stat.S_ISREG(info.st_mode),
                                   "evidence entries must be regular files or directories; no symlinks")
                limit = comparator.MAX_JSON_BYTES if child.name.endswith(".json") else MAX_FILE_BYTES
                comparator.require(info.st_size <= limit, "evidence file exceeds byte limit")
                total += info.st_size
                comparator.require(total <= MAX_TOTAL_BYTES, "evidence tree exceeds total byte limit")


def admit_origin(root):
    path = root / "origin.json"
    comparator.require(stat.S_ISREG(path.lstat().st_mode), "origin must be a regular file")
    with path.open("rb") as stream:
        raw = stream.read(MAX_ORIGIN_BYTES + 1)
    comparator.require(len(raw) <= MAX_ORIGIN_BYTES, "origin declaration exceeds 4096 bytes")
    origin = comparator.parse_json(raw, path)
    comparator.require(isinstance(origin, dict) and set(origin) == {"schema_version", "origin"} and
                       type(origin["schema_version"]) is int and origin["schema_version"] == 1,
                       "expected origin declaration schema 1 with exactly schema_version and origin")
    comparator.require(origin["origin"] in ("synthetic_fixture", "operator_supplied_measurement"),
                       "unknown evidence origin")
    comparator.require(origin["origin"] == "operator_supplied_measurement",
                       "synthetic fixture evidence is not admitted by performance dispatch", "UNSUPPORTED")


def evaluate(repository, directory, *, evaluator=None):
    try:
        root = evidence_path(repository, directory)
        inspect_tree(root)
        admit_origin(root)
        bundle = root / "bundle.json"
        comparator.require(stat.S_ISREG(bundle.lstat().st_mode), "bundle must be a regular file")
        return (comparator.evaluate if evaluator is None else evaluator)(bundle)
    except comparator.EvidenceError as exc:
        return {"status": exc.status, "reason": str(exc), "policy": comparator.POLICY}
    except FileNotFoundError as exc:
        return {"status": "MISSING", "reason": str(exc), "policy": comparator.POLICY}
    except (OSError, ValueError, TypeError) as exc:
        return {"status": "INADMISSIBLE", "reason": f"invalid CI evidence: {exc}",
                "policy": comparator.POLICY}


def run(repository, directory, runner_temp, *, evaluator=None, stream=None):
    """Retain exact decision JSON and exit mapping; output errors fail closed."""
    temporary = Path(runner_temp)
    comparator.require(temporary.is_absolute() and temporary.is_dir() and
                       not temporary.is_symlink() and
                       not temporary.resolve().is_relative_to(Path(repository).resolve()),
                       "trusted RUNNER_TEMP must be an existing directory outside the checkout")
    result = evaluate(repository, directory, evaluator=evaluator)
    raw = json.dumps(result, indent=2, allow_nan=False) + "\n"
    # Exclusive creation also refuses an existing output symlink or hard link.
    # No evidence path is ever an output target. A write error prints no PASS.
    with (temporary / OUTPUT_NAME).open("x", encoding="utf-8") as output:
        output.write(raw)
    print(raw, end="", file=sys.stdout if stream is None else stream)
    return comparator.EXITS[result["status"]]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("evidence_directory", help="directory relative to benchmarks/performance/evidence/")
    args = parser.parse_args()
    return run(Path(__file__).resolve().parents[2], args.evidence_directory, os.environ["RUNNER_TEMP"])


if __name__ == "__main__":
    raise SystemExit(main())
