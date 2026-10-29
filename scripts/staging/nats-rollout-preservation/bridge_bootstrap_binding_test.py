import copy
import unittest
import bridge_root
import transaction
from controller import Blocked

class ActorBootstrapBindingTest(unittest.TestCase):
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
