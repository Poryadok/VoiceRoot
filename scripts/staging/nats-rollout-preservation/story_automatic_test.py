"""Automatic root admission retains the actual-writer and revocation gates."""
import copy
import hashlib
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch
import actor_root
import guard
import story_witness
import source_authority
import transaction
from controller import Blocked


def automatic_witness():
    h='a'*64
    return {'schema':'voice-story-current4-automatic-witness-v1',
        'repository':'Poryadok/VoiceRoot','service':'story','platform':'linux/amd64',
        'content':{'child_sha256':'b'*64,'config_sha256':'c'*64,
            'binary_sha256':'d'*64,'build_source_sha':'e'*40},
        'component':{'source_sha':'e'*40,'manifest_sha256':'b'*64},
        'database':{'name':'story_db','version':4,'dirty':False,
            'up_sha256':copy.deepcopy(actor_root.STORY_UP),
            'catalog_before_sha256':h,'catalog_after_sha256':h},
        'fixture':{'contract_sha256':h,'input_sha256':h,'source_sha256':h,'result_sha256':h,
            'cases':['health','text_writer','sql_defaults','missing_profile','wrong_count','schema_preservation','cleanup'],
            'positive_exit':0,'negative_exit':17,'missing_profile_code':'Unauthenticated',
            'row_count':1,'cleanup_verified':True},
        'tools':{'postgres':'docker.io/library/postgres@sha256:'+'f'*64,
            'caller':'ghcr.io/poryadok/voiceroot/story@sha256:'+'b'*64},
        'execution':{'helper_binding_sha256':h,'caller_sha256':h,'fixture_sha256':h}}


class AutomaticAdmissionTests(unittest.TestCase):
    def setUp(self):
        self.temp=tempfile.TemporaryDirectory();self.addCleanup(self.temp.cleanup)
        self.root=Path(self.temp.name);(self.root/'installed').mkdir()
        self.patch=patch.object(guard,'ROOT',self.root);self.patch.start();self.addCleanup(self.patch.stop)
        self.row=automatic_witness();self.calls=[]
    def ensure(self,row=None):
        def prove():self.calls.append('actual-fixture');return copy.deepcopy(row or self.row)
        image='ghcr.io/poryadok/voiceroot/story@sha256:'+'b'*64
        chain={'requested_image':image,'manifest_image':image,'config_sha256':'c'*64}
        with patch.object(source_authority,'immutable_image_identity',return_value=chain):
            content=actor_root.story_content(image,lambda child,config:story_witness.ensure_content(child,config,prove,lambda:self.calls.append('unchanged')))
        return story_witness.approve_content(content)
    def test_missing_pair_runs_actual_fixture_once_and_forward_and_rollback_use_same_authority(self):
        selected=self.ensure()
        self.assertEqual(self.calls.count('actual-fixture'),1)
        self.assertEqual(selected,story_witness.lookup('b'*64,'c'*64))
        self.assertEqual(selected,story_witness.approve_content(('b'*64,'c'*64,'e'*40),selected))
        self.assertEqual(self.ensure(),selected)
        self.assertEqual(self.calls.count('actual-fixture'),1)
    def test_new_automatic_pair_does_not_revoke_unchanged_rollback_origin(self):
        original=self.ensure()
        image='ghcr.io/poryadok/voiceroot/story@sha256:'+'b'*64
        authority={'roles':['story'],'proofs':{'story':{'root_verified':True}},'mounts':{'story':{'exact':'captured'}},'binding':{'same':'account'},'story_schema':{'clean_version':4}}
        authority['compatible_story_candidate']={'schema':'voice-reviewed-story-compatible-pair-v1','image':image,'child_sha256':'b'*64,'config_sha256':'c'*64,'component_source_sha':'e'*40,'schema_sha256':transaction.canonical(authority['story_schema']),'actor_sha256':transaction.canonical({'binding':authority['binding'],'mount':authority['mounts']['story'],'proof':authority['proofs']['story']}),'root_witness_authority':original}
        previous={'status':'PASS','input_target':{'mode':'images-only','changed_services':['story'],'template_hashes':{'voice-story':'x'}},'context':{'original_snapshots':{'voice-story':{'spec':{'template':{'spec':{'containers':[{'name':'story','image':'old'}]}}}}},'old_images':{'voice-story/story':image}},'service_actor_authority':authority,'nonnats':{'checks':[]},'target':{'images':{}},'contract':{}}
        self.assertEqual(transaction.rollback_package(previous)['target']['images']['voice-story/story'],image)
        second=copy.deepcopy(self.row);second['content']['child_sha256']='0'*64
        second['content']['config_sha256']='1'*64;second['component']['manifest_sha256']='0'*64
        story_witness.ensure_content('0'*64,'1'*64,lambda:second,lambda:None)
        current=story_witness.approve_content(('b'*64,'c'*64,'e'*40),original)
        self.assertEqual(current,original)
        self.assertEqual(transaction.rollback_package(previous)['target']['images']['voice-story/story'],image)
        changed=copy.deepcopy(original);changed['witness_sha256']='f'*64
        with self.assertRaises(Blocked):story_witness.approve_content(('b'*64,'c'*64,'e'*40),changed)
        corrupted=copy.deepcopy(previous);corrupted['service_actor_authority']['compatible_story_candidate']['root_witness_authority']=changed
        with self.assertRaises(Blocked):transaction.rollback_package(corrupted)
        raw=story_witness.encoded(self.row);story_witness.promote_locked(raw,hashlib.sha256(raw).hexdigest(),lambda:None,revoke=True)
        with self.assertRaises(Blocked):story_witness.approve_content(('b'*64,'c'*64,'e'*40),original)
        with self.assertRaises(Blocked):transaction.rollback_package(previous)
    def test_absent_build_label_is_honest_metadata_not_fabricated_source_approval(self):
        row=copy.deepcopy(self.row);row['content']['build_source_sha']=None;row['component']['source_sha']=None
        selected=self.ensure(row)
        self.assertIsNone(selected['component_source_sha'])
        self.assertEqual(story_witness.approve_content(('b'*64,'c'*64,None),selected),selected)
    def test_failure_or_incomplete_cleanup_never_registers_and_revoked_pair_never_runs_again(self):
        for mutate in (lambda r:r['fixture'].update(positive_exit=17),
                       lambda r:r['fixture'].update(cleanup_verified=False),
                       lambda r:r['database'].update(catalog_after_sha256='0'*64),
                       lambda r:r['content'].update(child_sha256='0'*64)):
            row=copy.deepcopy(self.row);mutate(row)
            with self.assertRaises(Blocked):self.ensure(row)
            self.assertFalse((self.root/'installed/story-witnesses/selector.json').exists())
        self.ensure();raw=story_witness.encoded(self.row);sha=hashlib.sha256(raw).hexdigest()
        story_witness.promote_locked(raw,sha,lambda:None,revoke=True)
        before=len(self.calls)
        with self.assertRaises(Blocked):self.ensure()
        self.assertEqual(len(self.calls),before)
