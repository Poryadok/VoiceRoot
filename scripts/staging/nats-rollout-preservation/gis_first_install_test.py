"""Mocked ownership, replay and delete-precondition tests for GIS first install."""
import copy
import hashlib
import json
import unittest

import gis_first_install as gis
import source_plan


class NotFound(Exception):
    status = 404


class Conflict(Exception):
    status = 409


def manifests():
    service = {'apiVersion': 'v1', 'kind': 'Service', 'metadata': {'name': 'voice-gameintegration',
        'namespace': gis.NS, 'labels': {'app': 'voice-gameintegration'}}, 'spec': {
        'selector': {'app': 'voice-gameintegration'}, 'ports': [
            {'name': 'http', 'port': 8080, 'targetPort': 'http'},
            {'name': 'principal-jwks', 'port': 8443, 'targetPort': 'principal-jwks'}]}}
    deployment = {'apiVersion': 'apps/v1', 'kind': 'Deployment', 'metadata': {
        'name': 'voice-gameintegration', 'namespace': gis.NS, 'labels': {'app': 'voice-gameintegration'}},
        'spec': {'replicas': 1, 'selector': {'matchLabels': {'app': 'voice-gameintegration'}},
                 'template': {'metadata': {'labels': {'app': 'voice-gameintegration'}}, 'spec': {
                     'containers': [{'name': 'gameintegration', 'image': 'ghcr.io/poryadok/voiceroot/gameintegration@sha256:'+'a'*64,
                         'ports': [{'name': 'http', 'containerPort': 8080}, {'name': 'principal-jwks', 'containerPort': 8443}]}]}}}}
    return service, deployment


def plan():
    service, deployment = manifests()
    source = {'schema': gis.SOURCE_SCHEMA, 'objects': [service, deployment]}
    result = gis.bind_operation(source, 'd'*12, {'mode': 'app-only', 'tag': 'a'*40, 'source_hashes': {}})
    result['prerequisite_receipt'] = prerequisite(result)
    return result


def prerequisite(operation):
    uid = '00000000-0000-0000-0000-000000000001'
    inputs = {'schema': 'voice-gis-prerequisite-v1', 'database': 'game_integration_db',
        'configmaps': {'provision': 'voice-gameintegration-runtime-role-sql',
            'migrations': 'voice-gameintegration-db-migrations', 'verify': 'voice-gameintegration-db-verifier-sql'},
        'migrations': [{'name': '0000000001_init.up.sql', 'sha256': 'a'*64}],
        'migration_inputs_sha256': 'b'*64, 'provision_sql_sha256': 'c'*64,
        'verifier_sql_sha256': 'd'*64,
        'templates': {'provision': 'e'*64, 'migrate': 'f'*64, 'verify': '1'*64},
        'images': {'postgres': 'postgres@sha256:'+'2'*64, 'migrate': 'migrate@sha256:'+'3'*64},
        'job_images': {'provision': {'provision': 'postgres'},
            'migrate': {'migrate': 'migrate'}, 'verify': {'verify': 'postgres'}},
        'jobs': {'provision': 'voice-provision-gameintegration-runtime-role',
            'migrate': 'voice-migrate-gameintegration-db', 'verify': 'voice-verify-gameintegration-db'},
        'secret_refs': {}}
    operation['inputs'] = inputs
    operation['inputs_sha256'] = gis.digest(inputs)
    operation['source_ci'] = {'run_id': 1, 'run_attempt': 1, 'workflow_id': 1,
        'tree_sha': 'a'*40, 'lock_artifact_id': 1, 'lock_sha256': '4'*64}
    operation['custody'] = {'operation': operation['operation'], 'namespace_uid': uid,
        'marker_uid': uid, 'claim_uid': uid, 'pv_uid': uid, 'target_sha': operation['source_sha'],
        'archive_sha256': '6'*64, 'manifest_sha256': '7'*64, 'census_sha256': '8'*64,
        'baseline_workload_count': 22, 'baseline_snapshots_sha256': '9'*64,
        'off_node_verified': True}
    operation['custody_binding_sha256'] = gis.digest(operation['custody'])
    operation['database_identity'] = {'namespace_uid': uid, 'service_uid': uid,
        'service_spec_sha256': '6'*64, 'endpoints': [{'uid': uid, 'pod_uids': [uid]}]}
    operation['secret_metadata'] = {'uid': uid, 'resourceVersion': '7'}
    operation['config_metadata'] = {'uid': uid, 'resourceVersion': '8'}
    verifier = {'schema': 'voice-gis-verifier-v1', 'database': 'game_integration_db',
        'migration_version': 31, 'dirty': False, 'runtime_role': 'gameintegration_runtime',
        'role_restricted': True, 'table_grants': 'select_insert_update_delete',
        'sequence_grants': 'usage_select', 'ddl_denied': True,
        'schema_ledger_runtime_mutation_denied': True}
    jobs = {}
    for role, name in inputs['jobs'].items():
        image_role = inputs['job_images'][role]
        jobs[role] = {'name': name, 'uid': uid, 'spec_sha256': 'a'*64, 'pod_uid': uid,
            'image_ids': {container: inputs['images'][image].replace('@', '@')
                          for container, image in image_role.items()},
            'complete': True, 'succeeded': 1, 'completion_time': '2026-10-17T00:00:00Z'}
    # The execution role is `migrate`; its source-owned ConfigMap key is
    # `migrations`. Keep the fixture receipt shaped like execute()'s producer.
    configmaps = {('migrations' if role == 'migrate' else role): {
        'name': inputs['configmaps']['migrations' if role == 'migrate' else role],
        'uid': uid, 'spec_sha256': 'b'*64}
        for role in ('provision', 'migrate', 'verify')}
    database_identity = copy.deepcopy(operation['database_identity'])
    revalidation = {'namespace_uid': uid, 'service_uid': uid,
        'service_spec_sha256': '6'*64, 'endpoints': [{'uid': uid, 'pod_uids': [uid]}],
        'secret_metadata': {'name': 'voice-app-secrets', **operation['secret_metadata']},
        'config_metadata': copy.deepcopy(operation['config_metadata'])}
    return {'schema': gis.PREREQUISITE_SCHEMA, 'operation': operation['operation'],
        'source_sha': operation['source_sha'], 'source_plan_sha256': operation['source_plan_sha256'],
        'source_ci': copy.deepcopy(operation['source_ci']),
        'inputs_sha256': operation['inputs_sha256'], 'inputs': copy.deepcopy(inputs),
        'namespace_uid': uid, 'database': 'game_integration_db',
        'database_identity': database_identity, 'configmaps': configmaps,
        'jobs': jobs, 'verifier': verifier, 'revalidation': revalidation,
        'custody_binding_sha256': operation['custody_binding_sha256']}


