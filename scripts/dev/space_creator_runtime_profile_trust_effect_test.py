"""Closed truststore classification and private custody boundaries."""

import base64
import copy
import hashlib
import json
import os
from pathlib import Path
import unittest
from unittest import mock

import full_cohort_inventory_phase as inventory
import full_cohort_package_phase as package
import frontend_runtime_executor as transport
import media_operation_journal as journals
import resource_fencing
import space_creator_runtime_profile_jdk as jdk
import space_creator_runtime_profile_trust_effect as effect
import dev_peer_issuer_preflight as issuer
import dev_peer_issuer_metadata as peer_metadata
import writer_cut
import writer_cut_test


def listing(*entries, store_type='PKCS12'):
    rows = ['Keystore type: ' + store_type, 'Your keystore contains %d entries' % len(entries)]
    for kind, digest in entries:
        rows += ['Entry type: ' + kind, 'SHA256: ' + ':'.join(digest[i:i + 2].upper() for i in range(0, 64, 2))]
    return ('\n'.join(rows) + '\n').encode()


class TrustClassificationTest(unittest.TestCase):
    def test_compatible_store_is_preserved_and_public_mismatch_is_replaceable(self):
        roots = {'a' * 64}; ca = 'b' * 64
        self.assertEqual(effect.classify_listing(listing(('trustedCertEntry', 'a' * 64),
                                                         ('trustedCertEntry', ca)), roots, ca), 'preserve')
        self.assertEqual(effect.classify_listing(listing(('trustedCertEntry', 'c' * 64)), roots, ca), 'replace')

    def test_unknown_store_and_private_entries_are_never_replaceable(self):
        roots = {'a' * 64}; ca = 'b' * 64
        for raw in (b'not a keystore', listing(('trustedCertEntry', 'a' * 64), store_type='JKS'),
                    listing(('PrivateKeyEntry', 'a' * 64))):
            with self.subTest(raw=raw), self.assertRaises(effect.TrustEffectError):
                effect.classify_listing(raw, roots, ca)

    def test_secret_metadata_and_other_keys_are_preserved(self):
        value = {'apiVersion': 'v1', 'kind': 'Secret', 'type': 'Opaque',
                 'metadata': {'name': 'voice-media-jvm-trust', 'namespace': 'voice-dev', 'uid': 'uid', 'resourceVersion': '2'},
                 'data': {'truststore.p12': base64.b64encode(b'store').decode(), 'other': base64.b64encode(b'preserve').decode()}}
        self.assertEqual(effect.secret_data(value)['other'], b'preserve')
        for field in ('uid', 'resourceVersion'):
            bad = {**value, 'metadata': {k: v for k, v in value['metadata'].items() if k != field}}
            with self.assertRaises(effect.TrustEffectError): effect.secret_data(bad)
        with self.assertRaises(effect.TrustEffectError): effect.secret_data(value | {'type': 'kubernetes.io/tls'})


