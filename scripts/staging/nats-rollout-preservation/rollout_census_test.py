import copy
import unittest
import rollout_census as module

ACCOUNT = 'A' + 'B' * 55
CREATED = '2026-10-05T10:00:00Z'


def consumer(stream, name, ephemeral=False):
    return {'stream_name': stream, 'name': name, 'created': CREATED,
        'config': {'name': name, 'durable_name': '' if ephemeral else name,
            'inactive_threshold': 120000000000 if ephemeral else 0, 'ack_policy': 'explicit',
            'filter_subject': 'private.subject', 'metadata': {'private': 'authority'}},
        'delivered': {'consumer_seq': 5, 'stream_seq': 7, 'last_active': CREATED},
        'ack_floor': {'consumer_seq': 3, 'stream_seq': 3, 'last_active': CREATED},
        'num_ack_pending': 2, 'num_redelivered': 1, 'num_pending': 3, 'num_waiting': 0, 'ts': CREATED}


def stream(name, consumers):
    return {'name': name, 'created': CREATED, 'config': {'name': name, 'storage': 'file', 'max_msgs': -1,
        'subjects': ['private.subject']}, 'state': {'messages': 10, 'bytes': 100, 'first_seq': 1,
        'last_seq': 10, 'consumer_count': len(consumers)}, 'consumer_detail': consumers}


def fixture():
    streams = [stream('BASE', [consumer('BASE', 'durable')]),
        stream('EXTRA_DYNAMIC', [consumer('EXTRA_DYNAMIC', 'additional'), consumer('EXTRA_DYNAMIC', 'ephemeral', True)])]
    return {'streams': 2, 'consumers': 3, 'messages': 20, 'bytes': 200, 'total': 2,
        'account_details': [{'id': ACCOUNT, 'name': 'display-tag', 'stream_detail': streams},
            {'id': '$SYS', 'name': 'system', 'memory': 0, 'storage': 0}]}


class Runtime:
    def __init__(self, tree): self.tree = tree
    def monitor_jsz(self, broker): return copy.deepcopy(self.tree)


class CensusTests(unittest.TestCase):
    def census(self, tree=None): return module.census(Runtime(tree or fixture()), 'owned-copy', ACCOUNT)

    def test_dynamic_extra_durable_ephemeral_full_delivery_state(self):
        result = self.census()
        self.assertEqual(result['account'], ACCOUNT)
        self.assertEqual([s['name'] for s in result['streams']], ['BASE', 'EXTRA_DYNAMIC'])
        self.assertEqual(len(result['consumers']), 3)
        durable = next(c for c in result['consumers'] if c['name'] == 'durable')
        self.assertEqual(durable['num_ack_pending'], 2)
        self.assertEqual(durable['num_redelivered'], 1)
        self.assertEqual(durable['ack_floor'], {'consumer_seq': 3, 'stream_seq': 3})
        self.assertEqual(len(durable['config_sha256']), 64)

    def test_global_or_consumer_counts_truncation_rejected(self):
        for field in ('streams', 'consumers', 'messages', 'bytes', 'total'):
            tree = fixture(); tree[field] += 1
            with self.subTest(field=field), self.assertRaises(module.CensusError): self.census(tree)
        tree = fixture(); tree['account_details'][0]['stream_detail'][1]['consumer_detail'].pop()
        with self.assertRaises(module.CensusError): self.census(tree)

    def test_foreign_populated_account_rejected(self):
        tree = fixture(); tree['account_details'][1]['stream_detail'] = [stream('FOREIGN', [])]
        with self.assertRaises(module.CensusError): self.census(tree)

    def test_display_name_cannot_substitute_public_identity(self):
        tree = fixture(); tree['account_details'][0].update(id='foreign', name=ACCOUNT)
        with self.assertRaises(module.CensusError): self.census(tree)

    def test_config_and_ack_forgery_changes_semantic(self):
        first = self.census()
        tree = fixture(); c = tree['account_details'][0]['stream_detail'][0]['consumer_detail'][0]
        c['config']['metadata']['private'] = 'changed'; c['ack_floor']['consumer_seq'] = 2
        second = self.census(tree)
        self.assertNotEqual(module.semantic(first), module.semantic(second))

    def test_observed_activity_only_does_not_change_semantics(self):
        first = self.census()
        tree = fixture(); c = tree['account_details'][0]['stream_detail'][0]['consumer_detail'][0]
        c['ts'] = '2026-10-05T10:01:00Z'; c['delivered']['last_active'] = c['ts']; c['num_waiting'] = 1
        second = self.census(tree)
        self.assertNotEqual(first, second)
        self.assertEqual(module.semantic(first), module.semantic(second))

    def test_negative_noninteger_or_ack_ahead_rejected(self):
        for value in (-1, 1.5, True, '2'):
            tree = fixture(); tree['account_details'][0]['stream_detail'][0]['consumer_detail'][0]['num_ack_pending'] = value
            with self.subTest(value=value), self.assertRaises(module.CensusError): self.census(tree)
        tree = fixture(); tree['account_details'][0]['stream_detail'][0]['consumer_detail'][0]['ack_floor']['consumer_seq'] = 6
        with self.assertRaises(module.CensusError): self.census(tree)

    def test_missing_config_duplicate_identity_and_implicit_ephemeral_rejected(self):
        for mutate in (lambda s: s.pop('config'), lambda s: s['consumer_detail'].append(copy.deepcopy(s['consumer_detail'][0])),
            lambda s: s['consumer_detail'][0]['config'].update(durable_name='', inactive_threshold=0)):
            tree = fixture(); mutate(tree['account_details'][0]['stream_detail'][0])
            with self.subTest(), self.assertRaises(module.CensusError): self.census(tree)

    def test_complete_stream_state_and_config_changes_are_preserved(self):
        first = module.semantic(self.census())
        for change in ('deleted', 'config', 'created'):
            tree = fixture(); s = tree['account_details'][0]['stream_detail'][0]
            if change == 'deleted': s['state']['deleted'] = [2, 4]
            elif change == 'config': s['config']['metadata'] = {'new': 'private-value'}
            else: s['created'] = '2026-10-05T09:00:00Z'
            with self.subTest(change=change): self.assertNotEqual(first, module.semantic(self.census(tree)))

    def test_unknown_consumer_state_cannot_be_silently_discarded(self):
        tree = fixture(); tree['account_details'][0]['stream_detail'][0]['consumer_detail'][0]['unrecognized_persisted_state'] = 1
        with self.assertRaises(module.CensusError): self.census(tree)

    def test_anonymous_config_empty_name_is_valid_ephemeral(self):
        tree = fixture(); tree['account_details'][0]['stream_detail'][1]['consumer_detail'][1]['config']['name'] = ''
        self.assertEqual(len(self.census(tree)['consumers']), 3)


if __name__ == '__main__': unittest.main()
