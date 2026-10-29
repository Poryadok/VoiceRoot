"""Real native image/PG dump/restore test; source, CI, Kube and TT are fixtures.

The image is an owned build, never a publication or release pin. Set the three
VOICE_COHORT_NATIVE_* variables only in the owner's isolated Linux runner.
"""
import copy
import hashlib
import importlib
import io
import json
import os
from pathlib import Path
import re
import secrets
import subprocess
import sys
import tarfile
import tempfile
import time
import unittest
from unittest.mock import patch

sys.path.insert(0,str(Path(__file__).resolve().parents[1]/'nats-known-baseline'))
sys.path.insert(0,str(Path(__file__).resolve().parents[2]/'dev'))
import commands
import guard
import resource_claim
import staging_cohort_preservation as owner
import staging_cohort_preservation_pg as pg
import staging_cohort_preservation_space as native
import staging_cohort_preservation_test as cut_tests
from staging_cohort_preservation_runtime import DOCKER
from auth_user_adoption_runtime import QUALIFIED_TOOLS


class NativeAuthorityTests(unittest.TestCase):
    def test_handoff_requires_bound_offnode_result_and_original_identity(self):
        with tempfile.TemporaryDirectory(dir='/root') as directory:
            root=Path(directory);(root/'installed').mkdir(mode=0o700)
            with patch.object(guard,'ROOT',root),patch.object(owner,'_source'):
                session,_=cut_tests.CutEffectTests().fixture(root,{'spec':{'replicas':1}})
                with resource_claim.command_scope(session.check):
                    base=owner._base(session);plan=owner.load_plan(session)
                    archive={'binding':{'snapshot_id':'original'}};tool={'source_sha':session.context['commit']}
                    native_record={'status':'COMPLETE','attempt':'c'*32}
                    offnode={'operation':session.operation,'plan_sha256':owner.digest(plan),'nonce':'c'*32,
                        'phase':'offnode','status':'COMPLETE','proof':{'native_space':native_record,
                            'space_native_rehearsal_sha256':owner.digest(native_record)}}
                    qualified={'archive_sha256':owner.digest(archive),'offnode_restore_sha256':owner.digest(offnode)}
                    owner._write(base/'archive-record.json',archive);owner._write(base/'offnode-restore.json',offnode)
                    owner._write(base/'restore-attempt.json',offnode)
                    original={'engine':{'system_identifier':'original-system'},'server_start':'start',
                        'database_name':'space_db','database_oid':42,'database_owner':'original_owner',
                        'schema_owner':'original_owner','installer_role':'original_owner'}
                    method=getattr(native,'load_native_authority',None);self.assertTrue(callable(method))
                    with patch.object(owner,'load_qualified',return_value=qualified),patch.object(native,'load_tool',return_value=tool),\
                        patch.object(native,'_retained_rehearsal',return_value=native_record),patch.object(pg,'guard_space_identity',return_value=original):
                        result=method(session)
                        self.assertEqual(result,{'schema':'voice-staging-space-native-authority-v1','operation':session.operation,
                            'preservation_sha256':owner.digest(qualified),'tool':tool,'original':original,'rehearsal':native_record})
                        owner.save(base/'restore-attempt.json',{**offnode,'nonce':'d'*32})
                        with self.assertRaisesRegex(Exception,'cohort_space_offnode_changed'):method(session)
                        owner.save(base/'restore-attempt.json',offnode)
                        for change in ({'phase':'local'},{'status':'PENDING'},{'proof':{'native_space':native_record,'space_native_rehearsal_sha256':'0'*64}}):
                            bad={**offnode,**change};owner.save(base/'offnode-restore.json',bad)
                            owner.save(base/'restore-attempt.json',bad);qualified['offnode_restore_sha256']=owner.digest(bad)
                            with self.assertRaisesRegex(Exception,'cohort_space_offnode_changed'):method(session)


