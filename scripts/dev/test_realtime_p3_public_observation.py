import json
import unittest
from unittest.mock import patch

import realtime_p3_public_observation as probe


def obj(kind, name, keys):
    return {'apiVersion': 'v1', 'kind': kind,
            'metadata': {'namespace': 'voice-dev', 'name': name, 'uid': name, 'resourceVersion': '1'},
            'data': {key: 'eA==' if kind == 'Secret' else 'x' for key in keys},
            **({'immutable': True} if name == 'voice-p3-principal-public' else {})}


class ObservationTests(unittest.TestCase):
    def setUp(self):
        self.objects = {name: obj(kind, name, keys) for name, (kind, keys) in probe.OBJECTS.items()}

    def run_probe(self, objects=None, guard=lambda: None):
        objects = self.objects if objects is None else objects
        def read(kind, name):
            return json.dumps(objects[name]).encode() if name in objects else b''
        with patch.object(probe.reset, 'verify_p3_public'), patch.object(probe.reset, 'verify_certificate', return_value={}):
            return probe.inspect(read, guard)

    def test_missing_is_observed_not_ready(self):
        del self.objects['voice-realtime-p3-tls']
        result = self.run_probe()
        self.assertEqual(result['assets']['voice-realtime-p3-tls'], {'status': 'missing'})
        self.assertFalse(result['admission_qualified'])

    def test_stale_resource_version_fails_closed(self):
        count = 0
        def read(kind, name):
            nonlocal count
            count += 1
            value = obj(kind, name, probe.OBJECTS[name][1])
            if count > len(probe.OBJECTS): value['metadata']['resourceVersion'] = '2'
            return json.dumps(value).encode()
        self.assertEqual(probe.inspect(read, lambda: None), {'status': 'hold', 'reason': 'observation_failed'})

    def test_guard_or_private_exception_is_redacted(self):
        def guard(): raise RuntimeError('secret material')
        with self.assertRaises(probe.metadata.GuardLost) as error:
            self.run_probe(guard=guard)
        self.assertNotIn('secret material', str(error.exception))

    def test_false_guard_stops_further_reads(self):
        calls = []
        def read(kind, name):
            calls.append(name)
            return json.dumps(self.objects[name]).encode()
        def guard(): return len(calls) < 1
        with self.assertRaises(probe.metadata.GuardLost):
            probe.inspect(read, guard)
        self.assertEqual(len(calls), 1)

    def test_inventory_reports_only_allowlisted_key_names(self):
        self.objects['voice-app-secrets']['data']['unexpected-private-name'] = 'eA=='
        result = self.run_probe()
        self.assertNotIn('unexpected-private-name', repr(result))
        self.assertNotIn('eA==', repr(result))

    def test_invalid_crypto_never_outputs_fingerprints(self):
        def read(kind, name): return json.dumps(self.objects[name]).encode()
        with patch.object(probe.reset, 'verify_p3_public', side_effect=ValueError('private payload')), \
             patch.object(probe.reset, 'verify_certificate', side_effect=ValueError('private payload')):
            result = probe.inspect(read, lambda: None)
        self.assertEqual(result['public_hashes'], {})
        self.assertNotIn('private payload', repr(result))
        self.assertFalse(result['admission_qualified'])


if __name__ == '__main__': unittest.main()