class FakeKube:
    def __init__(self):
        self.objects = {}
        self.writes = []
        self.next_uid = 10
        self.ambiguous_after_create = False
        self.create_race = None
        self.delete_conflict = False

    def get(self, kind, name):
        key = (kind, name)
        if key not in self.objects:
            raise NotFound()
        return copy.deepcopy(self.objects[key])

    def create(self, obj):
        key = (obj['kind'], obj['metadata']['name'])
        if key in self.objects:
            raise Conflict()
        if self.create_race is not None:
            self.objects[key] = copy.deepcopy(self.create_race)
            self.create_race = None
            raise Conflict()
        self.next_uid += 1
        created = copy.deepcopy(obj)
        created['metadata'].update(uid=str(self.next_uid), resourceVersion='1')
        self.objects[key] = created
        self.writes.append(('create', key))
        if self.ambiguous_after_create:
            self.ambiguous_after_create = False
            raise TimeoutError()
        return copy.deepcopy(created)

    def start(self, live, *, replicas, expected_uid, expected_resource_version):
        key = (live['kind'], live['metadata']['name'])
        current = self.objects[key]
        if current['metadata']['uid'] != expected_uid or current['metadata']['resourceVersion'] != expected_resource_version:
            raise Conflict()
        current['spec']['replicas'] = replicas
        current['metadata']['resourceVersion'] = str(int(expected_resource_version) + 1)
        self.writes.append(('start', key))
        return copy.deepcopy(current)

    def delete(self, kind, name, *, uid, resource_version):
        key = (kind, name)
        current = self.objects[key]
        if self.delete_conflict or current['metadata']['uid'] != uid or current['metadata']['resourceVersion'] != resource_version:
            raise Conflict()
        del self.objects[key]
        self.writes.append(('delete', key))


