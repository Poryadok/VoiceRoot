import copy
import unittest
from unittest.mock import Mock
import nonnats_plan as plan
import nonnats_runtime as runtime
from controller import Blocked

NAME='voice-web-runtime-config'
TARGET={'apiVersion':'v1','kind':'ConfigMap','metadata':{},'data':{'voice-runtime-config.json':'{"version":1}'}}

class Kube:
    def __init__(self):self.objects=[];self.fail=False;self.lost=False;self.creates=0
    def run(self,args,body=None):
        if args==['get','--raw','/api/v1/namespaces/voice-staging/configmaps?fieldSelector=metadata.name%3D'+NAME+'&limit=2']:
            return {'apiVersion':'v1','kind':'ConfigMapList','metadata':{'resourceVersion':'1'},'items':copy.deepcopy(self.objects)}
        if args[0]=='create':
            self.creates+=1
            if self.fail:raise RuntimeError('lost response')
            obj=copy.deepcopy(body);obj['metadata'].update(uid='01234567-89ab-cdef-0123-456789abcdef',resourceVersion='7')
            self.objects.append(obj)
            if self.lost:raise RuntimeError('committed response lost')
            return copy.deepcopy(obj)
        raise AssertionError(args)

def fixture():
    descriptor={'kind':'ConfigMap','name':NAME,'namespace':'voice-staging','desired':copy.deepcopy(TARGET),'disposition':'create'}
    source={'schema':'test','checks':[{'id':'frontends','disposition':'preserve','objects':[copy.deepcopy(descriptor)]}],'actions':[]}
    return source,{'schema':plan.BINDING_SCHEMA,'mode':'full','plan_sha256':plan.digest(source),'objects':[descriptor]}

class BootstrapTests(unittest.TestCase):
    def test_catalog_failure_and_incomplete_list_are_not_absence(self):
        for reply in (RuntimeError('forbidden'),{'items':[],'metadata':{'continue':'next'}}, {'items':'invalid'}):
            kube=Mock()
            if isinstance(reply,Exception):kube.run.side_effect=reply
            else:kube.run.return_value=reply
            with self.subTest(reply=reply),self.assertRaises(plan.PlanError):plan.frontend_live(kube,NAME)

    def test_create_records_pending_then_verified_uid_and_checks_live(self):
        source,binding=fixture();kube=Kube();state={};events=[];stage=Mock()
        def journal(event):events.append((event['kind'],copy.deepcopy(state)))
        runtime.create_frontends(kube,source,binding,state,stage,journal,lambda:None)
        self.assertEqual([e[0] for e in events],['frontend_bootstrap_pending','frontend_bootstrap_created'])
        self.assertEqual(events[0][1]['frontend_bootstrap'][NAME]['status'],'pending')
        self.assertEqual(binding['objects'][0]['uid'],kube.objects[0]['metadata']['uid'])
        self.assertTrue(plan.revalidate(kube,binding))
        kube.objects[0]['data']['voice-runtime-config.json']='drift'
        with self.assertRaises(plan.PlanError):plan.revalidate(kube,binding)

    def test_foreign_presence_and_lost_response_never_adopt(self):
        source,binding=fixture();kube=Kube();state={};stage=Mock();events=[]
        kube.objects.append({'apiVersion':'v1','kind':'ConfigMap','metadata':{'name':NAME,'namespace':'voice-staging'}})
        with self.assertRaises(Blocked):runtime.create_frontends(kube,source,binding,state,stage,events.append,lambda:None)
        self.assertEqual(kube.creates,0)
        kube.objects=[];kube.fail=True
        with self.assertRaisesRegex(Blocked,'frontend_bootstrap_outcome_unknown'):
            runtime.create_frontends(kube,source,binding,state,stage,events.append,lambda:None)
        self.assertEqual(state['frontend_bootstrap'][NAME]['status'],'pending')
        with self.assertRaisesRegex(Blocked,'reconciliation_required'):
            runtime.create_frontends(kube,source,binding,state,stage,events.append,lambda:None)
        self.assertEqual(kube.creates,1)

    def test_expired_authority_after_pending_never_creates(self):
        source,binding=fixture();kube=Kube();state={};stage=Mock();events=[]
        def expired():raise Blocked('expired')
        with self.assertRaisesRegex(Blocked,'expired'):
            runtime.create_frontends(kube,source,binding,state,stage,events.append,expired)
        self.assertEqual(kube.creates,0)
        self.assertEqual(state['frontend_bootstrap'][NAME]['status'],'pending')

    def test_gis_first_install_cannot_create_frontend(self):
        source,binding=fixture();kube=Kube();state={'gis_first_install':{}};stage=Mock()
        with self.assertRaisesRegex(Blocked,'gis_first_install_forbidden'):
            runtime.create_frontends(kube,source,binding,state,stage,lambda event:None,lambda:None)
        self.assertEqual(kube.creates,0)

    def test_committed_lost_response_cannot_be_adopted_and_save_failure_blocks_create(self):
        source,binding=fixture();kube=Kube();kube.lost=True;state={};stage=Mock()
        with self.assertRaisesRegex(Blocked,'outcome_unknown'):
            runtime.create_frontends(kube,source,binding,state,stage,lambda event:None,lambda:None)
        self.assertEqual(len(kube.objects),1)
        self.assertEqual(state['frontend_bootstrap'][NAME]['status'],'pending')
        with self.assertRaises(Blocked):runtime.create_frontends(kube,source,binding,state,stage,lambda event:None,lambda:None)
        self.assertEqual(kube.creates,1)
        source,binding=fixture();kube=Kube();state={}
        def fail_save(_event):raise OSError('checkpoint unavailable')
        with self.assertRaises(OSError):runtime.create_frontends(kube,source,binding,state,stage,fail_save,lambda:None)
        self.assertEqual(kube.creates,0)

if __name__=='__main__':unittest.main()
