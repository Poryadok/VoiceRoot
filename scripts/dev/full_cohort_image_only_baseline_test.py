import copy
import hashlib
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import full_cohort_image_only_baseline as baseline
import frontend_runtime_executor
import media_operation_guard as guard
import media_operation_journal
import resource_fencing


class HistoryTests(unittest.TestCase):
    def history(self):
        image = 'registry/voice@sha256:' + 'c' * 64
        before = {'revision': 0, 'operation': 2, 'owner': 'scheduled_devops',
                  'claim': 'claim', 'window': 'window', 'generation': 1,
                  'state': 'held', 'phase': 'deploy',
                  'candidate': {'state': 'known', 'commit': 'a' * 40,
                                'images': {'voice-auth/auth': image}}}
        after = copy.deepcopy(before)
        after.update(revision=1, phase='verify', observed={
            'state': 'known', 'actor': 'scheduled_devops', 'claim': 'claim',
            'window': 'window', 'operation': 2, 'commit': 'a' * 40,
            'images': {'voice-auth/auth': image}, 'deployment': 'deployment-1'})
        return {'code': 'DEV', 'environment': 'dev', 'revision': 1,
                'events': [{'id': 'r1', 'kind': 'deployment', 'actor': 'scheduled_devops',
                            'before': before, 'after': after}]}

    def test_full_history_requires_contiguity_and_source_qualified_deployments(self):
        history = self.history()
        self.assertEqual(baseline.history_floors(history), ['a' * 40])
        for fault in ('gap', 'tip', 'observed', 'candidate', 'ambiguous'):
            with self.subTest(fault=fault):
                bad = copy.deepcopy(history)
                if fault == 'gap': bad['events'][0]['id'] = 'r2'
                if fault == 'tip': bad['revision'] = 2
                if fault == 'observed': bad['events'][0]['after']['observed']['claim'] = 'other'
                if fault == 'candidate': bad['events'][0]['after']['candidate']['commit'] = 'b' * 40
                if fault == 'ambiguous': bad['events'][0]['after']['observed'].pop('commit')
                with self.assertRaises(baseline.BaselineError):
                    baseline.history_floors(bad)

    def test_two_hundred_events_are_not_treated_as_truncated(self):
        card = self.history()
        previous = card['events'][0]['after']
        for index in range(2, 201):
            after = copy.deepcopy(previous); after['revision'] = index
            card['events'].append({'id':'r'+str(index), 'kind':'checkpoint', 'before':previous, 'after':after})
            previous = after
        card['revision'] = 200
        self.assertEqual(baseline.history_floors(card), ['a'*40])
        card['events'].pop(80)
        with self.assertRaises(baseline.BaselineError): baseline.history_floors(card)


