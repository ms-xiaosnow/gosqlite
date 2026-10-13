"""Pure temporary-file CI admission tests; no collection, subprocess, or network."""
import io
import json
import os
from pathlib import Path
import stat
import tempfile
import unittest
from unittest import mock

import ci_input as ci


class CIInputTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.base = Path(temporary.name)
        self.repository = self.base / "repository"
        self.evidence = self.repository / ci.EVIDENCE_ROOT / "run-1"
        self.evidence.mkdir(parents=True)
        self.runner_temp = self.base / "runner"
        self.runner_temp.mkdir()
        self.origin = {"schema_version": 1, "origin": "operator_supplied_measurement"}
        self.write_origin(self.origin)
        (self.evidence / "bundle.json").write_text("{}", encoding="utf-8")
        # An injected verdict is only forwarding test data, never a measurement.
        self.evaluator = mock.Mock(return_value={"status": "PASS", "reason": "synthetic mock"})

    def write_origin(self, value):
        (self.evidence / "origin.json").write_text(json.dumps(value), encoding="utf-8")

    def evaluate(self, directory="run-1"):
        return ci.evaluate(self.repository, directory, evaluator=self.evaluator)

    def assert_rejected(self, status="INADMISSIBLE", directory="run-1"):
        self.assertEqual(self.evaluate(directory)["status"], status)
        self.evaluator.assert_not_called()

    def test_all_seven_verdicts_and_exact_exit_codes_are_forwarded(self):
        expected = {"PASS": 0, "REGRESSION": 2, "FAILED": 3, "INADMISSIBLE": 3,
                    "MISSING": 4, "UNSUPPORTED": 4, "INCONCLUSIVE": 4}
        self.assertEqual(ci.comparator.EXITS, expected)
        for status, code in expected.items():
            with self.subTest(status=status), tempfile.TemporaryDirectory(dir=self.runner_temp) as output:
                result = {"status": status, "reason": "mock only", "untouched": [1, {"x": 2}]}
                self.evaluator.return_value = result
                stdout = io.StringIO()
                actual = ci.run(self.repository, "run-1", output,
                                evaluator=self.evaluator, stream=stdout)
                self.assertEqual(actual, code)
                raw = (Path(output) / ci.OUTPUT_NAME).read_text(encoding="utf-8")
                self.assertEqual(stdout.getvalue(), raw)
                self.assertEqual(json.loads(raw), result)
                self.evaluator.assert_called_with(self.evidence / "bundle.json")

    def test_rejects_absolute_traversal_and_shell_like_input(self):
        for directory in ("", ".", "..", "/tmp", "../run-1", "run-1/../run-1",
                          "run-1/./x", "run-1//x", "run-1/", "C:/run-1", "run-1\\x",
                          "$(touch x)", "run-1; true", "run-1\nother", "run-1\x00", "é",
                          "a" * 256, "/".join(["x"] * 17)):
            with self.subTest(directory=directory):
                self.assert_rejected(directory=directory)

    def test_accepts_nested_relative_directory(self):
        nested = self.evidence.parent / "batch" / "run.2"
        nested.parent.mkdir()
        self.evidence.rename(nested)
        self.assertEqual(self.evaluate("batch/run.2")["status"], "PASS")
        self.evaluator.assert_called_once_with(nested / "bundle.json")

    def test_missing_directory_origin_or_bundle_is_missing(self):
        self.assert_rejected("MISSING", "absent")
        for name in ("origin.json", "bundle.json"):
            with self.subTest(name=name):
                path = self.evidence / name
                raw = path.read_bytes()
                path.unlink()
                self.assert_rejected("MISSING")
                path.write_bytes(raw)

    def test_symlink_directory_and_file_are_rejected(self):
        link = self.evidence.parent / "alias"
        link.symlink_to(self.evidence, target_is_directory=True)
        self.assert_rejected(directory="alias")
        extra = self.evidence / "link"
        for target in (self.evidence / "bundle.json", self.evidence, self.base / "missing"):
            with self.subTest(target=target):
                extra.symlink_to(target)
                self.assert_rejected()
                extra.unlink()

    def test_symlink_in_fixed_root_is_rejected(self):
        benchmarks = self.repository / "benchmarks"
        storage = self.base / "storage"
        benchmarks.rename(storage)
        benchmarks.symlink_to(storage, target_is_directory=True)
        self.assert_rejected()

    def test_required_files_cannot_be_directories(self):
        for name in ("origin.json", "bundle.json"):
            with self.subTest(name=name):
                path = self.evidence / name
                raw = path.read_bytes()
                path.unlink()
                path.mkdir()
                self.assert_rejected()
                path.rmdir()
                path.write_bytes(raw)

    def test_nonregular_entry_is_rejected_without_opening_it(self):
        child = mock.Mock()
        child.stat.return_value = mock.Mock(st_mode=stat.S_IFIFO)
        with mock.patch.object(ci.os, "scandir") as scan:
            scan.return_value.__enter__.return_value = iter([child])
            self.assert_rejected()
        child.stat.assert_called_once_with(follow_symlinks=False)

    def test_origin_labels_are_exact_untrusted_declarations(self):
        self.write_origin(dict(self.origin, origin="synthetic_fixture"))
        self.assert_rejected("UNSUPPORTED")
        for origin in ("PASS", "measured", "operator_supplied_measurement\n", None, True, [], {}):
            with self.subTest(origin=origin):
                self.write_origin(dict(self.origin, origin=origin))
                self.assert_rejected()
        for origin in ({}, [], {"schema_version": True, "origin": self.origin["origin"]},
                       dict(self.origin, schema_version=2), dict(self.origin, authenticated=True)):
            with self.subTest(declaration=origin):
                self.write_origin(origin)
                self.assert_rejected()

    def test_origin_strict_json_and_bounded_read(self):
        for raw in (b'{"schema_version":1,"origin":"operator_supplied_measurement","origin":"synthetic_fixture"}',
                    b'{"schema_version":NaN}', b'{"schema_version":1e309}', b'\xff', b'{',
                    b" " * (ci.MAX_ORIGIN_BYTES + 1)):
            with self.subTest(raw_prefix=raw[:40]):
                (self.evidence / "origin.json").write_bytes(raw)
                self.assert_rejected()

    def test_file_json_total_and_entry_limits(self):
        extra = self.evidence / "artifact"
        extra.write_bytes(b"x" * 9)
        with mock.patch.object(ci, "MAX_FILE_BYTES", 8):
            self.assert_rejected()
        (self.evidence / "bundle.json").write_bytes(b" " * 129)
        with mock.patch.object(ci.comparator, "MAX_JSON_BYTES", 128):
            self.assert_rejected()
        with mock.patch.object(ci, "MAX_TOTAL_BYTES", 1):
            self.assert_rejected()
        with mock.patch.object(ci, "MAX_ENTRIES", 2):
            self.assert_rejected()

    def test_io_error_is_inadmissible(self):
        with mock.patch.object(ci.os, "scandir", side_effect=PermissionError("unreadable")):
            self.assert_rejected()

    def test_real_comparator_handles_bundle_json_without_schema_changes(self):
        # No native fixtures, commands, or reports are executed by this test.
        result = ci.evaluate(self.repository, "run-1")
        self.assertEqual(result["status"], "UNSUPPORTED")
        self.assertEqual(result, ci.comparator.evaluate(self.evidence / "bundle.json"))
        (self.evidence / "bundle.json").write_bytes(b'{"x":1,"x":2}')
        self.assertEqual(ci.evaluate(self.repository, "run-1")["status"], "INADMISSIBLE")

    def test_full_synthetic_bundle_real_comparator_wiring(self):
        # Import is pure: fixture JSON, stdlib, compare/validation; no test run.
        import test_compare_offline as fixtures

        builder = fixtures.ComparisonTests()
        builder.root = self.evidence
        for cost, status, exit_code in ((1, "PASS", 0), (2, "REGRESSION", 2)):
            with self.subTest(cost=cost), tempfile.TemporaryDirectory(dir=self.runner_temp) as output:
                bundle = builder.bundle(n=8, cost=cost)
                path = self.evidence / "bundle.json"
                path.write_bytes(ci.comparator.canonical(bundle))
                self.write_origin(dict(self.origin, origin="synthetic_fixture"))
                self.assert_rejected("UNSUPPORTED")
                # Deliberately false declaration ONLY inside this temporary test
                # exercises wiring and shows why origin cannot authenticate data.
                self.write_origin(self.origin)
                expected = ci.comparator.evaluate(path)
                self.assertEqual(expected["status"], status)
                stdout = io.StringIO()
                self.assertEqual(ci.run(self.repository, "run-1", output, stream=stdout), exit_code)
                raw = json.dumps(expected, indent=2, allow_nan=False) + "\n"
                self.assertEqual(stdout.getvalue(), raw)
                self.assertEqual((Path(output) / ci.OUTPUT_NAME).read_text(encoding="utf-8"), raw)

    def test_missing_input_is_retained_and_exits_four(self):
        stdout = io.StringIO()
        code = ci.run(self.repository, "absent", self.runner_temp, stream=stdout)
        self.assertEqual(code, 4)
        self.assertEqual(json.loads(stdout.getvalue())["status"], "MISSING")

    def test_output_cannot_overwrite_evidence_or_an_existing_file(self):
        original = (self.evidence / "bundle.json").read_bytes()
        with self.assertRaises(ci.comparator.EvidenceError):
            ci.run(self.repository, "run-1", self.evidence, evaluator=self.evaluator)
        self.evaluator.assert_not_called()
        self.assertEqual((self.evidence / "bundle.json").read_bytes(), original)
        output = self.runner_temp / ci.OUTPUT_NAME
        output.write_text("keep", encoding="utf-8")
        stdout = io.StringIO()
        with self.assertRaises(FileExistsError):
            ci.run(self.repository, "run-1", self.runner_temp,
                   evaluator=self.evaluator, stream=stdout)
        self.assertEqual(output.read_text(encoding="utf-8"), "keep")
        self.assertEqual(stdout.getvalue(), "")
        output.unlink()
        output.symlink_to(self.evidence / "bundle.json")
        with self.assertRaises(FileExistsError):
            ci.run(self.repository, "run-1", self.runner_temp,
                   evaluator=self.evaluator, stream=stdout)
        self.assertEqual((self.evidence / "bundle.json").read_bytes(), original)

    def test_cli_uses_trusted_temp_and_propagates_nonzero_without_subprocess(self):
        self.evaluator.return_value = {"status": "REGRESSION", "reason": "mock only"}
        with mock.patch.object(ci, "__file__", str(self.repository / "benchmarks/performance/ci_input.py")), \
                mock.patch.object(ci.sys, "argv", ["ci_input.py", "--", "run-1"]), \
                mock.patch.dict(os.environ, RUNNER_TEMP=str(self.runner_temp)), \
                mock.patch.object(ci.comparator, "evaluate", self.evaluator), \
                mock.patch.object(ci.sys, "stdout", io.StringIO()) as stdout:
            self.assertEqual(ci.main(), 2)
            self.assertEqual(json.loads(stdout.getvalue())["status"], "REGRESSION")


if __name__ == "__main__":
    unittest.main()
