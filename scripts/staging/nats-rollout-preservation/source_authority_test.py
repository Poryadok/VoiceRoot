import copy
from datetime import datetime, timezone, timedelta
import hashlib
import io
import json
import os
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
    def test_equivalent_master_requires_ancestor_and_identical_bounded_build_inputs(self):
        source, current = 'a' * 40, 'b' * 40
        tree_root = '1' * 40
        entries = {
            'root': {'src': ('tree', '2' * 40), '.github': ('tree', '3' * 40), 'scripts': ('tree', '4' * 40)},
            'src': {'backend': ('tree', '5' * 40)},
            '.github': {'workflows': ('tree', '6' * 40)},
            'workflows': {'ci.yml': ('blob', '7' * 40)},
            'scripts': {'ci': ('tree', '8' * 40)},
            'ci': {'backend-docker-context.txt': ('blob', '9' * 40),
                'staging-image-catalog.json': ('blob', 'c' * 40)},
        }

        active_commit = source
        drift = None

        def api(url, _headers, _deadline):
            nonlocal active_commit
            if '/compare/' in url:
                # GitHub's compare response has base_commit and merge_base_commit,
                # but no head_commit; the requested head is separately pinned by ref.
                return {'base_commit': {'sha': source},
                    'merge_base_commit': {'sha': source}, 'status': 'ahead', 'ahead_by': 3}
            if '/git/commits/' in url:
                sha = url.rsplit('/', 1)[-1]
                active_commit = sha
                return {'sha': sha, 'tree': {'sha': tree_root}}
            tree_sha = url.rsplit('/', 1)[-1]
            reverse = {'2' * 40: 'src', '3' * 40: '.github', '4' * 40: 'scripts',
                '6' * 40: 'workflows', '8' * 40: 'ci'}
            if tree_sha == tree_root:
                values = entries['root']
            else:
                values = entries[reverse[tree_sha]]
            if active_commit == current:
                if drift == 'src/backend' and tree_sha == '2' * 40:
                    values = {'backend': ('tree', 'f' * 40)}
                elif drift == '.github/workflows/ci.yml' and tree_sha == '6' * 40:
                    values = {'ci.yml': ('blob', 'f' * 40)}
                elif drift == 'scripts/ci/backend-docker-context.txt' and tree_sha == '8' * 40:
                    values = {'backend-docker-context.txt': ('blob', 'f' * 40),
                        'staging-image-catalog.json': ('blob', 'c' * 40)}
                elif drift == 'scripts/ci/staging-image-catalog.json' and tree_sha == '8' * 40:
                    values = {'backend-docker-context.txt': ('blob', '9' * 40),
                        'staging-image-catalog.json': ('blob', 'f' * 40)}
            return {'sha': tree_sha, 'truncated': False,
                'tree': [{'path': name, 'type': kind, 'sha': digest}
                    for name, (kind, digest) in values.items()]}

        with patch.object(module, '_get', side_effect=api):
            result = module._equivalent_master_source({}, source, current, 1)
        self.assertEqual(result['current_master_sha_before'], current)
        self.assertTrue(result['source_is_ancestor'])
        self.assertEqual(result['equivalent_inputs']['src/backend']['git_object_sha1'], '5' * 40)

        for drift_path in ('src/backend', '.github/workflows/ci.yml',
            'scripts/ci/backend-docker-context.txt', 'scripts/ci/staging-image-catalog.json'):
            with self.subTest(drift=drift_path):
                drift = drift_path
                with patch.object(module, '_get', side_effect=api), self.assertRaises(module.SourceError):
                    module._equivalent_master_source({}, source, current, 1)
        drift = None

    def test_equivalent_master_rejects_diverged_or_descendant_sources(self):
        source, current = 'a' * 40, 'b' * 40
        for status, merge_base in (('diverged', 'c' * 40), ('behind', source), ('ahead', 'c' * 40)):
            with self.subTest(status=status, merge_base=merge_base):
                compare = {'base_commit': {'sha': source},
                    'merge_base_commit': {'sha': merge_base}, 'status': status, 'ahead_by': 2}
                with patch.object(module, '_get', return_value=compare), self.assertRaises(module.SourceError):
                    module._equivalent_master_source({}, source, current, 1)

    def test_closed_image_authority_matches_catalog_including_gameintegration(self):
        catalog=json.loads((SOURCE/'scripts/ci/staging-image-catalog.json').read_text())
        names={row['name'] for row in catalog['images']}
        self.assertIn('gameintegration',names)
        self.assertEqual(names|{'nats-hub-config-renderer'},module.IMAGE_NAMES)
        with self.assertRaises(module.SourceError):
            module._resolve_image('not-a-catalog-image',SHA,1)

    def test_api_pagination_keeps_full_item_limit_with_small_pages(self):
        requests = []

        def page(url, headers, deadline):
            requests.append(url)
            self.assertIn('?per_page=20&page=', url)
            number = int(url.rsplit('page=', 1)[1])
            first = (number - 1) * 20
            return {'total_count': 10000, 'jobs': [{'id': item} for item in range(first + 1, first + 21)]}

        with patch.object(module, '_get', side_effect=page):
            rows = module._pages('https://api.github.com/jobs', 'jobs', {}, 1)

        self.assertEqual(len(rows), 10000)
        self.assertEqual(len(requests), 500)
        self.assertEqual(rows[0]['id'], 1)
        self.assertEqual(rows[-1]['id'], 10000)

    def test_api_pagination_rejects_over_limit_and_inconsistent_pages(self):
        with patch.object(module, '_get', return_value={'total_count': 10001, 'jobs': []}):
            with self.assertRaises(module.SourceError):
                module._pages('https://api.github.com/jobs', 'jobs', {}, 1)

        responses = [
            {'total_count': 2, 'jobs': [{'id': 1}]},
            {'total_count': 3, 'jobs': [{'id': 2}]},
        ]
        with patch.object(module, '_get', side_effect=responses):
            with self.assertRaises(module.SourceError):
                module._pages('https://api.github.com/jobs', 'jobs', {}, 1)

        responses = [
            {'total_count': 2, 'jobs': [{'id': 1}]},
            {'total_count': 2, 'jobs': [{'id': 1}]},
        ]
        with patch.object(module, '_get', side_effect=responses):
            with self.assertRaises(module.SourceError):
                module._pages('https://api.github.com/jobs', 'jobs', {}, 1)

    def test_cached_archive_runs_complete_capture_without_remote_tarball(self):
        private=tempfile.TemporaryDirectory(dir='/home/pmd');self.addCleanup(private.cleanup)
        cache=Path(private.name)/'source-archives';cache.mkdir(mode=0o750)
        # Shared cache uses the exact full-SHA codeload root.
        buffer=io.BytesIO()
        with tarfile.open(fileobj=buffer,mode='w:gz') as archive:
            for path,content in self.files.items():
                member=tarfile.TarInfo('VoiceRoot-'+SHA+'/'+path)
                member.size=len(content);member.mode=0o664
                archive.addfile(member,io.BytesIO(content))
        cached=cache/(SHA+'.tar.gz');cached.write_bytes(buffer.getvalue());cached.chmod(0o440)
        calls=[]
        def fetch(url,*args,**kwargs):
            calls.append(url)
            if url.endswith('/tarball/'+SHA):raise AssertionError('cache must avoid archive network')
            return self.fetch(url,*args,**kwargs)
        with patch.object(module,'SOURCE_CACHE_DIR',cache,create=True),patch.object(module,'_fetch',fetch):
            result=module.capture_source('private-token',123,SHA,self.destination)
        self.assertTrue(result['verified']);self.assertEqual(result['tree_sha'],TREE)
        self.assertTrue(any('/git/trees/' in url for url in calls))
        self.assertTrue(any('/actions/artifacts/99/zip' in url for url in calls))
        self.assertTrue(any('/manifests/' in url for url in calls))

    def test_archive_executable_mode_mismatch_is_rejected(self):
        tree=self.tree();tree['tree'][0]['mode']='100755'
        with patch.object(self,'tree',return_value=tree),self.assertRaises(module.SourceError):
            self.capture()

    def test_cache_custody_rejects_invalid_present_and_changed_held_bytes(self):
        with tempfile.TemporaryDirectory(dir='/home/pmd') as td:
            root=Path(td);root.chmod(0o750)
            cache=root/'cache';cache.mkdir(mode=0o750)
            path=cache/(SHA+'.tar.gz')
            with patch.object(module,'SOURCE_CACHE_DIR',cache):
                for drift in ('writable','hardlink','symlink','directory'):
                    with self.subTest(drift=drift):
                        path.write_bytes(b'archive');path.chmod(0o440)
                        if drift=='writable':path.chmod(0o660)
                        elif drift=='hardlink':os.link(path,root/'linked')
                        elif drift=='symlink':path.unlink();path.symlink_to(root/'missing')
                        else:path.unlink();path.mkdir()
                        with self.assertRaises((module.SourceError,OSError)):
                            with module.cached_archive(SHA):pass
                        if path.is_dir():path.rmdir()
                        else:path.unlink()
                        if (root/'linked').exists():(root/'linked').unlink()
                path.write_bytes(b'archive');path.chmod(0o440)
                with self.assertRaises(module.SourceError):
                    with module.cached_archive(SHA) as held:
                        self.assertEqual(held.read(),b'archive')
                        path.chmod(0o600)
                        path.write_bytes(b'changed')
                path.unlink();cache.rmdir();cache.symlink_to(root)
                with self.assertRaises(OSError):
                    with module.cached_archive(SHA):pass

    def test_immutable_runtime_index_and_child_require_same_content_chain(self):
        prefix=module.REGISTRY+'/auth@sha256:'
        actual=prefix+'a'*64; target=prefix+'b'*64
        rows={actual:{'requested_image':actual,'manifest_image':target,'config_sha256':'c'*64},
              target:{'requested_image':target,'manifest_image':target,'config_sha256':'c'*64}}
        with patch.object(module,'immutable_image_identity',side_effect=lambda image,deadline:rows[image],create=True):
            self.assertTrue(module.same_image_content(actual,target,1))
            rows[actual]['config_sha256']='d'*64
            self.assertFalse(module.same_image_content(actual,target,1))
            rows[actual]['config_sha256']='c'*64;rows[actual]['manifest_image']=prefix+'e'*64
            self.assertFalse(module.same_image_content(actual,target,1))
            with self.assertRaises(module.SourceError):module.same_image_content(actual,module.REGISTRY+'/user@sha256:'+'b'*64,1)
            for bad in ('sha256:'+'a'*64,prefix+'not-a-hash','evil.example/auth@sha256:'+'a'*64):
                with self.assertRaises(module.SourceError):module.same_image_content(bad,target,1)

    def test_immutable_identity_validates_actual_index_child_and_config_bytes(self):
        index={'schemaVersion':2,'mediaType':'application/vnd.oci.image.index.v1+json','manifests':[
            {'digest':self.manifest_digest,'size':len(self.manifest),'platform':{'os':'linux','architecture':'amd64'}}]}
        def lookup(index_value,*,bad_config=False):
            raw=json.dumps(index_value).encode();root='sha256:'+hashlib.sha256(raw).hexdigest()
            def fetch(url,headers,limit,deadline,target=None):
                if url.endswith('/manifests/'+root):return raw,{'Docker-Content-Digest':root}
                if bad_config and '/blobs/' in url:return self.config+b' ',{}
                return self.fetch(url,headers,limit,deadline,target)
            with patch.object(module,'_fetch',fetch):
                return module.immutable_image_identity(module.REGISTRY+'/auth@'+root,1)
        row=lookup(index)
        self.assertEqual(row['manifest_image'],module.REGISTRY+'/auth@'+self.manifest_digest)
        self.assertEqual(row['config_sha256'],hashlib.sha256(self.config).hexdigest())
        bad=copy.deepcopy(index);bad['manifests'][0]['size']+=1
        with self.assertRaises(module.SourceError):lookup(bad)
        bad=copy.deepcopy(index);bad['manifests'].append(copy.deepcopy(bad['manifests'][0]))
        with self.assertRaises(module.SourceError):lookup(bad)
        with self.assertRaises(module.SourceError):lookup(index,bad_config=True)

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
        elif '/jobs?' in url:
            query=dict(part.split('=',1) for part in url.split('?',1)[1].split('&'))
            page=int(query['page']);per_page=int(query['per_page']);start=(page-1)*per_page
            data={'total_count':len(self.jobs),'jobs':self.jobs[start:start+per_page]}
        elif url.endswith('/actions/runs/123/artifacts?per_page=20&page=1'): data={'total_count':1,'artifacts':[self.artifact]}
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
