"""Public protected prepare must reach dispatcher under a real root Session."""
import copy
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
from types import SimpleNamespace

import bridge
import bridge_root
import guard
import resource_claim
import resource_claim_test as fixtures
import source_authority
from controller import Blocked


class ProtectedPrepareEntryTest(unittest.TestCase):
    def test_dispatch_failure_precedes_source_receipt_for_full_and_app(self):
        for mode in ("full", "app-only"):
            with self.subTest(mode=mode), tempfile.TemporaryDirectory(dir="/root") as tmp:
                root = Path(tmp)
                installed = root / "installed"
                installed.mkdir(mode=0o700)
                (installed / "sources").mkdir(mode=0o700)
                requests = root / "requests"
                requests.mkdir(mode=0o700)
                context = fixtures.context()
                request = dict(action="prepare", nonce="a" * 64, source_sha=context["commit"],
                    run_id=17, mode=mode, changed_services=[], token='synthetic-token',
                    resource_context=context)
                names = ("nats-rollout-preservation/resource_claim.py", "nats-rollout-preservation/runner.py",
                    "nats-rollout-preservation/apply.py", "nats-rollout-preservation/guard.py",
                    "nats-rollout-preservation/bridge.py", "nats-rollout-preservation/bridge_root.py",
                    "nats-known-baseline/commands.py")
                with patch.object(bridge_root.root_cli, 'code_binding', return_value={name: 'b' * 64 for name in names}):
                    actions = bridge_root.Actions(root)
                card = fixtures.card(context)
                class Check:
                    def __init__(self, value): self.context = value
                    def __call__(self): return copy.deepcopy(card)
                    def uncertain(self, reason): pass
                with patch.object(guard, "ROOT", root), patch.object(bridge_root, "INSTALLED", installed), \
                     patch.object(resource_claim, "Check", Check), \
                     patch("grp.getgrnam", return_value=SimpleNamespace(gr_gid=0)), \
                     patch.object(source_authority, "_get", side_effect=RuntimeError("dispatcher transport failed")) as get:
                    with self.assertRaisesRegex(RuntimeError, "dispatcher transport failed"):
                        with actions.resource_scope(request) as session:
                            bridge.dispatch(requests, request, actions.execute, actions.recover,
                                execute_observed=actions.execute_observed, admit=session.admit_request, validate_cached=actions.validate_cached)
                    get.assert_called_once()
                    self.assertFalse((installed / "source-authority").exists())
                    self.assertFalse((installed / "sources" / ("a" * 12)).exists())
                    self.assertNotIn(request['token'], (requests / ("a" * 64 + ".json")).read_text())

    def test_complete_prepare_cache_requires_protected_cohort_proof(self):
        import hashlib
        import json
        with tempfile.TemporaryDirectory(dir="/root") as tmp:
            root = Path(tmp)
            installed = root / "installed"
            installed.mkdir(mode=0o700)
            requests = root / "requests"
            requests.mkdir(mode=0o700)
            context = fixtures.context()
            request = dict(action="prepare", nonce="a" * 64, source_sha=context["commit"],
                run_id=17, mode="full", changed_services=[], token='synthetic-token',
                resource_context=context)
            public = {k: v for k, v in request.items() if k != "token"}
            digest = hashlib.sha256(json.dumps(public, sort_keys=True, separators=(",", ":")).encode()).hexdigest()
            bridge._save(requests / (request["nonce"] + ".json"), dict(
                schema="voice-nats-bridge-request-v1", request_sha256=digest,
                request=public, phase="COMPLETE", result={"status": "WAITING"}))
            names = ("nats-rollout-preservation/resource_claim.py", "nats-rollout-preservation/runner.py",
                "nats-rollout-preservation/apply.py", "nats-rollout-preservation/guard.py",
                "nats-rollout-preservation/bridge.py", "nats-rollout-preservation/bridge_root.py",
                "nats-known-baseline/commands.py")
            with patch.object(bridge_root.root_cli, 'code_binding', return_value={name: 'b' * 64 for name in names}):
                actions = bridge_root.Actions(root)
            card = fixtures.card(context)
            class Check:
                def __init__(self, value): self.context = value
                def __call__(self): return copy.deepcopy(card)
                def uncertain(self, reason): pass
            with patch.object(guard, "ROOT", root), patch.object(bridge_root, "INSTALLED", installed), \
                 patch.object(resource_claim, "Check", Check), \
                 patch("grp.getgrnam", return_value=SimpleNamespace(gr_gid=0)):
                with self.assertRaises((bridge.BridgeError, RuntimeError, Blocked)):
                    with actions.resource_scope(request) as session:
                        bridge.dispatch(requests, request, actions.execute, actions.recover,
                            execute_observed=actions.execute_observed, admit=session.admit_request, validate_cached=actions.validate_cached)


if __name__ == "__main__":
    unittest.main()