class EffectStateTest(unittest.TestCase):
    """Actual phase/journal transitions with synthetic external authorities and tools."""

    def setUp(self):
        self.base = writer_cut_test.WriterCutTest('test_drain_and_resume_use_current_template_not_old')
        self.base.setUp(); self.addCleanup(self.base.doCleanups)
        base = self.base; client_type = type(base.client)

        class Ledger:
            def __init__(self, client): self.client = client
            def read(self): return base.ledger.read()
            def write(self, revision, row): return base.ledger.write(revision, row)

        self.enterContext(mock.patch.object(resource_fencing, 'KubeLedger', Ledger))

        def facade(name, extra=None):
            intent = {'resource_context': copy.deepcopy(base.context), 'namespace_uid': 'ns', **(extra or {})}
            journal = journals.Journal(Path(base.temp.name) / name, intent); self.addCleanup(journal.close)
            client = client_type(base.lock, journal); client.workloads = base.client.workloads
            fence = resource_fencing.Fence(base.context, lambda: copy.deepcopy(base.card), Ledger(client))
            return journal, fence, client
        self.facade = facade

        nodes = [{'name': 'node', 'uid': 'node', 'hostname': 'node', 'architecture': 'amd64'}]
        j, f, c = facade('effect-package', {'full_cohort_package': {'run_id': 1, 'run_attempt': 1, 'nodes': nodes}})
        pkg = package.PackagePhase(j, f, c, base.lock, 'ns')
        j, f, c = facade('effect-inventory'); inv = inventory.InventoryPhase(j, f, c, base.lock, 'ns', pkg)
        j, f, c = facade('effect-jdk'); actual_jdk = jdk.TargetJdkPhase(j, f, c, base.lock, 'ns', pkg, inv)
        j, f, c = facade('effect-issuer', {'dev_peer_issuer_preflight': {'source_root': '/fixture', 'source_sha': base.context['commit'], 'profile': 'creator'}})
        actual_issuer = issuer.PeerIssuerPreflightPhase(j, f, c, base.lock, 'ns', pkg, base.cut, '/fixture', 'token', 'creator')
        j, f, c = facade('effect-result')
        self.phase = effect.TrustEffectPhase(j, f, c, base.lock, 'ns', pkg, inv, actual_jdk, actual_issuer)
        self.roots = {'root_fingerprints': ['a' * 64], 'image': 'image', 'config_digest': 'sha256:' + 'd' * 64}
        self.ca_sha = hashlib.sha256(b'der').hexdigest()
        self.source = {'schema': 'voice-dev-peer-custody-v1', 'profile': 'creator',
                       'scope': 'root-peer-custody-available-not-issued', 'operation_id': base.context['operation_id'],
                       'source_sha': base.context['commit'], 'namespace_uid': 'ns',
                       'nonce': 'f' * 32, 'context_sha256': writer_cut.digest(base.context),
                       'lock': {key: base.lock['metadata'][key] for key in ('uid', 'resourceVersion')},
                       'ca': {'uid': 'ca', 'resourceVersion': '1', 'der_sha256': self.ca_sha},
                       'creator_secrets': {name: {'state': 'absent'} for name in peer_metadata.MEMBERS}}
        actual_jdk.record = {'status': 'done', 'receipt_sha256': writer_cut.digest(self.roots)}
        actual_jdk.journal.state['steps'][jdk.STEP] = copy.deepcopy(actual_jdk.record) | {'receipt': copy.deepcopy(self.roots)}
        actual_issuer.record = {'status': 'done', 'receipt_sha256': writer_cut.digest(self.source)}
        actual_issuer.journal.state['steps'][actual_issuer.step] = copy.deepcopy(actual_issuer.record) | {'receipt': copy.deepcopy(self.source)}
        actual_jdk.journal.save(); actual_issuer.journal.save()
        base.ledger.row[jdk.LEDGER] = copy.deepcopy(actual_jdk.record)
        base.ledger.row[actual_issuer.ledger] = copy.deepcopy(actual_issuer.record)
        self.enterContext(mock.patch.object(jdk, 'verify_completed', return_value=self.roots))
        self.enterContext(mock.patch.object(issuer, 'verify_current', return_value=self.source))
        self.enterContext(mock.patch.object(effect.target.material, '_cert_der', return_value=b'der'))
        self.secret = None; self.writes = []; self.lost = False
        self.enterContext(mock.patch.object(effect.private, '_run', side_effect=self.kube))
        self.enterContext(mock.patch.object(self.phase, '_tool', side_effect=self.tool))
        if self._testMethodName != 'test_undrained_cut_rejects_before_tool_or_secret':
            base.cut.drain()

    def tool(self, roots, script, payload):
        if script == effect.GENERATE: return b'generated-public-store'
        if payload == b'generated-public-store':
            return listing(('trustedCertEntry', 'a' * 64), ('trustedCertEntry', self.ca_sha))
        if payload == b'compatible':
            return listing(('trustedCertEntry', 'a' * 64), ('trustedCertEntry', self.ca_sha))
        if payload == b'incompatible': return listing(('trustedCertEntry', 'c' * 64))
        return listing(('PrivateKeyEntry', 'a' * 64))

    def kube(self, args, payload=None):
        if args[:3] == ['get', 'secret', 'voice-media-ca']:
            return b'ca\n1\n\n' + base64.b64encode(b'public-ca')
        if args[:3] == ['get', 'secret', effect.SECRET]:
            return b'' if self.secret is None else json.dumps(self.secret).encode()
        self.writes.append(copy.deepcopy(args))
        if args[0] == 'create':
            self.secret = json.loads(payload); self.secret['metadata'].update(uid='trust-uid', resourceVersion='1')
        elif args[0] == 'patch':
            self.assertIn('--patch-file=/dev/stdin', args)
            ops = json.loads(payload); meta = self.secret['metadata']
            self.assertEqual([op['value'] for op in ops[:2]], [meta['uid'], meta['resourceVersion']])
            self.secret['data'] = ops[2]['value']; meta['resourceVersion'] = str(int(meta['resourceVersion']) + 1)
        else: raise AssertionError(args)
        if self.lost: raise OSError('synthetic committed reply lost')
        return json.dumps(self.secret).encode()

    def existing(self, store):
        self.secret = {'apiVersion': 'v1', 'kind': 'Secret', 'type': 'Opaque',
                       'metadata': {'name': effect.SECRET, 'namespace': 'voice-dev', 'uid': 'trust-uid', 'resourceVersion': '2'},
                       'data': {effect.KEY: base64.b64encode(store).decode(), 'other': base64.b64encode(b'preserve').decode()}}

    def prepare_run(self):
        self.phase.prepare('token')
        return self.phase.run('token')

    def test_missing_create_has_private_keyed_receipt_and_detects_drift(self):
        receipt = self.prepare_run()
        self.assertEqual(receipt['mode'], 'create'); self.assertEqual(len(self.writes), 1)
        self.assertNotIn('generated-public-store', json.dumps(receipt))
        self.assertEqual(effect.verify_current(self.phase, 'token'), receipt)
        self.secret['data'][effect.KEY] = base64.b64encode(b'changed').decode()
        with self.assertRaises(effect.TrustEffectError): effect.verify_current(self.phase, 'token')

    def test_existing_compatible_is_preserved_and_incompatible_uses_cas(self):
        self.existing(b'compatible'); before = copy.deepcopy(self.secret)
        receipt = self.prepare_run()
        self.assertEqual(receipt['mode'], 'preserve'); self.assertEqual(self.secret, before); self.assertEqual(self.writes, [])

    def test_known_incompatible_uses_cas_and_preserves_other_key(self):
        self.existing(b'incompatible'); receipt = self.prepare_run()
        self.assertEqual(receipt['mode'], 'replace'); self.assertEqual(len(self.writes), 1)
        self.assertEqual(self.secret['metadata']['uid'], 'trust-uid')
        self.assertEqual(base64.b64decode(self.secret['data']['other']), b'preserve')

    def test_missing_key_uses_cas_and_preserves_other_key(self):
        self.existing(b'compatible'); del self.secret['data'][effect.KEY]
        receipt = self.prepare_run()
        self.assertEqual(receipt['mode'], 'replace'); self.assertEqual(len(self.writes), 1)
        self.assertEqual(base64.b64decode(self.secret['data']['other']), b'preserve')

    def test_checkpoint_failure_has_no_secret_or_tool_effect(self):
        with mock.patch.object(self.phase.journal, 'save', side_effect=OSError('fixture')):
            with self.assertRaises(effect.TrustEffectError): self.phase.prepare('token')
        self.assertEqual(self.writes, []); self.phase._tool.assert_not_called()

    def test_changed_secret_before_cas_is_rejected(self):
        self.existing(b'incompatible')
        def changed(roots, script, payload):
            result = self.tool(roots, script, payload)
            if script == effect.GENERATE: self.secret['metadata']['resourceVersion'] = '3'
            return result
        self.phase.prepare('token')
        self.phase._tool.side_effect = changed
        with self.assertRaises(effect.TrustEffectError): self.phase.run('token')
        self.assertEqual(self.writes, [])

    def test_undrained_cut_rejects_before_tool_or_secret(self):
        self.phase.prepare('token')
        tool_calls = self.phase._tool.call_count
        with self.assertRaises(effect.TrustEffectError): self.phase.run('token')
        self.assertEqual(self.writes, []); self.assertEqual(self.phase._tool.call_count, tool_calls)
        self.assertEqual(self.phase.journal.state['steps'][effect.STEP]['status'], 'prepared')

    def test_resumed_cut_before_cas_is_rejected(self):
        self.existing(b'incompatible')
        def resumed(roots, script, payload):
            result = self.tool(roots, script, payload)
            if script == effect.GENERATE:
                targets = [copy.deepcopy(item) for item in self.base.client.workloads.values()]
                self.base.cut.resume(targets)
            return result
        self.phase.prepare('token')
        self.phase._tool.side_effect = resumed
        with self.assertRaises(effect.TrustEffectError): self.phase.run('token')
        self.assertEqual(self.writes, [])

    def test_replaced_private_key_inode_is_rejected(self):
        self.phase._key(create=True)
        replacement = self.phase.journal.path / 'other.key'
        replacement.write_bytes(b'x' * 32); replacement.chmod(0o600)
        os.replace(replacement, self.phase.journal.path / effect.KEY_FILE)
        with self.assertRaises(ValueError): self.phase._key()

    def test_wrong_peer_profile_is_rejected_at_construction(self):
        base = self.base; pkg = self.phase.package; inv = self.phase.inventory; peer = self.phase.preflight
        intent = {'dev_peer_issuer_preflight': {'source_root': '/fixture', 'source_sha': base.context['commit'], 'profile': 'space'}}
        j, f, c = self.facade('wrong-peer', intent)
        foreign = issuer.PeerIssuerPreflightPhase(j, f, c, base.lock, 'ns', pkg, base.cut, '/fixture', 'token', 'space')
        j, f, c = self.facade('wrong-effect')
        with self.assertRaises(effect.TrustEffectError):
            effect.TrustEffectPhase(j, f, c, base.lock, 'ns', pkg, inv, self.phase.jdk, foreign)

    def test_private_store_is_rejected_without_writing(self):
        self.existing(b'private')
        with self.assertRaises(effect.TrustEffectError): self.phase.prepare('token')
        self.assertEqual(self.writes, [])
        with self.assertRaises(effect.target.metadata.GuardLost): self.phase.prepare('token')

    def test_committed_lost_reply_requires_reconciliation(self):
        self.lost = True
        self.phase.prepare('token')
        with self.assertRaises(effect.TrustEffectError): self.phase.run('token')
        self.assertIsNotNone(self.secret); self.assertEqual(len(self.writes), 1)
        with self.assertRaises(effect.target.metadata.GuardLost): self.phase.run('token')
        self.assertEqual(len(self.writes), 1)

    def test_actual_tool_state_machine_uses_tmpfs_and_cleans_exact_ids(self):
        commands = []; current = {}; cid = 'd' * 64
        def docker(args, limit=1048576):
            commands.append(copy.deepcopy(args))
            if args[:2] == ['image', 'inspect']:
                return json.dumps([{'Id': self.roots['config_digest'], 'Os': 'linux', 'Architecture': 'amd64',
                    'RepoDigests': [self.roots['image']], 'Config': {'Volumes': None}}]).encode()
            if args[0] == 'create':
                current['name'] = args[args.index('--name') + 1]; current['script'] = args[-1]
                self.assertIn('--log-driver', args); self.assertIn('none', args)
                self.assertNotIn('public-ca', repr(args)); return cid.encode()
            if args[0] == 'inspect':
                return json.dumps([{'Id': cid, 'Image': self.roots['config_digest'], 'Config': {
                    'Labels': {'voice.test.owner': current['name']}, 'Entrypoint': ['/bin/sh'],
                    'Cmd': ['-ec', current['script']], 'User': '65532:65532', 'OpenStdin': True},
                    'HostConfig': {'NetworkMode': 'none', 'ReadonlyRootfs': True, 'CapDrop': ['ALL'],
                    'SecurityOpt': ['no-new-privileges'], 'Tmpfs': {'/scratch': effect.target.TMPFS},
                    'LogConfig': {'Type': 'none'}}, 'Mounts': [], 'State': {'Status': 'exited', 'ExitCode': 0}}]).encode()
            if args[0] == 'rm': self.assertEqual(args[-1], cid); return b''
            if args[0] == 'ps': return b''
            raise AssertionError(args)
        def input_data(args, payload):
            self.assertEqual(args, ['docker', 'start', '-ai', cid])
            if current['script'] == effect.GENERATE:
                self.assertEqual(payload, b'public-ca'); return b'generated-public-store'
            self.assertEqual(payload, b'generated-public-store')
            return listing(('trustedCertEntry', 'a' * 64), ('trustedCertEntry', self.ca_sha))
        actual_tool = effect.TrustEffectPhase._tool.__get__(self.phase)
        with mock.patch.object(self.phase, '_tool', actual_tool), mock.patch.object(self.phase, '_docker', side_effect=docker), \
             mock.patch.object(effect.target.metadata, '_bounded_process', side_effect=lambda args, limit: docker(args[1:], limit)), \
             mock.patch.object(effect.target, '_input', side_effect=input_data):
            receipt = self.prepare_run()
        self.assertEqual(receipt['mode'], 'create')
        self.assertEqual(sum(args[0] == 'rm' for args in commands), 4)
        self.assertEqual(self.phase.record['tool']['status'], 'removed')

    def _timeout_owned_container(self, guard_lost):
        commands = []; current = {}; cid = 'e' * 64
        def docker(args, limit=1048576):
            commands.append(copy.deepcopy(args))
            if args[:2] == ['image', 'inspect']:
                return json.dumps([{'Id': self.roots['config_digest'], 'Os': 'linux', 'Architecture': 'amd64',
                    'RepoDigests': [self.roots['image']], 'Config': {'Volumes': None}}]).encode()
            if args[0] == 'create':
                current['name'] = args[args.index('--name') + 1]; current['script'] = args[-1]
                return cid.encode()
            if args[0] == 'inspect':
                return json.dumps([{'Id': cid, 'Image': self.roots['config_digest'], 'Config': {
                    'Labels': {'voice.test.owner': current['name']}, 'Entrypoint': ['/bin/sh'],
                    'Cmd': ['-ec', current['script']], 'User': '65532:65532', 'OpenStdin': True},
                    'HostConfig': {'NetworkMode': 'none', 'ReadonlyRootfs': True, 'CapDrop': ['ALL'],
                    'SecurityOpt': ['no-new-privileges'], 'Tmpfs': {'/scratch': effect.target.TMPFS},
                    'LogConfig': {'Type': 'none'}}, 'Mounts': [], 'State': {'Status': 'running'}}]).encode()
            if args[0] == 'rm': self.assertEqual(args[-1], cid); return b''
            if args[0] == 'ps': return b''
            raise AssertionError(args)
        actual_tool = effect.TrustEffectPhase._tool.__get__(self.phase)
        def guarded_docker(args, limit=1048576):
            self.phase._guard()
            return docker(args, limit)
        def timeout(args, payload):
            if guard_lost: self.base.card['generation'] += 1
            raise TimeoutError('fixture')
        with mock.patch.object(self.phase, '_tool', actual_tool), mock.patch.object(self.phase, '_docker', side_effect=guarded_docker), \
             mock.patch.object(effect.target.metadata, '_bounded_process', side_effect=lambda args, limit: docker(args[1:], limit)), \
             mock.patch.object(effect.target, '_input', side_effect=timeout):
            with self.assertRaises((effect.TrustEffectError, effect.target.metadata.GuardLost)):
                self.phase.prepare('token')
        self.assertEqual([args for args in commands if args[0] == 'rm'], [['rm', '-f', '-v', cid]])
        if not guard_lost:
            self.assertEqual(self.phase.journal.state['steps'][effect.STEP]['tool']['status'], 'removed')
        self.assertEqual(self.writes, [])

    def test_timed_out_owned_container_is_removed_before_reconciliation(self):
        self._timeout_owned_container(False)

    def test_timed_out_owned_container_is_removed_after_tt_guard_loss(self):
        self._timeout_owned_container(True)

    def test_unknown_create_reply_never_attempts_name_based_cleanup(self):
        def docker(args, limit=1048576):
            if args[:2] == ['image', 'inspect']:
                return json.dumps([{'Id': self.roots['config_digest'], 'Os': 'linux', 'Architecture': 'amd64',
                    'RepoDigests': [self.roots['image']], 'Config': {'Volumes': None}}]).encode()
            if args[0] == 'create': raise TimeoutError('lost create reply')
            raise AssertionError(args)
        actual_tool = effect.TrustEffectPhase._tool.__get__(self.phase)
        with mock.patch.object(self.phase, '_tool', actual_tool), mock.patch.object(self.phase, '_docker', side_effect=docker), \
             mock.patch.object(self.phase, '_cleanup_owned') as cleanup:
            with self.assertRaises(effect.TrustEffectError): self.phase.prepare('token')
            cleanup.assert_not_called()
        self.assertEqual(self.writes, [])
        with self.assertRaises(effect.target.metadata.GuardLost): self.phase.prepare('token')

    def test_cleanup_rejects_foreign_owner_without_remove(self):
        cid = 'f' * 64; name = 'voice-creator-trust-effect-' + 'a' * 32
        def bounded(args, limit):
            self.assertEqual(args, ['docker', 'inspect', cid])
            return json.dumps([{'Id': cid, 'Image': 'sha256:' + 'd' * 64,
                                'Config': {'Labels': {'voice.test.owner': 'foreign'}}}]).encode()
        with mock.patch.object(effect.target.metadata, '_bounded_process', side_effect=bounded) as called:
            with self.assertRaises(ValueError): effect.TrustEffectPhase._cleanup_owned(cid, name, 'sha256:' + 'd' * 64)
        self.assertEqual(called.call_count, 1)


if __name__ == '__main__':
    unittest.main()
