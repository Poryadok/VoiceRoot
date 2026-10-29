import tempfile
import unittest
import json
import copy
import hashlib
from types import SimpleNamespace
from pathlib import Path
from unittest.mock import patch

import fcntl
import root_main
import resource_claim
import resource_claim_test
import guard
import commands
import staging_core_release as core
import source_authority
import migrations


class OperationLockProofTest(unittest.TestCase):
    def test_proof_tracks_actual_lock_and_scope(self):
        with tempfile.TemporaryDirectory() as directory, patch.object(root_main, 'ROOT', Path(directory)):
            with root_main.operation_lock() as proof:
                self.assertIs(type(proof), root_main.OperationLockProof)
                proof.check()
                with self.assertRaisesRegex(root_main.Blocked, 'operation_already_running'):
                    with root_main.operation_lock():
                        self.fail('concurrent lock acquired')
                fcntl.flock(proof._fd, fcntl.LOCK_UN)
                with self.assertRaisesRegex(root_main.Blocked, 'operation_lock_proof_invalid'):
                    proof.check()
            with self.assertRaisesRegex(root_main.Blocked, 'operation_lock_proof_invalid'):
                proof.check()

    def test_replaced_lock_path_invalidates_proof(self):
        with tempfile.TemporaryDirectory() as directory, patch.object(root_main, 'ROOT', Path(directory)):
            with root_main.operation_lock() as proof:
                path = Path(directory) / 'known-baseline.lock'
                path.rename(path.with_name('old.lock'))
                path.touch(mode=0o600)
                with self.assertRaisesRegex(root_main.Blocked, 'operation_lock_proof_invalid'):
                    proof.check()

    def test_real_root_scope_binds_live_proof_and_journal(self):
        context = resource_claim_test.context()
        helper = {'fixture': 'b' * 64}
        request = {'action': 'authorize', 'operation': 'a' * 12, 'nonce': 'e' * 64,
                   'resource_context': context}
        with tempfile.TemporaryDirectory(dir=Path(__file__).resolve().parents[3]) as directory:
            root = Path(directory)
            (root / 'installed').mkdir(mode=0o700)
            journal = root / 'installed' / resource_claim.JOURNAL
            journal.write_text(json.dumps({'schema': 'voice-staging-resource-fence-v1',
                'context': context, 'operation': 'a' * 12, 'helper_sha256': resource_claim.digest(helper),
                'status': 'active', 'phase': 'apply-ready', 'target_sha256': 'd' * 64}), encoding='utf8')
            journal.chmod(0o600)
            with patch.object(root_main, 'ROOT', root), patch.object(guard, 'ROOT', root), \
                 patch.object(resource_claim, 'require_helper'), \
                 patch.object(resource_claim.Check, '__call__', return_value=resource_claim_test.card(context)):
                with root_main.operation_lock() as proof:
                    actions = SimpleNamespace(binding=helper, _operation_lock=proof, tt_session=None)
                    with resource_claim.root_scope(actions, request) as session:
                        session.admit_request(request)
                        self.assertIs(type(session.journal), resource_claim.RootJournal)
                        self.assertIs(session.operation_lock, proof)
                        self.assertEqual(session.check()['phase'], 'apply-ready')
                        proof.check()
                with self.assertRaisesRegex(root_main.Blocked, 'operation_lock_proof_invalid'):
                    session.operation_lock.check()


