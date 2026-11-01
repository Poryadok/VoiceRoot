import copy
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import Mock, patch
import bridge_root
import commands
import resource_claim
import resource_claim_test as fixtures
import transaction
from controller import Blocked

class ActorBootstrapBindingTest(unittest.TestCase):
    def source_fixture(self,installed):
        code=installed/'code';code.mkdir(mode=0o700)
        binding={'fixture.py':'b'*64};context=fixtures.context();current=fixtures.card(context)
        session=resource_claim.Session(context,'a'*12,resource_claim.digest(binding),
            lambda:copy.deepcopy(current),fixtures.MemoryJournal())
        session.admit(start=True)
        return code,binding,current,session

    def test_pre_effect_candidate_requires_source_pins_and_exact_gis_subset(self):
        selected={'voice-gameintegration/gameintegration':'registry/gis@sha256:'+'a'*64,
                  'voice-web/web':'registry/web@sha256:'+'b'*64}
        session=Mock();session.context={'images':{'voice-gameintegration/gameintegration':selected['voice-gameintegration/gameintegration']}}
        bridge_root.verify_bootstrap_candidate_images(session,selected,None,True)
        for candidate in ({}, {'voice-gameintegration/gameintegration':'registry/gis@sha256:'+'f'*64},
                          {'voice-web/web':selected['voice-web/web']},
                          {'voice-gameintegration/gameintegration':selected['voice-gameintegration/gameintegration'],'foreign/x':'bad'}):
            session.context={'images':candidate}
            with self.subTest(candidate=candidate),self.assertRaises(Blocked):
                bridge_root.verify_bootstrap_candidate_images(session,selected,None,True)
        session.context={'images':{'voice-web/web':selected['voice-web/web']}}
        bridge_root.verify_bootstrap_candidate_images(session,selected,None,False)
        session.context={'images':{'voice-web/web':'wrong'}}
        with self.assertRaises(Blocked):bridge_root.verify_bootstrap_candidate_images(session,selected,None,False)

    def test_source_capture_receipt_is_private_bound_and_exclusive(self):
        with tempfile.TemporaryDirectory() as temp:
            installed=Path(temp);installed.chmod(0o700)
            code,binding,current,session=self.source_fixture(installed)
            approved={'source_sha':'c'*40};execution={'run_id':7}
            with patch.object(bridge_root,'INSTALLED',installed),patch.object(commands,'authority_guard',session.check), \
                 patch.object(bridge_root.root_cli,'code_binding',return_value=binding):
                row=bridge_root.record_source_authority(session,approved,execution,code=code,binding=binding)
                path=installed/'source-authority'/('a'*12+'.json')
                self.assertEqual(json.loads(path.read_text()),row)
                self.assertEqual(path.stat().st_mode&0o777,0o600)
                self.assertEqual(row['approved_sha256'],transaction.canonical(approved))
                self.assertEqual(row['execution_sha256'],transaction.canonical(execution))
                with self.assertRaises(FileExistsError):bridge_root.record_source_authority(session,approved,execution,code=code,binding=binding)

    def test_post_write_session_loss_leaves_record_but_raises(self):
        with tempfile.TemporaryDirectory() as temp:
            installed=Path(temp);installed.chmod(0o700)
            code,binding,current,session=self.source_fixture(installed)
            save=bridge_root.save
            def lose_after_custody(path,value):
                save(path,value)
                if path.name=='code-custody.json':current['owner']='foreign'
            with patch.object(bridge_root,'INSTALLED',installed),patch.object(commands,'authority_guard',session.check), \
                 patch.object(bridge_root.root_cli,'code_binding',return_value=binding),patch.object(bridge_root,'save',side_effect=lose_after_custody):
                with self.assertRaisesRegex(RuntimeError,'staging_resource_authority_changed'):
                    bridge_root.record_source_authority(session,{'source_sha':'c'*40},{'run_id':7},code=code,binding=binding)
                self.assertTrue((installed/'source-authority'/('a'*12+'.json')).is_file())

    # Source capture unit coverage assumes admission; the actual gate is tested separately.
    @patch('migrations.require_preserving_admission', new=lambda *args: None)
    @patch('staging_prerequisites.run_prepare', return_value=None)
    def test_prepare_records_only_successful_dispatcher_and_source_outputs(self, _prepare):
        import source_plan
        class StopAfterCapture(Exception):pass
        for fault in ('dispatcher','source','session','none'):
            with self.subTest(fault=fault),tempfile.TemporaryDirectory() as temp:
                installed=Path(temp);installed.chmod(0o700);(installed/'sources').mkdir()
                code,binding,current,session=self.source_fixture(installed)
                with patch.object(bridge_root.root_cli,'code_binding',return_value=binding):actions=bridge_root.Actions(code)
                actions.tt_session=session
                approved={'source_sha':'c'*40,'source_files':{'path':'d'*64}}
                execution={'head_sha':'c'*40,'run_id':7}
                dispatcher=Mock(return_value=(7,execution))
                capture=Mock(return_value=approved)
                if fault=='dispatcher':dispatcher.side_effect=Blocked('dispatch_failed')
                if fault=='source':capture.side_effect=Blocked('source_failed')
                if fault=='session':
                    def capture_then_lose(*args,**kwargs):current['owner']='foreign';return approved
                    capture.side_effect=capture_then_lose
                with patch.object(bridge_root,'INSTALLED',installed),patch.object(actions,'_idle'), \
                     patch.object(actions,'_dispatcher',dispatcher),patch.object(bridge_root.source_authority,'capture_source',capture), \
                     patch.object(commands,'authority_guard',session.check),patch.object(bridge_root.root_cli,'code_binding',return_value=binding), \
                     patch.object(bridge_root,'Kube',return_value=Mock()),patch.object(source_plan,'principal_publisher_source',return_value=False), \
                     patch('gis_existing_update.load_authority',return_value=None), \
                     patch.object(bridge_root.RolloutStage,'capture',side_effect=StopAfterCapture):
                    with self.assertRaises(StopAfterCapture if fault=='none' else RuntimeError if fault=='session' else Blocked):
                        actions._prepare({'nonce':'a'*64,'token':'private','mode':'full','changed_services':[]})
                path=installed/'source-authority'/('a'*12+'.json')
                if fault!='none':self.assertFalse(path.exists());continue
                row=json.loads(path.read_text())
                self.assertEqual(row['approved_sha256'],transaction.canonical(approved))
                self.assertEqual(row['execution_sha256'],transaction.canonical(execution))
                self.assertNotIn('private',path.read_text())
                dispatcher.assert_called_once();capture.assert_called_once()
    def fixture(self):
        old={'story_schema':{'database_secret':{'name':'voice-app-secrets','uid':'uid','resourceVersion':'1'},
               'authority':{'secret':{'uid':'uid','resourceVersion':'1'},'namespace':{'uid':'ns'},'space_url_sha256':'a'},
               'database_url_sha256':'b'},'binding':{'stable':'proof'},'compatible_story_candidate':{'schema_sha256':'old','actor_sha256':'stable'}}
        new=copy.deepcopy(old);new['story_schema']['database_secret']['resourceVersion']='2'
        new['story_schema']['authority']['secret']['resourceVersion']='2'
        new['compatible_story_candidate']['schema_sha256']=transaction.canonical(new['story_schema'])
        return old,new,{'uid':'uid','resourceVersion':'1'},{'uid':'uid','resourceVersion':'2'}
    def test_only_exact_secret_rv_and_derived_schema_change(self):
        old,new,before,after=self.fixture()
        self.assertEqual(bridge_root.actor_after_realtime_bootstrap(old,new,before,after),new)
        for fault in ('secret_uid','secret_rv','space_url','story_url','namespace','actor','witness'):
            candidate=copy.deepcopy(new)
            if fault=='secret_uid':candidate['story_schema']['database_secret']['uid']='other'
            if fault=='secret_rv':candidate['story_schema']['authority']['secret']['resourceVersion']='3'
            if fault=='space_url':candidate['story_schema']['authority']['space_url_sha256']='other'
            if fault=='story_url':candidate['story_schema']['database_url_sha256']='other'
            if fault=='namespace':candidate['story_schema']['authority']['namespace']['uid']='other'
            if fault=='actor':candidate['binding']['stable']='other'
            if fault=='witness':candidate['compatible_story_candidate']['actor_sha256']='other'
            with self.subTest(fault=fault),self.assertRaises(Blocked):
                bridge_root.actor_after_realtime_bootstrap(old,candidate,before,after)
        with self.assertRaises(Blocked):bridge_root.actor_after_realtime_bootstrap(old,new,{'uid':'uid','resourceVersion':'3'},after)
    def test_absent_story_requires_exact_actor_authority(self):
        actor={'story_schema':None,'binding':'stable'}
        self.assertEqual(bridge_root.actor_after_realtime_bootstrap(actor,copy.deepcopy(actor),{},{}),actor)
        with self.assertRaises(Blocked):bridge_root.actor_after_realtime_bootstrap(actor,{'story_schema':None,'binding':'other'},{},{})

    def test_p2_secret_successor_retains_whole_source_plan_profile_and_retained_image(self):
        old,new,before,after=self.fixture()
        for value in (old,new):
            baseline=value['story_schema']
            value['story_schema']={'schema':'voice-story-baseline4-target8-composition-v1','baseline':baseline,
                'candidate':{'version':8,'source':'unchanged'},'binding':{'migration_plan_sha256':'a'*64},'retained_image':'immutable'}
            value['compatible_story_candidate']['schema_sha256']=transaction.canonical(value['story_schema'])
        self.assertEqual(bridge_root.actor_after_realtime_bootstrap(old,new,before,after),new)
        for field in ('candidate','binding','retained_image'):
            changed=copy.deepcopy(new);changed['story_schema'][field]='foreign'
            with self.subTest(field=field),self.assertRaises(Blocked):
                bridge_root.actor_after_realtime_bootstrap(old,changed,before,after)

if __name__=='__main__':unittest.main()
