import copy
import unittest
import pathlib
import tempfile
import contextlib
import io
import json
from unittest.mock import patch
from auth_user_adoption import RecoveryRequired
import auth_user_candidate_probe as probe


def model_card():
    captured=json.loads((pathlib.Path(__file__).parent/'testdata/auth-user-probe-resource-prepare.json').read_text())['card']
    captured.update(claim='claim',generation=2,window='window',revision=42)
    return captured


def resource_binding(card):
    return {**{key:card[key] for key in ('claim','generation','window')},'candidate_sha256':probe.digest(card['candidate'])}


class MemoryJournal:
    def __init__(self):
        self.state={'intent':{'protocol':probe.PROTOCOL,'operation':'a'*24,'fingerprint':'a'*64,
            'binding':{'probe_source':'b'*40},'baseline':{'unit_frame':True},
            'resource':resource_binding(model_card())},'steps':{}}
        self.saved=[]
    def save(self):self.saved.append(copy.deepcopy(self.state))


class FakeKube:
    def __init__(self):self.objects={};self.writes=[];self.lost=False;self.output='';self.auto_finish=False
    def logs(self,name):
        if self.auto_finish:
            item=probe.IMAGES[name.rsplit('-',1)[1]]
            return item['sha256']+'  '+item['path']+'\n'+str(item['bytes'])+'\n'
        return self.output
    def call(self,*args,body=None):
        if args[0]=='get' and ',' in args[1]:return {'items':[]}
        if args[:2]==('get','jobs'):return {'items':[copy.deepcopy(v) for (kind,name),v in self.objects.items() if kind=='job']}
        if args[:2]==('get','pods'):return {'items':[copy.deepcopy(v) for (kind,name),v in self.objects.items() if kind=='pod']}
        if args[0]=='get':return copy.deepcopy(self.objects.get((args[1],args[2]),{}))
        if args[0]=='create':
            result=copy.deepcopy(body);result['metadata'].update(uid='uid-'+body['kind'],resourceVersion='1')
            if body['kind']=='Job':
                result['spec']['selector']={'matchLabels':{'batch.kubernetes.io/controller-uid':'uid-Job'}}
                result['spec']['template']['metadata']['labels']['batch.kubernetes.io/controller-uid']='uid-Job'
                result['spec']['template']['metadata']['creationTimestamp']=None
            created_reply=copy.deepcopy(result)
            if '--dry-run=server' not in args:
                if self.auto_finish and body['kind']=='Pod':
                    result['spec']['nodeName']='node'
                    result['status']={'phase':'Succeeded','containerStatuses':[{'name':'artifact','imageID':body['spec']['containers'][0]['image'],
                        'containerID':'containerd://actual','state':{'terminated':{'exitCode':0,'startedAt':'2026-10-27T20:00:00Z','finishedAt':'2026-10-27T20:00:01Z'}}}]}
                if self.auto_finish and body['kind']=='Job':
                    result['status']={'active':0,'failed':1,'conditions':[{'type':'Failed','status':'True','reason':'DeadlineExceeded'}]}
                    child=copy.deepcopy(result['spec']['template']);name=body['metadata']['name']+'-child'
                    child.update(apiVersion='v1',kind='Pod');child['metadata'].update(namespace='voice-dev',name=name,uid='child',resourceVersion='1',
                        ownerReferences=[{'uid':'uid-Job','kind':'Job','controller':True}])
                    child['status']={'phase':'Failed','containerStatuses':[{'name':'artifact','containerID':'containerd://sleep',
                        'state':{'terminated':{'exitCode':137,'startedAt':'2026-10-27T20:00:00Z','finishedAt':'2026-10-27T20:00:05Z'}}}]}
                    self.objects[('pod',name)]=child
                self.writes.append(copy.deepcopy(body));self.objects[(body['kind'].lower(),body['metadata']['name'])]=result
                if self.lost and body['kind']=='Pod':raise RecoveryRequired('lost create response')
            return created_reply
        if args[0]=='delete':
            self.writes.append(copy.deepcopy(body))
            if self.auto_finish:
                parts=args[1].split('/');kind={'configmaps':'configmap','pods':'pod','jobs':'job'}[parts[-2]];name=parts[-1]
                actual=self.objects[(kind,name)];self.assert_precondition(actual,body)
                self.objects.pop((kind,name))
                if kind=='job':
                    for key,value in list(self.objects.items()):
                        if any(o.get('uid')==actual['metadata']['uid'] for o in value.get('metadata',{}).get('ownerReferences',[])):self.objects.pop(key)
            return {'kind':'Status','status':'Success'}
        raise AssertionError(args)

    def assert_precondition(self,actual,body):
        assert body['preconditions']=={k:actual['metadata'][k] for k in ('uid','resourceVersion')}


