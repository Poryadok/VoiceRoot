import copy
import unittest
import json,hashlib
import tempfile,os,stat
from pathlib import Path
from unittest.mock import patch
from contextlib import contextmanager
import actor_root
import source_authority,transaction
import story_witness
from controller import Blocked

def witness():
    h='a'*64; sha='b'*40
    return {'schema':'voice-story-current4-actual-witness-v1','repository':'Poryadok/VoiceRoot',
        'service':'story','platform':'linux/amd64',
        'content':{'child_sha256':'5ce15edc313b06f63608674ab0e898be7f50cd8ca3123dbbe78417a7c62fe8b5',
            'config_sha256':'dcd36d3b279b69532c2f0563910e088276b20f34a781dcb2c8e21b5d5cee94b2',
            'binary_sha256':'45380cb8551df05469f76af593b1783ca29649960153c56047f0f6a35a64422b',
            'build_source_sha':sha},
        'component':{'source_sha':sha,'manifest_sha256':h},
        'database':{'name':'story_db','version':4,'dirty':False,'up_sha256':copy.deepcopy(actor_root.STORY_UP),
            'catalog_before_sha256':h,'catalog_after_sha256':h},
        'publisher':{'workflow':'.github/workflows/ci.yml','run_id':37640701939,'job_id':112858686057,
            'source_sha':sha,'catalog_sha256':h},
        'fixture':{'contract_sha256':h,'input_sha256':h,'source_sha256':h,'result_sha256':h,
            'cases':['health','text_writer','sql_defaults','missing_profile','wrong_count','schema_preservation','cleanup'],
            'positive_exit':0,'negative_exit':17,'missing_profile_code':'Unauthenticated',
            'row_count':1,'cleanup_verified':True},
        'tools':{'postgres':'docker.io/library/postgres@sha256:'+'c'*64,
            'caller':'golang:1.26-alpine@sha256:'+'d'*64},'review_sha256':h}

