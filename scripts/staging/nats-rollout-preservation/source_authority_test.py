import copy
from datetime import datetime, timezone, timedelta
import hashlib
import io
import json
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest.mock import patch
import zipfile
import source_authority as module

SOURCE = Path(__file__).resolve().parents[3]
SHA = 'a' * 40
TREE = 'b' * 40


class SourceTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(); self.addCleanup(self.temp.cleanup)
        self.destination = Path(self.temp.name) / 'source'; self.destination.mkdir(mode=0o700)
        self.files = {path: (SOURCE / path).read_bytes() for path in ('scripts/ci/staging-image-catalog.json',
            '.github/workflows/ci.yml', 'deploy/staging/services.yaml')}
        self.repo = {'full_name': 'Poryadok/VoiceRoot', 'id': 111, 'private': False, 'default_branch': 'master'}
        self.run = {'id': 123, 'workflow_id': 263230665, 'head_sha': SHA, 'head_branch': 'master',
            'repository': self.repo, 'head_repository': self.repo, 'status': 'completed', 'conclusion': 'success',
            'event': 'push', 'path': '.github/workflows/ci.yml', 'run_attempt': 1}
        now = datetime.now(timezone.utc)
        self.jobs = [{'id': i, 'run_id': 123, 'head_sha': SHA, 'run_attempt': 1, 'name': name, 'status': 'completed', 'conclusion': 'success',
            'started_at': (now-timedelta(seconds=30)).isoformat(), 'completed_at': now.isoformat()}
            for i, name in enumerate(('ci-gate','staging-stack-lock'), 1)]
        self.artifact = {'id': 99, 'name': 'staging-stack-lock', 'expired': False,
            'workflow_run': {'id': 123, 'head_sha': SHA, 'repository_id': 111, 'head_repository_id': 111},
            'created_at': (now-timedelta(seconds=10)).isoformat(), 'expires_at': (now+timedelta(days=1)).isoformat()}
        self.bad_tar = None; self.bad_lock = False; self.bad_digest = False; self.master_moved = False; self.master_calls = 0
        self.config=json.dumps({'architecture':'amd64','os':'linux','config':{'Labels':{}}}).encode()
        manifest = {'schemaVersion': 2, 'mediaType': 'application/vnd.oci.image.manifest.v1+json',
            'config': {'digest': 'sha256:'+hashlib.sha256(self.config).hexdigest(), 'size': len(self.config)}, 'layers': []}
        self.manifest = json.dumps(manifest).encode(); self.manifest_digest = 'sha256:' + hashlib.sha256(self.manifest).hexdigest()

    def tree(self):
        rows = []
        for path, content in self.files.items():
            sha = hashlib.sha1(b'blob '+str(len(content)).encode()+b'\0'+content).hexdigest()
            rows.append({'path': path,'type':'blob','mode':'100644','size':len(content),'sha':sha})
        return {'sha': TREE, 'truncated': False, 'tree': rows}

    def tar(self):
        buffer = io.BytesIO()
        with tarfile.open(fileobj=buffer,mode='w:gz') as archive:
            for path, content in self.files.items():
                member = tarfile.TarInfo('Poryadok-VoiceRoot-'+SHA[:7]+'/'+path)
                member.size=len(content); member.mode=0o644
                if self.bad_tar == 'escape': member.name='Poryadok-VoiceRoot-'+SHA[:7]+'/../escape'
                if self.bad_tar == 'link': member.type=tarfile.SYMTYPE; member.linkname='elsewhere'; member.size=0
                archive.addfile(member, io.BytesIO(content) if member.isfile() else None)
                if self.bad_tar == 'duplicate': archive.addfile(member,io.BytesIO(content))
        return buffer.getvalue()

    def lock(self):
        names=[image['name'] for image in json.loads(self.files['scripts/ci/staging-image-catalog.json'])['images']]+['nats-hub-config-renderer']
        return ('version: 1\nregistry: ghcr.io/poryadok/voiceroot\ntag: '+SHA+'\nimages:\n'+''.join('  '+n+': '+('d'*40 if self.bad_lock else SHA)+'\n' for n in names)).encode()

    def fetch(self, url, headers, limit, deadline, target=None):
        if url.endswith('/repos/Poryadok/VoiceRoot'): data=self.repo
        elif url.endswith('/git/ref/heads/master'):
            self.master_calls+=1
            data={'ref':'refs/heads/master','object':{'type':'commit','sha':'d'*40 if self.master_moved and self.master_calls>1 else SHA}}
        elif url.endswith('/actions/runs/123'): data=self.run
        elif '/jobs?' in url: data={'total_count':len(self.jobs),'jobs':self.jobs}
        elif url.endswith('/actions/runs/123/artifacts?per_page=100&page=1'): data={'total_count':1,'artifacts':[self.artifact]}
        elif url.endswith('/actions/artifacts/99'):
            data=copy.deepcopy(self.artifact)
            buffer=io.BytesIO()
            with zipfile.ZipFile(buffer,'w') as archive: archive.writestr('stack.lock.yaml',self.lock())
            data['size_in_bytes']=len(buffer.getvalue())
        elif '/git/commits/' in url: data={'sha':SHA,'tree':{'sha':TREE}}
        elif '/git/trees/' in url: data=self.tree()
        elif '/token?' in url: data={'token':'private-registry-token'}
        elif '/manifests/' in url:
            return self.manifest, {'Docker-Content-Digest': 'sha256:'+'e'*64 if self.bad_digest else self.manifest_digest}
        elif '/blobs/' in url: return self.config, {}
        elif url.endswith('/tarball/'+SHA):
            target.write(self.tar()); return len(self.tar()), {}
        elif url.endswith('/actions/artifacts/99/zip'):
            buffer=io.BytesIO()
            with zipfile.ZipFile(buffer,'w') as archive: archive.writestr('stack.lock.yaml',self.lock())
            raw=buffer.getvalue(); target.write(raw); return len(raw), {}
        else: raise AssertionError('unexpected fixed request')
        return json.dumps(data).encode(), {}

    def capture(self):
        self.destination = Path(self.temp.name) / ('source-' + str(len(list(Path(self.temp.name).iterdir()))))
        self.destination.mkdir(mode=0o700)
        with patch.object(module,'_fetch',self.fetch,create=True):
            return module.capture_source('private-token',123,SHA,self.destination)

    def test_valid_fixed_ci_actual_canonical_tree_and_images(self):
        receipt=self.capture()
        self.assertEqual(receipt['source_sha'],SHA)
        self.assertTrue(receipt['images'])
        self.assertEqual((self.destination/'deploy/staging/services.yaml').read_bytes(),self.files['deploy/staging/services.yaml'])
        self.assertNotIn('private-token',json.dumps(receipt))

    def failed_deploy_jobs(self):
        self.run['conclusion'] = 'failure'
        self.jobs.append(dict(self.jobs[0], id=3, name='deploy-staging / deploy', conclusion='failure'))

    def test_completed_failure_only_exact_deployment_can_capture(self):
        self.failed_deploy_jobs()
        self.jobs.append(dict(self.jobs[0], id=4, name='backend-go-integration-pr', conclusion='skipped'))
        self.assertTrue(self.capture()['verified'])

    def test_completed_failure_incomplete_job_pagination_is_rejected(self):
        self.failed_deploy_jobs()
        original = self.fetch
        def fetch(url, headers, limit, deadline, target=None):
            raw, metadata = original(url, headers, limit, deadline, target)
            if '/jobs?' in url:
                value = json.loads(raw); value['total_count'] += 1
                return json.dumps(value).encode(), metadata
            return raw, metadata
        with patch.object(module, '_fetch', fetch), self.assertRaises(module.SourceError):
            module.capture_source('private-token', 123, SHA, self.destination)

    def test_completed_failure_requires_complete_owned_terminal_jobs(self):
        self.failed_deploy_jobs()
        baseline = copy.deepcopy(self.jobs)
        changes = [(2, 'name', 'deploy-staging / deployment-spoof'),
                   (2, 'conclusion', 'cancelled'), (2, 'status', 'in_progress'),
                   (2, 'run_attempt', 2), (2, 'run_id', 124), (2, 'head_sha', 'f'*40),
                   (0, 'conclusion', 'failure')]
        for index, key, value in changes:
            self.jobs = copy.deepcopy(baseline); self.jobs[index][key] = value
            with self.subTest(key=key, value=value), self.assertRaises(module.SourceError): self.capture()
        for conclusion in ('failure', 'cancelled', 'timed_out', 'neutral', 'action_required', None):
            self.jobs = copy.deepcopy(baseline)
            self.jobs.append(dict(self.jobs[0], id=4, name='backend-auth', conclusion=conclusion))
            with self.subTest(other=conclusion), self.assertRaises(module.SourceError): self.capture()
        self.jobs = copy.deepcopy(baseline)
        self.jobs.append(dict(self.jobs[0], id=4, name='backend-auth', conclusion='skipped'))
        with self.assertRaises(module.SourceError): self.capture()
        self.jobs = copy.deepcopy(baseline); self.jobs.pop(0)
        with self.assertRaises(module.SourceError): self.capture()
        self.jobs = copy.deepcopy(baseline); self.jobs.append(dict(self.jobs[0], id=4))
        with self.assertRaises(module.SourceError): self.capture()
        self.jobs = baseline; self.run['conclusion'] = 'cancelled'
        with self.assertRaises(module.SourceError): self.capture()

    def test_wrong_ci_authority_job_states(self):
        for key,value in [('workflow_id',1),('head_sha','f'*40),('head_branch','feature'),('event','pull_request')]:
            old=copy.deepcopy(self.run); self.run[key]=value
            with self.subTest(key=key), self.assertRaises(module.SourceError): self.capture()
            self.run=old
        for status,conclusion in [('queued',None),('completed','skipped'),('completed','failure')]:
            self.jobs[0].update(status=status,conclusion=conclusion)
            with self.subTest(status=status,conclusion=conclusion), self.assertRaises(module.SourceError): self.capture()

    def test_wrong_repo_default_branch_and_head_race(self):
        self.repo['full_name']='other/repo'
        with self.assertRaises(module.SourceError): self.capture()
        self.repo['full_name']='Poryadok/VoiceRoot'; self.repo['default_branch']='feature'
        with self.assertRaises(module.SourceError): self.capture()
        self.repo['default_branch']='master'; self.master_moved=True
        with self.assertRaises(module.SourceError): self.capture()

    def test_bad_archive_members(self):
        for bad in ('escape','link','duplicate'):
            self.bad_tar=bad
            with self.subTest(bad=bad), self.assertRaises(module.SourceError): self.capture()

    def test_wrong_lock_and_registry_digest(self):
        self.bad_lock=True
        with self.assertRaises(module.SourceError): self.capture()

    def test_tar_reader_bounds_metadata_allocation(self):
        with self.assertRaises(module.SourceError):
            module._TarReader(io.BytesIO(b'x'), 9999999999).read(2 * 1024 ** 2)

    def test_oci_index_exact_platform_child_size(self):
        original = self.fetch
        child = self.manifest
        index = {'schemaVersion': 2, 'mediaType': 'application/vnd.oci.image.index.v1+json',
            'manifests': [{'digest': self.manifest_digest, 'size': len(child),
                'platform': {'os': 'linux', 'architecture': 'amd64'}}]}
        def fetch(url, headers, limit, deadline, target=None):
            if '/manifests/' in url and url.endswith('/'+SHA):
                raw=json.dumps(index).encode()
                return raw, {'Docker-Content-Digest':'sha256:'+hashlib.sha256(raw).hexdigest()}
            return original(url,headers,limit,deadline,target)
        with patch.object(module,'_fetch',fetch):
            self.assertTrue(module._resolve_image('auth',SHA,9999999999).endswith('@'+self.manifest_digest))
            index['manifests'][0]['size'] += 1
            with self.assertRaises(module.SourceError): module._resolve_image('auth',SHA,9999999999)
            index['manifests'][0]['size'] -= 1
            index['manifests'].append(copy.deepcopy(index['manifests'][0]))
            with self.assertRaises(module.SourceError): module._resolve_image('auth',SHA,9999999999)
            index['manifests']=index['manifests'][:1]; index['manifests'][0]['platform']['architecture']='arm64'
            with self.assertRaises(module.SourceError): module._resolve_image('auth',SHA,9999999999)

    def test_foreign_redirect_drops_authorization_and_rejects_url(self):
        class Response:
            status=302
            headers={'Location':'https://codeload.github.com/approved'}
        calls=[]
        def request(url,headers,limit,deadline):
            calls.append(dict(headers))
            response=Response()
            if len(calls)==2: response.headers={'Location':'https://evil.example/token'}
            return response
        with patch.object(module.transport,'_request',request), patch.object(module.transport,'_close'):
            with self.assertRaises(module.SourceError): module._fetch(module.API,{'Authorization':'Bearer private'},1,9999999999)
        self.assertEqual(calls,[{'Authorization':'Bearer private'},{}])

    def test_image_config_revision_platform_and_namespace(self):
        for config in ({'architecture':'arm64','os':'linux'},
                {'architecture':'amd64','os':'linux','config':{'Labels':{'org.opencontainers.image.revision':'d'*40}}},
                {'architecture':'amd64','os':'linux','config':{'Labels':{'org.opencontainers.image.source':'https://github.com/foreign/repo'}}}):
            self.config=json.dumps(config).encode()
            body=json.loads(self.manifest); body['config']={'digest':'sha256:'+hashlib.sha256(self.config).hexdigest(),'size':len(self.config)}
            self.manifest=json.dumps(body).encode(); self.manifest_digest='sha256:'+hashlib.sha256(self.manifest).hexdigest()
            with self.subTest(config=config), self.assertRaises(module.SourceError): self.capture()
        with self.assertRaises(module.SourceError): module._resolve_image('../foreign',SHA,9999999999)

    def test_artifact_wrong_authority_and_tree_truncation(self):
        for key,value in [('name','other'),('expired',True)]:
            original=copy.deepcopy(self.artifact); self.artifact[key]=value
            with self.subTest(key=key), self.assertRaises(module.SourceError): self.capture()
            self.artifact=original
        self.artifact['workflow_run']['head_sha']='d'*40
        with self.assertRaises(module.SourceError): self.capture()
        self.artifact['workflow_run']['head_sha']=SHA
        tree=self.tree(); tree['truncated']=True
        with patch.object(self,'tree',return_value=tree), self.assertRaises(module.SourceError): self.capture()

    def test_in_progress_run_with_completed_required_jobs_is_approved(self):
        self.run.update(status='in_progress',conclusion=None)
        self.assertTrue(self.capture()['verified'])

    def test_real_postmerge_known_failure_cannot_capture_while_parent_active(self):
        fixture = json.loads(Path(__file__).with_name('source_authority_postmerge_fixture.json').read_text())
        actual = fixture['observed_run']
        self.run = dict(actual, id=123, status='in_progress', conclusion=None)
        self.jobs = [dict(job, run_id=123) for job in fixture['jobs']]
        self.repo = actual['repository'] | {'private': False, 'default_branch': 'master'}
        self.artifact['workflow_run'].update(head_sha=actual['head_sha'],
            repository_id=self.repo['id'], head_repository_id=self.repo['id'])
        self.assertTrue(any(job['name'] == 'grafana-analytics-smoke' and job['conclusion'] == 'failure'
                            for job in self.jobs))
        self.assertTrue(all(any(job['name'] == name and job['conclusion'] == 'success' for job in self.jobs)
                            for name in ('ci-gate', 'staging-stack-lock')))
        lock_job = next(job for job in self.jobs if job['name'] == 'staging-stack-lock')
        self.artifact['created_at'] = lock_job['started_at']
        with patch(__name__ + '.SHA', actual['head_sha']):
            with patch.object(module, '_capture_tar', wraps=module._capture_tar) as capture_tar:
                with self.assertRaises(module.SourceError): self.capture()
                capture_tar.assert_not_called()
            # All fake source/artifact identities are coherent: changing only the
            # known failed job outcome must permit the complete passive capture.
            failed = next(job for job in self.jobs if job['name'] == 'grafana-analytics-smoke')
            failed['conclusion'] = 'success'
            self.assertTrue(self.capture()['verified'])

    def test_in_progress_known_nondeployment_failures_reject_capture(self):
        self.run.update(status='in_progress', conclusion=None)
        baseline = copy.deepcopy(self.jobs)
        for name in ('grafana-analytics-smoke', 'backend-auth'):
            for conclusion in ('failure', 'cancelled', 'timed_out', 'neutral', 'action_required'):
                self.jobs = copy.deepcopy(baseline)
                self.jobs.append(dict(self.jobs[0], id=3, name=name, conclusion=conclusion))
                self.jobs.append(dict(self.jobs[0], id=4, name='compose-e2e', status='in_progress', conclusion=None))
                with self.subTest(name=name, conclusion=conclusion), self.assertRaises(module.SourceError):
                    self.capture()

    def test_in_progress_complete_job_inventory_ownership_is_required(self):
        self.run.update(status='in_progress', conclusion=None)
        baseline = copy.deepcopy(self.jobs)
        for key, value in (('run_id', 124), ('run_attempt', 2), ('head_sha', 'f'*40)):
            self.jobs = copy.deepcopy(baseline)
            self.jobs.append(dict(self.jobs[0], id=3, name='compose-e2e', status='in_progress', conclusion=None))
            self.jobs[-1][key] = value
            with self.subTest(key=key), self.assertRaises(module.SourceError): self.capture()
        self.jobs = copy.deepcopy(baseline)
        self.jobs.append(dict(self.jobs[0], id=3))
        with self.assertRaises(module.SourceError): self.capture()

    def test_in_progress_exact_deployment_and_pending_jobs_remain_approved(self):
        self.run.update(status='in_progress', conclusion=None)
        self.jobs.append(dict(self.jobs[0], id=3, name='deploy-staging / deploy', conclusion='failure'))
        self.jobs.append(dict(self.jobs[0], id=4, name='compose-e2e', status='in_progress', conclusion=None))
        self.assertTrue(self.capture()['verified'])

    def test_archive_metadata_bomb_and_source_bytes_tamper(self):
        import gzip
        member=tarfile.TarInfo('oversized-pax'); member.type=tarfile.XHDTYPE; member.size=2*1024**2
        archive=io.BytesIO(gzip.compress(member.tobuf()+b'\0'*1024))
        with self.assertRaises(module.SourceError):
            module._capture_tar(archive,self.destination,self.tree(),SHA,9999999999)
        original=self.tree()
        self.files['deploy/staging/services.yaml']+=b'\nforged'
        with patch.object(self,'tree',return_value=original), self.assertRaises(module.SourceError): self.capture()
        self.bad_lock=False; self.bad_digest=True
        with self.assertRaises(module.SourceError): self.capture()


if __name__=='__main__': unittest.main()
