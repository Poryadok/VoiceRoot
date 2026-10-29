import copy
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import Mock, patch
import bridge_root
import transaction
from controller import Blocked

class ActorBootstrapBindingTest(unittest.TestCase):
    def test_source_capture_receipt_is_private_bound_and_exclusive(self):
        with tempfile.TemporaryDirectory() as temp:
            installed=Path(temp);installed.chmod(0o700)
            session=Mock(operation='a'*12,context={'claim':'test'},helper='b'*64)
            approved={'source_sha':'c'*40};execution={'run_id':7}
            with patch.object(bridge_root,'INSTALLED',installed):
                row=bridge_root.record_source_authority(session,approved,execution)
                path=installed/'source-authority'/('a'*12+'.json')
                self.assertEqual(json.loads(path.read_text()),row)
                self.assertEqual(path.stat().st_mode&0o777,0o600)
                self.assertEqual(row['approved_sha256'],transaction.canonical(approved))
                self.assertEqual(row['execution_sha256'],transaction.canonical(execution))
                self.assertEqual(session.check.call_count,2)
                with self.assertRaises(FileExistsError):bridge_root.record_source_authority(session,approved,execution)

    def test_post_write_session_loss_leaves_record_but_raises(self):
        with tempfile.TemporaryDirectory() as temp:
            installed=Path(temp);installed.chmod(0o700)
            session=Mock(operation='a'*12,context={'claim':'test'},helper='b'*64)
            session.check.side_effect=[None,Blocked('session_lost')]
            with patch.object(bridge_root,'INSTALLED',installed):
                with self.assertRaisesRegex(Blocked,'session_lost'):
                    bridge_root.record_source_authority(session,{'source_sha':'c'*40},{'run_id':7})
                self.assertTrue((installed/'source-authority'/('a'*12+'.json')).is_file())

    def test_prepare_records_only_successful_dispatcher_and_source_outputs(self):
        import source_plan
        class StopAfterCapture(Exception):pass
        for fault in ('dispatcher','source','session','none'):
            with self.subTest(fault=fault),tempfile.TemporaryDirectory() as temp:
                installed=Path(temp);installed.chmod(0o700);(installed/'sources').mkdir()
                actions=object.__new__(bridge_root.Actions)
                actions.tt_session=Mock(operation='a'*12,context={'claim':'bound'},helper='b'*64)
                approved={'source_sha':'c'*40,'source_files':{'path':'d'*64}}
                execution={'head_sha':'c'*40,'run_id':7}
                dispatcher=Mock(return_value=(7,execution))
                capture=Mock(return_value=approved)
                if fault=='dispatcher':dispatcher.side_effect=Blocked('dispatch_failed')
                if fault=='source':capture.side_effect=Blocked('source_failed')
                if fault=='session':actions.tt_session.check.side_effect=Blocked('session_failed')
                with patch.object(bridge_root,'INSTALLED',installed),patch.object(actions,'_idle'), \
                     patch.object(actions,'_dispatcher',dispatcher),patch.object(bridge_root.source_authority,'capture_source',capture), \
                     patch.object(bridge_root,'Kube',return_value=Mock()),patch.object(source_plan,'principal_publisher_source',return_value=False), \
                     patch('gis_existing_update.load_authority',return_value=None), \
                     patch.object(bridge_root.RolloutStage,'capture',side_effect=StopAfterCapture):
                    with self.assertRaises(StopAfterCapture if fault=='none' else Blocked):
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

if __name__=='__main__':unittest.main()
