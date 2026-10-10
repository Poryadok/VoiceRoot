"""Real client/dispatcher/action composition; filesystem/readback proofs are separate."""
import copy
import datetime as dt
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch
import actor_root
import bridge_client
import bridge_root
import preserved_upload


class ActionsTests(unittest.TestCase):
    def fixture(self, base):
        state={'operation':preserved_upload.OPERATION,'phase':'AWAITING_OFF_NODE','status':'WAITING',
            'fence_status':'VERIFIED','cipher_binding':{'immutable':'original'},
            'cut':{'manifest':{'archive_sha256':'a'*64},'manifest_sha256':'b'*64},
            'contract':{'scripts':[]},'service_actor_services':['story'],
            'service_actor_authority':{'root':'captured'}}
        env={'GITHUB_RUN_ID':'37910000000','GITHUB_RUN_ATTEMPT':'2','GITHUB_JOB':'deploy','GITHUB_SHA':'f'*40,'GITHUB_TOKEN':'synthetic'}
        prepare=bridge_client.request(['resume-cipher-upload',preserved_upload.OPERATION,env['GITHUB_RUN_ID']],env)
        request=bridge_client.request(['authorize-preserved-upload',preserved_upload.OPERATION,'123',prepare['nonce'],env['GITHUB_RUN_ID']],env)
        run={'id':37910000000,'repository':{'full_name':'Poryadok/VoiceRoot'},'head_repository':{'full_name':'Poryadok/VoiceRoot'},
            'head_branch':'master','status':'in_progress','workflow_id':263689731,'path':'.github/workflows/staging-deploy.yml',
            'event':'workflow_dispatch','head_sha':'f'*40,'run_attempt':2}
        expected={'dispatcher_run_id':run['id'],'run_attempt':2,'head_sha':'f'*40,'event':'workflow_dispatch',
            'workflow_id':263689731,'path':run['path'],'repository':'Poryadok/VoiceRoot','nonce':prepare['nonce']}
        record={'deadline':(dt.datetime.now(dt.timezone.utc)+dt.timedelta(minutes=5)).isoformat(),'upload_execution':expected}
        actions=object.__new__(bridge_root.Actions);actions.code=Path('/captured');actions.binding={'root':'hash'}
        return state,request,run,expected,record,actions

    def invoke(self,base,*,actor_error=False,deadline_error=False,run_change=None,execution_change=False):
        state,request,run,expected,record,actions=self.fixture(base);original=copy.deepcopy(state);order=[]
        if run_change:run.update(run_change)
        def api(url,*args):
            return {'workflow_runs':[{'head_sha':'f'*40,'event':'push','head_branch':'master',
                'workflow_id':bridge_root.source_authority.WORKFLOW,'status':'completed','id':37898884009}]} if '/workflows/' in url else run
        def readback(token,path,observed,continuity,nonce,execution,artifact):
            if execution!=expected:raise preserved_upload.UploadError('preserved_upload_upload_dispatcher_changed')
            self.assertEqual(observed['cipher_binding'],original['cipher_binding'])
            self.assertEqual(nonce,expected['nonce']);self.assertEqual(artifact,123)
            order.append('readback');return {'verified':True,'artifact_id':123}
        def load(*args,**kwargs):
            if execution_change:raise preserved_upload.UploadError('preserved_upload_upload_dispatcher_changed')
            self.assertEqual(kwargs['execution'],expected);return record,b'record',b'snapshot'
        def actor(*args):
            order.append('actor')
            if actor_error:raise bridge_root.Blocked('actor_drift')
        def deadline(*args):
            order.append('deadline')
            if deadline_error:raise preserved_upload.UploadError('preserved_upload_deadline_expired')
        def authorize(*args,**kwargs):
            order.append('authorize');self.assertEqual(kwargs['authorization_deadline'],dt.datetime.fromisoformat(record['deadline']))
            self.assertTrue(state['custody']['verified'])
            for name in ('apply-authorization.json','apply-manifests.json'):(base/name).write_text('{}')
        with patch.object(actions,'state',return_value=(base,state)),patch.object(preserved_upload,'verify_helper_continuity',return_value=({},{})),\
             patch.object(actions,'_verify_decrypted_restore') as decrypt,\
             patch.object(bridge_root.source_authority,'_get',side_effect=api),patch.object(bridge_root.source_authority,'_head'),\
             patch.object(preserved_upload,'verify_artifact',side_effect=readback),patch.object(preserved_upload,'load',side_effect=load),\
             patch.object(preserved_upload,'verify_deadline',side_effect=deadline),patch.object(actor_root,'revalidate',side_effect=actor),\
             patch.object(bridge_root.transaction,'reconstruct'),patch.object(bridge_root.transaction,'authorize',side_effect=authorize),\
             patch.object(bridge_root,'Kube'),patch.object(bridge_root,'save') as save,\
             patch.object(bridge_root.os,'chown'),patch.object(bridge_root.grp,'getgrnam',return_value=SimpleNamespace(gr_gid=1000)):
            if actor_error or deadline_error or run_change or execution_change:
                with self.assertRaises((bridge_root.Blocked,preserved_upload.UploadError)):actions.execute(request)
                self.assertNotIn('authorize',order);self.assertEqual(state,original)
            else:
                result=actions.execute(request);self.assertIn('authorization',result)
                self.assertEqual(order,['readback','actor','deadline','authorize'])
            save.assert_not_called()

    def test_real_dispatcher_current_attempt_and_explicit_upload_nonce_reach_authorize(self):
        with tempfile.TemporaryDirectory() as directory:self.invoke(Path(directory))

    def test_rejections_before_authorize_preserve_checkpoint_and_capture(self):
        for changes in ({'actor_error':True},{'deadline_error':True},{'execution_change':True},
                        {'run_change':{'status':'completed'}},{'run_change':{'run_attempt':3}},
                        {'run_change':{'head_sha':'a'*40}}):
            with self.subTest(changes=changes),tempfile.TemporaryDirectory() as directory:self.invoke(Path(directory),**changes)

    def test_ordinary_authorize_has_no_preserved_record_fallback(self):
        with tempfile.TemporaryDirectory() as directory:
            base=Path(directory);(base/'cipher-upload').mkdir();state,_,_,_,_,actions=self.fixture(base)
            with patch.object(actions,'state',return_value=(base,state)),patch.object(bridge_root.github_custody,'verify_artifact') as normal,\
                 patch.object(preserved_upload,'verify_artifact') as dedicated:
                with self.assertRaisesRegex(bridge_root.Blocked,'explicit_authorization_required'):
                    actions.execute({'action':'authorize','operation':state['operation'],'token':'synthetic','artifact_id':123})
                normal.assert_not_called();dedicated.assert_not_called()


if __name__=='__main__':unittest.main()
