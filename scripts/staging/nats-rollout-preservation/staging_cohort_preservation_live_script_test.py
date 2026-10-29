"""Actual fixed Stage shell scripts on an owned PG clone and private mount.

This proves executable/SQL/file mechanics, not Kubernetes scheduling, PVC
provisioning, authenticated publication, target authority or live admission.
The immutable Tool is a retained test build; its source revision is explicit.
"""
import hashlib
import json
import os
from pathlib import Path
import re
import secrets
import stat
import subprocess
import sys
import tempfile
import time
import unittest

sys.path.insert(0,str(Path(__file__).resolve().parents[1]/'nats-known-baseline'))
sys.path.insert(0,str(Path(__file__).resolve().parents[2]/'dev'))
from auth_user_adoption_runtime import QUALIFIED_TOOLS
from staging_cohort_preservation_runtime import DOCKER
from staging_space_native import LIVE_UPGRADE_SCRIPT,LIVE_VERIFY_SCRIPT


@unittest.skipUnless(os.environ.get('VOICE_COHORT_NATIVE_IMAGE') and os.environ.get('VOICE_COHORT_NATIVE_ROOT'),
                     'owned immutable Tool and daemon-shared volume required')
class LiveScriptTests(unittest.TestCase):
    def test_actual_scripts_preserve_private_origin_and_verify_same_clone(self):
        image=os.environ['VOICE_COHORT_NATIVE_IMAGE'];revision=os.environ['VOICE_COHORT_NATIVE_SOURCE']
        operation='live-script-'+secrets.token_hex(8);owned=[];removed_volumes=[]
        def docker(*args,body=None,timeout=120):
            result=subprocess.run(DOCKER+list(args),input=body,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=timeout)
            if result.returncode:
                # No private origin, SQL dump, credentials or completed events.
                self.fail('owned '+str(args[:2])+' failed: '+result.stderr.decode(errors='replace')[-1200:])
            return result.stdout
        def create(*args):
            cid=docker('create','--label','voice.test.owner='+operation,*args).decode().strip()
            self.assertRegex(cid,r'^[a-f0-9]{64}$');owned.append(cid);return cid
        def postgres(label):
            cid=create('--name',operation+'-'+label,'--network','none','--memory','512m','--pids-limit','128',
                '-e','POSTGRES_PASSWORD=fixture-only','-e','POSTGRES_USER=postgres',QUALIFIED_TOOLS['postgres'])
            docker('start',cid);deadline=time.monotonic()+60
            while True:
                ready=subprocess.run(DOCKER+['exec',cid,'pg_isready','-h','127.0.0.1','-U','postgres'],
                    stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,timeout=5)
                if ready.returncode==0:return cid
                if time.monotonic()>deadline:self.fail('owned postgres unavailable')
                time.sleep(.2)
        def sql(cid,text):
            return docker('exec','-i',cid,'psql','-X','-qAt','-v','ON_ERROR_STOP=1','-U','postgres','-d','space_db',
                          body=text.encode()).decode().strip()
        try:
            info=json.loads(docker('image','inspect',image))[0]
            self.assertEqual(info['Id'],image)
            self.assertEqual(info['Config']['Labels']['org.opencontainers.image.revision'],revision)
            original=postgres('source')
            docker('exec','-i',original,'psql','-X','-q','-v','ON_ERROR_STOP=1','-U','postgres',
                   body=b'CREATE DATABASE space_db OWNER postgres;')
            sql(original,'ALTER SCHEMA public OWNER TO postgres;')
            migrator=create('--network','container:'+original,'--entrypoint','/usr/local/bin/migrate',image,
                '-path','/migrations','-database','postgres://postgres:fixture-only@127.0.0.1:5432/space_db?sslmode=disable','goto','24')
            docker('start',migrator);self.assertEqual(docker('wait',migrator).strip(),b'0')
            sql(original,"INSERT INTO spaces(id,name,owner_profile_id) VALUES('11111111-1111-4111-8111-111111111111','retained source','22222222-2222-4222-8222-222222222222');")
            original_before=sql(original,"SELECT version||':'||dirty FROM schema_migrations; SELECT name FROM spaces;")
            self.assertEqual(original_before,'24:false\nretained source')
            dump=docker('exec',original,'pg_dump','-U','postgres','-Fc','-d','space_db')
            clone=postgres('clone')
            docker('exec','-i',clone,'psql','-X','-q','-v','ON_ERROR_STOP=1','-U','postgres',
                   body=b'CREATE DATABASE space_db OWNER postgres;')
            docker('exec','-i',clone,'pg_restore','-U','postgres','-d','space_db','--exit-on-error',body=dump)
            source_id=sql(original,'SELECT system_identifier FROM pg_control_system();')
            clone_id=sql(clone,'SELECT system_identifier FROM pg_control_system();')
            self.assertNotEqual(source_id,clone_id)
            source=Path(__file__).resolve().parents[3];backend=source/'src/backend'
            manifest=''.join(hashlib.sha256(p.read_bytes()).hexdigest()+'  ./'+p.relative_to(backend).as_posix()+'\n'
                for p in sorted(backend.rglob('*.go'),key=lambda p:p.relative_to(backend).as_posix().encode()) if not p.name.endswith('_test.go'))
            build=hashlib.sha256(manifest.encode()).hexdigest()
            with tempfile.TemporaryDirectory(dir=os.environ['VOICE_COHORT_NATIVE_ROOT']) as directory:
                evidence=Path(directory);os.chown(evidence,65532,65532);evidence.chmod(0o700)
                def run_script(kind,script,readonly):
                    cid=create('--name',operation+'-'+kind,'--network','container:'+clone,
                        '--user','65532:65532','--read-only','--cap-drop','ALL','--security-opt','no-new-privileges:true',
                        '--memory','512m','--pids-limit','128',
                        '--mount','type=bind,source='+str(evidence)+',target=/evidence'+(',readonly' if readonly else ''),
                        '-e','PGHOST=127.0.0.1','-e','PGPORT=5432','-e','PGUSER=postgres','-e','PGPASSWORD=fixture-only',
                        '-e','PGSSLMODE=disable','-e','SPACE_NATIVE_REQUIRED_SOURCE_SHA256='+build,
                        '--entrypoint','/bin/sh',image,'-ceu',script,'native-space',operation,'original-snapshot')
                    docker('start',cid);exitcode=docker('wait',cid,timeout=360).strip()
                    self.assertEqual(exitcode,b'0','fixed '+kind+' script failed (private output retained only in owned mount)')
                    output=docker('logs',cid).decode()
                    self.assertTrue(all(re.fullmatch(r'[a-f0-9]{64}  /evidence/(origin.json|completed.jsonl)',line)
                                        or kind.startswith('verify') and line=='verification=ok'
                                        for line in output.splitlines()),
                        'script output classes: '+json.dumps([{'bytes':len(line),
                            'numeric':line if re.fullmatch(r'[0-9]{1,3}',line) else None,
                            'hash_line':bool(re.match(r'^[a-f0-9]{64} ',line)),
                            'json_keys':sorted(json.loads(line)) if line.startswith('{') and line.endswith('}') else None}
                            for line in output.splitlines()]))
                    return output
                upgraded=run_script('upgrade',LIVE_UPGRADE_SCRIPT,False)
                original_bytes=(evidence/'origin.json').read_bytes();completed=(evidence/'completed.jsonl').read_bytes()
                for path in (evidence/'origin.json',evidence/'completed.jsonl'):
                    row=path.lstat();self.assertEqual(stat.S_IMODE(row.st_mode),0o600)
                    self.assertEqual(row.st_uid,65532);self.assertEqual(row.st_nlink,1)
                self.assertEqual(len(completed.splitlines()),8)
                expected=''.join(hashlib.sha256(raw).hexdigest()+'  /evidence/'+name+'\n'
                                 for name,raw in (('origin.json',original_bytes),('completed.jsonl',completed)))
                self.assertEqual(upgraded,expected)
                verified=run_script('verify',LIVE_VERIFY_SCRIPT,True)
                self.assertEqual(verified,expected+'verification=ok\n')
                self.assertEqual((evidence/'origin.json').read_bytes(),original_bytes)
                self.assertEqual((evidence/'completed.jsonl').read_bytes(),completed)
                again=run_script('verify-again',LIVE_VERIFY_SCRIPT,True)
                self.assertEqual(again,expected+'verification=ok\n')
                self.assertEqual((evidence/'origin.json').read_bytes(),original_bytes)
                self.assertEqual((evidence/'completed.jsonl').read_bytes(),completed)
                self.assertEqual(sql(clone,'SELECT version||\':\'||dirty FROM schema_migrations; SELECT name FROM spaces;'),
                                 '51:false\nretained source')
                self.assertEqual(sql(original,'SELECT version||\':\'||dirty FROM schema_migrations; SELECT name FROM spaces;'),original_before)
                result={'schema':'voice-space-live-script-fixture-v1','tool_image':image,'tool_source':revision,
                    'scripts':{'upgrade':hashlib.sha256(LIVE_UPGRADE_SCRIPT.encode()).hexdigest(),
                               'verify':hashlib.sha256(LIVE_VERIFY_SCRIPT.encode()).hexdigest()},
                    'original_version':24,'clone_version':51,'completed_events':8,'private_mode':'0600',
                    'repeat_verification':True,'original_unchanged':True,'kubernetes_proved':False,'live_admission':False}
        finally:
            for cid in reversed(owned):
                row=json.loads(docker('inspect',cid))[0]
                self.assertEqual(row['Config']['Labels']['voice.test.owner'],operation)
                volumes=[mount for mount in row.get('Mounts',[]) if mount.get('Type')=='volume']
                for mount in volumes:
                    self.assertEqual(row['Config']['Image'],QUALIFIED_TOOLS['postgres'])
                    self.assertEqual(mount['Destination'],'/var/lib/postgresql/data')
                    self.assertRegex(mount['Name'],r'^[a-f0-9]{64}$')
                    self.assertNotIn(mount['Name'],removed_volumes)
                docker('rm','-f','-v',cid)
                self.assertFalse(docker('ps','-a','--filter','id='+cid,'--format','{{.ID}}').strip())
                for mount in volumes:
                    self.assertFalse(docker('volume','ls','--filter','name=^'+mount['Name']+'$','--format','{{.Name}}').strip())
                    removed_volumes.append(mount['Name'])
        self.assertEqual(len(removed_volumes),2)
        result['anonymous_pg_volumes_removed']=len(removed_volumes)
        result['cleanup_verified']=True
        print(json.dumps(result,sort_keys=True),flush=True)


if __name__=='__main__':unittest.main()
