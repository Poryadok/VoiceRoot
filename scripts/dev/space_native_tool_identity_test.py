import copy
import hashlib
import json
import os
from pathlib import Path
import runpy
import shutil
import subprocess
import sys
import tempfile
from types import SimpleNamespace
import unittest
from unittest import mock

import space_native_tool_identity as tool


class SourceInventoryTests(unittest.TestCase):
    def test_canonical_import_cannot_select_staging_migrations(self):
        directory=Path(__file__).resolve().parent
        code=("import pathlib,canonical_application_package,space_native_tool_identity as t; "
              "assert pathlib.Path(t.migrations.__file__).resolve()==pathlib.Path('migrations.py').resolve()")
        result=subprocess.run([sys.executable,'-B','-c',code],cwd=directory,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=30)
        self.assertEqual(result.returncode,0,result.stderr.decode(errors='replace'))
    def test_isolated_compiled_probe_accepts_only_exact_post_gate_refusal(self):
        import publish_candidate
        cid='a'*64; calls=[]
        def command(args,**kwargs):
            calls.append(args); return (cid+'\n').encode() if args[1]=='create' else b''
        def start(args,**kwargs):
            kwargs['stderr'].write(b'administrative database unavailable\n')
            return SimpleNamespace(returncode=1)
        with mock.patch.object(publish_candidate,'command',side_effect=command),mock.patch.object(tool.subprocess,'run',side_effect=start):
            self.assertEqual(tool._run_compiled_probe('image@sha256:'+'b'*64,'c'*64),'native51-source-gates-reached-v1')
        self.assertIn('--network',calls[0]); self.assertIn('none',calls[0]); self.assertEqual(calls[-1],['docker','rm','--force',cid])
        def wrong(args,**kwargs):
            kwargs['stderr'].write(b'HOLD: current native51 source/session qualification is not admitted\n')
            return SimpleNamespace(returncode=1)
        with mock.patch.object(publish_candidate,'command',side_effect=command),mock.patch.object(tool.subprocess,'run',side_effect=wrong):
            with self.assertRaises(tool.ToolIdentityError): tool._run_compiled_probe('image@sha256:'+'b'*64,'c'*64)
        self.assertEqual(calls[-1],['docker','rm','--force',cid])
        with mock.patch.object(publish_candidate,'command',side_effect=command),mock.patch.object(tool.subprocess,'run',side_effect=subprocess.TimeoutExpired('docker',30)):
            with self.assertRaises(subprocess.TimeoutExpired): tool._run_compiled_probe('image@sha256:'+'b'*64,'c'*64)
        self.assertEqual(calls[-1],['docker','rm','--force',cid])

    def test_help_timeout_removes_exact_owned_container(self):
        import publish_candidate
        cid='a'*64; calls=[]
        def command(args,**kwargs):
            calls.append(args)
            return (cid+'\n').encode() if args[1]=='create' else b''
        with mock.patch.object(publish_candidate,'command',side_effect=command), \
             mock.patch.object(tool.subprocess,'run',side_effect=subprocess.TimeoutExpired('docker',30)):
            with self.assertRaises(subprocess.TimeoutExpired): tool._run_help('image@sha256:'+'b'*64)
        self.assertEqual(calls[-1],['docker','rm','--force',cid])

    def test_actual_preflight_cli_reaches_source_inventory(self):
        original=Path(__file__).resolve().parents[2]; folder=tempfile.TemporaryDirectory(); self.addCleanup(folder.cleanup)
        root=Path(folder.name); sha='a'*40
        paths=[p for p in (original/'src/backend').rglob('*.go') if not p.name.endswith('_test.go')]
        paths += list((original/'src/backend/migrations/space_db').iterdir())+[original/'src/backend/space/Dockerfile.native-migrate']
        for source in paths:
            dest=root/source.relative_to(original); dest.parent.mkdir(parents=True,exist_ok=True); shutil.copyfile(source,dest)
        script=root/'scripts/dev/space_native_tool_identity.py'; script.parent.mkdir(parents=True,exist_ok=True); shutil.copyfile(tool.__file__,script)
        shutil.copyfile(original/'scripts/dev/migrations.py',script.parent/'migrations.py')
        files={p.relative_to(root).as_posix():hashlib.sha256(p.read_bytes()).hexdigest()
               for p in (root/'src/backend').rglob('*.go') if not p.name.endswith('_test.go')}
        for path in (root/'src/backend/migrations/space_db').iterdir():
            files[path.relative_to(root).as_posix()]=hashlib.sha256(path.read_bytes()).hexdigest()
        path=root/'src/backend/space/Dockerfile.native-migrate'; files[path.relative_to(root).as_posix()]=hashlib.sha256(path.read_bytes()).hexdigest()
        request=root/'.github/ci/full-application-build-request.json'; request.parent.mkdir(parents=True,exist_ok=True)
        raw=b'{"schema":"voice-full-application-build-request-v1","cycle":1,"request_id":"11111111111111111111111111111111"}'
        request.write_bytes(raw); files[request.relative_to(root).as_posix()]=hashlib.sha256(raw).hexdigest()
        env={'GH_TOKEN':'fixture','GITHUB_ACTOR':'fixture','GITHUB_REPOSITORY':'Poryadok/VoiceRoot','GITHUB_EVENT_NAME':'push',
             'GITHUB_REF':'refs/heads/master','GITHUB_SHA':sha,'GITHUB_RUN_ID':'7','GITHUB_RUN_ATTEMPT':'1'}
        run={'id':7,'run_attempt':1,'head_sha':sha,'head_branch':'master','event':'push','path':'.github/workflows/ci.yml',
             'status':'in_progress','conclusion':None,'repository':{'full_name':'Poryadok/VoiceRoot'},'head_repository':{'full_name':'Poryadok/VoiceRoot'}}
        fake_auth=SimpleNamespace(API='fixture',_headers=lambda token:{},_head=lambda *args:None,
                                  _get=lambda url,*args:run if '/actions/runs/' in url else {'sha':sha,'parents':[{'sha':'b'*40}]})
        fake_canonical=SimpleNamespace(REQUEST=request.relative_to(root).as_posix(),tree_blob=lambda *args:None,
                                       validate_request=lambda current,previous:json.loads(current))
        fake_common=SimpleNamespace(source_snapshot=lambda root,expected:{'source_sha':sha,'files':files},
                                    Registry=lambda actor,token:SimpleNamespace(get=lambda *args,**kwargs:None))
        with mock.patch.dict(os.environ,env),mock.patch.dict(sys.modules,{'canonical_application_package':fake_canonical,
                'publish_candidate':fake_common,'source_authority':fake_auth}),mock.patch.object(sys,'argv',[str(script),'ci-preflight']):
            with self.assertRaises(SystemExit) as result: runpy.run_path(str(script),run_name='__main__')
        self.assertEqual(result.exception.code,0)
        gate=root/'src/backend/space/internal/nativeinstaller/runner.go'; gate.write_bytes(gate.read_bytes()+b'\n// unreviewed\n')
        files[gate.relative_to(root).as_posix()]=hashlib.sha256(gate.read_bytes()).hexdigest()
        with self.assertRaises(tool.ToolIdentityError): tool.source_inventory(root,{'source_sha':sha,'files':files})

    def test_tool_job_requires_exact_run_and_both_steps(self):
        source='a'*40
        job={'name':'Space native tool image','run_id':7,'run_attempt':2,'head_sha':source,'status':'completed','conclusion':'success',
             'id':12,'steps':[{'name':'Verify Space native tool source','conclusion':'success'},
                              {'name':'Capture Space native tool identity','conclusion':'success'}]}
        self.assertEqual(tool.tool_job([job],source,7,2),job)
        for fault in ('wrong_source','missing_step','failed_step','duplicate'):
            jobs=[copy.deepcopy(job)]
            if fault=='wrong_source': jobs[0]['head_sha']='b'*40
            if fault=='missing_step': jobs[0]['steps'].pop()
            if fault=='failed_step': jobs[0]['steps'][0]['conclusion']='failure'
            if fault=='duplicate': jobs.append(copy.deepcopy(job))
            with self.subTest(fault=fault),self.assertRaises(tool.ToolIdentityError): tool.tool_job(jobs,source,7,2)

    def test_publication_receipt_rejects_cross_run_or_wrong_tool(self):
        identity={'source_sha':'a'*40,'run_id':7,'run_attempt':2,
                  'tag':'ghcr.io/poryadok/voiceroot/space-native-installer:source-'+'a'*40+'-7-2'}
        source={'source_sha':'a'*40,'build_source_sha256':'b'*64,'sql_inventory_sha256':'c'*64}
        receipt={'schema':'voice-space-native-tool-publication-v1',**identity,'index_digest':'sha256:'+'d'*64,
                 'platform_digest':'sha256:'+'e'*64,'config_digest':'sha256:'+'f'*64,
                 'immutable_image':'ghcr.io/poryadok/voiceroot/space-native-installer@sha256:'+'e'*64,
                 'source_inventory':source,'request':{'schema':'voice-full-application-build-request-v1','cycle':1,'request_id':'1'*32},
                 'content':{'executable_sha256':'2'*64,'migrator_sha256':'3'*64,'sql_inventory_sha256':'c'*64,
                            'embedded_values_observed':True,'runtime_receipt_verified':False,'help_exit_code':0,
                            'compiled_gate_probe':'native51-source-gates-reached-v1',
                            'closure':{'schema':'voice-space-native-local-closure-v1','packages':{},'files':{},'raw_sha256':'9'*64}},
                 'runtime_qualified':False,'deployment_performed':False}
        self.assertEqual(tool.validate_publication(receipt,identity,source),receipt)
        for name,value in [('run_id',8),('source_sha','b'*40),('immutable_image','ghcr.io/poryadok/voiceroot/space@sha256:'+'e'*64),('runtime_qualified',True)]:
            wrong=copy.deepcopy(receipt); wrong[name]=value
            with self.subTest(name=name),self.assertRaises(tool.ToolIdentityError): tool.validate_publication(wrong,identity,source)

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
        proof={'executable_sha256':'d'*64,'migrator_sha256':'e'*64,'sql_inventory_sha256':'c'*64,'embedded_values_observed':True,
               'runtime_receipt_verified':False,'help_exit_code':0,'compiled_gate_probe':'native51-source-gates-reached-v1',
               'closure':{'schema':'voice-space-native-local-closure-v1','packages':{},'files':{},'raw_sha256':'9'*64}}
        request={'schema':'voice-full-application-build-request-v1','cycle':1,'request_id':'1'*32}
        context=(Path('/source'),identity,snapshot,inventory,request,Registry())
        with tempfile.TemporaryDirectory() as folder, mock.patch.dict(os.environ,{'RUNNER_TEMP':folder}), \
             mock.patch.object(tool,'_ci_context',return_value=context),mock.patch.object(tool,'_extract_image',return_value=proof):
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
        for path in (root/'src/backend/migrations/space_db').iterdir():
            files[path.relative_to(root).as_posix()]=hashlib.sha256(path.read_bytes()).hexdigest()
        dockerfile=root/'src/backend/space/Dockerfile.native-migrate'
        files[dockerfile.relative_to(root).as_posix()]=hashlib.sha256(dockerfile.read_bytes()).hexdigest()
        snapshot={'source_sha':'a'*40,'files':files}
        proof=tool.source_inventory(root,snapshot)
        self.assertEqual(proof['source_sha'],'a'*40)
        self.assertEqual(len(proof['build_source_sha256']),64)
        self.assertEqual(len(proof['sql_inventory_sha256']),64)
        for fault in ('source','sql','dockerfile','extra_sql','extra_non_sql'):
            wrong=copy.deepcopy(snapshot)
            if fault=='source': wrong['files']['src/backend/space/main.go']='0'*64
            if fault=='sql': wrong['files']['src/backend/migrations/space_db/000001_init.up.sql']='0'*64
            if fault=='dockerfile': wrong['files']['src/backend/space/Dockerfile.native-migrate']='0'*64
            if fault=='extra_sql': wrong['files']['src/backend/migrations/space_db/999999_wrong.up.sql']='0'*64
            if fault=='extra_non_sql': wrong['files']['src/backend/migrations/space_db/foreign.txt']='0'*64
            with self.subTest(fault=fault),self.assertRaises(tool.ToolIdentityError): tool.source_inventory(root,wrong)


if __name__=='__main__': unittest.main()