class RunKube:
    """Exercise the run/get surface used by the protected Kubernetes adapter."""
    def __init__(self): self.objects = {}; self.events = []; self.next_uid = 10
    def run(self, args, body=None):
        self.events.append((args[0], tuple(args[1:])))
        if args[0] == 'get':
            items = copy.deepcopy(list(self.objects.values()))
            return {'apiVersion': 'v1', 'kind': 'List', 'metadata': {}, 'items': items}
        if args[0] == 'create':
            obj = copy.deepcopy(body); key = (obj['kind'], obj['metadata']['name'])
            if key in self.objects: raise Conflict()
            self.next_uid += 1
            obj['metadata'].update(uid=str(self.next_uid), resourceVersion='1')
            self.objects[key] = obj
            return copy.deepcopy(obj)
        if args[0] == 'patch':
            key = ('Deployment', args[2]); row = self.objects[key]
            patch = json.loads(args[args.index('--patch') + 1])
            if row['metadata']['uid'] != patch[0]['value'] or row['metadata']['resourceVersion'] != patch[1]['value']:
                raise Conflict()
            row['spec']['replicas'] = 1; row['metadata']['resourceVersion'] = '2'
            return copy.deepcopy(row)
        if args[0] == 'delete':
            kind, name = args[1], args[2]; key = (kind.capitalize(), name)
            preconditions = next(v for v in args if v.startswith('--preconditions='))
            values = preconditions.split('--preconditions=', 1)[1]
            uid, rv = values.split('uid=', 1)[1].split(',resourceVersion=')
            row = self.objects[key]
            if row['metadata']['uid'] != uid or row['metadata']['resourceVersion'] != rv: raise Conflict()
            del self.objects[key]
            return {'status': 'Success'}
        raise AssertionError(args)


class NamespaceTests(unittest.TestCase):
    def test_default_namespace_remains_staging_and_dev_is_explicit(self):
        service, deployment = manifests()
        staging = gis.build_plan(service=service, deployment=deployment, operation='a'*12)
        self.assertEqual({obj['metadata']['namespace'] for obj in staging['objects']}, {'voice-staging'})

        service['metadata']['namespace'] = 'voice-dev'
        deployment['metadata']['namespace'] = 'voice-dev'
        image = next(row['image'] for row in deployment['spec']['template']['spec']['containers']
                     if row['name'] == 'gameintegration')
        builder_tree = 'f' * 40
        build = {'schema': 'voice-dev-source-build-v2', 'source_sha': 'c'*40,
            'image_ref': image, 'build_context_sha256': hashlib.sha256(builder_tree.encode()).hexdigest(),
            'image_config_sha256': 'e'*64, 'working_tree_clean': True,
            'builder_kind': 'github-actions', 'builder_run': 11, 'builder_job': 12,
            'builder_sha': 'c'*40, 'builder_tree': builder_tree,
            'builder_artifact_id': 13, 'builder_artifact_sha256': 'a'*64,
            'builder_output_index_sha256': image.rsplit('@sha256:', 1)[-1],
            'builder_provenance_sha256': 'b'*64}
        source = {'schema': gis.SOURCE_SCHEMA, 'objects': [service, deployment]}
        self.assertTrue(gis.validate_source_plan(source, namespace='voice-dev'))
        dev = gis.bind_operation(source, 'b'*12,
            {'mode': 'app-only', 'tag': 'c'*40, 'source_hashes': {},
             'source_build_receipt': build}, namespace='voice-dev')
        self.assertEqual(dev['namespace'], 'voice-dev')
        self.assertEqual({obj['metadata']['namespace'] for obj in dev['objects']}, {'voice-dev'})
        self.assertEqual(dev['source_build_receipt'], build)

        dirty = dict(build, working_tree_clean=False)
        with self.assertRaises(gis.Blocked):
            gis.bind_operation(source, 'c'*12,
                {'mode': 'app-only', 'tag': 'c'*40, 'source_hashes': {},
                 'source_build_receipt': dirty}, namespace='voice-dev')

    def test_namespace_is_not_an_arbitrary_target(self):
        service, deployment = manifests()
        with self.assertRaises(gis.Blocked):
            gis.build_plan(service=service, deployment=deployment,
                operation='a'*12, namespace='voice-production')