@unittest.skipUnless(os.environ.get('VOICE_COHORT_NATIVE_IMAGE'),'owned native image required')
class NativeSpaceTests(unittest.TestCase):
    def test_real_dump_clone_native_upgrade_original_verify_and_tamper(self):
        image=os.environ['VOICE_COHORT_NATIVE_IMAGE'];revision=os.environ['VOICE_COHORT_NATIVE_SOURCE']
        original_run=subprocess.run;original_popen=subprocess.Popen
        def docker(*args,body=None,timeout=120):
            result=original_run(DOCKER+list(args),input=body,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=timeout)
            if result.returncode:raise AssertionError('owned fixture '+str(args[:2])+': '+result.stderr.decode()[-1500:])
            return result.stdout
        name='voice-cohort-space-source-'+secrets.token_hex(12)
        cid=docker('create','--name',name,'--network','none','--label','voice.test.owner='+name,
            '-e','POSTGRES_PASSWORD=fixture-only','-e','POSTGRES_USER=postgres',QUALIFIED_TOOLS['postgres']).decode().strip()
        carrier=None
        try:
            docker('start',cid);deadline=time.monotonic()+60
            while original_run(DOCKER+['exec',cid,'pg_isready','-h','127.0.0.1','-U','postgres'],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL).returncode:
                if time.monotonic()>deadline:self.fail('owned source unavailable')
                time.sleep(.2)
            docker('exec','-i',cid,'psql','-X','-q','-v','ON_ERROR_STOP=1','-U','postgres',body=
                b"CREATE DATABASE space_db OWNER postgres;")
            docker('exec','-i',cid,'psql','-X','-q','-v','ON_ERROR_STOP=1','-U','postgres','-d','space_db',body=
                b'ALTER SCHEMA public OWNER TO postgres;')
            docker('run','--rm','--network','container:'+cid,'--entrypoint','/usr/local/bin/migrate',image,
                '-path','/migrations','-database','postgres://postgres:fixture-only@127.0.0.1:5432/space_db?sslmode=disable','goto','24')
            docker('exec','-i',cid,'psql','-X','-q','-v','ON_ERROR_STOP=1','-U','postgres','-d','space_db',body=
                b"INSERT INTO public.spaces(id,name,owner_profile_id) VALUES('11111111-1111-4111-8111-111111111111','original','22222222-2222-4222-8222-222222222222'); INSERT INTO public.space_members(space_id,profile_id,nickname) VALUES('11111111-1111-4111-8111-111111111111','33333333-3333-4333-8333-333333333333','original member');")
            carrier=docker('create','--network','none','--entrypoint','/bin/true',image).decode().strip()
            def image_hash(path):
                with tarfile.open(fileobj=io.BytesIO(docker('cp',carrier+':'+path,'-'))) as stream:
                    return hashlib.sha256(stream.extractfile(stream.getmembers()[0]).read()).hexdigest()
            executable=image_hash('/usr/local/bin/native-migrate');migrator=image_hash('/usr/local/bin/migrate')
            source=Path(__file__).resolve().parents[3]
            catalog_sql=(source/'src/backend/space/internal/nativeinstaller/legacy25_catalog.sql').read_text()
            def structural(target):
                return docker('exec','-i',target,'psql','-X','-qAt','-v','ON_ERROR_STOP=1','-U','postgres','-d','space_db',
                    body=('BEGIN READ ONLY; SET LOCAL search_path=pg_catalog;'+catalog_sql+';ROLLBACK;').encode()).strip()
            source_catalog=structural(cid)
            self.assertEqual(hashlib.sha256(source_catalog).hexdigest(),'9152b7fb2f8e3d1f836832760d11059c528296688f79022dc6d19c282c16e650')
            backend=source/'src/backend'
            source_manifest=''.join(hashlib.sha256(p.read_bytes()).hexdigest()+'  ./'+p.relative_to(backend).as_posix()+'\n'
                for p in sorted(backend.rglob('*.go'),key=lambda p:p.relative_to(backend).as_posix().encode()) if not p.name.endswith('_test.go'))
            # Only the CI/artifact claim is synthetic. Actual executable bytes,
            # source build identity, SQL inventory and clone work are checked.
            inventory=re.search(r'const InventorySHA256 = "([a-f0-9]{64})"',
                (source/'src/backend/space/internal/nativeinstaller/source_manifest.go').read_text())[1]
            publication={'source_sha':revision,'config_digest':image,'platform_digest':'sha256:'+'f'*64,
                'immutable_image':'ghcr.io/poryadok/voiceroot/space-native-installer@sha256:'+'f'*64,
                'runtime_qualified':False,'deployment_performed':False,
                'source_inventory':{'build_source_sha256':hashlib.sha256(source_manifest.encode()).hexdigest()},
                'content':{'executable_sha256':executable,'migrator_sha256':migrator,'sql_inventory_sha256':inventory}}
            with tempfile.TemporaryDirectory(dir=os.environ['VOICE_COHORT_NATIVE_ROOT'],delete=False) as directory:
                root=Path(directory);(root/'installed').mkdir(mode=0o700)
                approved={'source_files':{p.relative_to(source).as_posix():hashlib.sha256(p.read_bytes()).hexdigest()
                    for prefix in ('src/backend/migrations','src/backend/auth/src/main/resources/db/migration')
                    for p in (source/prefix).rglob('*.sql')}}
                actual_capture=commands.capture
                def capture(argv,**kwargs):
                    for fixed in (pg.IDENTITY_ARGS,pg.SPACE_IDENTITY_ARGS):
                        if argv==owner.PREFIX+fixed:
                            commands.authority_guard(argv)
                            return docker('exec',cid,*fixed[fixed.index('--')+1:])
                    if argv[:len(pg.EXEC)]==pg.EXEC:
                        commands.authority_guard(argv)
                        return docker('exec','-i',cid,*argv[len(pg.EXEC):],body=kwargs.get('body'))
                    return actual_capture(argv,**kwargs)
                def popen(argv,*args,**kwargs):
                    for fixed in (pg.IDENTITY_ARGS,pg.SPACE_IDENTITY_ARGS):
                        if argv==owner.PREFIX+fixed:
                            argv=DOCKER+['exec',cid,*fixed[fixed.index('--')+1:]]
                            break
                    if argv[:len(pg.EXEC)]==pg.EXEC:argv=DOCKER+['exec','-i',cid,*argv[len(pg.EXEC):]]
                    return original_popen(argv,*args,**kwargs)
                namespace='11111111-1111-4111-8111-111111111111'
                engine={'pod':{'uid':'22222222-2222-4222-8222-222222222222','container_id':'containerd://'+'c'*64,'restart_count':0},
                    'pvc':{'uid':'33333333-3333-4333-8333-333333333333'},'pv':{'uid':'44444444-4444-4444-8444-444444444444'}}
                with patch.object(guard,'ROOT',root),patch.object(owner,'_source',return_value=(approved,{},{})),\
                    patch.object(owner,'revalidate_paused'),patch.object(owner,'shared_module',side_effect=lambda s,m:importlib.import_module(m)),\
                    patch('realtime_bootstrap.engine_snapshot',return_value=(namespace,engine)),\
                    patch.object(commands,'capture',side_effect=capture),patch.object(subprocess,'Popen',side_effect=popen):
                    c=cut_tests.context();c['commit']=revision;card=cut_tests.card(c)
                    session=resource_claim.Session(c,'a'*12,'b'*64,lambda:copy.deepcopy(card),resource_claim.RootJournal(root))
                    session.admit(start=True);base=owner._base(session);owner.private._private_directory(base.parent);owner.private._private_directory(base)
                    precut={'context':{'original_snapshots':{},'marker':{}}}
                    plan={key:'c'*64 for key in owner.PLAN_FIELDS}
                    plan.update(schema=owner.SCHEMA,operation=session.operation,context=c,helper_sha256=session.helper,
                        namespace_uid=namespace,actual_before=card['observed']['images'],candidate={'commit':revision,'images':c['images']},
                        precut_capture_sha256=owner.digest(precut))
                    tool={'schema':'voice-staging-cohort-space-tool-v1','operation':session.operation,
                        'source_capture_sha256':plan['source_capture_sha256'],'approved_sha256':plan['approved_sha256'],
                        'selected':True,'authenticated_publication':{'artifact_id':42,'artifact_sha256':'f'*64,'publication':publication}}
                    plan['space_tool_sha256']=owner.digest(tool)
                    owner._write(base/'space-tool.json',tool);owner._write(base/'precut.json',precut)
                    owner._write(base/'checkpoint.json',{'schema':'voice-staging-cohort-preservation-checkpoint-v1','plan':plan})
                    session.bind_cohort_preservation(plan,plan['actual_before'])
                    owner.private._private_directory(root/'installed'/'sources');workspace=root/'installed'/'sources'/session.operation
                    owner.private._private_directory(workspace)
                    import shutil
                    shutil.copytree(source/'src/backend/migrations',workspace/'src/backend/migrations')
                    with resource_claim.command_scope(session.check):
                        archive=owner._capture_archive(session)
                        self.assertEqual(set(archive['databases']),{'postgres','template1','space_db'})
                        original_identity=pg.guard_space_identity(session)
                        self.assertEqual(original_identity['database_owner'],'postgres')
                        self.assertEqual(original_identity['installer_role'],'postgres')
                        self.assertEqual(original_identity['database_oid'],owner._read(base/'catalog.json')['databases']['space_db'])
                        failures=[]
                        def trace(frame,event,arg):
                            if event=='exception' and ('cohort' in frame.f_code.co_filename or 'commands.py' in frame.f_code.co_filename):
                                failures.append((Path(frame.f_code.co_filename).name,frame.f_lineno,type(arg[1]).__name__,str(arg[1])[:200]))
                            return trace
                        sys.settrace(trace)
                        try:result=owner._qualify_restore(session,'local')
                        except Exception:
                            print('Owned diagnostic root:',root,'; failures:',failures[-14:],file=sys.stderr)
                            for native_path in base.glob('restore/*/native-space.json'):
                                row=owner._read(native_path);restored_catalog=structural(row['clone_id'])
                                (root/'source-structural.json').write_bytes(source_catalog)
                                (root/'restored-structural.json').write_bytes(restored_catalog)
                                a=json.loads(source_catalog);b=json.loads(restored_catalog)
                                print('Structural changed categories:',[k for k in a if a[k]!=b[k]],file=sys.stderr)
                            raise
                        finally:sys.settrace(None)
                        self.assertEqual(result['proof']['databases']['space_db'],{'mode':'migrate','from':24,'to':51})
                        record=owner.load_native_space_rehearsal(session)
                        self.assertEqual(record,result['proof']['native_space'])
                        self.assertEqual(record['receipt']['initialization_count'],1)
                        self.assertEqual(record['receipt']['space_count'],1)
                        self.assertEqual(record['receipt']['final']['version'],51)
                        self.assertNotEqual(record['clone_identity']['system_identifier'],owner._read(base/'catalog.json')['engine']['system_identifier'])
                        retained=base/'restore'/record['attempt']/'native-space.json'
                        changed=copy.deepcopy(record);changed['status']='PENDING';owner.save(retained,changed)
                        with self.assertRaisesRegex(Exception,'rehearsal_incomplete'):owner.load_native_space_rehearsal(session)
                        owner.save(retained,record)
                        evidence=base/'restore'/record['attempt']/'native-origin.json'
                        raw=evidence.read_bytes();evidence.write_bytes(raw+b'\n')
                        with self.assertRaisesRegex(Exception,'evidence_changed'):owner.load_native_space_rehearsal(session)
                        evidence.write_bytes(raw)
                        effect=base/'effects'/(record['commands'][3]['effect_id']+'.json')
                        original=owner._read(effect);owner.save(effect,{**original,'response_sha256':'0'*64})
                        with self.assertRaisesRegex(Exception,'command_changed'):owner.load_native_space_rehearsal(session)
                        owner.save(effect,original)
                        self.assertEqual(owner.load_native_space_rehearsal(session),record)
                        with self.assertRaisesRegex(Exception,'attempt_invalid'):owner.native_space_rehearsal(session)
                    live=docker('exec',cid,'psql','-X','-qAt','-U','postgres','-d','space_db','-c',
                        "SELECT version FROM public.schema_migrations; SELECT name FROM public.spaces")
                    self.assertEqual(live.splitlines(),[b'24',b'original'])
        finally:
            if carrier:docker('rm','-f',carrier)
            docker('rm','-f','-v',cid)


if __name__=='__main__':unittest.main()
