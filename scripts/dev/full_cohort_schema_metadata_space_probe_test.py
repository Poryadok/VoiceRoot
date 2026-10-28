import unittest
import json
from unittest import mock
import full_cohort_schema_metadata_space_probe as probe


class SpaceProbeTest(unittest.TestCase):
    def test_absent_and_empty_follow_different_source_rules(self):
        values = {'SPACE_AUDIT_PRINCIPAL_GRPC_LISTEN': '',
                  'SPACE_CHAT_COMPOSITE_GRPC_LISTEN': ''}
        result = probe.classify(values)
        self.assertEqual(result['audit']['activation'], 'configured')
        self.assertEqual(result['composite']['activation'], 'disabled')
        self.assertEqual(result['authority_source']['activation'], 'disabled')

    def test_source_flag_and_composite_partial_do_not_claim_validity(self):
        result = probe.classify({'SPACE_AUTHORITY_SOURCE_ENABLED': 'maybe',
                                 'SPACE_CHAT_COMPOSITE_GRPC_LISTEN': ':9090'})
        self.assertEqual(result['authority_source']['activation'], 'invalid')
        self.assertEqual(result['composite']['activation'], 'configured')
        self.assertEqual(result['composite']['shape'], 'incomplete')
        self.assertNotIn(':9090', repr(result))

    def test_unknown_or_duplicate_private_env_is_rejected(self):
        with self.assertRaises(probe.Hold):
            probe.decode_private(b'SPACE_LIFECYCLE_ENABLED=true\0SPACE_LIFECYCLE_ENABLED=false\0')
        with self.assertRaises(probe.Hold):
            probe.decode_private(b'PASSWORD=secret\0')

    def test_probe_redacts_values_and_fences_identity(self):
        source = {'source_sha': 'a' * 40, 'files': dict(probe.SOURCE_FILES)}
        pod = {key: 'value' for key in probe.common.IDENTITY}
        pod['restart_count'] = 0
        engine = {'uid': 'engine'}
        session = {'database_oid': 5, 'server_start': 'time'}
        baseline = {'engine': engine, 'session': session, 'ledger': {'version': 25, 'dirty': False, 'ledger_rows': 1}}
        dbrow = {'database': 'space_db', 'database_oid': 5, 'server_start': 'time', 'role': 'voice',
                 'read_only': 'on', 'data_directory': '/var/lib/postgresql/data', 'version': 25,
                 'dirty': False, 'ledger_rows': 1, 'native_guard_present': False, 'ordinary_heads_present': False,
                 'composite_fences_present': False, 'composite_evidence_present': False,
                 'purge_fences_present': False, 'audit_lock_present': False}
        with mock.patch.object(probe.common, 'kube_workload_snapshot', return_value=lambda: pod.copy()), \
             mock.patch.object(probe.common, 'kube_key_reader'), mock.patch.object(probe.common, 'kube_runtime_reader'), \
             mock.patch.object(probe.common, 'kube_dns_reader'), mock.patch.object(probe.common, 'catalog'), \
             mock.patch.object(probe.common, 'resolve', return_value={'binding': 'fixed'}), \
             mock.patch.object(probe.common, 'kube_engine_snapshot', return_value=lambda: engine.copy()), \
             mock.patch.object(probe.common, 'observe', return_value=baseline):
            result = probe.probe(source, 'voice-space-a', 'voice', lambda: None,
                                 lambda _: b'SPACE_CHAT_COMPOSITE_PRINCIPAL_REDIS_PASSWORD=secret\0',
                                 lambda _: json.dumps(dbrow))
        self.assertEqual(result['status'], 'HOLD')
        self.assertEqual(result['source_catalog'], 'not_evaluated')
        self.assertNotIn('secret', repr(result))


if __name__ == '__main__':
    unittest.main()
