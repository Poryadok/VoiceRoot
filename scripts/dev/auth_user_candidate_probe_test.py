import copy
import unittest
from auth_user_adoption import RecoveryRequired
import auth_user_candidate_probe as probe


class MemoryJournal:
    def __init__(self):
        self.state={'intent':{'operation':'a'*24,'resource':{'claim':'claim','generation':2,'window':'window'}},'steps':{}}
        self.saved=[]
    def save(self):self.saved.append(copy.deepcopy(self.state))


class FakeKube:
    def __init__(self):self.objects={};self.writes=[];self.lost=False
    def call(self,*args,body=None):
        if args[0]=='get' and ',' in args[1]:return {'items':[]}
        if args[0]=='get':return copy.deepcopy(self.objects.get((args[1],args[2]),{}))
        if args[0]=='create':
            result=copy.deepcopy(body);result['metadata'].update(uid='uid-'+body['kind'],resourceVersion='1')
            if '--dry-run=server' not in args:
                self.writes.append(copy.deepcopy(body));self.objects[(body['kind'].lower(),body['metadata']['name'])]=result
                if self.lost and body['kind']=='Pod':raise RecoveryRequired('lost create response')
            return result
        raise AssertionError(args)


class CandidateContractTest(unittest.TestCase):
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