class CaptureTests(unittest.TestCase):
    def test_real_journal_and_fence_reject_drift_before_any_ledger_write(self):
        context = dict(project='voice', resource='DEV', actor='scheduled_devops',
            claim='11111111-1111-4111-8111-111111111111', window='22222222-2222-4222-8222-222222222222',
            generation=2, revision=2, operation_id='media-c1-'+'b'*32, commit='b'*40,
            images={'voice-auth/auth':'registry/auth@sha256:'+'c'*64})
        old = dict(context, generation=1, operation_id='old', commit='a'*40, status='complete')
        old.update(claim='33333333-3333-4333-8333-333333333333',
                   window='44444444-4444-4444-8444-444444444444', actual=copy.deepcopy(old['images']))
        lock = {'metadata': {'name': guard.LOCK, 'namespace': 'voice-dev', 'uid': 'lock',
                             'resourceVersion': '4', 'labels': dict(guard.LABELS)},
                'data': {'resource_operation': context['operation_id'], 'operation': 'run',
                         'resource_context': json.dumps(context, sort_keys=True, separators=(',', ':'))}}
        namespace = {'metadata': {'name': 'voice-dev', 'uid': 'ns', 'labels': dict(guard.LABELS)}}
        document = {'metadata': {'name': resource_fencing.LEDGER, 'namespace': 'voice-dev',
                                 'uid': 'ledger', 'resourceVersion': '5'},
                    'data': {'executor': json.dumps(old)}}
        client = frontend_runtime_executor.KubectlTransport.__new__(frontend_runtime_executor.KubectlTransport)
        writes = []
        def call(*args, **kwargs):
            if args[0] == 'patch':
                writes.append(args)
                body = kwargs['body']
                if (body[0]['value'] != document['metadata']['uid'] or
                        body[1]['value'] != document['metadata']['resourceVersion']): raise RuntimeError('CAS')
                document['data'] = copy.deepcopy(body[2]['value'])
                document['metadata']['resourceVersion'] = str(int(document['metadata']['resourceVersion']) + 1)
                return copy.deepcopy(document)
            if args[0] != 'get': raise AssertionError('unexpected write')
            if args[1] == 'namespace': return namespace
            if args[1] == 'node':
                return {'metadata':{'name':'node', 'uid':'node-uid', 'labels':{
                            'kubernetes.io/os':'linux', 'kubernetes.io/arch':'amd64', 'kubernetes.io/hostname':'node'}},
                        'spec':{}, 'status':{'nodeInfo':{'operatingSystem':'linux', 'architecture':'amd64'},
                                             'conditions':[{'type':'Ready','status':'True'}]}}
            if args[2] == guard.LOCK: return lock
            return document
        client.call = call
        check = resource_fencing.ResourceCheck(baseline.predecessor.TT_ENDPOINT, context)
        fence = resource_fencing.Fence(context, check, resource_fencing.KubeLedger(client))
        card = dict(code='DEV', environment='dev', owner=context['actor'], claim=context['claim'],
                    generation=2, window=context['window'], revision=2, operation=3, state='held', phase='deploy',
                    candidate={'state':'known', 'commit':context['commit'], 'images':context['images']})
        history = HistoryTests().history()
        event = history['events'][0]
        for state in (event['before'], event['after']):
            state.update(owner=old['actor'], claim=old['claim'], window=old['window'], generation=1)
            state['candidate']['images'] = copy.deepcopy(old['images'])
        event['after']['observed'].update(actor=old['actor'], claim=old['claim'], window=old['window'],
                                          images=copy.deepcopy(old['images']))
        card['observed'] = copy.deepcopy(event['after']['observed'])
        tip = copy.deepcopy(event['after']); tip.update(card)
        history['events'].append({'id':'r2', 'kind':'claim', 'actor':context['actor'],
                                  'before':copy.deepcopy(event['after']), 'after':tip})
        history['revision'] = 2
        with tempfile.TemporaryDirectory() as root, patch.object(resource_fencing.ResourceCheck, '__call__', return_value=card), \
             patch.object(baseline.predecessor, '_no_tt_proxy'), \
             patch.object(baseline, '_read_history', return_value=history):
            nodes = [{'name':'node','uid':'node-uid','hostname':'node','architecture':'amd64'}]
            intent = {'resource_context': context, 'namespace_uid': 'ns',
                      'full_cohort_package': {'run_id': 1, 'run_attempt': 1, 'nodes':nodes}}
            with media_operation_journal.Journal(root + '/first', intent) as journal:
                client.journal = journal
                descriptor = baseline.capture(fence, client, lock, 'ns', journal, old, ('ledger', '5'))
                self.assertEqual(descriptor['old_row_sha256'], baseline.predecessor.digest(old))
                self.assertEqual(journal.state['steps'][baseline.STEP]['snapshot']['floors'], ['a'*40])
                self.assertEqual(fence.ledger.read(), (old, ('ledger', '5')))
                self.assertEqual(writes, [])
                package_root = Path(root) / context['operation_id']; package_root.mkdir(mode=0o700)
                package_file = package_root / 'canonical-application-package.json'
                package_file.write_bytes(b'{}'); package_file.chmod(0o600)
                receipt = {'operation_id':context['operation_id'], 'source_sha':context['commit'],
                           'images':context['images'], 'nodes':nodes, 'run_id':1, 'run_attempt':1,
                           'package_sha256':hashlib.sha256(b'{}').hexdigest()}
                package_done = {'status':'done', 'receipt_sha256':baseline.predecessor.digest(receipt)}
                active = dict(context, status='active', step='admitted', image_only_baseline=descriptor)
                fence.ledger.write(('ledger','5'), active)  # Models the caller's single begin CAS.
                package_journal = media_operation_journal.Journal(root + '/package', intent)
                package_journal.state['steps'][baseline.full_cohort_package_phase.STEP] = package_done | {'receipt':receipt}
                package_journal.save()
                active['full_cohort_package'] = package_done
                fence.ledger.write(('ledger','6'), active)  # Synthetic package publisher boundary.
                package_client = frontend_runtime_executor.KubectlTransport.__new__(frontend_runtime_executor.KubectlTransport)
                package_client.call = call; package_client.journal = package_journal
                package_fence = resource_fencing.Fence(context, resource_fencing.ResourceCheck(
                    baseline.predecessor.TT_ENDPOINT, context), resource_fencing.KubeLedger(package_client))
                package_phase = baseline.full_cohort_package_phase.PackagePhase(
                    package_journal, package_fence, package_client, lock, 'ns')
                card['observed'] = {'state':'known', 'actor':context['actor'], 'claim':context['claim'],
                                    'window':context['window'], 'operation':2,
                                    'images':{'workload-'+str(i):'image-'+str(i) for i in range(49)}}
                actual = {'namespace_uid':'ns', 'images':copy.deepcopy(card['observed']['images'])}
                comparison = {'base_commit':{'sha':'a'*40}, 'merge_base_commit':{'sha':'a'*40},
                              'status':'ahead', 'ahead_by':1}
                with patch.object(baseline.full_cohort_package_phase, 'ROOT', Path(root)), \
                     patch.object(baseline.media_live_baseline, 'collect', return_value=actual) as collect, \
                     patch.object(baseline.package.source, '_head') as head, \
                     patch.object(baseline.package.source, '_get', return_value=comparison) as compare:
                    initial_writes = len(writes)
                    phase = baseline.ImageOnlyPhase(journal, fence, client, lock, 'ns', package_phase)
                    package_journal.close()
                    with self.assertRaises(baseline.BaselineError): phase.run('synthetic-token')
                    package_journal = media_operation_journal.Journal(root + '/package', intent)
                    package_client.journal = package_journal
                    package_phase = baseline.full_cohort_package_phase.PackagePhase(
                        package_journal, package_fence, package_client, lock, 'ns')
                    phase = baseline.ImageOnlyPhase(journal, fence, client, lock, 'ns', package_phase)
                    original_verify = package_phase.verify_current
                    def close_during_verify(token):
                        result = original_verify(token)
                        package_journal.close()
                        return result
                    with patch.object(package_phase, 'verify_current', side_effect=close_during_verify):
                        with self.assertRaises(baseline.BaselineError): phase.run('synthetic-token')
                    package_journal = media_operation_journal.Journal(root + '/package', intent)
                    package_client.journal = package_journal
                    package_phase = baseline.full_cohort_package_phase.PackagePhase(
                        package_journal, package_fence, package_client, lock, 'ns')
                    phase = baseline.ImageOnlyPhase(journal, fence, client, lock, 'ns', package_phase)
                    def change_package(*args, **kwargs):
                        package_file.write_bytes(b'{"changed":true}')
                        return actual
                    collect.side_effect = change_package
                    with self.assertRaises(baseline.full_cohort_package_phase.PackageError): phase.run('synthetic-token')
                    package_file.write_bytes(b'{}')
                    collect.side_effect = None
                    head.side_effect = ValueError('master moved')
                    with self.assertRaisesRegex(ValueError, 'master moved'): phase.run('synthetic-token')
                    head.side_effect = None
                    compare.return_value = dict(comparison, status='diverged')
                    with self.assertRaises(baseline.BaselineError): phase.run('synthetic-token')
                    compare.return_value = comparison
                    card['observed']['images']['workload-0'] = 'drift'
                    with self.assertRaises(baseline.BaselineError): phase.run('synthetic-token')
                    card['observed']['images']['workload-0'] = 'image-0'
                    self.assertEqual(len(writes), initial_writes)
                    self.assertNotIn(baseline.QUALIFIED, journal.state['steps'])
                    proof = phase.run('synthetic-token')
                    self.assertEqual(proof['floors'], ['a'*40])
                    self.assertEqual(phase.verify_current('synthetic-token'), proof)
                    card['observed']['images']['workload-0'] = 'drift'
                    with self.assertRaises(baseline.BaselineError): phase.verify_current('synthetic-token')
                    card['observed']['images']['workload-0'] = 'image-0'
                    current, current_rev = fence.ledger.read()
                    del current[baseline.QUALIFIED]
                    fence.ledger.write(current_rev, current)
                    del journal.state['steps'][baseline.QUALIFIED]
                    journal.save()
                    writer = fence.ledger.write
                    def lost_response(revision, row):
                        writer(revision, row)
                        if row.get(baseline.QUALIFIED, {}).get('status') == 'done':
                            raise OSError('injected lost response')
                    retry = baseline.ImageOnlyPhase(journal, fence, client, lock, 'ns', package_phase)
                    with patch.object(fence.ledger, 'write', side_effect=lost_response), \
                         patch.object(fence, 'uncertain', side_effect=OSError('injected uncertainty failure')):
                        with self.assertRaisesRegex(OSError, 'lost response'): retry.run('synthetic-token')
                    self.assertTrue(retry.poisoned)
                    self.assertEqual(journal.state['steps'][baseline.QUALIFIED]['marker']['status'], 'pending')
                    with self.assertRaises(baseline.BaselineError): retry.verify_current('synthetic-token')
                    reopened = baseline.ImageOnlyPhase(journal, fence, client, lock, 'ns', package_phase)
                    with self.assertRaises(baseline.BaselineError): reopened.verify_current('synthetic-token')
                package_journal.close()
                document['data']['executor'] = json.dumps(old)
                document['metadata']['resourceVersion'] = '5'
                writes.clear()
            for fault in ('missing', 'revision', 'namespace', 'claim'):
                with self.subTest(fault=fault), media_operation_journal.Journal(root + '/' + fault, intent) as journal:
                    client.journal = journal
                    if fault == 'namespace': namespace['metadata']['uid'] = 'other'
                    if fault == 'claim': card['claim'] = 'other'
                    try:
                        with self.assertRaises((baseline.BaselineError, RuntimeError)):
                            baseline.capture(fence, client, lock, 'ns', journal,
                                None if fault == 'missing' else old, ('ledger', '6') if fault == 'revision' else ('ledger', '5'))
                        self.assertNotIn(baseline.STEP, journal.state['steps'])
                    finally:
                        namespace['metadata']['uid'] = 'ns'; card['claim'] = context['claim']
            self.assertEqual(writes, [])


if __name__ == '__main__':
    unittest.main()
