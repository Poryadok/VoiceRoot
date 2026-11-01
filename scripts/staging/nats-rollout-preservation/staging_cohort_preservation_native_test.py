"""Real isolated NATS copy/restore transport; source/Kube admission is synthetic.

VOICE_COHORT_NATIVE_ROOT must be an owned test volume mounted at the same
absolute path in this runner and its Docker daemon. No live namespace, PVC,
account or production credential is used. The fixture has a populated stream
and no consumers; native consumer decoding remains separate evidence.
"""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import staging_cohort_preservation_test as cut_tests
import staging_cohort_preservation as owner
import staging_cohort_preservation_runtime as runtime
import guard
import resource_claim
from docker_runtime import DockerRuntime
from scenario import ready
from rollout_census import census,semantic
import preserve


@unittest.skipUnless(os.environ.get('VOICE_COHORT_NATIVE_ROOT'),'owned shared Docker test volume required')
class NativeTransportTests(unittest.TestCase):
    def test_populated_closed_copy_and_readback_with_real_root_session(self):
        volume=Path(os.environ['VOICE_COHORT_NATIVE_ROOT']).resolve(strict=True)
        with tempfile.TemporaryDirectory(dir=volume) as directory:
            root=Path(directory);(root/'installed').mkdir(mode=0o700)
            before={'metadata':{'name':'voice-user','namespace':'voice-staging','uid':'user',
                'resourceVersion':'3','generation':1},'spec':{'replicas':1}}
            with patch.object(guard,'ROOT',root),patch.object(owner,'_source'),patch.object(owner,'revalidate_paused'),\
                patch.object(owner,'_closed_copy_authority'):
                session,_=cut_tests.CutEffectTests().fixture(root,before)
                base=owner._runtime_base(session);base.mkdir(mode=0o700)
                inputs=base/'inputs';inputs.mkdir(mode=0o700)
                account='A'*56;(inputs/'account.public').write_text(account);(inputs/'account.public').chmod(0o440)
                config=('port:4222\nhttp_port:8222\njetstream { store_dir:"/data" }\n'
                    'accounts { '+account+': { jetstream:enabled, users:[{user:fixture,password:isolated}] } }\n')
                (base/'server.conf').write_text(config);(base/'server.conf').chmod(0o440);os.chown(base/'server.conf',0,65532)
                store=base/'fixture-store';store.mkdir(mode=0o700);os.chown(store,65532,65532)
                source= DockerRuntime(base,'source'+session.operation)
                broker=source.start_broker('fixture',store)
                source_cid=source.owned[broker]['id']
                try:
                    ready(source,broker)
                    stream=json.dumps({'name':'FIXTURE','subjects':['fixture.subject'],'storage':'file','num_replicas':1},separators=(',',':')).encode()
                    def protocol(payload):
                        result=subprocess.run(runtime.DOCKER+['exec','-i',source_cid,'/bin/busybox','sh','-c',
                            '(cat; sleep 1) | /bin/busybox nc -w 2 127.0.0.1 4222'],
                            input=b'CONNECT {"user":"fixture","pass":"isolated","verbose":false}\r\n'+payload,
                            stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=10)
                        self.assertEqual(result.returncode,0);return result.stdout
                    created=protocol(b'SUB _INBOX.CREATE 1\r\nPUB $JS.API.STREAM.CREATE.FIXTURE _INBOX.CREATE '+str(len(stream)).encode()+b'\r\n'+stream+b'\r\nPING\r\n')
                    self.assertIn(b'"name":"FIXTURE"',created)
                    written=protocol(b'SUB _INBOX.WRITE 2\r\nPUB fixture.subject _INBOX.WRITE 9\r\npreserved\r\nPING\r\n')
                    self.assertIn(b'"stream":"FIXTURE"',written)
                    live=census(source,broker,account)
                    self.assertEqual(live['streams'][0]['state']['messages'],1)
                    source.stop(broker)
                finally:
                    # Exact test-owned source only. It is never part of the
                    # production NativeRuntime command authorization surface.
                    subprocess.run(runtime.DOCKER+['rm','-f',source_cid],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,check=True,timeout=20)
                with resource_claim.command_scope(session.check):
                    native=runtime.NativeRuntime(session)
                    cut=preserve.capture_cut(native,store,session.check,live_catalog=live)
                    self.assertEqual(semantic(cut['census']),semantic(live))
                    self.assertEqual(cut['census']['streams'][0]['state']['messages'],1)
                    owner._write(owner._base(session)/'native-cut.json',cut)
                    readback=owner._native_readback(session,base/'rollout-before.tar')
                    self.assertEqual(readback['native_messages'],preserve.native_message_files(cut['manifest']))
                    self.assertTrue(readback['native_messages']['files'])
                    self.assertTrue(readback['native_archive_verified'])
                    owner._no_pending(session)
                    native.no_operation_containers()
                    # Continue through the actual successor permit and Docker
                    # transport; only source/Kubernetes authority is synthetic.
                    row=session.journal.read();row.update(phase='prerequisites-captured',target_sha256='d'*64)
                    session.journal.save(row);session.expected_target='d'*64
                    continuation={'cut':cut}
                    with patch.object(owner,'load_rollout_continuation',return_value=continuation), \
                         patch.object(owner,'_phase_continuity'), \
                         patch.object(owner,'_closed_copy_authority',return_value={'plan':'qualified'}) as authority:
                        self.assertEqual(owner.successor_nats_authority(session),{'plan':'qualified'})
                        self.assertEqual(authority.call_count,1)
                    owner._no_pending(session)
                    remaining=subprocess.run(runtime.DOCKER+['ps','-a','--filter',
                        'label=voice.known-nats.operation='+session.operation,'--format','{{.ID}}'],
                        capture_output=True,check=True,timeout=20)
                    self.assertEqual(remaining.stdout.strip(),b'')


if __name__=='__main__':unittest.main()
