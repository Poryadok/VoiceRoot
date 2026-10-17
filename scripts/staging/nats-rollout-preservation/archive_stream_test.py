"""Nonroot tests for production cold-cut exec-stream consumer."""
import base64
import hashlib
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import archive_stream
import native_store
import preserve
import tokenless_job
from tokenless_job_test import BINDING, FakeAPI, RUNNER_IMAGE


OP = 'a' * 12
NONCE = 'b' * 32
POD_UID = '12345678-1234-1234-1234-123456789abc'


def frames(archive, manifest, *, operation=OP, nonce=NONCE, pod_uid=POD_UID,
           corrupt_digest=False):
    raw = Path(archive).read_bytes()
    for sequence, offset in enumerate(range(0, len(raw), 48 << 10)):
        yield (json.dumps({'schema':archive_stream.SCHEMA,'operation':operation,'nonce':nonce,
            'pod_uid':pod_uid,'kind':'data','sequence':sequence,
            'data':base64.b64encode(raw[offset:offset+(48<<10)]).decode('ascii')},
            sort_keys=True,separators=(',',':'))+'\n').encode()
    manifest_raw = json.dumps(manifest, sort_keys=True, separators=(',', ':')).encode()
    for sequence, offset in enumerate(range(0, len(manifest_raw), 48 << 10)):
        yield (json.dumps({'schema':archive_stream.SCHEMA,'operation':operation,'nonce':nonce,
            'pod_uid':pod_uid,'kind':'manifest','sequence':sequence,
            'data':base64.b64encode(manifest_raw[offset:offset+(48<<10)]).decode('ascii')},
            sort_keys=True,separators=(',',':'))+'\n').encode()
    yield (json.dumps({'schema':archive_stream.SCHEMA,'operation':operation,'nonce':nonce,
        'pod_uid':pod_uid,'kind':'complete','frames':(len(raw)+(48<<10)-1)//(48<<10),
        'manifest_frames':(len(manifest_raw)+(48<<10)-1)//(48<<10),
        'archive_sha256':('0'*64 if corrupt_digest else hashlib.sha256(raw).hexdigest()),
        'manifest_sha256':hashlib.sha256(manifest_raw).hexdigest()},
        sort_keys=True,separators=(',',':'))+'\n').encode()


class ArchiveStreamTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name)
        self.root.chmod(0o700)
        self.source = self.root / 'store'
        self.source.mkdir(mode=0o700)
        (self.source / 'meta.inf').write_bytes(b'known native state')
        self.source.joinpath('stream').mkdir(mode=0o700)
        (self.source / 'stream' / 'messages.dat').write_bytes(b'payload' * 8192)
        self.archive = self.root / 'fixture.tar'
        self.manifest = native_store.archive_closed_store(self.source, self.archive)
        self.artifacts = self.root / 'artifacts'
        self.artifacts.mkdir(mode=0o700)
        self.artifact = self.artifacts / 'rollout-before.tar'
        self.events = []

    def tearDown(self):
        self.temp.cleanup()

    def consume(self, source=None, **kwargs):
        return archive_stream.consume(source or frames(self.archive, self.manifest), self.artifact,
            operation=OP,nonce=NONCE,pod_uid=POD_UID,
            persist_intent=lambda row:self.events.append(('intent',row)),
            persist_result=lambda row:self.events.append(('result',row)),
            verify_archive=native_store.verify_archive,**kwargs)

    def test_streamed_archive_is_verified_then_atomically_adopted_for_capture(self):
        result = self.consume()
        self.assertTrue(native_store.verify_archive(self.artifact, self.manifest))
        self.assertEqual(self.artifact.read_bytes(), self.archive.read_bytes())
        self.assertEqual(self.artifact.stat().st_mode & 0o777, 0o600)
        self.assertEqual([name for name,_ in self.events], ['intent','result'])
        self.assertEqual(self.events[-1][1]['state'], 'ARCHIVE_RESULT_READBACK')
        self.assertEqual(result['manifest'], self.manifest)

    def test_stream_interrupt_never_publishes_partial_artifact_or_result(self):
        stream = frames(self.archive, self.manifest)
        incomplete = []
        for _ in range(2): incomplete.append(next(stream))
        with self.assertRaisesRegex(archive_stream.ArchiveStreamError, 'stream_incomplete'):
            self.consume(iter(incomplete))
        self.assertFalse(self.artifact.exists())
        self.assertEqual([name for name,_ in self.events], ['intent'])

    def test_digest_or_bound_pod_identity_mismatch_fails_before_artifact_publish(self):
        with self.assertRaisesRegex(archive_stream.ArchiveStreamError, 'archive_digest_mismatch'):
            self.consume(frames(self.archive,self.manifest,corrupt_digest=True))
        self.assertFalse(self.artifact.exists())
        with self.assertRaisesRegex(archive_stream.ArchiveStreamError, 'frame_identity_invalid'):
            self.consume(frames(self.archive,self.manifest,pod_uid='f'*36))
        self.assertFalse(self.artifact.exists())

    def test_artifact_path_is_create_only(self):
        self.artifact.write_bytes(b'preexisting')
        with self.assertRaises(FileExistsError):
            self.consume()
        self.assertEqual(self.artifact.read_bytes(), b'preexisting')

    def test_recovery_adopts_only_same_verified_artifact_and_clears_stale_stream(self):
        pending = self.artifact.with_name('.' + self.artifact.name + '.' + NONCE + '.stream')
        pending.write_bytes(b'interrupted-partial')
        pending.chmod(0o600)
        self.artifact.write_bytes(self.archive.read_bytes())
        self.artifact.chmod(0o600)
        result = self.consume(allow_existing=True)
        self.assertEqual(result['archive_sha256'], self.manifest['archive_sha256'])
        self.assertFalse(pending.exists())
        self.assertTrue(native_store.verify_archive(self.artifact,self.manifest))

    def test_archive_limit_is_enforced_while_decoding(self):
        with self.assertRaisesRegex(archive_stream.ArchiveStreamError, 'archive_size_limit'):
            self.consume(max_bytes=128)
        self.assertFalse(self.artifact.exists())

    def test_production_capture_cut_uses_fixed_job_stream_and_canonical_readback(self):
        public_nonce = 'd' * 64
        request = {'operation': OP, 'nonce':hashlib.sha256((public_nonce + ':cold-cut-archive').encode()).hexdigest()[:32], 'phase': 'archive'}
        wanted = tokenless_job.document(request, BINDING, RUNNER_IMAGE)

        class API(FakeAPI):
            def __init__(inner):
                super().__init__(wanted)
                inner.archive_nonce = request['nonce']
                inner.list_calls = 0
            def list_operation_jobs(inner, namespace, operation):
                inner.list_calls += 1
                self.assertEqual((namespace, operation), (tokenless_job.NAMESPACE, OP))
                return []
            def exec_pod_archive(inner, namespace, pod_name, pod_uid, operation, nonce, *, existing=False):
                self.assertEqual((namespace,pod_name,pod_uid), (tokenless_job.NAMESPACE,
                    inner.pod['metadata']['name'], inner.pod['metadata']['uid']))
                self.assertEqual(operation, OP)
                self.assertFalse(existing)
                return frames(self.archive,self.manifest,nonce=nonce,pod_uid=pod_uid)

        api = API()
        base = self.root / ('rollout-' + OP)
        base.mkdir(mode=0o700)
        journal = []
        runtime = tokenless_job.PvcArchiveRuntime(api,base,BINDING,RUNNER_IMAGE,OP,public_nonce,
            persist_intent=journal.append,persist_result=journal.append)
        with patch.object(preserve,'native_catalog',return_value={
                'account':'A','streams':[],'consumers':[]}), \
             patch.object(preserve,'isolated_census',return_value={'streams':[]}), \
             patch.object(preserve,'verify_catalog'), \
             patch.object(preserve,'closed_native_states',return_value={}):
            cut = preserve.capture_cut(runtime,'/host/pv/path-must-not-be-opened',lambda:None)
        self.assertTrue(native_store.verify_archive(base/'rollout-before.tar',cut['manifest']))
        self.assertEqual(cut['manifest'],self.manifest)
        self.assertEqual(api.list_calls,1)
        self.assertTrue(api.deleted)
        self.assertEqual(journal[-1]['state'],'ARCHIVE_RESULT_READBACK')
        created = next(call[2] for call in api.calls if call[0] == 'create_job')
        job = created['spec']
        pod_spec = job['template']['spec']
        container = pod_spec['containers'][0]
        self.assertEqual(job['activeDeadlineSeconds'],43200)
        self.assertNotIn('ttlSecondsAfterFinished',job)
        self.assertFalse(pod_spec['automountServiceAccountToken'])
        self.assertEqual(pod_spec['securityContext']['runAsUser'],65532)
        self.assertEqual(pod_spec['nodeName'],BINDING['node_name'])
        self.assertEqual(container['command'],['/voice-nats-runner/phase'])
        self.assertEqual(container['args'],['--serve','--operation',OP,'--nonce',request['nonce']])
        self.assertEqual(container['env'],[{'name':'POD_UID','valueFrom':{'fieldRef':{'fieldPath':'metadata.uid'}}}])
        self.assertEqual([volume['name'] for volume in pod_spec['volumes']],['data','scratch'])
        self.assertTrue(pod_spec['volumes'][0]['persistentVolumeClaim']['readOnly'])
        self.assertEqual(pod_spec['volumes'][1]['emptyDir']['sizeLimit'],'49Gi')
        self.assertTrue(next(mount for mount in container['volumeMounts'] if mount['name']=='data')['readOnly'])
        self.assertEqual(container['resources']['limits']['ephemeral-storage'],'49Gi')
        self.assertEqual(len(pod_spec['containers']),1)

    def test_interrupted_production_stream_reconciles_same_job_and_pod_only(self):
        request = {'operation': OP, 'nonce': 'c' * 32, 'phase': 'archive'}
        wanted = tokenless_job.document(request,BINDING,RUNNER_IMAGE)

        class API(FakeAPI):
            def __init__(inner):
                super().__init__(wanted)
                inner.stream_count = 0
            def exec_pod_archive(inner, namespace, pod_name, pod_uid, operation, nonce, *, existing=False):
                inner.stream_count += 1
                self.assertEqual(operation, OP)
                self.assertEqual(nonce, request['nonce'])
                self.assertTrue(existing if inner.stream_count > 1 else not existing)
                all_frames = list(frames(self.archive,self.manifest,nonce=nonce,pod_uid=pod_uid))
                return iter(all_frames[:1] if inner.stream_count == 1 else all_frames)
        api = API(); events=[]
        base = self.root / ('rollout-' + OP); base.mkdir(mode=0o700)
        target = base/'rollout-before.tar'
        persist = lambda row: events.append(dict(row))
        with self.assertRaisesRegex(archive_stream.ArchiveStreamError,'stream_incomplete'):
            tokenless_job.run_archive(api,request,BINDING,RUNNER_IMAGE,target,
                persist_intent=persist,persist_result=persist,verify_archive=native_store.verify_archive)
        journal = events[-1]
        self.assertEqual(journal['state'],'ARCHIVE_STREAM_INTENT')
        self.assertFalse(target.exists()); self.assertFalse(getattr(api,'deleted',False))
        result=tokenless_job.reconcile_archive(api,request,BINDING,RUNNER_IMAGE,journal,target,
            persist_intent=persist,persist_result=persist,verify_archive=native_store.verify_archive)
        self.assertEqual(api.stream_count,2)
        self.assertEqual(result['archive_sha256'],self.manifest['archive_sha256'])
        self.assertTrue(api.deleted)


if __name__ == '__main__': unittest.main()
