"""Host-only tests for the TT cooperative executor boundary; no stand access."""
import copy
import io
import json
import unittest
import tempfile
from pathlib import Path
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

    def test_image_only_begin_binds_original_revision_and_lost_reply_is_uncertain(self):
        import full_cohort_image_only_baseline as baseline
        from types import SimpleNamespace
        context=identity();context['operation_id']='media-c1-'+'a'*32
        class Ledger(MemoryLedger):
            def __init__(self):super().__init__();self.row={'status':'complete'};self.revision=('uid','old')
            def write(self,revision,row):
                if revision!=self.revision:raise RuntimeError('CAS conflict')
                self.row=copy.deepcopy(row);self.revision=('uid','new')
        ledger=Ledger();check=fencing.ResourceCheck('http://192.168.0.109:30990',context)
        journal=SimpleNamespace(dirfd=1,lockfd=2,state={'steps':{baseline.STEP:{'status':'captured'}}},save=lambda:None)
        descriptor={'snapshot_sha256':'d'*64}; seen=[]; notices=[]
        def capture(_f,_c,_l,_n,_j,previous,revision):
            seen.append((previous,revision));return descriptor
        with patch.object(fencing.ResourceCheck,'__call__',return_value=card(context)), \
             patch.object(fencing.ResourceCheck,'mark_uncertain',autospec=True,side_effect=lambda _s,r:notices.append(r)), \
             patch.object(baseline,'capture',side_effect=capture):
            fence=fencing.Fence(context,check,ledger,on_uncertain=check.mark_uncertain)
            fence.begin_image_only(None,None,'ns',journal)
            self.assertEqual(seen, [({'status':'complete'},('uid','old'))])
            self.assertEqual(ledger.row['image_only_baseline'],descriptor)
            ledger.row={'status':'complete'};ledger.revision=('uid','old')
            journal.state['steps'].pop('image-only-admission')
            write=ledger.write
            def lost(revision,row):write(revision,row);raise OSError('lost reply')
            with patch.object(ledger,'write',side_effect=lost),self.assertRaises(OSError):
                fence.begin_image_only(None,None,'ns',journal)
            self.assertEqual(ledger.row['image_only_baseline'],descriptor)
            self.assertEqual(journal.state['steps']['image-only-admission']['status'],'pending')
            self.assertEqual(notices,['image-only begin outcome uncertain'])
            with self.assertRaises(RuntimeError):fence.begin_image_only(None,None,'ns',journal)

    def test_image_only_lost_remote_ack_stays_pending_after_reopen(self):
        import full_cohort_image_only_baseline as baseline
        import media_operation_journal
        context=identity();context['operation_id']='media-c1-'+'a'*32
        class Ledger(MemoryLedger):
            def __init__(self):super().__init__();self.row={'status':'complete'};self.revision=('uid','old')
            def write(self,revision,row):
                if revision!=self.revision:raise RuntimeError('CAS conflict')
                self.row=copy.deepcopy(row);self.revision=('uid','new');raise OSError('lost reply')
        ledger=Ledger();check=fencing.ResourceCheck('http://192.168.0.109:30990',context)
        intent={'resource_context':context,'namespace_uid':'ns','cohort_route_mode':'full-cohort-v1'}
        with tempfile.TemporaryDirectory() as folder:
            path=Path(folder)/'journal'
            with media_operation_journal.Journal(path,intent) as journal, \
                 patch.object(fencing.ResourceCheck,'__call__',return_value=card(context)), \
                 patch.object(fencing.ResourceCheck,'mark_uncertain',autospec=True,side_effect=OSError('unknown notice')):
                def capture(_f,_c,_l,_n,j,_p,_r):
                    descriptor={'snapshot_sha256':'d'*64}
                    j.state['steps'][baseline.STEP]={'status':'captured','descriptor':descriptor};j.save()
                    return descriptor
                with patch.object(baseline,'capture',side_effect=capture):
                    fence=fencing.Fence(context,check,ledger,on_uncertain=check.mark_uncertain)
                    with self.assertRaises(OSError):fence.begin_image_only(None,None,'ns',journal)
            with media_operation_journal.Journal(path,intent) as reopened:
                self.assertEqual(reopened.state['steps']['image-only-admission']['status'],'pending')
                self.assertEqual(ledger.row['status'],'active')

    def test_predecessor_begin_lost_reply_marks_uncertain(self):
        import full_cohort_predecessor_phase as predecessor
        from types import SimpleNamespace
        context=identity(); context['operation_id']='media-c2-'+'a'*32
        ledger=MemoryLedger(); ledger.row={'status':'complete'};ledger.revision=('uid','old')
        journal=SimpleNamespace(dirfd=1,lockfd=2,state={'steps':{}},save=lambda:None)
        def capture(_f,_c,_l,_n,j,_p,_r):
            descriptor={'snapshot_sha256':'d'*64};j.state['steps']['full-cohort-predecessor']={'status':'captured','descriptor':descriptor};return descriptor
        notices=[]; check=fencing.ResourceCheck('http://192.168.0.109:30990',context)
        def lost(previous,row):
            ledger.row=copy.deepcopy(row); ledger.revision=('uid','new')
            raise OSError('reply lost')
        with patch.object(fencing.ResourceCheck,'__call__',return_value=card(context)), \
             patch.object(fencing.ResourceCheck,'mark_uncertain',autospec=True,side_effect=lambda obj,reason:notices.append(reason)), \
             patch.object(predecessor,'capture',side_effect=capture), patch.object(ledger,'write',side_effect=lost):
            fence=fencing.Fence(context,check,ledger,on_uncertain=check.mark_uncertain)
            with self.assertRaises(OSError): fence.begin_with_predecessor(None,None,'ns',journal)
        self.assertEqual(ledger.row['status'],'active')
        self.assertEqual(ledger.row['predecessor']['snapshot_sha256'],'d'*64)
        self.assertEqual(journal.state['steps']['predecessor-admission']['status'],'pending')
        self.assertEqual(notices,['C2 predecessor begin outcome uncertain'])

    def test_predecessor_requires_uncertainty_handler_before_capture(self):
        context=identity(); context['operation_id']='media-c2-'+'a'*32
        fence=fencing.Fence(context,lambda:card(context),MemoryLedger())
        with self.assertRaises(RuntimeError): fence.begin_with_predecessor(None,None,'ns',None)

    def test_predecessor_lost_ack_stays_pending_after_reopen(self):
        import full_cohort_predecessor_phase as predecessor
        import media_operation_journal
        context=identity();context['operation_id']='media-c2-'+'a'*32
        class Ledger(MemoryLedger):
            def __init__(self):super().__init__();self.row={'status':'complete'};self.revision=('uid','old')
            def write(self,revision,row):
                if revision!=self.revision:raise RuntimeError('CAS conflict')
                self.row=copy.deepcopy(row);self.revision=('uid','new');raise OSError('lost reply')
        ledger=Ledger();check=fencing.ResourceCheck('http://192.168.0.109:30990',context)
        intent={'resource_context':context,'namespace_uid':'ns','cohort_route_mode':'full-cohort-v1'}
        with tempfile.TemporaryDirectory() as folder:
            path=Path(folder)/'journal'
            with media_operation_journal.Journal(path,intent) as journal, \
                 patch.object(fencing.ResourceCheck,'__call__',return_value=card(context)), \
                 patch.object(fencing.ResourceCheck,'mark_uncertain',autospec=True,side_effect=OSError('unknown notice')):
                def capture(_f,_c,_l,_n,j,_p,_r):
                    descriptor={'snapshot_sha256':'d'*64}
                    j.state['steps']['full-cohort-predecessor']={'status':'captured','descriptor':descriptor};j.save()
                    return descriptor
                with patch.object(predecessor,'capture',side_effect=capture):
                    fence=fencing.Fence(context,check,ledger,on_uncertain=check.mark_uncertain)
                    with self.assertRaises(OSError):fence.begin_with_predecessor(None,None,'ns',journal)
            with media_operation_journal.Journal(path,intent) as reopened:
                self.assertEqual(reopened.state['steps']['predecessor-admission']['status'],'pending')
                self.assertEqual(ledger.row['status'],'active')

    def test_predecessor_ack_and_readback_confirm_admission(self):
        import full_cohort_predecessor_phase as predecessor
        from types import SimpleNamespace
        context=identity();context['operation_id']='media-c2-'+'a'*32
        class Ledger(MemoryLedger):
            def __init__(self):super().__init__();self.row={'status':'complete'};self.revision=('uid','old')
            def write(self,revision,row):
                if revision!=self.revision:raise RuntimeError('CAS conflict')
                self.row=copy.deepcopy(row);self.revision=('uid','new')
        ledger=Ledger();check=fencing.ResourceCheck('http://192.168.0.109:30990',context)
        journal=SimpleNamespace(dirfd=1,lockfd=2,state={'steps':{}},save=lambda:None)
        def capture(_f,_c,_l,_n,j,_p,_r):
            descriptor={'snapshot_sha256':'d'*64};j.state['steps']['full-cohort-predecessor']={'status':'captured','descriptor':descriptor};return descriptor
        with patch.object(fencing.ResourceCheck,'__call__',return_value=card(context)), \
             patch.object(predecessor,'capture',side_effect=capture),patch.object(predecessor,'load') as load:
            fence=fencing.Fence(context,check,ledger,on_uncertain=check.mark_uncertain)
            fence.begin_with_predecessor(None,None,'ns',journal)
            self.assertEqual(journal.state['steps']['predecessor-admission']['status'],'done')
            load.assert_called_once_with(fence,None,None,'ns',journal)

    def test_predecessor_callback_failure_preserves_unknown_effect(self):
        import full_cohort_predecessor_phase as predecessor
        from types import SimpleNamespace
        context=identity(); context['operation_id']='media-c2-'+'a'*32
        ledger=MemoryLedger(); ledger.row={'status':'complete'};ledger.revision=('uid','old')
        journal=SimpleNamespace(dirfd=1,lockfd=2,state={'steps':{}},save=lambda:None)
        def capture(_f,_c,_l,_n,j,_p,_r):
            descriptor={'snapshot_sha256':'d'*64};j.state['steps']['full-cohort-predecessor']={'status':'captured','descriptor':descriptor};return descriptor
        check=fencing.ResourceCheck('http://192.168.0.109:30990',context)
        def lost(previous,row): ledger.row=copy.deepcopy(row); raise OSError('reply lost')
        with patch.object(fencing.ResourceCheck,'__call__',return_value=card(context)), \
             patch.object(fencing.ResourceCheck,'mark_uncertain',autospec=True,side_effect=RuntimeError('notice failed')), \
             patch.object(predecessor,'capture',side_effect=capture), patch.object(ledger,'write',side_effect=lost):
            fence=fencing.Fence(context,check,ledger,on_uncertain=check.mark_uncertain)
            with self.assertRaises(OSError): fence.begin_with_predecessor(None,None,'ns',journal)
        self.assertEqual(ledger.row['status'],'active')
        self.assertEqual(journal.state['steps']['predecessor-admission']['status'],'pending')

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