class Tests(unittest.TestCase):
    def test_v2_receipt_cannot_reuse_unqualified_v1_source(self):
        value=plan();receipt=copy.deepcopy(value['prerequisite_receipt'])
        receipt['schema']=gis.PREREQUISITE_SCHEMA_V2;receipt['verifier']=gis.verifier_output(32)
        value['prerequisite_receipt']=receipt
        with self.assertRaisesRegex(gis.Blocked,'source_revision_unsupported'):
            gis.validate_prerequisite_receipt(receipt,value)
    def test_database_prerequisite_receipt_is_required_before_any_mutation(self):
        current = plan(); kube = FakeKube(); current.pop('prerequisite_receipt')
        with self.assertRaises(gis.Blocked):
            gis.execute(kube, current, lambda _row: None)
        self.assertEqual(kube.writes, [])
        current = plan(); current['prerequisite_receipt']['verifier']['schema_ledger_runtime_mutation_denied'] = False
        kube = FakeKube()
        with self.assertRaises(gis.Blocked):
            gis.execute(kube, current, lambda _row: None)
        self.assertEqual(kube.writes, [])

    def test_protected_adapter_uses_create_cas_and_delete_preconditions(self):
        kube = RunKube(); current = plan(); saved = []
        receipt = gis.execute(kube, current, saved.append)
        self.assertEqual(receipt['schema'], gis.RESOURCE_RECEIPT_SCHEMA)
        self.assertTrue(any(event[0] == 'create' for event in kube.events))
        self.assertTrue(any(event[0] == 'patch' for event in kube.events))
        self.assertTrue(gis.validate_receipt(receipt, kube, operation=current['operation']))
        for kind, name in gis.IDENTITIES:
            key = f'{kind}/{name}'
            live = next(row for (actual_kind, actual_name), row in kube.objects.items() if (actual_kind, actual_name) == (kind, name))
            self.assertEqual(current['created'][key]['uid'], live['metadata']['uid'])
            self.assertEqual(current['created'][key]['resourceVersion'], live['metadata']['resourceVersion'])
        gis.rollback(kube, current, saved.append)
        self.assertFalse(kube.objects)
        self.assertTrue(any(event[0] == 'delete' for event in kube.events))

    def test_planner_absence_check_requires_bounded_complete_inventory(self):
        class Inventory:
            def __init__(self, response): self.response = response
            def run(self, args):
                self.assert_args = args
                if isinstance(self.response, Exception): raise self.response
                return self.response
        empty = {'apiVersion': 'v1', 'kind': 'List', 'metadata': {}, 'items': []}
        self.assertTrue(source_plan.require_gis_absent(Inventory(empty)))
        present = {'apiVersion': 'v1', 'kind': 'List', 'metadata': {}, 'items': [
            {'kind': 'Service', 'metadata': {'name': 'voice-gameintegration'}}]}
        with self.assertRaises(source_plan.PlanError):
            source_plan.require_gis_absent(Inventory(present))
        with self.assertRaises(source_plan.PlanError):
            source_plan.require_gis_absent(Inventory(RuntimeError('forbidden')))
        malformed = {'apiVersion': 'v1', 'kind': 'List', 'metadata': {}, 'items': []}
        malformed['items'] = [None] * 2049
        with self.assertRaises(source_plan.PlanError):
            source_plan.require_gis_absent(Inventory(malformed))

    def test_source_plan_requires_exact_pair_and_operation_binding(self):
        service, deployment = manifests()
        source = {'schema': gis.SOURCE_SCHEMA, 'objects': [service, deployment]}
        self.assertTrue(gis.validate_source_plan(source))
        bound = gis.bind_operation(source, '012345abcdef', {'mode': 'app-only', 'tag': 'a' * 40, 'source_hashes': {}})
        self.assertEqual(bound['phase'], 'PLANNED')
        self.assertEqual(bound['source_plan_sha256'], gis.digest(source))
        invalid = (
            {'schema': gis.SOURCE_SCHEMA, 'objects': [service]},
            {'schema': gis.SOURCE_SCHEMA, 'objects': [service, deployment, service]},
            {'schema': gis.SOURCE_SCHEMA, 'objects': [service, {**deployment, 'metadata': {'name': 'voice-user', 'namespace': gis.NS}}]},
            {**source, 'caller_supplied_success': True},
        )
        for row in invalid:
            with self.subTest(row=row), self.assertRaises(gis.Blocked):
                gis.validate_source_plan(row)
        with self.assertRaises(gis.Blocked):
            gis.bind_operation(source, '012345abcdef', {'mode': 'images-only', 'tag': 'a' * 40})

    def test_positive_pair_create_start_and_transition_receipt(self):
        kube = FakeKube(); saved = []
        result = gis.execute(kube, plan(), saved.append)
        self.assertEqual([entry[0] for entry in kube.writes], ['create', 'create', 'start'])
        self.assertEqual({entry[1] for entry in kube.writes}, set(gis.IDENTITIES))
        self.assertEqual(kube.objects[gis.DEPLOYMENT]['spec']['replicas'], 1)
        self.assertEqual(result['schema'], gis.RESOURCE_RECEIPT_SCHEMA)
        self.assertTrue(gis.validate_receipt(result, kube, operation='d'*12))

    def test_standard_apply_keeps_pair_paused_then_start_revalidates_live_uids(self):
        kube=FakeKube(); candidate=plan(); saved=[]
        receipt=gis.execute(kube,candidate,saved.append,start=False)
        self.assertEqual(candidate['phase'],'CREATED')
        self.assertEqual(kube.objects[gis.DEPLOYMENT]['spec']['replicas'],0)
        self.assertEqual([event[0] for event in kube.writes],['create','create'])
        # Standard declarative apply can advance RV, but it cannot replace the
        # operation-owned objects or change their semantic content.
        for identity in gis.IDENTITIES:
            kube.objects[identity]['metadata']['resourceVersion']='2'
        started=gis.start_owned(kube,candidate,lambda row:saved.append(copy.deepcopy(row)))
        self.assertEqual([event[0] for event in kube.writes],['create','create','start'])
        self.assertEqual(kube.objects[gis.DEPLOYMENT]['spec']['replicas'],1)
        self.assertEqual(started['objects']['Deployment/voice-gameintegration']['uid'],
            receipt['objects']['Deployment/voice-gameintegration']['uid'])
        self.assertEqual(candidate['created']['Service/voice-gameintegration']['resourceVersion'],'2')
        self.assertTrue(gis.validate_receipt(started,kube,operation='d'*12))

    def test_same_operation_rollback_removes_only_owned_pair_after_paused_apply(self):
        kube=FakeKube(); candidate=plan(); saved=[]
        gis.execute(kube,candidate,saved.append,start=False)
        for identity in gis.IDENTITIES:
            kube.objects[identity]['metadata']['resourceVersion']='2'
        gis.rollback(kube,candidate,lambda row:saved.append(copy.deepcopy(row)))
        self.assertEqual(candidate['phase'],'ROLLED_BACK')
        self.assertFalse(kube.objects)
        self.assertEqual([identity for action,identity in kube.writes if action=='delete'],
            [gis.DEPLOYMENT,gis.SERVICE])

    def test_same_operation_rollback_can_remove_started_pair_with_replica_guard(self):
        kube=FakeKube(); candidate=plan()
        gis.execute(kube,candidate,lambda row:None)
        gis.rollback(kube,candidate,lambda row:None)
        self.assertFalse(kube.objects)

    def test_preflight_requires_both_absent_and_non_404_is_not_absence(self):
        kube = FakeKube(); service, _ = manifests()
        kube.objects[gis.SERVICE] = service
        with self.assertRaisesRegex(gis.Blocked, 'already_present'):
            gis.preflight(kube, plan())

        class ForbiddenKube(FakeKube):
            def get(self, kind, name):
                raise PermissionError()
        with self.assertRaisesRegex(gis.Blocked, 'live_read_failed'):
            gis.preflight(ForbiddenKube(), plan())

    def test_create_race_with_foreign_object_is_never_adopted(self):
        kube = FakeKube(); foreign, _ = manifests()
        foreign['metadata'].update(uid='foreign', resourceVersion='1')
        kube.create_race = foreign
        with self.assertRaisesRegex(gis.Blocked, 'create_race'):
            gis.execute(kube, plan(), lambda row: None)
        self.assertEqual(kube.writes, [])

    def test_second_identity_is_checked_before_first_create(self):
        kube = FakeKube(); _, foreign = manifests()
        foreign['metadata'].update(uid='foreign', resourceVersion='1')
        kube.objects[gis.DEPLOYMENT] = foreign
        with self.assertRaisesRegex(gis.Blocked, 'create_race'):
            gis.execute(kube, plan(), lambda row: None)
        self.assertEqual(kube.writes, [])

    def test_lost_create_response_is_resumed_by_same_operation_only(self):
        kube = FakeKube(); kube.ambiguous_after_create = True; saved = []
        first = plan()
        receipt = gis.execute(kube, first, saved.append)
        self.assertEqual(receipt['schema'], gis.RESOURCE_RECEIPT_SCHEMA)
        self.assertEqual([entry[0] for entry in kube.writes].count('create'), 2)

        other = plan(); other['operation'] = 'e'*12
        for obj in other['objects']:
            obj['metadata']['annotations'][gis.MARKER] = other['operation']
        with self.assertRaisesRegex(gis.Blocked, 'prerequisite_receipt_binding_invalid'):
            gis.execute(kube, other, lambda row: None)
        self.assertEqual([entry[0] for entry in kube.writes].count('create'), 2)

    def test_crash_after_first_record_resumes_without_duplicate_create(self):
        kube = FakeKube(); candidate = plan(); durable = []
        def crash_after_service(record):
            durable.append(copy.deepcopy(record))
            if record.get('created', {}).get('Service/voice-gameintegration'):
                raise OSError('simulated journal interruption')
        with self.assertRaisesRegex(OSError, 'journal interruption'):
            gis.execute(kube, candidate, crash_after_service)
        resumed = copy.deepcopy(durable[-1])
        resumed.pop('receipt', None)
        receipt = gis.execute(kube, resumed, lambda row: durable.append(copy.deepcopy(row)))
        self.assertEqual(receipt['schema'], gis.RESOURCE_RECEIPT_SCHEMA)
        creates = [event for event in kube.writes if event[0] == 'create']
        self.assertEqual([identity for _, identity in creates], [gis.SERVICE, gis.DEPLOYMENT])

    def test_receipt_rejects_replaced_or_modified_object(self):
        kube = FakeKube(); receipt = gis.execute(kube, plan(), lambda row: None)
        kube.objects[gis.SERVICE]['metadata']['uid'] = 'replacement'
        with self.assertRaisesRegex(gis.Blocked, 'receipt_object_changed'):
            gis.validate_receipt(receipt, kube, operation='d'*12)

    def test_rollback_deletes_only_unchanged_owned_objects_in_reverse_order(self):
        kube = FakeKube(); journal = plan()
        # Force an incomplete operation after both creates, before start.
        for obj in journal['objects']:
            created = kube.create(obj)
            gis._record(journal, lambda row: None, obj['kind'], obj['metadata']['name'], created)
        gis.rollback(kube, journal, lambda row: None)
        self.assertEqual([kind for kind, _ in kube.writes[-2:]], ['delete', 'delete'])
        self.assertEqual([identity for _, identity in kube.writes[-2:]], [gis.DEPLOYMENT, gis.SERVICE])
        self.assertFalse(kube.objects)

    def test_rollback_refuses_resource_version_drift_without_deleting(self):
        kube = FakeKube(); journal = plan()
        created = kube.create(journal['objects'][0])
        gis._record(journal, lambda row: None, gis.SERVICE[0], gis.SERVICE[1], created)
        kube.objects[gis.SERVICE]['metadata']['resourceVersion'] = '2'
        with self.assertRaisesRegex(gis.Blocked, 'rollback_object_changed'):
            gis.rollback(kube, journal, lambda row: None)
        self.assertIn(gis.SERVICE, kube.objects)

    def test_rollback_refuses_spec_and_marker_drift_without_deleting(self):
        for change in ('spec', 'marker'):
            with self.subTest(change=change):
                kube = FakeKube(); journal = plan()
                created = kube.create(journal['objects'][0])
                gis._record(journal, lambda row: None, gis.SERVICE[0], gis.SERVICE[1], created)
                if change == 'spec':
                    kube.objects[gis.SERVICE]['spec']['ports'][0]['port'] = 8081
                else:
                    kube.objects[gis.SERVICE]['metadata']['annotations'].pop(gis.MARKER)
                with self.assertRaisesRegex(gis.Blocked, 'rollback_object_changed'):
                    gis.rollback(kube, journal, lambda row: None)
                self.assertIn(gis.SERVICE, kube.objects)

    def test_delete_conflict_preserves_owned_object(self):
        kube = FakeKube(); journal = plan()
        created = kube.create(journal['objects'][0])
        gis._record(journal, lambda row: None, gis.SERVICE[0], gis.SERVICE[1], created)
        kube.delete_conflict = True
        with self.assertRaisesRegex(gis.Blocked, 'rollback_delete_conflict'):
            gis.rollback(kube, journal, lambda row: None)
        self.assertIn(gis.SERVICE, kube.objects)


if __name__ == '__main__':
    unittest.main()
