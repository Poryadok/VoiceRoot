"""PLAN A4 mandatory smoke cannot be replaced by sampled regression."""
import copy
import contextlib
import importlib.util
import io
import json
from pathlib import Path
import tempfile
import unittest

SPEC = importlib.util.spec_from_file_location("client_compatibility", Path(__file__).with_name("client_compatibility.py"))
gate = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(gate)


def candidate():
    # Synthetic exact tuples and hashes, not a supported-client declaration.
    return {"schema_version": 1, "source_sha": "a" * 40, "matrix_version": "fixture-v1",
            "regression_sampling_policy": "Full mandatory smoke; extended regression is unsampled.",
            "backends": {label: {"source_sha": char * 40, "artifact_sha256": char * 64}
                         for label, char in (("previous", "b"), ("current", "a"))},
            "clients": {platform: {label: {"version": version, "source_sha": char * 40,
                                           "artifact_sha256": char * 64}
                                   for label, version, char in (("previous", "1.0.0", "c"), ("current", "1.1.0", "d"))}
                        for platform in ("windows", "web")},
            "tuples": [{"id": "win-fixture", "platform": "windows", "os": "Windows",
                        "os_version": "10.0.26100.1", "arch": "x64"},
                       {"id": "web-fixture", "platform": "web", "os": "Linux-fixture",
                        "os_version": "6.8.0", "arch": "x64", "browser": "Chrome-fixture",
                        "browser_version": "123.0.0.1"}]}


def report(source):
    plan = gate.compile_suite(source)
    return {"schema_version": 1, "source_sha": source["source_sha"],
            "manifest_sha256": plan["manifest_sha256"],
            "observations": [{"case_id": case["case_id"], "result": "PASS",
                              "evidence_uri": "artifact://fixture/" + case["case_id"],
                              "evidence_sha256": "e" * 64} for case in plan["cases"]]}


