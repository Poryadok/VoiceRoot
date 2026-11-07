"""Real root journal/Session and isolated PostgreSQL, with synthetic Kubernetes/TT."""
import base64
import copy
import hashlib
import json
from pathlib import Path
import subprocess
import tempfile
import time
from types import SimpleNamespace
import unittest
import uuid
from unittest import mock

import commands
import bridge_root
import guard
import realtime_bootstrap as bootstrap
import resource_claim
import resource_claim_test as fixtures
import stage_runtime


class CompositionTest(unittest.TestCase):
    def test_real_session_journal_pg_and_lost_secret_reply(self):
        for lost in (None, 'role', 'secret', 'source'):
            with self.subTest(lost=lost): self._exercise(lost)

    def _exercise(self, lost):
        name = 'voice-stage-composition-' + uuid.uuid4().hex[:16]
        admin = b'synthetic-admin-only'
        result = subprocess.run(['docker', 'run', '-d', '--name', name, '--network', 'none',
                                 '--label', 'voice.test.owner=staging-realtime-composition',
                                 '-e', 'POSTGRES_USER=voice', '-e', 'POSTGRES_PASSWORD=synthetic-admin-only',
                                 'postgres:16-alpine'], capture_output=True, check=True, timeout=60)
        cid = result.stdout.decode().strip()
        self.addCleanup(lambda: subprocess.run(['docker', 'rm', '-f', '-v', cid], capture_output=True, timeout=30, check=True))
        def pg(script, body=b'', check=True):
            return subprocess.run(['docker', 'exec', '-i', cid, 'sh', '-ceu', script], input=body,
                                  capture_output=True, timeout=30, check=check)
        for _ in range(100):
            if pg(bootstrap._PSQL, b'SELECT 1;', check=False).returncode == 0: break
            time.sleep(.2)
        else: self.fail('fixture PostgreSQL unavailable')
        with tempfile.TemporaryDirectory(dir='/root') as directory:
            root = Path(directory); installed = root / 'installed'; installed.mkdir(mode=0o700)
            operation = 'a' * 12
            source = installed / 'sources' / operation; source.mkdir(parents=True, mode=0o700)
            context = fixtures.context(); card = fixtures.card(context)
            code=installed/'code';(code/'nats-rollout-preservation').mkdir(parents=True,mode=0o700)
            bindings={}
            for module in (bootstrap,resource_claim):
                name='nats-rollout-preservation/'+Path(module.__file__).name
                raw=Path(module.__file__).read_bytes();path=code/name;path.write_bytes(raw);path.chmod(0o400)
                bindings[name]=hashlib.sha256(raw).hexdigest()
            bootstrap.save(code/'capture-manifest.json',bindings)
            helper=bindings['nats-rollout-preservation/realtime_bootstrap.py']
            journal = resource_claim.RootJournal(root)
            session = resource_claim.Session(context, operation, resource_claim.digest(bindings), lambda: copy.deepcopy(card), journal)
            marker = {'metadata': {'name': 'voice-nats-generation', 'uid': 'marker'},
                      'data': {'phase': 'active','generation':session.installation.generation}}
            spec = {'replicas': 1, 'selector': {'matchLabels': {'app': 'voice-realtime'}},
                    'template': {'spec': {'containers': [{'name': 'realtime'}]}}}
            deployment = {'metadata': {'name': 'voice-realtime', 'namespace': 'voice-staging', 'uid': 'deploy', 'generation': 1}, 'spec': spec}
            stage = SimpleNamespace(marker=marker, snapshots={'voice-realtime': deployment},
                                    expected={'namespace_uid': 'ns','generation':session.installation.generation}, old_images=card['observed']['images'])
            secret = {'metadata': {'name': 'voice-app-secrets', 'namespace': 'voice-staging', 'uid': 'secret', 'resourceVersion': '1'},
                      'type': 'Opaque', 'data': {'POSTGRES_PASSWORD': base64.b64encode(admin).decode(), 'keep': 'c2VudGluZWw='}}
            class Kube:
                def get(self, kind, item):
                    if (kind, item) == ('secret', 'voice-app-secrets'): return copy.deepcopy(secret)
                    if (kind, item) == ('configmap', 'voice-nats-generation'): return copy.deepcopy(marker)
                    if (kind, item) == ('deployment', 'voice-realtime'): return copy.deepcopy(deployment)
                    raise AssertionError((kind, item))
                def run(self, args):
                    if args == ['get', 'pods', '-o', 'json']: return {'items': []}
                    raise AssertionError(args)
            pg_identity = {'statefulset': {'uid': 'sts', 'resourceVersion': '1'},
                           'pod': {'uid': 'pod', 'resourceVersion': '2', 'container_id': 'containerd://id', 'restart_count': 0},
                           'pvc': {'name': 'pgdata-voice-postgres-0', 'uid': 'pvc', 'resourceVersion': '3'},
                           'pv': {'name': 'pv', 'uid': 'pvuid', 'resourceVersion': '4'},
                           'service': {'uid': 'svc', 'resourceVersion': '5', 'cluster_ip': '10.0.0.1'},
                           'endpoints': {'uid': 'ep', 'resourceVersion': '6', 'pod_ip': '10.0.0.2'},
                           'slice': {'uid': 'sl', 'resourceVersion': '7'}}
            calls = []
            def capture(args, *, body=b'', **kwargs):
                session.check(args)
                if 'patch' in args:
                    calls.append('patch')
                    operations = json.loads(body)
                    self.assertEqual(operations[0]['value'], secret['metadata']['uid'])
                    self.assertEqual(operations[1]['value'], secret['metadata']['resourceVersion'])
                    self.assertEqual(operations[2]['value'], secret['data'])
                    secret['data']['REALTIME_DATABASE_URL'] = operations[3]['value']
                    secret['metadata']['resourceVersion'] = '2'
                    if lost == 'secret': raise OSError('synthetic reply loss after commit')
                    return b'secret/voice-app-secrets\n'
                script = args[-1].replace('-h voice-postgres ', '-h 127.0.0.1 ')
                result = pg(script, body).stdout
                if lost == 'role' and b'CREATE ROLE realtime' in body:
                    calls.append('role'); raise OSError('synthetic SQL reply loss after commit')
                if lost == 'source' and b'CREATE ROLE realtime' in body:
                    calls.append('role')
                    path = installed / 'source-authority' / (operation + '.json')
                    stale = json.loads(path.read_text()); stale['approved_sha256'] = '0' * 64
                    bootstrap.save(path, stale)
                return result
            approved = {'schema': 'voice-source-authority-v1', 'verified': True, 'source_sha': context['commit'],
                        'source_files': {'scripts/staging/nats-rollout-preservation/realtime_bootstrap.py': helper}}
            execution = {'head_sha': context['commit']}
            request = {'action': 'prepare', 'mode': 'full', 'nonce': operation + 'b' * 20}
            actions = bridge_root.Actions(code);actions.tt_session=session
            with mock.patch.object(guard, 'ROOT', root), mock.patch('grp.getgrnam', return_value=SimpleNamespace(gr_gid=0)), \
                 mock.patch.object(stage_runtime, 'Kube', return_value=Kube()), \
                 mock.patch.object(bootstrap, 'engine_snapshot', return_value=('ns', pg_identity)), \
                 mock.patch.object(bootstrap, '_source_contract'), mock.patch.object(commands, 'capture', side_effect=capture), \
                 mock.patch.object(bridge_root, 'INSTALLED', installed):
                session.admit(start=True)
                with resource_claim.command_scope(session.check):
                    bridge_root.record_source_authority(session, approved, execution,code=code,binding=bindings)
                    if lost:
                        with self.assertRaises(bootstrap.Blocked): bootstrap.run_prepare(actions, request, approved, execution, source, stage)
                        checkpoint = json.loads((installed / 'realtime-bootstrap' / operation / 'checkpoint.json').read_text())
                        step = 'role' if lost in ('role', 'source') else 'secret'
                        self.assertEqual(checkpoint['steps'][step]['status'], 'PENDING')
                        self.assertEqual(calls, ['role'] if lost in ('role', 'source') else ['patch'])
                        if lost in ('role', 'source'):
                            catalog = json.loads(pg(bootstrap._PSQL, bootstrap._CATALOG).stdout)
                            self.assertTrue(bootstrap.valid_role(catalog['role']))
                            self.assertIsNone(catalog['database'])
                        session.uncertain('synthetic lost reply')
                        self.assertEqual(journal.read()['status'], 'uncertain')
                    else:
                        receipt = bootstrap.run_prepare(actions, request, approved, execution, source, stage)
                        self.assertEqual(receipt, bootstrap.load_completed(session, actual=stage.old_images))
                        self.assertEqual(secret['data']['keep'], 'c2VudGluZWw=')
                        target = {'registry':'registry','tag':context['commit'],'mode':'full','changed_services':[],
                                  'source_hashes':{'source':'1'*64},'template_hashes':{'voice-realtime':'2'*64},
                                  'input_template_hashes':{'voice-realtime':'3'*64},'images':context['images'],
                                  'manifest_sha256':'4'*64,'migration_sha256':'5'*64,'nonnats_sha256':'6'*64,
                                  'installation_sha256':session.installation.sha256,
                                  'realtime_bootstrap_sha256':bootstrap.digest(receipt)}
                        session.advance_realtime_bootstrap(target, stage.old_images)
                        self.assertEqual(bootstrap.load_completed(session, rollout_target=target), receipt)
                        checkpoint = json.loads((installed / 'realtime-bootstrap' / operation / 'checkpoint.json').read_text())
                        passed = {'schema':'nats-rollout-root-v2','installation_sha256':session.installation.sha256,
                                  'status': 'PASS', 'operation': operation, 'target': target,
                                  'resource_fence': session.binding(), 'code_capture': bindings}
                        self.assertEqual(bootstrap.validate_historical_checkpoint(checkpoint, passed)[1], receipt)
                        changed = copy.deepcopy(checkpoint); changed['receipt']['role_oid'] += 1
                        with self.assertRaises(bootstrap.Blocked): bootstrap.validate_historical_checkpoint(changed, passed)
                        changed = copy.deepcopy(checkpoint)
                        changed['steps']['secret']['readback'] = {'uid': 'bogus'}
                        changed['receipt']['secret'] = {'uid': 'bogus'}
                        changed['receipt']['steps_sha256'] = bootstrap.digest(changed['steps'])
                        altered = copy.deepcopy(passed)
                        altered['target']['realtime_bootstrap_sha256'] = bootstrap.digest(changed['receipt'])
                        altered['resource_fence']['target_sha256'] = bootstrap.digest(altered['target'])
                        with self.assertRaises(bootstrap.Blocked): bootstrap.validate_historical_checkpoint(changed, altered)
                        secret['data']['keep'] = 'Y2hhbmdlZA=='
                        with self.assertRaises(bootstrap.Blocked): bootstrap.load_completed(session, rollout_target=target)


if __name__ == '__main__':
    unittest.main()
