"""Real WriterCut/Go phases; Kubernetes, PostgreSQL, source and clone are synthetic."""
import copy
from pathlib import Path
from types import SimpleNamespace
import unittest
from unittest import mock

import frontend_runtime_executor
import full_cohort_affected_plan as affected
import full_cohort_go_migration as go
import media_backup_phase_test as fixture
import media_operation_journal
import resource_fencing
import writer_cut
import writer_cut_migration
import writer_cut_test


class TwoGoCutTest(unittest.TestCase):
    def exercise(self, fault):
        base = fixture.PhaseTest(); base.setUp(); self.addCleanup(base.doCleanups)
        objects = {'ConfigMap': {}, 'Job': {}, 'Secret': {}}
        events = []
        pending_reads = {}; pending_seen = []
        versions = {'chat_db': 1, 'user_db': 1}
        engine = {'namespace_uid': 'ns', 'service_ip': '10.0.0.1'}
        databases = {'chat_db': 10, 'user_db': 11}
        scope = {'engine': engine, 'server_start': 'start', 'databases': sorted(databases), 'database_oids': databases}
        original = {'engine': engine, 'server_start': 'start', 'all_databases': databases, 'databases': databases,
                    'excluded': {}, 'properties': {db: {'allowconn': True} for db in databases}}
        class Catalog:
            def verify_current(self, token): return copy.deepcopy(original)
            def verify_receipt(self, token): return copy.deepcopy(original)
            def _guard(self): pass
        self.enterContext(mock.patch.object(affected, 'FullCatalogPhase', Catalog))
        probe = affected.CatalogScopedProbe(Catalog(), 'token')
        class Client(writer_cut_test.Client):
            def call(inner, *args, body=None):
                if args[:2] == ('get', 'nodes'):
                    return {'items': [{'metadata': {'name': 'node', 'uid': 'node-uid'},
                            'status': {'conditions': [{'type': 'Ready', 'status': 'True'}]}}]}
                if args[:2] in (('get', 'jobs'), ('get', 'configmaps')):
                    kind = 'Job' if args[1] == 'jobs' else 'ConfigMap'
                    return {'items': copy.deepcopy(list(objects[kind].values()))}
                if args[:2] == ('get', 'pods'):
                    pods = []
                    for obj in objects['Job'].values():
                        pod = copy.deepcopy(obj['_pod']); uid = obj['metadata']['uid']
                        if fault == 'pending' and pending_reads.get(uid, 0) > 0:
                            pending_reads[uid] -= 1; pending_seen.append(uid)
                            pod['spec'].pop('nodeName'); pod['status'] = {'phase': 'Pending', 'containerStatuses': []}
                        pods.append(pod)
                    return {'items': pods}
                if args[0] == 'get' and args[1] in ('job', 'configmap') and len(args) > 2:
                    if args[1] == 'configmap' and args[2] == 'voice-dev-image-update-lock':
                        return super().call(*args)
                    kind = 'Job' if args[1] == 'job' else 'ConfigMap'
                    obj = objects[kind].get(args[2]); return copy.deepcopy({k:v for k,v in obj.items() if k != '_pod'}) if obj else {}
                if args[0] == 'create':
                    obj = copy.deepcopy(body); kind = obj['kind']; meta = obj['metadata']
                    dry = '--dry-run=server' in args
                    if kind != 'Pod': meta.update(uid='uid-' + meta['name'], resourceVersion='5')
                    if kind == 'Job':
                        uid, name = meta['uid'], meta['name']
                        generated = {'batch.kubernetes.io/controller-uid': uid, 'controller-uid': uid,
                                     'batch.kubernetes.io/job-name': name, 'job-name': name}
                        meta['labels'] = {'app': name, **generated}
                        obj['spec']['selector'] = {'matchLabels': {'batch.kubernetes.io/controller-uid': uid}}
                        obj['spec']['template']['metadata']['labels']['batch.kubernetes.io/controller-uid'] = uid
                        obj['status'] = {'conditions': [{'type': 'Complete', 'status': 'True'}]}
                        if not dry:
                            service = name.removeprefix('voice-go-migrate-').rsplit('-', 1)[0]
                            versions[service + '_db'] = 2
                            template = obj['spec']['template']; spec = copy.deepcopy(template['spec']); spec['nodeName'] = 'node'
                            obj['_pod'] = {'apiVersion': 'v1', 'kind': 'Pod', 'metadata': {'name': name + '-pod',
                                'namespace': 'voice-dev', 'uid': 'pod-' + service, 'resourceVersion': '6',
                                'labels': copy.deepcopy(template['metadata']['labels']),
                                'ownerReferences': [{'kind': 'Job', 'uid': uid, 'controller': True}]},
                                'spec': spec, 'status': {'phase': 'Succeeded', 'podIP': '10.1.2.' + ('3' if service == 'chat' else '4'),
                                'containerStatuses': [{'name': 'migrate', 'containerID': 'containerd://fixture', 'restartCount': 0,
                                    'imageID': 'containerd://' + go.realtime_migration_job.PLATFORMS['amd64'],
                                    'state': {'terminated': {'exitCode': 0}}}]}}
                            pending_reads[uid] = 2
                    if not dry:
                        objects[kind][meta['name']] = copy.deepcopy(obj)
                        if kind == 'Job':
                            events.append(meta['name'])
                            if fault == 'lost': raise TimeoutError('lost committed response')
                    return {k:v for k,v in obj.items() if k != '_pod'}
                if args[0] == 'delete':
                    kind = 'Job' if '/jobs/' in args[1] else 'ConfigMap'; name = args[1].rsplit('/', 1)[-1]
                    actual = objects[kind][name]['metadata']
                    if body['preconditions'] != {'uid': actual['uid'], 'resourceVersion': actual['resourceVersion']}:
                        raise ValueError('UID/RV mismatch')
                    del objects[kind][name]; return {}
                return super().call(*args)
        backing = base.ledger
        class LedgerView:
            def __init__(self, client): self.client = client
            def read(self): return backing.read()
            def write(self, revision, row): return backing.write(revision, row)
        def journal(name, extra=None):
            obj = media_operation_journal.Journal(Path(base.temp.name) / name,
                   {'resource_context': base.context, 'namespace_uid': 'ns', **(extra or {})})
            self.addCleanup(obj.close); return obj
        def facade(j):
            client = Client(base.lock, j)
            return client, resource_fencing.Fence(base.context, lambda: copy.deepcopy(base.card), LedgerView(client))
        self.enterContext(mock.patch.object(frontend_runtime_executor, 'KubectlTransport', Client))
        source = writer_cut_test.workload('Deployment', 'voice-a', 'uid-a', 1, 'old')
        admission = {'controllers': [source], 'guards': {'jobs': [], 'cronjobs': [], 'hpas': []},
                     'linked': {'jobs': [], 'cronjobs': []}, 'session_scope': scope}
        cut_j = journal('cut', {'writer_cut': admission}); cut_client, cut_fence = facade(cut_j)
        cut_client.workloads = {('Deployment', 'voice-a'): copy.deepcopy(source)}
        cut = writer_cut.WriterCut(cut_j, cut_fence, cut_client, base.lock, 'ns', admission, probe)
        self.enterContext(mock.patch.object(cut, '_patch', side_effect=cut_client.patch))
        self.enterContext(mock.patch.object(affected, '_full_catalog', return_value={k:copy.deepcopy(original[k])
                          for k in ('all_databases', 'databases', 'excluded', 'properties')}))
        self.enterContext(mock.patch.object(affected.pg, 'inspect', side_effect=lambda s,g:{'scope':copy.deepcopy(s),
                          'active_sessions':0, 'active_transactions':0}))
        def migration_sessions(s, identity, guard):
            step = base.ledger.row.get('go_migrations', {}).get('chat_db', {}).get('steps', {}).get('job', {})
            busy = fault == 'busy' and step.get('status') == 'pending'
            return {'scope':copy.deepcopy(s),'active_sessions':int(busy),
                    'active_transactions':0,'attributed_sessions':[]}
        self.enterContext(mock.patch.object(writer_cut_migration.writer_cut_pg_probe, 'inspect_migration',
                                           side_effect=migration_sessions))
        self.enterContext(mock.patch.object(go.schema, 'kube_engine_snapshot', return_value=lambda:engine))
        cut.drain()
        plans = {db: {'engine': 'golang-migrate', 'from': 1, 'to': 2, 'files': {'fixture': 'a' * 64}} for db in databases}
        original_schemas = {db: {'history': [{'version': 1, 'dirty': False}]} for db in databases}
        observed = {'databases': sorted(databases), 'metadata': {'migrations': plans, 'catalog': original_schemas}}
        node = {'name': 'node', 'uid': 'node-uid', 'hostname': 'node', 'architecture': 'amd64'}
        meta = SimpleNamespace(source_root=Path('/fixture'), inventory=SimpleNamespace(_admin_role=lambda:'postgres'),
                               package=SimpleNamespace(verify_current=lambda token:{'nodes':[node]}))
        meta._read = lambda db,*args:{'history':[{'version':versions[db], 'dirty':False}]}
        creds = {'role':'voice','host':'voice-postgres','sslmode':'disable','password':'fixture-password',
                 'references':{'url':{'uid':'source','resource_version':'1'}}}
        safe = {k:v for k,v in creds.items() if k != 'password'}
        def proof(meta_phase, cohort, current_cut, token, db):
            go._check_order(current_cut, observed, db)
            return {'database':db,'plan':plans[db],'credential':safe,'metadata_receipt_sha256':'d'*64,
                    'source':{'engine':engine,'server_start':'start','database_oid':databases[db]}}
        def clone(cohort, meta_phase, token, db):
            return {'database':db,'role':'voice','from':1,'to':2,'source_sha':base.context['commit'],
                    'snapshot_id':'capture-'+'a'*32,'manifest_sha256':'b'*64,'migration_roles_sha256':'c'*64,
                    'scope':'isolated-clone-migration-login-only'}
        def private(args, body=None):
            if args[0]=='create':
                obj=copy.deepcopy(body); m=obj['metadata']
                if '--dry-run=server' not in args:
                    m.update(uid='uid-'+m['name'],resourceVersion='7'); objects['Secret'][m['name']]=copy.deepcopy(obj)
                return obj
            if args[0]=='get': return copy.deepcopy(objects['Secret'].get(args[2],{}))
            name=args[1].rsplit('/',1)[-1]; actual=objects['Secret'][name]['metadata']
            if body['preconditions']!={'uid':actual['uid'],'resourceVersion':actual['resourceVersion']}: raise ValueError()
            del objects['Secret'][name]; return {}
        files = {'000001_init.up.sql': b'SELECT 1;', '000002_next.up.sql': b'SELECT 2;'}
        self.enterContext(mock.patch.object(go, 'qualified_plan', side_effect=proof))
        self.enterContext(mock.patch.object(go.cohort_restore_journal, 'verify_go_role', side_effect=clone))
        self.enterContext(mock.patch.object(go, '_credential', return_value=creds))
        self.enterContext(mock.patch.object(go, '_login', return_value=safe))
        self.enterContext(mock.patch.object(go, '_private_kube', side_effect=private))
        self.enterContext(mock.patch.object(go.metadata, 'verify_completed', return_value=observed))
        self.enterContext(mock.patch.object(go.restore, 'migration_inventory', return_value={db:files for db in databases}))
        for db in sorted(databases):
            j=journal('go-'+db); client, fence=facade(j)
            secret = go.SecretSnapshot(j,fence,client,base.lock,'ns',meta,object(),cut,'token',db)
            secret.create(); phase=go.Migration(secret)
            if fault in ('lost', 'busy'):
                with self.assertRaises(go.MigrationError): phase.run()
                self.assertEqual(base.ledger.row['status'], 'uncertain')
                with self.assertRaises(go.MigrationError): phase.run()
                self.assertEqual(len(events), 1 if fault == 'lost' else 0)
                return
            self.assertEqual(phase.run()['to'],2); phase.cleanup()
            self.assertEqual(go.verify_completed(phase)['to'],2)
        self.assertEqual([b['phase'].database for b in cut.migration_history if b['kind']=='go'],sorted(databases))
        self.assertEqual(cut.verify_drained()['active_sessions'],0)
        if fault == 'pending': self.assertTrue(pending_seen)

    def test_two_ordered_migrations_recheck_historical_cleanup(self): self.exercise(None)
    def test_pending_busy_refuses_post(self): self.exercise('busy')
    def test_lost_committed_reply_never_replays(self): self.exercise('lost')
    def test_pending_pods_do_not_lose_identity(self): self.exercise('pending')


if __name__ == '__main__': unittest.main()
