import copy
import hashlib
import io
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import zipfile

import space_native_tool_identity_staging as staging


class SourceTest(unittest.TestCase):
    def setUp(self):
        self.tmp=tempfile.TemporaryDirectory(); self.addCleanup(self.tmp.cleanup)
        self.root=Path(self.tmp.name); (self.root/'nested').mkdir(); (self.root/'nested/a').write_bytes(b'actual')
        self.approved={'source_sha':'a'*40,'source_files':{'nested/a':hashlib.sha256(b'actual').hexdigest()}}

    def test_exact_tar_without_git_and_changes_fail(self):
        self.assertEqual(staging._source(self.root,self.approved)['files'],self.approved['source_files'])
        (self.root/'nested/a').write_bytes(b'changed')
        with self.assertRaises(ValueError): staging._source(self.root,self.approved)

    def test_extra_missing_symlink_and_fifo_fail(self):
        (self.root/'extra').write_bytes(b'x')
        with self.assertRaises(ValueError): staging._source(self.root,self.approved)
        (self.root/'extra').unlink(); (self.root/'nested/a').unlink()
        with self.assertRaises(ValueError): staging._source(self.root,self.approved)
        (self.root/'nested/a').symlink_to('/etc/passwd')
        with self.assertRaises((ValueError,OSError)): staging._source(self.root,self.approved)
        (self.root/'nested/a').unlink(); os.mkfifo(self.root/'nested/a')
        with self.assertRaises(ValueError): staging._source(self.root,self.approved)


class PublicationTest(unittest.TestCase):
    def setUp(self):
        self.tmp=tempfile.TemporaryDirectory(); self.addCleanup(self.tmp.cleanup)
        self.root=Path(self.tmp.name); (self.root/'request').write_bytes(b'request')
        self.approved={'schema':'voice-source-authority-v1','repository':'Poryadok/VoiceRoot','verified':True,
                       'source_sha':'a'*40,'run_id':8,'run_attempt':2,
                       'source_files':{'request':hashlib.sha256(b'request').hexdigest()}}
        self.inventory={'source_sha':'a'*40,'build_source_sha256':'1'*64,'sql_inventory_sha256':'2'*64,
                        'dockerfile_sha256':'3'*64,'source_manifest_sha256':'4'*64,'sql_files':{'1.sql':'5'*64}}
        self.closure={'schema':'voice-space-native-local-closure-v1','packages':{'nested':['nested/a.go']},
                      'files':{'nested/a.go':'6'*64},'workspace':[],'raw_sha256':'7'*64}
        identity={'source_sha':'a'*40,'run_id':8,'run_attempt':2,
                  'tag':'ghcr.io/poryadok/voiceroot/space-native-installer:source-'+'a'*40+'-8-2'}
        self.pub={'schema':'voice-space-native-tool-publication-v1',**identity,'index_digest':'sha256:'+'d'*64,
            'platform_digest':'sha256:'+'e'*64,'config_digest':'sha256:'+'f'*64,
            'immutable_image':'ghcr.io/poryadok/voiceroot/space-native-installer@sha256:'+'e'*64,
            'source_inventory':self.inventory,'request':{'schema':'voice-full-application-build-request-v1','cycle':1,'request_id':'a'*32},
            'content':{'executable_sha256':'6'*64,'migrator_sha256':'7'*64,'sql_inventory_sha256':'2'*64,
                       'embedded_values_observed':True,'runtime_receipt_verified':False,'help_exit_code':0,
                       'compiled_gate_probe':'native51-source-gates-reached-v1','closure':self.closure},
            'runtime_qualified':False,'deployment_performed':False}
        self.job={'started_at':'2025-01-01T00:00:00Z','completed_at':'2025-01-01T00:01:00Z'}
        self.artifact={'id':10,'name':'space-native-tool-8-2','expired':False,'created_at':'2025-01-01T00:00:30Z',
            'expires_at':'2099-01-01T00:00:00Z','workflow_run':{'id':8,'head_sha':'a'*40,'repository_id':9,'head_repository_id':9}}
        self.buf=io.BytesIO()
        with zipfile.ZipFile(self.buf,'w') as z: z.writestr('space-native-tool.json',json.dumps(self.pub))
        self.archive=self.buf.getvalue(); self.artifact['size_in_bytes']=len(self.archive)

    def run_reader(self,fetch=None):
        class Registry:
            def __init__(self,*args): pass
            def get(self,repository,path): return b'oci'
        def get(url,*args): return copy.deepcopy(self.artifact) if '/artifacts/10' in url else {'id':8}
        def pages(url,key,*args): return [{'id':10,'name':'space-native-tool-8-2'}] if key=='artifacts' else []
        def download(url,headers,limit,deadline,target):
            target.write(self.archive); return len(self.archive),None
        with patch.object(staging.tool,'source_inventory',return_value=self.inventory), \
             patch.object(staging.custody,'_equivalent'), patch.object(staging.canonical,'validate_run'), \
             patch.object(staging.canonical,'validate_jobs'),patch.object(staging.canonical,'REQUEST','request'), \
             patch.object(staging.canonical,'parse_request',return_value=self.pub['request']), \
             patch.object(staging.tool,'tool_job',return_value=self.job),patch.object(staging.authority,'_headers',return_value={}), \
             patch.object(staging.authority,'_head',return_value={'id':9}),patch.object(staging.authority,'_get',side_effect=get), \
             patch.object(staging.authority,'_pages',side_effect=pages),patch.object(staging.authority,'_fetch',side_effect=fetch or download), \
             patch.object(staging.common,'Registry',Registry),patch.object(staging.tool,'oci_identity',return_value={k:self.pub[k] for k in ('index_digest','platform_digest','config_digest','immutable_image')}):
            return staging.authenticated_staging_publication('token','actor',self.approved,self.root)

    def test_exact_receipt_and_final_source_reread(self):
        self.assertEqual(self.run_reader()['publication'],self.pub)
        def drift(url,headers,limit,deadline,target):
            target.write(self.archive); (self.root/'request').write_bytes(b'drift'); return len(self.archive),None
        with self.assertRaises(staging.StagingToolError): self.run_reader(drift)

    def test_wrong_source_and_duplicate_artifact_member_hold(self):
        self.pub['source_sha']='b'*40
        self.buf=io.BytesIO()
        with zipfile.ZipFile(self.buf,'w') as z: z.writestr('space-native-tool.json',json.dumps(self.pub))
        self.archive=self.buf.getvalue(); self.artifact['size_in_bytes']=len(self.archive)
        with self.assertRaises(staging.StagingToolError): self.run_reader()
        self.buf=io.BytesIO()
        with zipfile.ZipFile(self.buf,'w') as z:
            z.writestr('space-native-tool.json',json.dumps(self.pub)); z.writestr('extra',b'x')
        self.archive=self.buf.getvalue(); self.artifact['size_in_bytes']=len(self.archive)
        with self.assertRaises(staging.StagingToolError): self.run_reader()

if __name__=='__main__': unittest.main()
