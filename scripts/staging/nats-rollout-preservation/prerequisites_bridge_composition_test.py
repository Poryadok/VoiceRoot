"""Bridge entry joined to the real protected prerequisite producer.

External Kubernetes, registry, TLS and TT observations are synthetic, as in
PrepareCompositionTests. The bridge, source receipt, parent and Session run.
"""
import copy
import shutil
import unittest
from unittest.mock import patch

import bridge_root
import resource_claim
import runtime_stage
import source_authority
import staging_prerequisites as prereq
import staging_prerequisites_test


class ReachedPolicy(Exception):
    pass


class Joined(Exception):
    pass


class BridgePrerequisiteComposition(unittest.TestCase):
    def test_real_bridge_runs_parent_and_captures_its_receipt(self):
        self._exercise()

    def test_missing_issuer_refuses_before_any_effect_via_bridge(self):
        self._exercise('missing-issuer')

    def _exercise(self, fault=None):
        original = prereq.run_prepare

        def joined(actions, request, approved, execution, workspace):
            session = actions.tt_session
            installed = workspace.parents[1]
            saved = workspace.with_name(workspace.name + '-fixture')
            workspace.rename(saved)
            (installed / 'source-authority' / (session.operation + '.json')).unlink()
            stage = runtime_stage.RolloutStage.capture.return_value
            def source(_token, _run, _sha, destination):
                shutil.copytree(saved, destination, dirs_exist_ok=True)
                return copy.deepcopy(approved)
            def capture(*args, **kwargs):
                if session.check()['phase'] == 'prerequisites':
                    publisher = stage.kube.get('deployment', 'voice-p3-principal-jwks')
                    stage.snapshots['voice-p3-principal-jwks'] = publisher
                    stage.original_snapshots = copy.deepcopy(stage.snapshots)
                    stage.expected['deployment_uids']['voice-p3-principal-jwks'] = publisher['metadata']['uid']
                    stage.old_images = prereq._checkpoint(session)['receipt']['actual_after']
                return stage
            def report(name, status):
                if (name, status) == ('policy-validation', 'STARTED'):
                    raise ReachedPolicy()
            request = dict(request, nonce=session.operation + 'a' * 52, token='fixture', action='prepare')
            execution = dict(execution, head_sha=approved['source_sha'])
            with patch.object(prereq, 'run_prepare', original), \
                 patch.object(bridge_root, 'INSTALLED', installed), \
                 patch.object(actions, '_idle'), \
                 patch.object(actions, '_dispatcher', return_value=(17, execution)), \
                 patch.object(actions, '_report', side_effect=report), \
                 patch.object(source_authority, 'capture_source', side_effect=source), \
                 patch.object(bridge_root, 'Kube', return_value=stage.kube), \
                 patch.object(runtime_stage.RolloutStage, 'capture', side_effect=capture):
                if fault == 'missing-issuer':
                    try:
                        actions._prepare(request)
                    except FileNotFoundError as exc:
                        self.assertEqual(exc.filename, str(installed / 'prerequisite-inputs' / 'p3-issuer.json'))
                        raise
                    self.fail('missing issuer unexpectedly admitted')
                else:
                    with self.assertRaises(ReachedPolicy):
                        actions._prepare(request)
            row = session.check()
            self.assertEqual(row['phase'], 'prerequisites-captured')
            record = prereq.load_capture(session, actual=stage.old_images)
            self.assertEqual(row['target_sha256'], resource_claim.digest(record))
            self.assertEqual(prereq._checkpoint(session)['status'], 'COMPLETE')
            raise Joined()

        fixture = staging_prerequisites_test.PrepareCompositionTests()
        with patch.object(prereq, 'run_prepare', side_effect=joined):
            if fault:
                fixture._cold_prepare(fault)
            else:
                with self.assertRaises(Joined):
                    fixture._cold_prepare()


if __name__ == '__main__':
    unittest.main()