class CandidateContractTest(unittest.TestCase):
    def test_owned_review_can_qualify_artifacts_only_in_prepare_without_fake_stage_advancement(self):
        card=model_card();task={'status':'review','owner':'Dimas-16','assignee':'Dimas-16','review_required':False}
        class Response:
            def read(self):return json.dumps(task).encode()
            def __enter__(self):return self
            def __exit__(self,*args):pass
        with patch('auth_user_adoption_runtime.resource_card',return_value=card),patch('urllib.request.urlopen',return_value=Response()):
            self.assertEqual(probe.probe_resource_card(),card)
            for fault in ('phase','owner','assignee','impact','in-progress'):
                with self.subTest(fault=fault):
                    if fault=='phase':card['phase']='deploy'
                    if fault=='owner':task['owner']='other'
                    if fault=='assignee':task['assignee']='other'
                    if fault=='impact':task['review_required']=True
                    if fault=='in-progress':task['status']='in_progress'
                    with self.assertRaises(RecoveryRequired):probe.probe_resource_card()
                    card['phase']='prepare';task.update(status='review',owner='Dimas-16',assignee='Dimas-16',review_required=False)

    def test_actual_retained_prepare_card_and_typed_version_can_prepare(self):
        fixture=json.loads((pathlib.Path(__file__).parent/'testdata/auth-user-probe-resource-prepare.json').read_text())
        card=fixture['card'];self.assertTrue(fixture['provenance']['historical_only'])
        self.assertEqual(card['phase'],'prepare');self.assertIsInstance(card['candidate'],dict)
        self.assertEqual(len(card['candidate']['images']),49)
        binding={'protocol':probe.PROTOCOL,'candidate_source':probe.PRODUCT,'namespace_uid':probe.NAMESPACE_UID,
            'probe_source':'b'*40,'review':{'reviewer':'Dimas-8','source_sha':'b'*40,'fingerprint':'a'*64,'evidence':'http://tracker/review'},
            'resource':{k:card[k] for k in ('claim','generation','window')}}
        binding['resource']['candidate_sha256']=probe.digest(card['candidate'])
        with tempfile.TemporaryDirectory() as folder,patch.object(probe,'probe_fingerprint',return_value='a'*64),\
             patch.object(probe,'probe_baseline',return_value={'same':True}):
            result=probe.prepare_probe(pathlib.Path(folder)/'private',binding,FakeKube(),resource=lambda:card)
            self.assertEqual(result['result'],'PREPARED')
        bad=copy.deepcopy(card);bad['candidate']='invented-version-label'
        with tempfile.TemporaryDirectory() as folder,patch.object(probe,'probe_fingerprint',return_value='a'*64):
            with self.assertRaises(RecoveryRequired):probe.prepare_probe(pathlib.Path(folder)/'private',binding,FakeKube(),resource=lambda:bad)

    def test_realistic_job_null_creation_timestamp_default_is_normalized_only_when_null(self):
        body=probe.deadline_job('a'*24);actual=copy.deepcopy(body)
        actual['metadata'].update(uid='job',resourceVersion='1')
        actual['spec']['selector']={'matchLabels':{'batch.kubernetes.io/controller-uid':'job'}}
        actual['spec']['template']['metadata']['labels']['batch.kubernetes.io/controller-uid']='job'
        actual['spec']['template']['metadata']['creationTimestamp']=None
        probe.admitted_job(body,actual)
        for metadata in ({'creationTimestamp':'2026-10-27T00:00:00Z'},{'annotations':{'injected':'true'}}):
            bad=copy.deepcopy(actual);bad['spec']['template']['metadata'].update(metadata)
            with self.assertRaises(RecoveryRequired):probe.admitted_job(body,bad)
        bad=copy.deepcopy(actual)
        bad['spec']['template']['spec']['containers'].append({'name':'injected','image':'unknown','terminationMessagePath':'/dev/termination-log'})
        with self.assertRaises(RecoveryRequired):probe.admitted_job(body,bad)

    def test_public_health_requires_fixed_verified_route_and_status_without_retaining_body(self):
        class Response:
            status=200
            def geturl(self):return 'https://api-dev.comrade.click/health'
            def read(self,limit):return b'private-body-not-retained'
            def __enter__(self):return self
            def __exit__(self,*args):pass
        class Opener:
            def open(self,request,timeout):return response
        response=Response()
        result=probe.public_health(Opener())
        self.assertEqual(result['status'],200);self.assertNotIn('private-body',json.dumps(result))
        response.status=503
        with self.assertRaises(RecoveryRequired):probe.public_health(Opener())
        response.status=200;response.geturl=lambda:'https://voice.comrade.click/health'
        with self.assertRaises(RecoveryRequired):probe.public_health(Opener())

    def test_unapproved_create_reply_retains_uid_hint_without_authorizing_cleanup(self):
        controller,journal,kube,calls=self.controller();controller.acquire_lock();original=kube.call
        def changed(*args,body=None):
            result=original(*args,body=body)
            if args[0]=='create' and '--dry-run=server' not in args and body['kind']=='Pod':
                result['spec']['containers'][0]['envFrom']=[{'secretRef':{'name':'unexpected'}}]
            return result
        kube.call=changed
        with self.assertRaises(RecoveryRequired):controller.create_artifact('auth',[])
        record=journal.state['probe']['auth']
        self.assertEqual(record['state'],'pending');self.assertEqual(record['returned_uid_hint'],'uid-Pod')
        self.assertFalse(any(x.get('kind')=='DeleteOptions' for x in kube.writes))

    def test_baseline_binds_unrelated_metadata_without_retaining_secret_values(self):
        snapshot={'namespace':{'uid':probe.NAMESPACE_UID},'ready':{'voice-auth':True}}
        items=[{'kind':'Secret','metadata':{'name':'unrelated','uid':'secret','resourceVersion':'1'},
                'type':'Opaque','data':{'PASSWORD':'must-not-appear-in-baseline'}},
               {'kind':'DaemonSet','metadata':{'name':'daemon','uid':'daemon','resourceVersion':'1'},'spec':{'selector':{'matchLabels':{'app':'daemon'}}}}]
        kube=FakeKube();original=kube.call
        def inventory(*args,body=None):
            if args[0]=='get' and 'configmaps,secrets' in args[1]:return {'items':copy.deepcopy(items)}
            return original(*args,body=body)
        kube.call=inventory
        with patch('auth_user_adoption_runtime.capture',return_value=(snapshot,{},[])),patch.object(probe,'public_health',return_value={'status':200}):
            first=probe.probe_baseline(kube)
            self.assertNotIn('must-not-appear',json.dumps(first))
            items[0]['metadata']['resourceVersion']='2'
            self.assertNotEqual(first,probe.probe_baseline(kube))

    def test_status_version_supersedes_old_fenced_delete_without_blind_same_version_retry(self):
        controller,journal,kube,calls=self.controller();controller.acquire_lock();controller.create_artifact('user',[])
        with self.assertRaises(probe.ProbePending):controller.cleanup_artifact('user')
        count=len(kube.writes)
        with self.assertRaises(probe.ProbePending):controller.cleanup_artifact('user')
        self.assertEqual(len(kube.writes),count)
        record=journal.state['probe']['user'];pod=kube.objects[('pod',record['body']['metadata']['name'])]
        pod['metadata']['resourceVersion']='2';kube.auto_finish=True
        controller.cleanup_artifact('user')
        self.assertEqual(record['state'],'absent')
        self.assertEqual(record['delete_cas_reconciliations'][-1],{'old_version':'1','observed_version':'2'})

    def test_saved_creation_body_cannot_turn_owned_cleanup_into_an_app_secret_probe(self):
        controller,journal,kube,calls=self.controller();controller.acquire_lock();controller.create_artifact('user',[])
        journal.state['probe']['user']['body']['spec']['containers'][0]['envFrom']=[{'secretRef':{'name':'voice-app-secrets'}}]
        with self.assertRaises(RecoveryRequired):probe.ProbeController(journal,kube,lambda:None)

    def test_already_admitted_job_template_container_defaults_remain_valid_for_child(self):
        intended=probe.artifact_pod('a'*24,'user')
        intended['spec']['containers'][0].update(terminationMessagePath='/dev/termination-log',terminationMessagePolicy='File')
        probe.admitted_pod(intended,copy.deepcopy(intended))

    def test_owned_lock_foreground_gc_is_pending_not_an_identity_drift_or_second_delete(self):
        controller,journal,kube,calls=self.controller();controller.acquire_lock()
        current=kube.objects[('configmap',probe.LOCK)]
        journal.state['probe_abort_pending']=True
        current['metadata'].update(resourceVersion='2',deletionTimestamp='2026-10-27T20:00:00Z',finalizers=['foregroundDeletion'])
        with self.assertRaises(probe.ProbePending):controller.release_lock('abort')
        self.assertEqual([x['kind'] for x in kube.writes],['ConfigMap'])
        kube.objects.pop(('configmap',probe.LOCK));controller.release_lock('abort')

    def test_abort_request_never_allows_resuming_normal_probe_writes(self):
        journal=MemoryJournal();journal.state['probe_abort_requested']=True;kube=FakeKube();kube.auto_finish=True
        with self.assertRaises(RecoveryRequired):probe.run_probe(journal,kube,lambda:None)
        self.assertFalse(kube.writes)

    def test_cli_fingerprint_is_read_only_and_wrong_protocol_journal_never_calls_kubernetes(self):
        with contextlib.redirect_stdout(io.StringIO()) as output,patch.object(probe,'ProbeKube') as kube:
            self.assertEqual(probe.main(['fingerprint']),0)
            value=json.loads(output.getvalue());self.assertEqual(len(value['fingerprint']),64)
            kube.assert_not_called()
        with tempfile.TemporaryDirectory() as folder:
            from auth_user_adoption import Journal
            journal=Journal(pathlib.Path(folder)/'wrong',{'protocol':'not-a-node-probe'});journal.close()
            with contextlib.redirect_stdout(io.StringIO()) as output,patch.object(probe,'ProbeKube') as kube:
                self.assertEqual(probe.main(['inspect','--journal',str(pathlib.Path(folder)/'wrong')]),1)
                self.assertEqual(json.loads(output.getvalue())['result'],'INCOMPLETE')
                kube.assert_not_called()

    def test_abort_can_settle_owned_failed_probe_without_claiming_deadline_pass(self):
        controller,journal,kube,calls=self.controller();controller.acquire_lock();controller.create_artifact('auth',[])
        controller.create_deadline([]);kube.auto_finish=True
        result=probe.abort_probe(journal,kube,lambda:calls.append('guard'))
        self.assertEqual(result['result'],'ABORTED');self.assertFalse(kube.objects)
        self.assertNotIn('probe_finished',journal.state)
        with self.assertRaises(RecoveryRequired):probe.run_probe(journal,kube,lambda:None)

    def test_unknown_pending_creation_cannot_be_aborted_by_name_or_unlock_environment(self):
        controller,journal,kube,calls=self.controller();controller.acquire_lock();kube.lost=True
        with self.assertRaises(RecoveryRequired):controller.create_artifact('auth',[])
        podname=journal.state['probe']['auth']['body']['metadata']['name']
        kube.objects.pop(('pod',podname))
        count=len(kube.writes)
        with self.assertRaises(RecoveryRequired):probe.abort_probe(journal,kube,lambda:None)
        self.assertEqual(len(kube.writes),count)
        self.assertNotIn('probe_aborted',journal.state)

    def test_complete_run_only_mutates_owned_scratch_and_returns_after_verified_absence(self):
        journal=MemoryJournal();journal.state['intent']['protocol']=probe.PROTOCOL
        kube=FakeKube();kube.auto_finish=True;guards=[]
        result=probe.run_probe(journal,kube,lambda:guards.append('guard'))
        self.assertEqual(result['result'],'PASS');self.assertFalse(kube.objects)
        self.assertEqual({x['kind'] for x in kube.writes},{'ConfigMap','Pod','Job','DeleteOptions'})
        count=len(kube.writes)
        self.assertEqual(probe.run_probe(journal,kube,lambda:guards.append('guard')),result)
        self.assertEqual(len(kube.writes),count)
        self.assertGreater(len(guards),len(kube.writes))

    def test_prepare_requires_independent_frozen_probe_binding_before_any_live_read(self):
        binding={'protocol':'voice-auth-user-node-probe-v1','candidate_source':'f1946355a084bed4c62a602ec1dc04e2d57e52ee',
            'namespace_uid':probe.NAMESPACE_UID,'probe_source':'b'*40,
            'review':{'reviewer':'Dimas-8','source_sha':'b'*40,'fingerprint':'a'*64,'evidence':'http://tracker/review'},
            'resource':resource_binding(model_card())}
        def forbidden():raise AssertionError('source mismatch must precede live reads')
        with tempfile.TemporaryDirectory() as folder,patch.object(probe,'probe_fingerprint',return_value='c'*64):
            with self.assertRaises(RecoveryRequired):probe.prepare_probe(pathlib.Path(folder)/'private',binding,FakeKube(),resource=forbidden)
        card=model_card()
        with tempfile.TemporaryDirectory() as folder,patch.object(probe,'probe_fingerprint',return_value='a'*64),\
             patch.object(probe,'probe_baseline',return_value={'unchanged':True}):
            result=probe.prepare_probe(pathlib.Path(folder)/'private',binding,FakeKube(),resource=lambda:card,clock=lambda:100)
            self.assertEqual(result['result'],'PREPARED')
            state=__import__('json').loads((pathlib.Path(folder)/'private/state.json').read_text())
            self.assertEqual(state['intent']['baseline'],{'unchanged':True})
            self.assertEqual(state['intent']['expires_at'],1000)

    def test_window_guard_rechecks_resource_expiry_fingerprint_and_original_baseline(self):
        journal=MemoryJournal();card=model_card()
        baseline={'unchanged':True};journal.state['intent'].update(resource=resource_binding(card),
            fingerprint='approved',baseline=baseline,prepared_at=100,expires_at=1000)
        with patch.object(probe,'probe_fingerprint',return_value='approved'),patch.object(probe,'probe_baseline',return_value=baseline):
            guard=probe.WindowGuard(journal,FakeKube(),resource=lambda:copy.deepcopy(card),clock=lambda:900)
            guard()
            for fault in ('expired','source','baseline','claim','owner','phase','candidate'):
                with self.subTest(fault=fault):
                    if fault=='expired':guard.clock=lambda:1001
                    if fault=='claim':card['claim']='other'
                    if fault=='owner':card['owner']='other'
                    if fault=='phase':card['phase']='deploy'
                    if fault=='candidate':card['candidate']['evidence']='Changed typed candidate evidence'
                    context=(patch.object(probe,'probe_fingerprint',return_value='changed') if fault=='source' else
                             patch.object(probe,'probe_baseline',return_value={'changed':True}) if fault=='baseline' else
                             patch.object(probe,'probe_fingerprint',return_value='approved'))
                    with context,self.assertRaises(RecoveryRequired):guard()
                    guard.clock=lambda:900;card.clear();card.update(model_card())

    def test_observation_is_bound_to_current_uid_and_fixed_hash_command(self):
        controller,journal,kube,calls=self.controller();controller.acquire_lock();controller.create_artifact('user',[])
        record=journal.state['probe']['user'];pod=kube.objects[('pod',record['body']['metadata']['name'])]
        self.assertIsNone(controller.observe_artifact('user'))
        pod['spec']['nodeName']='node';pod['status']={'phase':'Succeeded','containerStatuses':[{'name':'artifact',
            'imageID':probe.IMAGES['user']['ref'],'containerID':'containerd://actual',
            'state':{'terminated':{'exitCode':0,'startedAt':'2026-10-27T20:00:00Z','finishedAt':'2026-10-27T20:00:01Z'}}}]}
        kube.output=probe.IMAGES['user']['sha256']+'  /service\n34406562\n'
        controller.observe_artifact('user')
        self.assertEqual(journal.state['probe']['user']['state'],'observed')
        pod['metadata']['uid']='replacement'
        with self.assertRaises(RecoveryRequired):controller.observe_artifact('user')

    def test_finish_cannot_unlock_without_every_receipt_and_actual_child_absence(self):
        controller,journal,kube,calls=self.controller();controller.acquire_lock()
        with self.assertRaises(RecoveryRequired):controller.finish()
        self.assertFalse(any(x.get('kind')=='DeleteOptions' for x in kube.writes))

    def test_deadline_cleanup_waits_for_actual_terminal_child_and_gc_absence(self):
        controller,journal,kube,calls=self.controller();controller.acquire_lock();controller.create_deadline([])
        record=journal.state['probe_deadline'];name=record['body']['metadata']['name']
        job=kube.objects[('job',name)]
        self.assertIsNone(controller.observe_deadline())
        child=copy.deepcopy(job['spec']['template'])
        child.update(apiVersion='v1',kind='Pod');child['metadata'].update(name=name+'-child',namespace='voice-dev',uid='child',resourceVersion='2',
            ownerReferences=[{'kind':'Job','uid':'uid-Job','controller':True}])
        child['status']={'phase':'Failed','containerStatuses':[{'name':'artifact','containerID':'containerd://sleep',
            'state':{'terminated':{'startedAt':'2026-10-27T20:00:00Z','finishedAt':'2026-10-27T20:00:05Z','exitCode':137}}}]}
        kube.objects[('pod',name+'-child')]=child
        job['status']={'active':0,'failed':1,'conditions':[{'type':'Failed','status':'True','reason':'DeadlineExceeded'}]}
        controller.observe_deadline()
        with self.assertRaises(RecoveryRequired):controller.cleanup_deadline()
        self.assertEqual(journal.state['probe_deadline']['state'],'deleting')
        kube.objects.pop(('job',name))
        with self.assertRaises(RecoveryRequired):controller.cleanup_deadline()
        kube.objects.pop(('pod',name+'-child'))
        controller.cleanup_deadline()
        self.assertEqual(journal.state['probe_deadline']['state'],'absent')

    def test_deadline_job_admission_allows_only_uid_derived_controller_labels(self):
        body=probe.deadline_job('a'*24);actual=copy.deepcopy(body)
        actual['metadata'].update(uid='job',resourceVersion='1')
        generated={'batch.kubernetes.io/controller-uid':'job','batch.kubernetes.io/job-name':body['metadata']['name']}
        actual['spec']['selector']={'matchLabels':{'batch.kubernetes.io/controller-uid':'job'}}
        actual['spec']['template']['metadata']['labels'].update(generated)
        actual['spec']['completionMode']='NonIndexed'
        probe.admitted_job(body,actual)
        for fault in ('selector','credentials','command','deadline','generated-owner'):
            bad=copy.deepcopy(actual)
            if fault=='selector':bad['spec']['selector']['matchLabels']={'app':'user'}
            if fault=='credentials':bad['spec']['template']['spec']['volumes']=[{'name':'secret','secret':{'secretName':'shared'}}]
            if fault=='command':bad['spec']['template']['spec']['containers'][0]['command']=['/service']
            if fault=='deadline':bad['spec']['activeDeadlineSeconds']=500
            if fault=='generated-owner':bad['spec']['template']['metadata']['labels']['batch.kubernetes.io/controller-uid']='foreign'
            with self.subTest(fault=fault),self.assertRaises(RecoveryRequired):probe.admitted_job(body,bad)

    def test_artifact_receipt_requires_real_runtime_identity_exit_and_exact_bytes(self):
        body=probe.artifact_pod('a'*24,'user');body['metadata'].update(uid='pod',resourceVersion='1')
        body['spec']['nodeName']='node'
        body['status']={'phase':'Succeeded','containerStatuses':[{'name':'artifact',
            'imageID':probe.IMAGES['user']['ref'],'containerID':'containerd://actual',
            'state':{'terminated':{'exitCode':0,'startedAt':'2026-10-27T20:00:00Z','finishedAt':'2026-10-27T20:00:01Z'}}}]}
        logs=probe.IMAGES['user']['sha256']+'  /service\n34406562\n'
        result=probe.artifact_receipt(body,'user','pod',logs)
        self.assertEqual(result['node'],'node')
        for bad in ('uid','image','exit','digest','size','foreign-owner'):
            actual=copy.deepcopy(body);output=logs
            if bad=='uid':actual['metadata']['uid']='other'
            if bad=='image':actual['status']['containerStatuses'][0]['imageID']='unknown'
            if bad=='exit':actual['status']['containerStatuses'][0]['state']['terminated']['exitCode']=1
            if bad=='digest':output='f'*64+'  /service\n34406562\n'
            if bad=='size':output=logs.replace('34406562','1')
            if bad=='foreign-owner':actual['metadata']['ownerReferences']=[{'uid':'foreign','controller':True}]
            with self.subTest(bad=bad),self.assertRaises(RecoveryRequired):probe.artifact_receipt(actual,'user','pod',output)

    def test_cleanup_intent_uses_uid_precondition_and_never_claims_absence_from_delete_ack(self):
        controller,journal,kube,calls=self.controller();controller.acquire_lock()
        controller.create_artifact('user',[])
        with self.assertRaises(RecoveryRequired):controller.cleanup_artifact('user')
        deletions=[x for x in kube.writes if x.get('kind')=='DeleteOptions']
        self.assertEqual(len(deletions),1)
        self.assertEqual(deletions[0]['preconditions']['uid'],'uid-Pod')
        self.assertEqual(journal.state['probe']['user']['state'],'deleting')
        # A lost deletion response is reconciled only through an actual absent read.
        kube.objects.pop(('pod',journal.state['probe']['user']['body']['metadata']['name']))
        controller.cleanup_artifact('user')
        self.assertEqual(journal.state['probe']['user']['state'],'absent')

    def controller(self):
        journal=MemoryJournal();kube=FakeKube();calls=[]
        controller=probe.ProbeController(journal,kube,lambda:calls.append('fresh-window'))
        return controller,journal,kube,calls

    def test_common_lock_is_mandatory_and_foreign_lock_prevents_all_scratch_writes(self):
        controller,journal,kube,calls=self.controller()
        with self.assertRaises(RecoveryRequired):controller.create_artifact('auth',[])
        self.assertFalse(kube.writes)
        kube.objects[('configmap','voice-dev-image-update-lock')]={'metadata':{'uid':'foreign'},'data':{'owner':'other'}}
        with self.assertRaises(RecoveryRequired):controller.acquire_lock()
        self.assertFalse(kube.writes)
        self.assertTrue(calls)

    def test_lost_pod_creation_is_durable_and_never_blindly_retried_or_deleted(self):
        controller,journal,kube,calls=self.controller();controller.acquire_lock();kube.lost=True
        with self.assertRaises(RecoveryRequired):controller.create_artifact('auth',[])
        self.assertEqual(journal.state['probe']['auth']['state'],'pending')
        pending=[row for row in journal.saved if row.get('probe',{}).get('auth',{}).get('state')=='pending']
        self.assertTrue(pending)
        with self.assertRaises(RecoveryRequired):controller.create_artifact('auth',[])
        self.assertEqual([x['kind'] for x in kube.writes],['ConfigMap','Pod'])

    def test_explicit_lost_creation_reconciliation_binds_existing_uid_without_second_create(self):
        controller,journal,kube,calls=self.controller();controller.acquire_lock();kube.lost=True
        journal.state['intent']['prepared_at']=100
        with self.assertRaises(RecoveryRequired):controller.create_artifact('user',[])
        record=journal.state['probe']['user'];pod=kube.objects[('pod',record['body']['metadata']['name'])]
        pod['metadata']['creationTimestamp']='1970-01-01T00:02:00Z'
        controller.reconcile_creation('user')
        self.assertEqual(record['state'],'created');self.assertEqual(record['uid'],'uid-Pod')
        self.assertEqual([x['kind'] for x in kube.writes],['ConfigMap','Pod'])
        record['state']='pending';pod['metadata']['uid']='new';pod['metadata']['labels']['voice.test.owner']='foreign'
        with self.assertRaises(RecoveryRequired):controller.reconcile_creation('user')

    def test_recovery_window_refresh_keeps_original_intent_and_requires_fresh_same_claim(self):
        journal=MemoryJournal();card=model_card()
        journal.state['intent'].update(resource=resource_binding(card),fingerprint='approved',baseline={'same':True},prepared_at=100,expires_at=1000)
        original=copy.deepcopy(journal.state['intent'])
        with patch.object(probe,'probe_fingerprint',return_value='approved'),patch.object(probe,'probe_baseline',return_value={'same':True}):
            guard=probe.WindowGuard(journal,FakeKube(),resource=lambda:copy.deepcopy(card),clock=lambda:1001)
            with self.assertRaises(RecoveryRequired):guard()
            guard.refresh('Reobserved same held claim and unchanged original environment')
            guard();self.assertEqual(journal.state['intent'],original)
            self.assertEqual(journal.state['probe_recovery_windows'][-1]['resource_revision'],42)
            card['claim']='replacement'
            with self.assertRaises(RecoveryRequired):guard.refresh('Never transfer stale operation to a different resource claim')

    def test_expired_window_or_replaced_lock_blocks_next_effect(self):
        controller,journal,kube,calls=self.controller();controller.acquire_lock()
        kube.objects[('configmap','voice-dev-image-update-lock')]['metadata']['uid']='replacement'
        with self.assertRaises(RecoveryRequired):controller.create_artifact('auth',[])
        self.assertEqual([x['kind'] for x in kube.writes],['ConfigMap'])
        controller,journal,kube,calls=self.controller();controller.acquire_lock()
        def expired():raise RecoveryRequired('window expired')
        controller.guard=expired
        with self.assertRaises(RecoveryRequired):controller.create_artifact('auth',[])
        self.assertEqual([x['kind'] for x in kube.writes],['ConfigMap'])

    def test_admission_injection_is_checked_before_scratch_create(self):
        controller,journal,kube,calls=self.controller();controller.acquire_lock()
        original=kube.call
        def injected(*args,body=None):
            result=original(*args,body=body)
            if '--dry-run=server' in args:result['spec']['imagePullSecrets']=[{'name':'shared'}]
            return result
        kube.call=injected
        with self.assertRaises(RecoveryRequired):controller.create_artifact('user',[])
        self.assertEqual([x['kind'] for x in kube.writes],['ConfigMap'])

    def test_fixed_pod_never_inherits_app_credentials_or_service_labels(self):
        body=probe.artifact_pod('a'*24,'auth')
        spec=body['spec']; container=spec['containers'][0]
        self.assertEqual(body['metadata']['namespace'],'voice-dev')
        self.assertFalse(spec['automountServiceAccountToken'])
        self.assertFalse(spec['enableServiceLinks'])
        self.assertNotIn('app',body['metadata']['labels'])
        self.assertEqual(container['image'],probe.IMAGES['auth']['ref'])
        self.assertEqual(container['command'],['/bin/sh','-ec','sha256sum /auth.jar; wc -c < /auth.jar'])
        self.assertFalse(set(container)&{'env','envFrom','volumeMounts','ports'})
        self.assertNotIn('volumes',spec)
        self.assertTrue(container['securityContext']['readOnlyRootFilesystem'])

    def test_service_and_controller_selector_collisions_fail_closed(self):
        labels=probe.artifact_pod('a'*24,'user')['metadata']['labels']
        key=next(iter(labels));value=labels[key]
        for obj in ({'kind':'Service','spec':{'selector':{key:value}}},
                    {'kind':'Deployment','spec':{'selector':{'matchLabels':{key:value}}}},
                    {'kind':'ReplicaSet','spec':{'selector':{'matchExpressions':[
                        {'key':key,'operator':'In','values':[value]}]}}}):
            with self.subTest(kind=obj['kind']),self.assertRaises(RecoveryRequired):
                probe.exclude_selectors(labels,[obj])
        probe.exclude_selectors(labels,[{'kind':'Service','spec':{'selector':{}}},
            {'kind':'StatefulSet','spec':{'selector':{'matchLabels':{'app':'user'}}}}])

    def test_admission_injection_of_secret_sidecar_or_writable_mount_is_rejected(self):
        intended=probe.artifact_pod('a'*24,'user')
        for change in ('env','volume','sidecar','token','command'):
            actual=copy.deepcopy(intended)
            if change=='env':actual['spec']['containers'][0]['envFrom']=[{'secretRef':{'name':'voice-app-secrets'}}]
            if change=='volume':actual['spec']['volumes']=[{'name':'host','hostPath':{'path':'/'}}]
            if change=='sidecar':actual['spec']['containers'].append({'name':'extra','image':'unknown'})
            if change=='token':actual['spec']['automountServiceAccountToken']=True
            if change=='command':actual['spec']['containers'][0]['command']=['/service']
            with self.subTest(change=change),self.assertRaises(RecoveryRequired):probe.admitted_pod(intended,actual)
        actual=copy.deepcopy(intended);actual['spec']['dnsPolicy']='ClusterFirst'
        actual['spec']['containers'][0]['terminationMessagePath']='/dev/termination-log'
        probe.admitted_pod(intended,actual)

    def test_uid_delete_never_authorizes_replacement_or_foreign_object(self):
        obj=probe.artifact_pod('a'*24,'user');obj['metadata'].update(uid='owned',resourceVersion='12')
        result=probe.delete_options(obj,'a'*24,'owned')
        self.assertEqual(result['preconditions'],{'uid':'owned','resourceVersion':'12'})
        for bad in ('replacement','owner'):
            foreign=copy.deepcopy(obj)
            if bad=='replacement':foreign['metadata']['uid']='foreign'
            else:foreign['metadata']['labels']['voice.test.owner']='foreign'
            with self.subTest(bad=bad),self.assertRaises(RecoveryRequired):probe.delete_options(foreign,'a'*24,'owned')

    def test_deadline_cannot_pass_without_actual_owned_container_start_and_terminal_job(self):
        pod={'metadata':{'uid':'pod','ownerReferences':[{'kind':'Job','uid':'job','controller':True}]},
            'status':{'phase':'Failed','containerStatuses':[{'name':'artifact','containerID':'containerd://abc',
                'state':{'terminated':{'startedAt':'2026-10-27T20:00:00Z','finishedAt':'2026-10-27T20:00:05Z','exitCode':137}}}]}}
        job={'metadata':{'uid':'job'},'status':{'active':0,'failed':1,'conditions':[{'type':'Failed','status':'True','reason':'DeadlineExceeded'}]}}
        probe.deadline_receipt(job,[pod],'job')
        for bad in ('no-start','running','wrong-owner','not-terminal'):
            child=copy.deepcopy(pod);parent=copy.deepcopy(job)
            if bad=='no-start':child['status']['containerStatuses'][0]['state']['terminated'].pop('startedAt')
            if bad=='running':child['status']['phase']='Running'
            if bad=='wrong-owner':child['metadata']['ownerReferences'][0]['uid']='foreign'
            if bad=='not-terminal':parent['status']['conditions']=[]
            with self.subTest(bad=bad),self.assertRaises(RecoveryRequired):probe.deadline_receipt(parent,[child],'job')


if __name__=='__main__':unittest.main()