class ReadinessTest(unittest.TestCase):
    def test_complete_migration_history_requires_all_source_files_and_events(self):
        plan = []; files = {}; operation = 'a' * 12
        for database in (*migrations.DATABASES, 'auth'):
            auth = database == 'auth'; name = 'V1__schema.sql' if auth else '1_schema.up.sql'
            digest = hashlib.sha256(b'SELECT 1;').hexdigest()
            plan.append({'database': database, 'image': migrations.FLYWAY_IMAGE if auth else migrations.IMAGE,
                         'secret_key': 'POSTGRES_PASSWORD' if auth else database.upper() + '_DATABASE_URL',
                         'files': {name: 'SELECT 1;'}, 'hashes': {name: digest}})
            prefix = 'src/backend/auth/src/main/resources/db/migration/' if auth else 'src/backend/migrations/' + database + '_db/'
            files[prefix + name] = digest
        jobs = []; events = []; documents = migrations.documents(plan, operation)
        for index, (row, cm, job) in enumerate(zip(plan, documents[::2], documents[1::2]), 1):
            if row['database'] == 'space': continue
            job_uid = '00000000-0000-4000-8000-%012d' % index
            cm_uid = '10000000-0000-4000-8000-%012d' % index
            saved = {'name': job['metadata']['name'], 'uid': job_uid, 'configmap_uid': cm_uid}
            jobs.append(saved)
            events.extend([{'kind': 'migration_resource_created', 'resource_kind': 'ConfigMap', 'name': cm['metadata']['name'], 'uid': cm_uid},
                           {'kind': 'migration_resource_created', 'resource_kind': 'Job', 'name': job['metadata']['name'], 'uid': job_uid},
                           {'kind': 'migration_complete', **saved}])
        state = {'operation': operation, 'migrations': plan, 'migration_jobs': jobs, 'events': events,
                 'target': {'mode': 'full', 'migration_sha256': resource_claim.digest(plan), 'source_hashes': files}}
        core._migration_history(state, {'source_files': files})
        approved_extra = dict(files, **{'src/backend/migrations/user_db/2_extra.up.sql': hashlib.sha256(b'SELECT 2;').hexdigest()})
        with self.assertRaisesRegex(core.Blocked, 'core_release_source_changed'):
            core._migration_history(state, {'source_files': approved_extra})
        missing = copy.deepcopy(state); missing['events'].pop()
        with self.assertRaisesRegex(core.Blocked, 'core_release_migration_changed'):
            core._migration_history(missing, {'source_files': files})
        changed = copy.deepcopy(state); changed['target']['source_hashes'] = dict(files); changed['target']['source_hashes'].pop(next(iter(files)))
        with self.assertRaisesRegex(core.Blocked, 'core_release_source_changed'):
            core._migration_history(changed, {'source_files': files})

    def test_23_live_workloads_and_drift(self):
        context = resource_claim_test.context()
        images = {name: 'registry/' + name + '@sha256:' + 'a' * 64 for name in source_authority.IMAGE_NAMES}
        deployments = {}; replicasets = []; pods = []; snapshots = {}; hashes = {}
        for service in sorted(set(images) - {'nats-hub-config-renderer'}):
            name = 'voice-' + service
            container_name = 'portal' if service == 'developer-portal' else service
            template = {'metadata': {'labels': {'app': name}}, 'spec': {'containers': [{'name': container_name, 'image': images[service]}]}}
            uid = 'dep-' + service; rs_uid = 'rs-' + service
            deployment = {'kind': 'Deployment', 'metadata': {'name': name, 'namespace': 'voice-staging', 'uid': uid, 'generation': 1},
                'spec': {'replicas': 1, 'template': template},
                'status': {'observedGeneration': 1, 'readyReplicas': 1, 'updatedReplicas': 1, 'availableReplicas': 1}}
            deployments[name] = deployment; snapshots[name] = copy.deepcopy(deployment)
            hashes[name] = resource_claim.digest(template)
            replicasets.append({'metadata': {'uid': rs_uid, 'namespace': 'voice-staging',
                'ownerReferences': [{'kind': 'Deployment', 'uid': uid, 'controller': True}]},
                'spec': {'replicas': 1, 'template': copy.deepcopy(template)}})
            pod_spec = copy.deepcopy(template['spec'])
            pod_spec['volumes'] = [{'name': 'kube-api-access-abcde', 'projected': {'defaultMode': 420, 'sources': [
                {'serviceAccountToken': {'path': 'token', 'expirationSeconds': 3600}},
                {'configMap': {'name': 'kube-root-ca.crt', 'items': [{'key': 'ca.crt', 'path': 'ca.crt'}]}},
                {'downwardAPI': {'items': [{'path': 'namespace', 'fieldRef': {'apiVersion': 'v1', 'fieldPath': 'metadata.namespace'}}]}}]}}]
            pod_spec['containers'][0]['volumeMounts'] = [{'name': 'kube-api-access-abcde', 'mountPath': '/var/run/secrets/kubernetes.io/serviceaccount', 'readOnly': True}]
            pods.append({'metadata': {'uid': 'pod-' + service, 'namespace': 'voice-staging',
                'ownerReferences': [{'kind': 'ReplicaSet', 'uid': rs_uid, 'controller': True}]}, 'spec': pod_spec,
                'status': {'phase': 'Running', 'conditions': [{'type': 'Ready', 'status': 'True'}],
                    'containerStatuses': [{'name': container_name, 'ready': True, 'imageID': 'docker-pullable://' + images[service]}]}})
        state = {'context': {'expected': {'namespace_uid': 'namespace-uid'}, 'snapshots': snapshots},
                 'target': {'template_hashes': hashes}}
        class Kube:
            def get(self, kind, name):
                if kind == 'namespace': return {'kind': 'Namespace', 'metadata': {'name': name, 'uid': 'namespace-uid'}}
                return copy.deepcopy(deployments[name])
            def run(self, args):
                return {'metadata': {'resourceVersion': '1'}, 'items': copy.deepcopy(replicasets if args[1] == 'replicasets' else pods)}
        with tempfile.TemporaryDirectory(dir=Path(__file__).resolve().parents[3]) as directory:
            root = Path(directory); (root/'installed').mkdir(mode=0o700)
            helper = 'b' * 64; journal = root/'installed'/resource_claim.JOURNAL
            row = {'schema': 'voice-staging-resource-fence-v1', 'context': context, 'operation': 'a'*12,
                   'helper_sha256': helper, 'status': 'active', 'phase': 'apply-ready', 'target_sha256': 'd'*64}
            journal.write_text(json.dumps(row), encoding='utf8'); journal.chmod(0o600)
            with patch.object(root_main, 'ROOT', root), patch.object(guard, 'ROOT', root), patch.object(core, 'Kube', Kube):
                with root_main.operation_lock() as proof:
                    session = resource_claim.Session(context, 'a'*12, helper, lambda: resource_claim_test.card(context), resource_claim.RootJournal(root))
                    session.admit(start=False); session.operation_lock = proof
                    with resource_claim.command_scope(session.check):
                        uid, observed = core._observe(session, state, {'images': images}, snapshots)
                        self.assertEqual(uid, 'namespace-uid'); self.assertEqual(len(observed), 23)
                        pods[0]['status']['containerStatuses'][0]['ready'] = False
                        with self.assertRaisesRegex(core.Blocked, 'core_release_pod_invalid'):
                            core._observe(session, state, {'images': images}, snapshots)
                        pods[0]['status']['containerStatuses'][0]['ready'] = True
                        first = sorted(deployments)[0]
                        selected_hash = hashes.pop(first)
                        core._observe(session, state, {'images': images}, snapshots)
                        deployments[first]['spec']['template']['spec']['containers'][0]['image'] = 'foreign'
                        with self.assertRaisesRegex(core.Blocked, 'core_release_template_changed'):
                            core._observe(session, state, {'images': images}, snapshots)
                        deployments[first] = copy.deepcopy(snapshots[first]); hashes[first] = selected_hash
                        service = first.removeprefix('voice-'); old = images[service]
                        images[service] = 'registry/' + service + '@sha256:' + 'b'*64
                        with self.assertRaisesRegex(core.Blocked, 'core_release_image_changed'):
                            core._observe(session, state, {'images': images}, snapshots)
                        images[service] = old
                        pods[0]['status']['containerStatuses'][0]['imageID'] = 'containerd://sha256:' + 'f'*64
                        with self.assertRaisesRegex(core.Blocked, 'core_release_image_changed'):
                            core._observe(session, state, {'images': images}, snapshots)


if __name__ == '__main__':
    unittest.main()
