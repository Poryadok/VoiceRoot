import copy
import json
import subprocess
import sys
import unittest
from pathlib import Path
from unittest.mock import patch
import tempfile
import os

import operation_journal as journal


UID = '15950fc8-0392-4053-9156-e95e78b596f9'


def fixture():
    return {
        'namespace': {'metadata': {'name': 'voice-dev', 'uid': UID,
            'resourceVersion': '40', 'labels': journal.LABELS}},
        'lock': {'metadata': {'name': journal.LOCK, 'namespace': 'voice-dev',
            'uid': UID, 'resourceVersion': '41', 'creationTimestamp': '2026-10-21T10:45:07Z'},
            'data': {'operation': 'd35062ce8e2caa6c907df8e14bcb990d'}},
        'workloads': {'versions': {'Deployment': '42', 'StatefulSet': '44'}, 'items': [
            {'kind': 'Deployment', 'metadata': {'name': 'voice-user', 'namespace': 'voice-dev',
             'uid': UID, 'resourceVersion': '43', 'generation': 2},
             'spec': {'template': {'spec': {'containers': [{'name': 'user',
                'image': 'ghcr.io/poryadok/voiceroot/user@sha256:' + 'a' * 64,
                'env': [{'name': 'PASSWORD', 'value': 'NEVER_DISCLOSE'}]}]}}},
             'status': {'observedGeneration': 2, 'availableReplicas': 1}}]},
    }


