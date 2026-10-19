"""A4/G1 release claims require complete, source-bound negative observations."""
import copy
import contextlib
import importlib.util
import io
import json
from pathlib import Path
import tempfile
import unittest

SPEC = importlib.util.spec_from_file_location("capability_manifest", Path(__file__).with_name("capability_manifest.py"))
manifest = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(manifest)


def candidate():
    return {
        "schema_version": 1,
        "profile": "free-invite-alpha",
        "source_sha": "a" * 40,
        "configuration": {"environment": "production", "checkout_mode": "live", "development_defaults": False},
        "capabilities": {name: "disabled" for name in manifest.ALPHA_CAPABILITIES},
        "inventories": {name: ["gateway", "auth"] for name in manifest.INVENTORIES},
    }


def results(suite):
    return {"schema_version": 1, "source_sha": suite["source_sha"],
            "manifest_sha256": suite["manifest_sha256"], "observations": [
                {"case_id": case["case_id"], "result": "PASS", "evidence_uri": "artifact://run/" + case["case_id"]}
                for case in suite["cases"]]}


class CapabilityManifestTests(unittest.TestCase):
    def test_disabled_entries_generate_all_three_surfaces_deterministically(self):
        source = candidate()
        suite = manifest.compile_suite(source)
        self.assertEqual(len(suite["cases"]), 3 * len(source["capabilities"]))
        self.assertEqual({c["surface"] for c in suite["cases"]}, {"ui", "rest", "ws"})
        self.assertEqual(suite, manifest.compile_suite(copy.deepcopy(source)))
        self.assertTrue(all(c["expected"] == "unavailable_without_effects" for c in suite["cases"]))

    def test_rejects_missing_unknown_and_enabled_alpha_capability(self):
        for mutate in [lambda c: c.pop("phone_auth"), lambda c: c.update(unknown="disabled"),
                       lambda c: c.update(paid="enabled"), lambda c: c.update(paid=False)]:
            with self.subTest(mutate=mutate):
                source = candidate()
                mutate(source["capabilities"])
                with self.assertRaises(ValueError):
                    manifest.compile_suite(source)

    def test_rejects_development_or_test_checkout(self):
        for key, value in [("environment", "dev"), ("checkout_mode", "test"),
                           ("development_defaults", True), ("development_defaults", "false")]:
            with self.subTest(key=key, value=value):
                source = candidate()
                source["configuration"][key] = value
                with self.assertRaises(ValueError):
                    manifest.compile_suite(source)

    def test_requires_full_source_sha_and_exact_schema(self):
        for key, value in [("source_sha", "a" * 7), ("source_sha", "A" * 40),
                           ("source_sha", "0" * 40), ("schema_version", True), ("schema_version", 2)]:
            source = candidate()
            source[key] = value
            with self.subTest(key=key, value=value), self.assertRaises(ValueError):
                manifest.compile_suite(source)

    def test_federation_is_excluded_from_every_inventory(self):
        for inventory in manifest.INVENTORIES:
            source = candidate()
            source["inventories"][inventory].append("federation")
            with self.subTest(inventory=inventory), self.assertRaises(ValueError):
                manifest.compile_suite(source)
        source = candidate()
        source["inventories"].pop("alerts")
        with self.assertRaises(ValueError):
            manifest.compile_suite(source)

    def test_closure_requires_every_exact_case_with_portable_evidence(self):
        suite = manifest.compile_suite(candidate())
        self.assertEqual(manifest.verify_results(candidate(), results(suite))["result"], "PASS")
        changes = [lambda r: r["observations"].pop(),
                   lambda r: r["observations"].append(copy.deepcopy(r["observations"][0])),
                   lambda r: r["observations"][0].update(result="SKIP"),
                   lambda r: r["observations"][0].update(result="FAIL"),
                   lambda r: r["observations"][0].update(evidence_uri="C:/tmp/proof.json"),
                   lambda r: r.update(source_sha="b" * 40),
                   lambda r: r.update(manifest_sha256="b" * 64),
                   lambda r: r["observations"][0].update(case_id="extra")]
        for change in changes:
            report = results(suite)
            change(report)
            with self.subTest(change=change), self.assertRaises(ValueError):
                manifest.verify_results(candidate(), report)

    def test_changed_manifest_invalidates_old_results(self):
        source = candidate()
        report = results(manifest.compile_suite(source))
        source["inventories"]["images"].append("voice")
        with self.assertRaises(ValueError):
            manifest.verify_results(source, report)

    def test_rejects_duplicate_json_keys(self):
        with self.assertRaises(ValueError):
            manifest.load_json('{"profile":"alpha","profile":"beta"}')
        with self.assertRaises(ValueError):
            manifest.load_json('{"value":NaN}')

    def test_malformed_shapes_and_inventory_aliases_reject(self):
        for key, value in [("capabilities", []), ("configuration", None), ("inventories", []),
                           ("inventories", {name: ["voice-federation"] for name in manifest.INVENTORIES})]:
            source = candidate()
            source[key] = value
            with self.subTest(key=key), self.assertRaises(ValueError):
                manifest.compile_suite(source)
        source = candidate()
        source["configuration"]["typo"] = False
        with self.assertRaises(ValueError):
            manifest.compile_suite(source)

    def test_credentials_in_evidence_are_rejected(self):
        suite = manifest.compile_suite(candidate())
        for uri in ("https://user:secret@host/proof", "file:///tmp/proof", "artifact://run", "", "https://host/a b"):
            report = results(suite)
            report["observations"][0]["evidence_uri"] = uri
            with self.subTest(uri=uri), self.assertRaises(ValueError):
                manifest.verify_results(candidate(), report)

    def test_cli_binds_external_source_and_never_echoes_rejected_input(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "manifest.json"
            path.write_text(json.dumps(candidate()), encoding="utf-8")
            output = io.StringIO()
            with contextlib.redirect_stdout(output):
                self.assertEqual(manifest.main([str(path), "--expected-source-sha", "a" * 40]), 0)
            self.assertEqual(json.loads(output.getvalue())["source_sha"], "a" * 40)
            output, errors = io.StringIO(), io.StringIO()
            with contextlib.redirect_stdout(output), contextlib.redirect_stderr(errors):
                self.assertEqual(manifest.main([str(path), "--expected-source-sha", "b" * 40]), 2)
            self.assertEqual(output.getvalue(), "")
            path.write_text('{"secret":"credential-do-not-log"}', encoding="utf-8")
            output, errors = io.StringIO(), io.StringIO()
            with contextlib.redirect_stdout(output), contextlib.redirect_stderr(errors):
                self.assertEqual(manifest.main([str(path), "--expected-source-sha", "a" * 40]), 2)
            self.assertNotIn("credential-do-not-log", output.getvalue() + errors.getvalue())


if __name__ == "__main__":
    unittest.main()
