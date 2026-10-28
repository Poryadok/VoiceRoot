"""Fault cases for the shared DEV authority check, with no live writes."""
import copy
import json
import unittest
from unittest import mock

import media_operation_guard as guard
import resource_fencing


class Fence:
    def __init__(self, context):
        self.context = context
        self.row = context | {'status': 'active', 'step': 'admitted'}
        self.ledger = self
        self.calls = 0

    def authority(self):
        self.calls += 1
        return {}

    def read(self):
        return copy.deepcopy(self.row), ('ledger-uid', '42')

    def owned(self, row):
        return all(row.get(k) == v for k, v in self.context.items())


class Client:
    def __init__(self, namespace, lock):
        self.namespace, self.lock = copy.deepcopy(namespace), copy.deepcopy(lock)
        self.ledger = None

    def call(self, *args):
        if args[1] == 'namespace': return copy.deepcopy(self.namespace)
        return copy.deepcopy(self.ledger if args[2] == resource_fencing.LEDGER else self.lock)


class GuardTest(unittest.TestCase):
    def setUp(self):
        self.context = {'project': 'voice', 'resource': 'DEV', 'actor': 'scheduled_devops',
                        'claim': '11111111-1111-4111-8111-111111111111',
                        'window': '22222222-2222-4222-8222-222222222222',
                        'revision': 4, 'generation': 3, 'operation_id': 'media-op',
                        'commit': 'a' * 40,
                        'images': {'voice-file/file': 'ghcr.io/x@sha256:' + 'b' * 64}}
        self.namespace = {'metadata': {'name': 'voice-dev', 'uid': 'ns', 'labels': guard.LABELS}}
        self.lock = {'metadata': {'name': guard.LOCK, 'namespace': 'voice-dev', 'uid': 'lock',
                                  'resourceVersion': '8', 'labels': guard.LABELS},
                     'data': {'operation': 'token', 'resource_operation': 'media-op',
                              'resource_context': json.dumps(self.context, sort_keys=True, separators=(',', ':'))}}
        self.fence = Fence(self.context)
        self.client = Client(self.namespace, self.lock)

    def test_owned_active_authority(self):
        guard.verify(self.fence, self.client, self.lock, 'ns')
        self.assertEqual(self.fence.calls, 2)

    def test_uncertain_fence_wrong_namespace_and_lock_drift_refused(self):
        for mutation in ('uncertain', 'namespace', 'lock', 'actor'):
            with self.subTest(mutation=mutation):
                self.setUp()
                if mutation == 'uncertain': self.fence.row['status'] = 'uncertain'
                if mutation == 'namespace': self.client.namespace['metadata']['uid'] = 'other'
                if mutation == 'lock': self.client.lock['metadata']['resourceVersion'] = '9'
                if mutation == 'actor': self.fence.context['actor'] = 'other'
                with self.assertRaises(ValueError):
                    guard.verify(self.fence, self.client, self.lock, 'ns')

    def test_ledger_revision_and_concurrent_drift(self):
        for values in ((('uid', ''), ('uid', '')), (('uid', '1'), ('uid', '2'))):
            with self.subTest(values=values):
                self.setUp()
                row = copy.deepcopy(self.fence.row)
                with mock.patch.object(self.fence, 'read', side_effect=[(row, values[0]), (row, values[1])]):
                    with self.assertRaises(ValueError):
                        guard.verify(self.fence, self.client, self.lock, 'ns')

    def test_real_fence_and_ledger_detect_lost_tt_authority(self):
        context = self.context
        card = dict(code='DEV', environment='dev', owner=context['actor'], claim=context['claim'],
                    generation=context['generation'], window=context['window'], revision=context['revision'],
                    state='held', phase='deploy', candidate={'state': 'known', 'commit': context['commit'],
                                                             'images': context['images']})
        self.client.ledger = {'metadata': {'name': resource_fencing.LEDGER, 'namespace': 'voice-dev',
                                           'uid': 'ledger-uid', 'resourceVersion': '42'},
                              'data': {'executor': json.dumps(self.fence.row)}}
        check = mock.Mock(side_effect=[card, card])
        real = resource_fencing.Fence(context, check, resource_fencing.KubeLedger(self.client))
        guard.verify(real, self.client, self.lock, 'ns')
        changed = copy.deepcopy(card)
        changed['candidate']['commit'] = 'c' * 40
        real = resource_fencing.Fence(context, mock.Mock(side_effect=[card, changed]),
                                      resource_fencing.KubeLedger(self.client))
        with self.assertRaises(RuntimeError):
            guard.verify(real, self.client, self.lock, 'ns')


if __name__ == '__main__':
    unittest.main()
