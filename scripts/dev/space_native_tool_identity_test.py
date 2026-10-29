import copy
import hashlib
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest import mock

import space_native_tool_identity as tool


class SourceInventoryTests(unittest.TestCase):
    def test_ci_capture_rechecks_registry_and_never_overwrites(self):
        sha='a'*40; encode=lambda x:json.dumps(x,sort_keys=True,separators=(',',':')).encode()
        conf=encode({'os':'linux','architecture':'amd64','config':{'User':'65532:65532',
              'Entrypoint':['/usr/local/bin/native-migrate'],'Labels':{'org.opencontainers.image.revision':sha}}})
        cd='sha256:'+hashlib.sha256(conf).hexdigest(); manifest=encode({'schemaVersion':2,'config':{'digest':cd},'layers':[]})
        md='sha256:'+hashlib.sha256(manifest).hexdigest(); index=encode({'schemaVersion':2,'manifests':[{'digest':md,'platform':{'os':'linux','architecture':'amd64'}}]})
        ind='sha256:'+hashlib.sha256(index).hexdigest(); tag='source-'+sha+'-1-1'
        class Registry:
            def get(self,repo,path,absent=False):
                self_repo='poryadok/voiceroot/space-native-installer'
                if repo!=self_repo: raise AssertionError(repo)
                return {'manifests/'+ind:index,'manifests/'+md:manifest,'blobs/'+cd:conf,'manifests/'+tag:index}[path]
        identity={'source_sha':sha,'run_id':1,'run_attempt':1,'tag':'ghcr.io/poryadok/voiceroot/space-native-installer:'+tag}
        snapshot={'source_sha':sha,'files':{}}; inventory={'source_sha':sha,'build_source_sha256':'b'*64,'sql_inventory_sha256':'c'*64}
        context=(Path('/source'),identity,snapshot,inventory,{'cycle':1},Registry())
        with tempfile.TemporaryDirectory() as folder, mock.patch.dict(os.environ,{'RUNNER_TEMP':folder}), \
             mock.patch.object(tool,'_ci_context',return_value=context),mock.patch.object(tool,'_extract_image',return_value={'executable_sha256':'d'*64}):
            out=Path(folder)/'receipt.json'; receipt=tool.capture_ci(ind,out)
            self.assertEqual(json.loads(out.read_bytes()),receipt)
            with self.assertRaises(tool.ToolIdentityError): tool.capture_ci(ind,out)
            with self.assertRaises(tool.ToolIdentityError): tool.capture_ci('../foreign',Path(folder)/'other.json')

    def test_oci_bytes_platform_config_and_runtime_role_are_bound(self):
        source='a'*40
        encode=lambda value:json.dumps(value,sort_keys=True,separators=(',',':')).encode()
        config=encode({'os':'linux','architecture':'amd64','config':{'User':'65532:65532',
            'Entrypoint':['/usr/local/bin/native-migrate'],'Labels':{'org.opencontainers.image.revision':source}}})
        config_digest='sha256:'+hashlib.sha256(config).hexdigest()
        manifest=encode({'schemaVersion':2,'config':{'digest':config_digest},'layers':[]})
        manifest_digest='sha256:'+hashlib.sha256(manifest).hexdigest()
        index=encode({'schemaVersion':2,'manifests':[{'digest':manifest_digest,'platform':{'os':'linux','architecture':'amd64'}}]})
        index_digest='sha256:'+hashlib.sha256(index).hexdigest()
        proof=tool.oci_identity(source,index_digest,index,manifest,config)
        self.assertEqual(proof['platform_digest'],manifest_digest)
        self.assertEqual(proof['config_digest'],config_digest)
        for args in ((source,index_digest,index+b' ',manifest,config),(source,index_digest,index,manifest,config+b' '),
                     ('b'*40,index_digest,index,manifest,config)):
            with self.assertRaises(tool.ToolIdentityError): tool.oci_identity(*args)
        bad=encode({'os':'linux','architecture':'amd64','config':{'User':'root','Entrypoint':['/usr/local/bin/native-migrate'],
                    'Labels':{'org.opencontainers.image.revision':source}}})
        bad_digest='sha256:'+hashlib.sha256(bad).hexdigest(); m=encode({'schemaVersion':2,'config':{'digest':bad_digest},'layers':[]})
        md='sha256:'+hashlib.sha256(m).hexdigest(); i=encode({'schemaVersion':2,'manifests':[{'digest':md,'platform':{'os':'linux','architecture':'amd64'}}]})
        with self.assertRaises(tool.ToolIdentityError): tool.oci_identity(source,'sha256:'+hashlib.sha256(i).hexdigest(),i,m,bad)

    def test_ci_run_must_be_same_canonical_master_push(self):
        env={'GITHUB_REPOSITORY':'Poryadok/VoiceRoot','GITHUB_EVENT_NAME':'push','GITHUB_REF':'refs/heads/master',
             'GITHUB_SHA':'a'*40,'GITHUB_RUN_ID':'123','GITHUB_RUN_ATTEMPT':'2'}
        run={'id':123,'run_attempt':2,'head_sha':'a'*40,'head_branch':'master','event':'push',
             'path':'.github/workflows/ci.yml','status':'in_progress','conclusion':None,
             'repository':{'full_name':'Poryadok/VoiceRoot'},'head_repository':{'full_name':'Poryadok/VoiceRoot'}}
        identity=tool.ci_identity(env,run)
        self.assertEqual(identity['tag'],'ghcr.io/poryadok/voiceroot/space-native-installer:source-'+'a'*40+'-123-2')
        for key,value in [('head_sha','b'*40),('event','workflow_dispatch'),('head_branch','other'),('id',124)]:
            wrong=copy.deepcopy(run); wrong[key]=value
            with self.subTest(key=key),self.assertRaises(tool.ToolIdentityError): tool.ci_identity(env,wrong)

    def test_image_content_requires_exact_binary_and_sql_inventory(self):
        source='a'*40; build='b'*64; driver=b'\x7fELF-driver'; driver_hash=hashlib.sha256(driver).hexdigest()
        sql={'000001_init.up.sql':b'SELECT 1;','000001_init.down.sql':b'SELECT 2;'}
        inventory={'source_sha':source,'build_source_sha256':build,'sql_inventory_sha256':'c'*64,
                   'sql_files':{name:hashlib.sha256(data).hexdigest() for name,data in sql.items()}}
        binary=b'\x7fELF:'+source.encode()+build.encode()+driver_hash.encode()
        proof=tool.content_identity(inventory,binary,driver,sql)
        self.assertEqual(proof['executable_sha256'],hashlib.sha256(binary).hexdigest())
        self.assertEqual(proof['migrator_sha256'],driver_hash)
        for wrong_binary,wrong_driver,wrong_sql in [(b'\x7fELF',driver,sql),(binary,b'other',sql),
                                                   (binary,driver,{**sql,'000001_init.up.sql':b'changed'})]:
            with self.assertRaises(tool.ToolIdentityError): tool.content_identity(inventory,wrong_binary,wrong_driver,wrong_sql)

    def test_actual_source_inventory_and_untrusted_drift(self):
        root=Path(__file__).resolve().parents[2]
        files={p.relative_to(root).as_posix():hashlib.sha256(p.read_bytes()).hexdigest()
               for p in (root/'src/backend').rglob('*.go') if not p.name.endswith('_test.go')}
        for path in (root/'src/backend/migrations/space_db').glob('*.sql'):
            files[path.relative_to(root).as_posix()]=hashlib.sha256(path.read_bytes()).hexdigest()
        dockerfile=root/'src/backend/space/Dockerfile.native-migrate'
        files[dockerfile.relative_to(root).as_posix()]=hashlib.sha256(dockerfile.read_bytes()).hexdigest()
        snapshot={'source_sha':'a'*40,'files':files}
        proof=tool.source_inventory(root,snapshot)
        self.assertEqual(proof['source_sha'],'a'*40)
        self.assertEqual(len(proof['build_source_sha256']),64)
        self.assertEqual(len(proof['sql_inventory_sha256']),64)
        for fault in ('source','sql','dockerfile','extra_sql'):
            wrong=copy.deepcopy(snapshot)
            if fault=='source': wrong['files']['src/backend/space/main.go']='0'*64
            if fault=='sql': wrong['files']['src/backend/migrations/space_db/000001_init.up.sql']='0'*64
            if fault=='dockerfile': wrong['files']['src/backend/space/Dockerfile.native-migrate']='0'*64
            if fault=='extra_sql': wrong['files']['src/backend/migrations/space_db/999999_wrong.up.sql']='0'*64
            with self.subTest(fault=fault),self.assertRaises(tool.ToolIdentityError): tool.source_inventory(root,wrong)


if __name__=='__main__': unittest.main()
