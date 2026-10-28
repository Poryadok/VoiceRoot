"""Host-only tests for the TT cooperative executor boundary; no stand access."""
import copy
import io
import json
import unittest
from unittest.mock import Mock, patch

import resource_fencing as fencing


def identity():
    return dict(project='voice', resource='DEV', actor='operator', claim='11111111-1111-4111-8111-111111111111',
                generation=7, window='22222222-2222-4222-8222-222222222222', revision=12,
                operation_id='dev-qualification-1', commit='c'*40,
                images={'voice-user/user':'registry/user@sha256:'+'c'*64})


def card(context):
    return dict(code='DEV', environment='dev', state='held', owner=context['actor'],
                claim=context['claim'], generation=context['generation'], window=context['window'],
                revision=context['revision'], phase='deploy', operation=2,
                candidate=dict(state='known', commit=context['commit'], images=context['images']),
                observed=dict(state='known', claim=context['claim'], window=context['window'], operation=1,
                              commit='b'*40, images={'voice-user/user':'registry/user@sha256:'+'b'*64}))


class MemoryLedger:
    def __init__(self): self.row=None; self.revision=0
    def read(self): return copy.deepcopy(self.row), self.revision
    def write(self, previous, row):
        if previous != self.revision: raise RuntimeError('CAS conflict')
        self.row=copy.deepcopy(row); self.revision+=1


class FenceTests(unittest.TestCase):
    def make(self, store=None):
        context=identity(); current=card(context)
        check=Mock(side_effect=lambda:copy.deepcopy(current))
        fence=fencing.Fence(context,check,store or MemoryLedger())
        return fence,current,check

    def test_required_identity_and_exact_authority_not_actor_label(self):
        for key in identity():
            context=identity(); del context[key]
            with self.subTest(key=key), self.assertRaises(ValueError):
                fencing.Fence(context,Mock(),MemoryLedger())
        for key in ('claim','generation','window','revision','owner','phase','state'):
            fence,current,_=self.make(); current[key]='stale'
            with self.subTest(key=key), self.assertRaises(RuntimeError): fence.begin()

    def test_recheck_after_admission_blocks_paused_old_executor(self):
        fence,current,check=self.make(); fence.begin()
        current['generation']+=1
        with self.assertRaises(RuntimeError): fence.before_step('patch-user')
        self.assertGreaterEqual(check.call_count,2)

    def test_closed_generation_and_operation_cannot_replay(self):
        ledger=MemoryLedger(); fence,_,_=self.make(ledger)
        fence.begin(); fence.complete({'voice-user/user':identity()['images']['voice-user/user']})
        retry,_,_=self.make(ledger)
        with self.assertRaises(RuntimeError): retry.begin()
        self.assertEqual(ledger.row['status'],'complete')

    def test_active_or_uncertain_executor_blocks_new_claim_until_reconciliation(self):
        for status in ('active','uncertain'):
            ledger=MemoryLedger(); fence,_,_=self.make(ledger); fence.begin()
            if status=='uncertain': fence.uncertain('lost-response')
            context=identity(); context.update(generation=8, claim='33333333-3333-4333-8333-333333333333')
            newer=fencing.Fence(context,lambda:card(context),ledger)
            with self.subTest(status=status),self.assertRaises(RuntimeError): newer.begin()
            self.assertEqual(ledger.row['generation'],7)

    def test_same_actor_new_claim_admitted_only_after_closed_previous_generation(self):
        ledger=MemoryLedger(); first,_,_=self.make(ledger); first.begin(); first.complete(identity()['images'])
        context=identity(); context.update(generation=8,claim='33333333-3333-4333-8333-333333333333',operation_id='dev-next')
        newer=fencing.Fence(context,lambda:card(context),ledger); newer.begin()
        with self.assertRaises(RuntimeError): first.before_step('late-patch')
        self.assertEqual(ledger.row['claim'],context['claim'])

    def test_baseline_is_actual_and_candidate_is_exact_not_rollback(self):
        fence,current,_=self.make(); fence.begin()
        baseline=current['observed']['images']
        ancestry=Mock(return_value=True)
        fence.validate_baseline(baseline,ancestry)
        ancestry.assert_called_once_with('b'*40,'c'*40)
        with self.assertRaises(RuntimeError): fence.validate_baseline(identity()['images'],ancestry)
        with self.assertRaises(RuntimeError): fence.validate_baseline(baseline,lambda old,new:False)
        current['candidate']['commit']='d'*40
        with self.assertRaises(RuntimeError): fence.before_step('wrong-candidate')

    def test_unknown_response_is_durable_and_not_completion(self):
        ledger=MemoryLedger(); fence,_,_=self.make(ledger); fence.begin(); fence.before_step('patch-user')
        fence.uncertain('transport-or-partial-apply')
        self.assertEqual(ledger.row['status'],'uncertain')
        self.assertEqual(ledger.row['step'],'patch-user')
        self.assertEqual(ledger.row['operation_id'],'dev-qualification-1')
        with self.assertRaises(RuntimeError): fence.before_step('retry')

    def test_competing_writers_cas_has_single_winner(self):
        ledger=MemoryLedger(); first,_,_=self.make(ledger); second,_,_=self.make(ledger)
        first.begin()
        with self.assertRaises(RuntimeError): second.begin()
        self.assertEqual(ledger.revision,1)

    def test_http_check_uses_actual_ttenrec_envelope_and_exact_query(self):
        context=identity(); resource=card(context)
        with patch.object(fencing.urllib.request,'build_opener') as factory:
            factory.return_value.open.return_value.__enter__.return_value=io.BytesIO(
                json.dumps(dict(valid=True,resource=resource,guarantee='TT admission only')).encode())
            self.assertEqual(fencing.ResourceCheck('http://localhost:8080',context)(),resource)
            url=factory.return_value.open.call_args.args[0]
            self.assertIn('/api/projects/voice/resources/DEV/check?',url)
            self.assertIn('claim='+context['claim'],url)
            self.assertIn('revision=12',url)

    def test_uncertain_persists_before_reporting_and_never_retries_lost_response(self):
        ledger=MemoryLedger(); context=identity(); notifications=[]
        def report(reason):
            notifications.append(ledger.row['status'])
            raise RuntimeError('response lost; reread before any new action')
        fence=fencing.Fence(context,lambda:card(context),ledger,on_uncertain=report)
        fence.begin()
        with self.assertRaises(RuntimeError): fence.uncertain('lost-response')
        self.assertEqual(notifications,['uncertain'])
        self.assertEqual(ledger.row['status'],'uncertain')

    def test_last_executor_journal_is_not_global_operation_id_deduplication(self):
        ledger=MemoryLedger(); old,_,_=self.make(ledger); old.begin(); old.complete(identity()['images'])
        for generation,operation in ((8,'second-operation'),(9,identity()['operation_id'])):
            context=identity(); context.update(generation=generation,operation_id=operation)
            context['claim']=str(__import__('uuid').uuid4())
            current=fencing.Fence(context,lambda:card(context),ledger)
            current.begin(); current.complete(context['images'])
        self.assertEqual(ledger.row['generation'],9)
        with self.assertRaises(RuntimeError): old.before_step('stale-generation')


if __name__=='__main__': unittest.main()
