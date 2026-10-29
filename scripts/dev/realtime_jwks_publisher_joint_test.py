"""Real predecessor loader and P3 cryptography at synthetic Kube/TT boundaries."""
import copy
import json
from pathlib import Path
import unittest
from unittest.mock import patch

import full_cohort_predecessor_phase as predecessor
import full_cohort_predecessor_phase_test as pred_fixture
import media_backup_phase_test as base_fixture
import media_operation_journal
import realtime_jwks_publisher as publisher
import realtime_p3_existing as p3
import realtime_p3_existing_test as p3_fixture
import resource_fencing


class JointTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        p3_fixture.ExistingTest.setUpClass()
        cls.addClassCleanup(p3_fixture.ExistingTest.doClassCleanups)

    def setUp(self):
        self.p3 = p3_fixture.ExistingTest('test_real_crypto_returns_public_identities_only'); self.p3.setUp()
        self.addCleanup(self.p3.doCleanups)
        self.base = base_fixture.PhaseTest('test_exact_terminal_receipt_and_no_replay'); self.base.setUp()
        self.addCleanup(self.base.doCleanups)

    def test_real_loader_and_crypto_bind_publisher_dependency(self):
        ctx = copy.deepcopy(self.base.context)
        ctx['operation_id'] = 'media-c2-' + 'a' * 32
        self.p3.context.clear(); self.p3.context.update(ctx)
        current = p3.observe(self.p3.fence, self.p3.client, {}, 'ns', '/source')
        old = pred_fixture.completed(); old['realtime_p3_handoff']['namespace_uid'] = 'ns'
        handoff = old['realtime_p3_handoff']; receipt = handoff['receipt']
        receipt['namespace_uid'] = 'ns'; receipt['issuer_source_sha'] = p3.source.REVIEWED_ISSUER_SHA
        handoff['issuer_source_sha'] = receipt['issuer_source_sha']
        receipt['public_sources'] = copy.deepcopy(current['public_sources'])
        for effect in old['realtime_p3_objects'].values():
            identity = current['objects'][effect['name']]
            effect['uid'], effect['resourceVersion'] = identity['uid'], identity['resourceVersion']
        receipt['objects'] = copy.deepcopy(old['realtime_p3_objects'])
        item = old['realtime_jwks_publisher']
        item['p3_handoff_sha256'] = predecessor.digest(handoff)
        for field, key in (('public', 'configmap-voice-p3-principal-public'), ('tls', 'secret-voice-p3-principal-jwks-tls')):
            for identity in ('uid', 'resourceVersion'): item[field][identity] = old['realtime_p3_objects'][key][identity]
        predecessor.validate_p3(old, 'ns'); predecessor.validate_publisher(old, 'ns')
        snapshot = {'schema':'voice-full-cohort-predecessor-snapshot-v1', 'operation_id':ctx['operation_id'],
                    'namespace_uid':'ns', 'ledger_uid':'ledger', 'ledger_resourceVersion':'8', 'old_row_sha256':'e'*64,
                    'previous_operation_id':old['operation_id'], 'previous_commit':old['commit'], 'deployment':{},
                    'p3_receipt':copy.deepcopy(receipt), 'p3_handoff':copy.deepcopy(handoff),
                    'p3_objects':copy.deepcopy(old['realtime_p3_objects']), 'publisher':copy.deepcopy(item)}
        descriptor = {'schema':'voice-full-cohort-predecessor-v1', 'snapshot_sha256':predecessor.digest(snapshot),
                      'old_row_sha256':snapshot['old_row_sha256'], 'ledger_uid':'ledger', 'ledger_resourceVersion':'8',
                      'previous_operation_id':old['operation_id']}
        row = ctx | {'status':'active', 'step':'admitted', 'predecessor':descriptor}
        document = {'metadata':{'name':resource_fencing.LEDGER, 'namespace':'voice-dev', 'uid':'ledger', 'resourceVersion':'9'},
                    'data':{'executor':json.dumps(row)}}
        self.p3.client.call = lambda *args, **kwargs: copy.deepcopy(document)
        ledger = resource_fencing.KubeLedger(self.p3.client)
        check = resource_fencing.ResourceCheck(predecessor.TT_ENDPOINT, ctx)
        fence = resource_fencing.Fence(ctx, check, ledger)
        journal = media_operation_journal.Journal(Path(self.base.temp.name) / 'joint',
                   {'resource_context':ctx, 'namespace_uid':'ns'})
        self.addCleanup(journal.close); self.p3.client.journal = journal
        journal.state['steps']['full-cohort-predecessor'] = {'status':'captured', 'descriptor':descriptor, 'snapshot':snapshot}
        journal.save()
        phase = object.__new__(publisher.PublisherReuse)
        phase.predecessor_fence, phase.predecessor_client, phase.predecessor_journal = fence, self.p3.client, journal
        phase.fence, phase.lock, phase.namespace_uid, phase.snapshot = fence, {}, 'ns', snapshot
        phase.plan = {'source_repo':'/source', 'source_sha':ctx['commit']}
        with patch.object(predecessor, '_no_tt_proxy'):
            self.assertEqual(predecessor.load(fence, self.p3.client, {}, 'ns', journal), snapshot)
            proof = phase._p3()
            self.assertEqual(proof['scope'], 'c2-p3-exact-predecessor-reuse')
            self.p3.items['Secret','voice-p3-principal-replay']['metadata']['resourceVersion'] = 'changed'
            with self.assertRaises(p3.ExistingError): phase._p3()


if __name__ == '__main__': unittest.main()