class JournalTest(unittest.TestCase):
    def test_read_only_projection_is_partial_and_redacts_all_unselected_text(self):
        raw = fixture()
        raw['lock']['data']['password'] = 'NEVER_DISCLOSE'
        raw['lock']['metadata']['annotations'] = {'token': 'NEVER_DISCLOSE'}
        raw['workloads']['items'][0]['status']['message'] = 'NEVER_DISCLOSE'
        raw['workloads']['items'][0]['spec']['template']['spec']['containers'][0]['env'].extend(
            {'name': name, 'value': value} for name, value in {
                'TOKEN': 'eyJhbGci.NEVER_DISCLOSE.signature',
                'PRIVATE_KEY': '-----BEGIN PRIVATE KEY-----NEVER_DISCLOSE',
                'DSN': 'postgres://user:NEVER_DISCLOSE@host/database',
                'SIGNED_URL': 'https://host/object?signature=NEVER_DISCLOSE',
                'CUSTOMER_DATA': 'NEVER_DISCLOSE',
            }.items())
        result = journal.snapshot(lambda: copy.deepcopy(raw))
        self.assertEqual(result['state'], 'PARTIAL')
        self.assertEqual(result['consistency'], 'STABLE_OBSERVATION')
        self.assertEqual(result['protected_inventory']['state'], 'UNKNOWN')
        self.assertIsNone(result['candidate'])
        self.assertIsNone(result['request_id'])
        self.assertFalse(result['mutation_authority'])
        self.assertNotIn('NEVER_DISCLOSE', json.dumps(result))
        self.assertEqual(result['namespace']['uid'], UID)
        self.assertEqual(result['lock']['operation_id'], 'd35062ce8e2caa6c907df8e14bcb990d')

    def test_changed_namespace_lock_or_workload_suppresses_mixed_evidence(self):
        for section in ('namespace', 'lock', 'workloads'):
            first, second = fixture(), fixture()
            if section == 'workloads':
                second[section]['versions']['Deployment'] = '999'
            else:
                second[section]['metadata']['resourceVersion'] = '999'
            rows = iter((first, second))
            result = journal.snapshot(lambda: next(rows))
            self.assertEqual(result['state'], 'UNAVAILABLE')
            self.assertEqual(result['reason'], 'changed_during_observation')
            self.assertNotIn('lock', result)

    def test_foreign_or_missing_authority_fails_closed_without_raw_errors(self):
        for mutate in (
                lambda r: r['namespace']['metadata'].update(name='voice-staging'),
                lambda r: r['namespace']['metadata'].update(labels={}),
                lambda r: r['namespace']['metadata'].update(deletionTimestamp='now'),
                lambda r: r['lock']['metadata'].update(namespace='voice-staging')):
            raw = fixture()
            mutate(raw)
            self.assertEqual(journal.snapshot(lambda: raw)['state'], 'UNAVAILABLE')
        def failed():
            raise RuntimeError('postgres://user:NEVER_DISCLOSE@host/db?token=SECRET')
        result = journal.snapshot(failed)
        self.assertEqual(result['state'], 'UNAVAILABLE')
        self.assertNotIn('NEVER_DISCLOSE', json.dumps(result))

    def test_values_in_allowlisted_fields_cannot_smuggle_free_text(self):
        for key in ('uid', 'resourceVersion'):
            raw = fixture()
            raw['lock']['metadata'][key] = 'https://private/?token=NEVER_DISCLOSE'
            result = journal.snapshot(lambda: raw)
            self.assertNotIn('NEVER_DISCLOSE', json.dumps(result))
            self.assertEqual(result['state'], 'UNAVAILABLE')
        raw = fixture()
        raw['workloads']['items'][0]['spec']['template']['spec']['containers'][0]['image'] = 'user:mutable'
        result = journal.snapshot(lambda: raw)
        self.assertIsNone(result['workloads'][0]['images'][0]['immutable_image'])

    def test_cli_denies_mutation_and_arbitrary_file_selectors(self):
        for action in ('--apply', '--unlock', '--recover', '--path=/etc/shadow', '--namespace=voice-staging'):
            proc = subprocess.run([sys.executable, str(Path(journal.__file__)), action], capture_output=True)
            self.assertNotEqual(proc.returncode, 0)
            self.assertNotIn(b'NEVER_DISCLOSE', proc.stdout + proc.stderr)

    def test_collector_issues_only_exact_dev_gets(self):
        calls = []
        def fake(*args):
            calls.append(args)
            if args[-1] in ('deployment', 'statefulset'):
                return {'metadata': {'resourceVersion': '42'}, 'items': []}
            return {}
        with patch.object(journal, 'kubectl', fake):
            journal.collect()
        self.assertEqual(calls, [('get', 'namespace', 'voice-dev'),
            ('-n', 'voice-dev', 'get', 'configmap', journal.LOCK),
            ('-n', 'voice-dev', 'get', 'deployment'),
            ('-n', 'voice-dev', 'get', 'statefulset')])

    @unittest.skipUnless(os.name == 'posix', 'POSIX directory descriptor contract')
    def test_known_receipt_observation_never_opens_private_payloads(self):
        with tempfile.TemporaryDirectory(dir=Path.home()) as directory:
            root = Path(directory)
            root.chmod(0o700)
            path = root / 'started.json'
            path.write_text('NEVER_DISCLOSE original_env token dsn customer data')
            path.chmod(0o600)
            with patch('builtins.open', side_effect=AssertionError('payload read forbidden')):
                result = journal.receipt_markers(root, ('started.json', 'complete.json'), os.getuid())
            self.assertEqual(result['state'], 'OBSERVED_MARKERS_ONLY')
            self.assertEqual(result['markers'][0]['name'], 'started.json')
            self.assertFalse(result['markers'][1]['present'])
            self.assertNotIn('NEVER_DISCLOSE', json.dumps(result))

    @unittest.skipUnless(os.name == 'posix', 'POSIX ownership and symlink security contract')
    def test_insecure_or_symlink_receipts_are_unknown_not_absent(self):
        with tempfile.TemporaryDirectory(dir=Path.home()) as directory:
            root = Path(directory)
            root.chmod(0o700)
            (root / 'started.json').symlink_to('/etc/shadow')
            result = journal.receipt_markers(root, ('started.json',), os.getuid())
            self.assertEqual(result['state'], 'UNKNOWN')


if __name__ == '__main__':
    unittest.main()
