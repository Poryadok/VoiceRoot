"""Bridge entry joined to the real protected prerequisite producer.

External Kubernetes, registry, TLS and TT observations are synthetic, as in
PrepareCompositionTests. The bridge, source receipt, parent and Session run.
"""
import copy
import base64
import shutil
import unittest
from unittest.mock import patch

import bridge_root
import resource_claim
import realtime_bootstrap as realtime
import runtime_stage
import source_authority
import stage_runtime
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

    def test_realtime_successor_rechecks_real_parent_capture(self):
        self._exercise(with_realtime=True)

    def _exercise(self, fault=None, *, with_realtime=False):
        original = prereq.run_prepare

        def bridge_missing(actions,request,approved,execution,workspace):
            import staging_cohort_preservation as cohort
            session=actions.tt_session;installed=workspace.parents[1]
            saved=workspace.with_name(workspace.name+'-fixture');workspace.rename(saved)
            (installed/'source-authority'/(session.operation+'.json')).unlink()
            execution=dict(execution,head_sha=approved['source_sha'])
            public=dict(request,action='prepare',nonce=session.operation+'a'*52,token='fixture',
                        source_sha=session.context['commit'],resource_context=session.context)
            def source(_token,_run,_sha,destination):
                shutil.copytree(saved,destination,dirs_exist_ok=True)
                return copy.deepcopy(approved)
            # Exercise real bridge/source capture. The handoff to pre-cut
            # inputs is a test boundary, not a qualified P2 proof.
            with patch.object(bridge_root,'INSTALLED',installed),patch.object(actions,'_idle'), \
                 patch.object(actions,'_dispatcher',return_value=(17,execution)), \
                 patch.object(source_authority,'capture_source',side_effect=source), \
                 patch.object(prereq,'P2_BINARY_PINS',{}), \
                 patch.object(cohort,'prepare',side_effect=lambda a,r,ap,ex,ws:prereq.prepare_inputs(a,r,ap,ex,ws)):
                with self.assertRaises(FileNotFoundError) as caught:actions.execute(public)
            self.assertEqual(caught.exception.filename,str(installed/'prerequisite-inputs'/'p3-issuer.json'))

        def joined(actions, request, approved, execution, workspace, *, preservation_ref):
            import staging_cohort_preservation as cohort
            session = actions.tt_session
            installed = workspace.parents[1]
            stage = runtime_stage.RolloutStage.capture.return_value
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
                    if with_realtime:
                        self._realtime_successor(actions, request, approved, execution, workspace, stage)
                    raise ReachedPolicy()
            request = dict(request, nonce=session.operation + 'a' * 52, token='fixture', action='prepare')
            loaded={'request':{'mode':request['mode'],'changed_services':request['changed_services'],
                               'gis_first_install':request.get('gis_first_install') is True},
                    'approved':approved,'execution':execution,'workspace':str(workspace)}
            authorization={'action':'authorize','operation':session.operation,'artifact_id':42,'token':'fixture',
                           'resource_context':session.context}
            original_read=cohort._read
            def read(path,*args,**kwargs):
                return request if path==cohort._request_record(session) else original_read(path,*args,**kwargs)
            with patch.object(prereq, 'run_prepare', original), \
                 patch.object(bridge_root, 'INSTALLED', installed), \
                 patch.object(actions, '_idle'), \
                 patch.object(actions, '_dispatcher', return_value=(17, execution)), \
                 patch.object(actions, '_report', side_effect=report), \
                 patch.object(bridge_root, 'Kube', return_value=stage.kube), \
                 patch.object(runtime_stage.RolloutStage, 'capture', side_effect=capture), \
                 patch.object(cohort,'capture_final_stage',side_effect=capture), \
                 patch.object(cohort,'qualify_offnode'), \
                 patch.object(cohort,'begin_continuation',return_value=(preservation_ref,loaded)), \
                 patch.object(cohort,'load_prepared',return_value=loaded), \
                 patch.object(cohort,'require_continuation'),patch.object(cohort,'_read',side_effect=read):
                with self.assertRaises(ReachedPolicy):
                    actions.execute(authorization)
            row = session.check()
            if not with_realtime:
                self.assertEqual(row['phase'], 'prerequisites-captured')
                record = prereq.load_capture(session, actual=stage.old_images)
                self.assertEqual(row['target_sha256'], resource_claim.digest(record))
            if not with_realtime:
                self.assertEqual(prereq._checkpoint(session)['status'], 'COMPLETE')
            raise Joined()

        fixture = staging_prerequisites_test.PrepareCompositionTests()
        with patch.object(prereq, 'run_prepare', side_effect=joined):
            if fault:
                fixture._cold_prepare(fault,bridge_missing=bridge_missing)
            else:
                with self.assertRaises(Joined):
                    fixture._cold_prepare(realtime_source=with_realtime)

    def _realtime_successor(self, actions, request, approved, execution, workspace, stage):
        """Run the real RT producer with synthetic PostgreSQL and Kube effects."""
        session = actions.tt_session
        parent = prereq.load_capture(session, actual=stage.old_images)
        old_get = stage_runtime.Kube.get.side_effect
        secret = old_get('secret', 'voice-app-secrets')
        role = {'oid': 13, 'login': True, 'superuser': False, 'createdb': False,
                'createrole': False, 'inherit': False, 'replication': False,
                'bypassrls': False, 'memberships': 0, 'connection_limit': -1,
                'valid_until': None, 'config': None}
        catalog = {'role': None, 'database': None}
        pg = {'statefulset': {'uid': 'sts', 'resourceVersion': '1'},
              'pod': {'uid': 'pod', 'resourceVersion': '2', 'container_id': 'containerd://id', 'restart_count': 0},
              'pvc': {'name': 'pgdata-voice-postgres-0', 'uid': 'pvc', 'resourceVersion': '3'},
              'pv': {'name': 'pv', 'uid': 'pvuid', 'resourceVersion': '4'},
              'service': {'uid': 'svc', 'resourceVersion': '5', 'cluster_ip': '10.0.0.1'},
              'endpoints': {'uid': 'ep', 'resourceVersion': '6', 'pod_ip': '10.0.0.2'},
              'slice': {'uid': 'sl', 'resourceVersion': '7'}}
        def get(kind, name):
            if (kind, name) == ('secret', 'voice-app-secrets'):
                return copy.deepcopy(secret)
            return old_get(kind, name)
        def create_role(_password, _system): catalog['role'] = copy.deepcopy(role)
        def create_database(_system, _oid): catalog['database'] = {'oid': 29, 'owner_oid': 13}
        def patch_secret(_before, password):
            url = b'postgres://realtime:' + password + b'@voice-postgres:5432/realtime_db?sslmode=disable'
            secret['data']['REALTIME_DATABASE_URL'] = base64.b64encode(url).decode()
            secret['metadata']['resourceVersion'] = str(int(secret['metadata']['resourceVersion']) + 1)
        with patch.object(stage_runtime.Kube.get, 'side_effect', get), \
             patch.object(realtime, 'engine_snapshot', return_value=(stage.expected['namespace_uid'], pg)), \
             patch.object(realtime, '_system_identifier', return_value='12345'), \
             patch.object(realtime, '_verify_admin'), patch.object(realtime, '_verify_runtime_auth'), \
             patch.object(realtime, '_writer_check'), \
             patch.object(realtime, '_catalog', side_effect=lambda: copy.deepcopy(catalog)), \
             patch.object(realtime, '_create_role', side_effect=create_role), \
             patch.object(realtime, '_create_database', side_effect=create_database), \
             patch.object(realtime, '_patch_secret', side_effect=patch_secret):
            receipt = realtime.run_prepare(actions, request, approved, execution, workspace, stage)
            import staging_cohort_preservation as cohort
            target = {'tag': session.context['commit'], 'images': session.context['images'],
                      'installation_sha256':session.installation.sha256,
                      'cohort_preservation_sha256':prereq.digest(cohort.load_qualified(session)),
                      'prerequisites_sha256': prereq.digest(parent),
                      'realtime_bootstrap_sha256': realtime.digest(receipt)}
            session.advance_realtime_bootstrap(target, stage.old_images)
            self.assertEqual(realtime.load_completed(session, rollout_target=target), receipt)
            self.assertEqual(prereq.load_capture(session, rollout_target=target, actual=stage.old_images), parent)
            secret['data']['unrelated-tamper'] = base64.b64encode(b'changed').decode()
            with self.assertRaisesRegex(prereq.Blocked, 'realtime_bootstrap_secret_readback_invalid'):
                prereq.load_capture(session, rollout_target=target, actual=stage.old_images)


if __name__ == '__main__':
    unittest.main()