class CompatibilityTests(unittest.TestCase):
    def test_all_tuples_receive_full_crossproduct_and_rollback(self):
        plan = gate.compile_suite(candidate())
        for tuple_id in ("win-fixture", "web-fixture"):
            smoke = [c for c in plan["cases"] if c["tuple_id"] == tuple_id and c["check"] == "smoke"]
            self.assertEqual({(c["client"], c["backend"]) for c in smoke},
                             {("previous", "previous"), ("previous", "current"),
                              ("current", "previous"), ("current", "current")})
            rollback = [c for c in plan["cases"] if c["tuple_id"] == tuple_id and c["check"] == "backend_rollback"]
            self.assertEqual(len(rollback), 1)
            self.assertEqual(rollback[0]["client"], "current")
            self.assertEqual(rollback[0]["backend_sequence"], ["current", "previous"])
            self.assertEqual(rollback[0]["client_sequence"], ["previous", "current"])
        self.assertEqual(len(plan["cases"]), 22)
        self.assertEqual(plan, gate.compile_suite(copy.deepcopy(candidate())))

    def test_windows_version_flow_is_checked_against_each_backend_and_client(self):
        cases = gate.compile_suite(candidate())["cases"]
        for check in ("version", "forced_update", "update_url"):
            rows = [c for c in cases if c["check"] == check]
            self.assertEqual(len(rows), 4)
            self.assertTrue(all(c["tuple_id"] == "win-fixture" for c in rows))

    def test_no_missing_platform_empty_or_wildcard_tuple(self):
        mutations = [lambda s: s.update(tuples=[]), lambda s: s["tuples"].pop(),
                     lambda s: s["tuples"][1].update(browser_version="latest"),
                     lambda s: s["tuples"][0].update(arch="*"),
                     lambda s: s["tuples"][0].update(os="Linux"),
                     lambda s: s["tuples"][1].update(browser_version="123.x"),
                     lambda s: s["tuples"][0].update(os_version="rolling"),
                     lambda s: s["tuples"].append(copy.deepcopy(s["tuples"][0])),
                     lambda s: s["tuples"][1].pop("os_version"),
                     lambda s: s["tuples"][0].update(browser="unexpected"),
                     lambda s: s.update(regression_sampling_policy="")]
        for mutate in mutations:
            source = candidate()
            mutate(source)
            with self.subTest(mutate=mutate), self.assertRaises(ValueError):
                gate.compile_suite(source)

    def test_artifacts_and_external_source_are_not_labels(self):
        mutations = [lambda s: s["backends"]["current"].update(source_sha="f" * 40),
                     lambda s: s["clients"]["web"].update(previous=s["clients"]["web"]["current"]),
                     lambda s: s["backends"]["current"].update(artifact_sha256="0" * 64),
                     lambda s: s["clients"]["windows"]["previous"].update(version="latest"),
                     lambda s: s.update(schema_version=True)]
        for mutate in mutations:
            source = candidate()
            mutate(source)
            with self.subTest(mutate=mutate), self.assertRaises(ValueError):
                gate.compile_suite(source)

    def test_client_versions_follow_semver_prerelease_and_build_identifier_rules(self):
        accepted = ("0.0.0", "1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-0",
                    "1.0.0-01alpha", "1.0.0+001", "1.0.0+build.42",
                    "1.1.0-rc.1+build.42", "1.0.0-x-y-z.--+build-1.00")
        rejected = ("1.0.0-01", "1.0.0-..", "1.0.0-alpha.01", "1.0.0-",
                    "1.0.0-alpha..1", "1.0.0-.alpha", "1.0.0-alpha.",
                    "1.0.0+", "1.0.0+build..42", "1.0.0+build.", "1.0.0+.build",
                    "1.0.0+build+42", "1.0.0-alpha_beta", "01.0.0", "1.00.0",
                    "1.0.01", "1.0", "v1.0.0", "1.0.0\n", "1.0.0-١")
        for platform in ("windows", "web"):
            for label in ("previous", "current"):
                for version in accepted:
                    source = candidate()
                    source["clients"][platform][label]["version"] = version
                    with self.subTest(platform=platform, label=label, version=version):
                        self.assertEqual(len(gate.compile_suite(source)["cases"]), 22)
                for version in rejected:
                    source = candidate()
                    source["clients"][platform][label]["version"] = version
                    with self.subTest(platform=platform, label=label, version=version), self.assertRaises(ValueError):
                        gate.compile_suite(source)

    def test_every_case_requires_fresh_portable_content_bound_pass(self):
        source = candidate()
        self.assertEqual(gate.verify_results(source, report(source))["scope"], "offline evidence completeness only")
        mutations = [lambda r: r["observations"].pop(),
                     lambda r: r["observations"].append(copy.deepcopy(r["observations"][0])),
                     lambda r: r["observations"][0].update(result="SKIP"),
                     lambda r: r["observations"][0].update(result="FAIL"),
                     lambda r: r["observations"][0].update(evidence_uri="C:/tmp/local.json"),
                     lambda r: r["observations"][0].update(evidence_uri="https://secret@host/proof"),
                     lambda r: r["observations"][0].update(evidence_sha256="0" * 64),
                     lambda r: r["observations"][0].update(case_id="extra"),
                     lambda r: r.update(source_sha="b" * 40),
                     lambda r: r.update(manifest_sha256="f" * 64)]
        for mutate in mutations:
            result = report(source)
            mutate(result)
            with self.subTest(mutate=mutate), self.assertRaises(ValueError):
                gate.verify_results(source, result)

    def test_cli_source_binding_strict_json_and_non_echoing_errors(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "matrix.json"
            path.write_text(json.dumps(candidate()), encoding="utf8")
            output = io.StringIO()
            with contextlib.redirect_stdout(output):
                self.assertEqual(gate.main([str(path), "--expected-source-sha", "a" * 40]), 0)
            self.assertEqual(len(json.loads(output.getvalue())["cases"]), 22)
            for raw, source_sha in [(json.dumps(candidate()), "b" * 40),
                                    ('{"secret":"do-not-echo","secret":42}', "a" * 40),
                                    ('{"secret":NaN}', "a" * 40)]:
                path.write_text(raw, encoding="utf8")
                output, errors = io.StringIO(), io.StringIO()
                with contextlib.redirect_stdout(output), contextlib.redirect_stderr(errors):
                    self.assertEqual(gate.main([str(path), "--expected-source-sha", source_sha]), 2)
                self.assertEqual(output.getvalue(), "")
                self.assertNotIn("do-not-echo", errors.getvalue())

    def test_changed_tuple_artifact_or_policy_invalidates_observations(self):
        for mutate in [lambda s: s["tuples"][1].update(browser_version="124.0.0.1"),
                       lambda s: s["clients"]["web"]["current"].update(artifact_sha256="f" * 64),
                       lambda s: s.update(regression_sampling_policy="Extended regression sampled by documented risk.")]:
            source = candidate()
            result = report(source)
            mutate(source)
            with self.assertRaises(ValueError):
                gate.verify_results(source, result)


if __name__ == "__main__":
    unittest.main()