class Tests(unittest.TestCase):
    def test_human_promotion_uses_real_locked_idle_gate_and_rechecks_marker(self):
        import bridge_root,root_main
        events=[];marker={'metadata':{'uid':'fixed','resourceVersion':'1'},'data':{'phase':'active'}}
        @contextmanager
        def lock():
            events.append('lock');yield;events.append('unlock')
        class Actions:
            def __init__(self,code):events.append('code')
            def _idle(self):events.append('idle')
        class Kube:
            def get(self,*args):events.append('marker');return copy.deepcopy(marker)
        def promoted(raw,sha,unchanged,**kwargs):
            unchanged();events.append('promotion');return {}
        with patch.object(os,'geteuid',return_value=0),patch.object(story_witness,'directory'),\
             patch.object(root_main,'operation_lock',lock),patch.object(bridge_root,'Actions',Actions),\
             patch.object(bridge_root,'Kube',Kube),patch.object(story_witness,'read',return_value=b'fixed'),\
             patch.object(story_witness,'promote_locked',promoted):
            story_witness.main(['--promote','/root/input','a'*64])
            self.assertEqual(events,['lock','code','idle','marker','idle','marker','promotion','unlock'])
            with patch.object(Actions,'_idle',side_effect=Blocked('bridge_active_operation_present')),self.assertRaises(Blocked):
                story_witness.main(['--promote','/root/input','a'*64])
            marker['data']['knownRolloutOperation']='owned-other'
            with self.assertRaises(Blocked):story_witness.main(['--promote','/root/input','a'*64])
            marker['data'].pop('knownRolloutOperation')
            def drift(raw,sha,unchanged,**kwargs):
                marker['metadata']['resourceVersion']='2';unchanged();self.fail('marker drift admitted')
            with patch.object(story_witness,'promote_locked',drift),self.assertRaises(Blocked):
                story_witness.main(['--promote','/root/input','a'*64])

    def test_selector_final_reread_and_conflicting_pair_do_not_change_authority(self):
        row=witness();raw=story_witness.encoded(row);sha=hashlib.sha256(raw).hexdigest()
        key=row['content']['child_sha256']+':'+row['content']['config_sha256']
        selected={'schema':'voice-story-witness-selector-v1','revision':1,'entries':{key:{'witness_sha256':sha,'enabled':True}}}
        def read(path):
            if path.name!='selector.json':return raw
            result=story_witness.encoded(selected);selected['revision']+=1;return result
        with patch.object(story_witness,'read',read),self.assertRaises(Blocked):
            story_witness.lookup(row['content']['child_sha256'],row['content']['config_sha256'])
        with self.assertRaises(Blocked):story_witness.selector(selected|{'caller_approval':True})
        with self.assertRaises(Blocked):story_witness.selector({**selected,'entries':{'../file':{'witness_sha256':sha,'enabled':True}}})

    def test_promotion_partial_retry_idempotence_revoke_and_no_unprivileged_admission(self):
        raw=story_witness.encoded(witness());sha=hashlib.sha256(raw).hexdigest();files={};calls=[]
        def read(path):
            if str(path) not in files:raise FileNotFoundError
            return files[str(path)]
        def write(path,data):
            if str(path) in files:raise FileExistsError
            files[str(path)]=data
        def replace(old,new):files[str(new)]=files.pop(str(old))
        def exists(path):return str(path) in files
        def unlink(path):files.pop(str(path),None)
        def unchanged():calls.append('checked')
        with patch.object(story_witness,'directory'),patch.object(Path,'mkdir'),\
             patch.object(story_witness,'read',read),patch.object(story_witness,'write_new',write),\
             patch.object(story_witness,'sync_directory'),patch.object(os,'replace',replace),\
             patch.object(Path,'exists',exists),patch.object(Path,'is_symlink',return_value=False),patch.object(Path,'unlink',unlink):
            # Complete record may survive a rejection, but selector remains absent.
            with self.assertRaises(Blocked):story_witness.promote_locked(raw,sha,lambda:story_witness.fail())
            self.assertFalse(any(p.endswith('selector.json') for p in files))
            selected=story_witness.promote_locked(raw,sha,unchanged)
            self.assertEqual(selected['revision'],1)
            self.assertEqual(story_witness.lookup(witness()['content']['child_sha256'],witness()['content']['config_sha256'])['witness_sha256'],sha)
            self.assertEqual(story_witness.promote_locked(raw,sha,unchanged),selected)
            second=witness();second['content']['child_sha256']='e'*64;second['content']['config_sha256']='f'*64
            second_raw=story_witness.encoded(second);second_sha=hashlib.sha256(second_raw).hexdigest()
            promoted=story_witness.promote_locked(second_raw,second_sha,unchanged)
            self.assertEqual(len(promoted['entries']),2)
            self.assertEqual(story_witness.lookup(witness()['content']['child_sha256'],witness()['content']['config_sha256'])['witness_sha256'],sha)
            self.assertEqual(story_witness.lookup('e'*64,'f'*64)['witness_sha256'],second_sha)
            revoked=story_witness.promote_locked(raw,sha,unchanged,revoke=True)
            self.assertEqual(revoked['revision'],3)
            self.assertEqual(story_witness.promote_locked(raw,sha,unchanged,revoke=True),revoked)
            with self.assertRaises(Blocked):story_witness.lookup(witness()['content']['child_sha256'],witness()['content']['config_sha256'])
            self.assertEqual(story_witness.lookup('e'*64,'f'*64)['witness_sha256'],second_sha)
            with self.assertRaises(Blocked):story_witness.promote_locked(raw,sha,unchanged)
            with self.assertRaises(Blocked):story_witness.promote_locked(raw,'f'*64,unchanged)
        with patch.object(os,'geteuid',return_value=1000),self.assertRaises(Blocked):story_witness.main(['--promote','/input',sha])

    def test_reader_root_custody_modes_inode_links_and_duplicate_keys(self):
        # Real Linux descriptor reads; root ownership is the only synthetic seam.
        with tempfile.TemporaryDirectory() as td:
            path=Path(td)/'record.json';path.write_bytes(b'{"safe":true}');path.chmod(0o600)
            actual=os.fstat
            def root_stat(fd):
                row=actual(fd)
                class S:pass
                value=S()
                for key in ('st_dev','st_ino','st_mode','st_uid','st_gid','st_nlink','st_size','st_mtime_ns','st_ctime_ns'):setattr(value,key,getattr(row,key))
                value.st_uid=0;return value
            with patch.object(story_witness,'directory'),patch.object(os,'fstat',root_stat):
                self.assertEqual(story_witness.read(path),b'{"safe":true}')
                path.chmod(0o644)
                with self.assertRaises(Blocked):story_witness.read(path)
                path.chmod(0o600);link=Path(td)/'link';link.symlink_to(path)
                with self.assertRaises(OSError):story_witness.read(link)
                hard=Path(td)/'hard';os.link(path,hard)
                with self.assertRaises(Blocked):story_witness.read(path)
            with self.assertRaises(Blocked):story_witness.parsed(b'{"a":1,"a":2}')

    def test_same_selected_authority_serves_forward_and_fresh_rollback(self):
        row=witness();raw=json.dumps(row,sort_keys=True,separators=(',',':')).encode()
        sha=hashlib.sha256(raw).hexdigest()
        key=row['content']['child_sha256']+':'+row['content']['config_sha256']
        second=copy.deepcopy(row);second['content']['child_sha256']='e'*64;second['content']['config_sha256']='f'*64
        second_raw=story_witness.encoded(second);second_sha=hashlib.sha256(second_raw).hexdigest()
        second_key='e'*64+':'+'f'*64
        selected={'schema':'voice-story-witness-selector-v1','revision':2,'entries':{key:{'witness_sha256':sha,'enabled':True},second_key:{'witness_sha256':second_sha,'enabled':True}}}
        image='ghcr.io/poryadok/voiceroot/story@sha256:'+row['content']['child_sha256']
        def read(path):return json.dumps(selected).encode() if path.name=='selector.json' else second_raw if path.name==second_sha+'.json' else raw
        chain={'requested_image':image,'manifest_image':image,'config_sha256':row['content']['config_sha256']}
        with patch.object(story_witness,'read',read),patch.object(source_authority,'immutable_image_identity',return_value=chain):
            content=actor_root.story_content(image)
            self.assertEqual(content,(row['content']['child_sha256'],row['content']['config_sha256'],row['content']['build_source_sha']))
            root_authority=story_witness.approve_content(content)
            authority={'roles':['story'],'proofs':{'story':{'root_verified':True}},'mounts':{'story':{'exact':'captured'}},'binding':{'same':'account'},'story_schema':{'clean_version':4}}
            candidate={'schema':'voice-reviewed-story-compatible-pair-v1','image':image,'child_sha256':content[0],'config_sha256':content[1],'component_source_sha':content[2],'schema_sha256':transaction.canonical(authority['story_schema']),'actor_sha256':transaction.canonical({'binding':authority['binding'],'mount':authority['mounts']['story'],'proof':authority['proofs']['story']}),'root_witness_authority':root_authority}
            authority['compatible_story_candidate']=candidate
            previous={'status':'PASS','input_target':{'mode':'images-only','changed_services':['story'],'template_hashes':{'voice-story':'x'}},'context':{'original_snapshots':{'voice-story':{'spec':{'template':{'spec':{'containers':[{'name':'story','image':'old'}]}}}}},'old_images':{'voice-story/story':image}},'service_actor_authority':authority,'nonnats':{'checks':[]},'target':{'images':{}},'contract':{}}
            self.assertEqual(transaction.rollback_package(previous)['target']['images']['voice-story/story'],image)
            # New B forward and re-upgrade both admit B, while fresh rollback's
            # root-captured prior A stays independently approved in this registry.
            image_b='ghcr.io/poryadok/voiceroot/story@sha256:'+'e'*64
            chain.update(requested_image=image_b,manifest_image=image_b,config_sha256='f'*64)
            self.assertEqual(actor_root.story_content(image_b),('e'*64,'f'*64,row['content']['build_source_sha']))
            self.assertEqual(transaction.rollback_package(previous)['target']['images']['voice-story/story'],image)
            self.assertEqual(actor_root.story_content(image_b)[0],'e'*64)
            chain.update(requested_image=image,manifest_image=image,config_sha256=row['content']['config_sha256'])
            selected['revision']=3
            with self.assertRaises(Blocked):transaction.rollback_package(previous)
            selected['entries'][key]['enabled']=False
            with self.assertRaises(Blocked):actor_root.story_content(image)
            with self.assertRaises(Blocked):transaction.rollback_package(previous)

    def test_root_selected_new_pair_and_revocation_reread(self):
        row=witness();raw=json.dumps(row,sort_keys=True,separators=(',',':')).encode()
        sha=hashlib.sha256(raw).hexdigest()
        key=row['content']['child_sha256']+':'+row['content']['config_sha256']
        selected={'schema':'voice-story-witness-selector-v1','revision':1,'entries':{key:{'witness_sha256':sha,'enabled':True}}}
        def read(path):
            return json.dumps(selected).encode() if path.name=='selector.json' else raw
        with patch.object(story_witness,'read',read,create=True):
            authority=story_witness.lookup(row['content']['child_sha256'],row['content']['config_sha256'])
            self.assertEqual(authority['witness_sha256'],sha)
            self.assertEqual(authority['registry_revision'],1)
            selected['entries'][key]['enabled']=False
            with self.assertRaises(Blocked):story_witness.lookup(row['content']['child_sha256'],row['content']['config_sha256'])

    def test_exact_new_content_evidence_is_structurally_valid_not_a_root_approval(self):
        row=witness()
        self.assertEqual(story_witness.validate(row),row)
        changed=copy.deepcopy(row);changed['content']['child_sha256']='e'*64
        self.assertEqual(story_witness.validate(changed),changed)
        # Structure does not authorize either pair; only separately root-promoted bytes do.

    def test_no_plain_pass_or_arbitrary_scope_evidence(self):
        for row in [{'status':'PASS'},witness()|{'caller_approved':True}]:
            with self.assertRaises(Blocked):story_witness.validate(row)
        for path,value in [('service','bot'),('platform','linux/arm64'),('repository','other/Repo')]:
            row=witness();row[path]=value
            with self.assertRaises(Blocked):story_witness.validate(row)

    def test_current4_clean_exact_up_schema_and_catalog_preservation(self):
        for field,value in [('version',3),('version',True),('dirty',True),('name','other_db'),
            ('up_sha256',{}),('catalog_after_sha256','e'*64)]:
            row=witness();row['database'][field]=value
            with self.subTest(field=field,value=value),self.assertRaises(Blocked):story_witness.validate(row)

    def test_actual_cases_controls_and_cleanup_required(self):
        for field,value in [('positive_exit',1),('negative_exit',0),('row_count',2),
            ('cleanup_verified',False),('missing_profile_code','OK'),('cases',['health'])]:
            row=witness();row['fixture'][field]=value
            with self.subTest(field=field),self.assertRaises(Blocked):story_witness.validate(row)

    def test_component_and_publisher_source_cannot_be_relabelled(self):
        for field in ['component','publisher']:
            row=witness();row[field]['source_sha']='f'*40
            with self.assertRaises(Blocked):story_witness.validate(row)
        for value in [False,0,-1,'37640701939']:
            row=witness();row['publisher']['run_id']=value
            with self.assertRaises(Blocked):story_witness.validate(row)
        for value in ['a'*63,'A'*64,'../file','https://untrusted.invalid']:
            row=witness();row['fixture']['result_sha256']=value
            with self.assertRaises(Blocked):story_witness.validate(row)

if __name__=='__main__':unittest.main()
