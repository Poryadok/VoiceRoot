"""Passive root source capture for the fixed Voice repository and CI workflow."""
import hashlib
import gzip
import io
import json
import os
from pathlib import Path, PurePosixPath
import re
import shutil
import stat
import tarfile
import tempfile
import time
from urllib.parse import urlsplit
import zipfile
import github_custody as transport

REPO = 'Poryadok/VoiceRoot'
API = 'https://api.github.com/repos/' + REPO
REGISTRY = 'ghcr.io/poryadok/voiceroot'
WORKFLOW = 263230665
MAX_SOURCE = 4 * 1024 ** 3
MAX_ARCHIVE = 2 * 1024 ** 3
MAX_FILE = 512 * 1024 ** 2
MAX_ITEMS = 100000
IMAGE_NAMES = frozenset(('analytics', 'bot', 'chat', 'file', 'gateway', 'matchmaking', 'messaging',
    'moderation', 'notification', 'realtime', 'role', 'search', 'social', 'space', 'story', 'subscription',
    'user', 'voice', 'auth', 'web', 'developer-portal', 'admin', 'nats-hub-config-renderer'))

class SourceError(ValueError):
    pass


def _fail():
    raise SourceError('source_authority_rejected')


def _json(raw):
    def unique(pairs):
        result = {}
        for key, value in pairs:
            if key in result: _fail()
            result[key] = value
        return result
    return json.loads(raw, object_pairs_hook=unique, parse_constant=lambda value: _fail())


def _safe_url(url):
    parsed = urlsplit(url)
    allowed = parsed.hostname in ('api.github.com', 'codeload.github.com', 'ghcr.io', 'pkg-containers.githubusercontent.com')
    if not allowed:
        try: transport._url(url)
        except Exception: _fail()
    if (parsed.scheme != 'https' or parsed.port not in (None, 443) or parsed.username or parsed.password or parsed.fragment): _fail()


def _fetch(url, headers, limit, deadline, target=None):
    for attempt in range(4):
        _safe_url(url)
        response = transport._request(url, headers if attempt == 0 else {}, limit, deadline)
        try:
            if response.status == 200:
                destination = target if target is not None else io.BytesIO()
                count = transport._copy(response, destination, limit, deadline)
                return (count if target is not None else destination.getvalue()), dict(response.headers)
            if response.status not in (301, 302, 303, 307, 308) or attempt == 3: _fail()
            url = response.headers.get('Location', '')
            _safe_url(url)
        finally:
            transport._close(response)
    _fail()


def _headers(token):
    if not isinstance(token, str) or not token or '\r' in token or '\n' in token: _fail()
    return {'Authorization': 'Bearer ' + token, 'Accept': 'application/vnd.github+json',
        'X-GitHub-Api-Version': '2022-11-28', 'User-Agent': 'Voice-root-source-authority'}


def _get(url, headers, deadline):
    return _json(_fetch(url, headers, 8 * 1024 ** 2, deadline)[0])


def _head(headers, sha, deadline):
    repo = _get(API, headers, deadline)
    if repo['full_name'] != REPO or repo['private'] is not False or repo['default_branch'] != 'master': _fail()
    head = _get(API + '/git/ref/heads/master', headers, deadline)
    if head['ref'] != 'refs/heads/master' or head['object']['type'] != 'commit' or head['object']['sha'] != sha: _fail()
    return repo


def _pages(url, field, headers, deadline):
    result, total, identifiers = [], None, set()
    for page in range(1, 101):
        body = _get(url + '?per_page=100&page=' + str(page), headers, deadline)
        observed = body['total_count']
        if type(observed) is not int or not 0 <= observed <= 10000 or total is not None and observed != total: _fail()
        total = observed
        rows = body[field]
        if not isinstance(rows, list) or len(rows) > 100: _fail()
        for row in rows:
            if type(row['id']) is not int or row['id'] in identifiers: _fail()
            identifiers.add(row['id']); result.append(row)
        if len(result) == total: return result
        if not rows or len(result) > total: _fail()
    _fail()


def _ci(headers, run_id, sha, repo, deadline):
    run = _get(API + '/actions/runs/' + str(run_id), headers, deadline)
    if (run['id'] != run_id or run['workflow_id'] != WORKFLOW or run['head_sha'] != sha or run['head_branch'] != 'master'
            or run['repository']['full_name'] != REPO or run['head_repository']['full_name'] != REPO
            or run['repository']['id'] != repo['id'] or run['event'] != 'push'
            or run['path'].split('@', 1)[0] != '.github/workflows/ci.yml'
            or run['status'] not in ('in_progress', 'completed')
            or run['status'] == 'completed' and run['conclusion'] not in ('success', 'failure')): _fail()
    attempt = run['run_attempt']
    if type(attempt) is not int or attempt <= 0: _fail()
    jobs = _pages(API + '/actions/runs/' + str(run_id) + '/attempts/' + str(attempt) + '/jobs', 'jobs', headers, deadline)
    required = {}
    for name in ('ci-gate', 'staging-stack-lock'):
        matches = [job for job in jobs if job['name'] == name]
        if len(matches) != 1: _fail()
        job = matches[0]
        if (job['run_id'] != run_id or job['head_sha'] != sha or job.get('run_attempt') != attempt
                or job['status'] != 'completed' or job['conclusion'] != 'success'): _fail()
        required[name] = job
    # An in-progress parent can already have a failed optional job after ci-gate.
    # Bind the whole observed attempt, and never approve a known unrelated failure.
    names = set()
    for job in jobs:
        name = job['name']
        if (job['run_id'] != run_id or job['head_sha'] != sha or job.get('run_attempt') != attempt
                or not isinstance(name, str) or not name or name in names
                or job['status'] not in ('queued', 'in_progress', 'completed')): _fail()
        names.add(name)
        if job['status'] != 'completed':
            if job['conclusion'] is not None: _fail()
        elif job['conclusion'] not in ('success', 'skipped'):
            if not (name == 'deploy-staging / deploy' and job['conclusion'] == 'failure'
                    and (run['status'] == 'in_progress' or run['conclusion'] == 'failure')): _fail()
    if run['status'] == 'completed' and run['conclusion'] == 'failure':
        # Exact caller/callee IDs in ci.yml and staging-deploy.yml; no prefix aliases.
        deployment = 'deploy-staging / deploy'
        skipped_branches = {'backend-go-integration', 'local-ci-parity', 'backend-go-integration-pr',
            'grafana-analytics-smoke', 'analytics-clickhouse-integration', 'ci-skip-gate',
            'staging-images-promote'}
        failed = []
        names = set()
        for job in jobs:
            if (job['run_id'] != run_id or job['head_sha'] != sha or job.get('run_attempt') != attempt
                    or job['status'] != 'completed' or job['name'] in names): _fail()
            names.add(job['name'])
            if job['conclusion'] == 'failure' and job['name'] == deployment:
                failed.append(job)
            elif job['conclusion'] == 'success':
                continue
            elif job['conclusion'] == 'skipped' and job['name'] in skipped_branches:
                continue
            else: _fail()
        if len(failed) != 1: _fail()
    return run, required


def _path(value):
    if not isinstance(value, str) or '\\' in value or '\x00' in value or ':' in value: _fail()
    path = PurePosixPath(value)
    if path.is_absolute() or not value or any(part in ('', '.', '..') for part in value.split('/')): _fail()
    return path


def _destination(path):
    path = Path(path)
    observed = path.lstat()
    if (not stat.S_ISDIR(observed.st_mode) or path.is_symlink() or any(path.iterdir())
            or os.name == 'posix' and (observed.st_uid != os.geteuid() or observed.st_mode & 0o077)): _fail()
    return path


class _TarReader:
    """Bound tar metadata reads before tarfile allocates an untrusted PAX body."""
    def __init__(self, stream, deadline):
        self.stream, self.deadline, self.expanded = stream, deadline, 0

    def read(self, size):
        if type(size) is not int or not 0 <= size <= transport.CHUNK or time.monotonic() > self.deadline: _fail()
        data = self.stream.read(size); self.expanded += len(data)
        if self.expanded > MAX_SOURCE + MAX_ITEMS * 4096: _fail()
        return data

    def tell(self):
        return self.stream.tell()

    def seek(self, offset, whence=0):
        position = self.tell()
        target = offset if whence == 0 else position + offset if whence == 1 else -1
        if not position <= target <= position + 512: _fail()
        self.read(target - position)
        return self.tell()


def _capture_tar(archive, destination, tree, sha, deadline):
    expected, total, tree_paths = {}, 0, set()
    if tree['truncated'] is not False or len(tree['tree']) > MAX_ITEMS: _fail()
    for row in tree['tree']:
        _path(row['path'])
        if row['path'] in tree_paths: _fail()
        tree_paths.add(row['path'])
        if row['type'] == 'tree' and row['mode'] == '040000': continue
        if row['type'] != 'blob' or row['mode'] not in ('100644', '100755') or row['path'] in expected: _fail()
        if type(row['size']) is not int or not 0 <= row['size'] <= MAX_FILE or not re.fullmatch(r'[a-f0-9]{40}', row['sha']): _fail()
        expected[row['path']] = row; total += row['size']
    if not expected or total > MAX_SOURCE or shutil.disk_usage(destination).free < total + transport.CHUNK: _fail()
    prefix, seen, hashes, items = 'Poryadok-VoiceRoot-' + sha[:7], set(), {}, 0
    archive_names = set()
    archive.seek(0)
    with gzip.GzipFile(fileobj=archive) as expanded, tarfile.open(fileobj=_TarReader(expanded, deadline), mode='r:') as tar:
        for member in tar:
            items += 1
            if items > MAX_ITEMS or time.monotonic() > deadline: _fail()
            path = _path(member.name.rstrip('/'))
            if str(path) in archive_names: _fail()
            archive_names.add(str(path))
            if path.parts[0] != prefix: _fail()
            relative = '/'.join(path.parts[1:])
            if member.isdir():
                if relative: (destination / relative).mkdir(parents=True, exist_ok=True, mode=0o700)
                continue
            if not member.isfile() or member.issparse() or not relative or relative in seen or relative not in expected: _fail()
            row = expected[relative]
            if member.size != row['size']: _fail()
            seen.add(relative)
            target = destination / relative
            target.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
            flags = os.O_WRONLY | os.O_CREAT | os.O_EXCL | getattr(os, 'O_NOFOLLOW', 0) | getattr(os, 'O_BINARY', 0)
            blob = hashlib.sha1(b'blob ' + str(member.size).encode() + b'\0'); digest = hashlib.sha256(); count = 0
            with tar.extractfile(member) as content, os.fdopen(os.open(target, flags, 0o600), 'wb') as output:
                while chunk := content.read(transport.CHUNK):
                    if time.monotonic() > deadline: _fail()
                    count += len(chunk)
                    if count > member.size: _fail()
                    blob.update(chunk); digest.update(chunk); output.write(chunk)
            if count != member.size or blob.hexdigest() != row['sha']: _fail()
            hashes[relative] = digest.hexdigest()
    if seen != set(expected): _fail()
    return hashes


def _lock(raw, sha, destination):
    lines = raw.decode('utf-8').splitlines()
    if lines[:4] != ['version: 1', 'registry: ' + REGISTRY, 'tag: ' + sha, 'images:']: _fail()
    images = {}
    for line in lines[4:]:
        match = re.fullmatch(r'  ([a-z][a-z0-9-]{0,63}): ([a-f0-9]{40})', line)
        if not match or match[1] in images or match[2] != sha: _fail()
        images[match[1]] = match[2]
    catalog = _json((destination / 'scripts/ci/staging-image-catalog.json').read_bytes())
    names = [row['name'] for row in catalog['images']]
    if catalog['version'] != 1 or len(set(names)) != len(names) or set(names) | {'nats-hub-config-renderer'} != IMAGE_NAMES or set(images) != IMAGE_NAMES: _fail()
    return images


def _manifest(name, reference, headers, deadline):
    raw, returned = _fetch('https://ghcr.io/v2/poryadok/voiceroot/' + name + '/manifests/' + reference, headers, 4 * 1024 ** 2, deadline)
    digest = 'sha256:' + hashlib.sha256(raw).hexdigest()
    declared = next((value for key, value in returned.items() if key.lower() == 'docker-content-digest'), None)
    if declared != digest or reference.startswith('sha256:') and reference != digest: _fail()
    body = _json(raw)
    if body['schemaVersion'] != 2 or body.get('mediaType') not in (
            'application/vnd.oci.image.index.v1+json', 'application/vnd.oci.image.manifest.v1+json',
            'application/vnd.docker.distribution.manifest.list.v2+json', 'application/vnd.docker.distribution.manifest.v2+json'): _fail()
    return digest, body, len(raw)


def _resolve_image(name, sha, deadline):
    if name not in IMAGE_NAMES or not re.fullmatch(r'[a-f0-9]{40}', sha): _fail()
    token = _get('https://ghcr.io/token?service=ghcr.io&scope=repository:poryadok/voiceroot/' + name + ':pull', {}, deadline)['token']
    if not isinstance(token, str) or not token or '\r' in token or '\n' in token: _fail()
    accept = ', '.join(('application/vnd.oci.image.index.v1+json', 'application/vnd.oci.image.manifest.v1+json',
        'application/vnd.docker.distribution.manifest.list.v2+json', 'application/vnd.docker.distribution.manifest.v2+json'))
    headers = {'Authorization': 'Bearer ' + token, 'Accept': accept}
    digest, manifest, _ = _manifest(name, sha, headers, deadline)
    if 'manifests' in manifest:
        matches = [row for row in manifest['manifests'] if row.get('platform', {}).get('os') == 'linux'
            and row.get('platform', {}).get('architecture') == 'amd64' and not row.get('platform', {}).get('variant')]
        if len(matches) != 1 or not re.fullmatch(r'sha256:[a-f0-9]{64}', matches[0]['digest']): _fail()
        digest, manifest, length = _manifest(name, matches[0]['digest'], headers, deadline)
        if 'manifests' in manifest or type(matches[0].get('size')) is not int or matches[0]['size'] != length: _fail()
    descriptor = manifest['config']
    if not re.fullmatch(r'sha256:[a-f0-9]{64}', descriptor['digest']) or type(descriptor['size']) is not int or not 0 < descriptor['size'] <= 4 * 1024 ** 2: _fail()
    raw, _ = _fetch('https://ghcr.io/v2/poryadok/voiceroot/' + name + '/blobs/' + descriptor['digest'], headers, 4 * 1024 ** 2, deadline)
    if len(raw) != descriptor['size'] or 'sha256:' + hashlib.sha256(raw).hexdigest() != descriptor['digest']: _fail()
    config = _json(raw)
    if config['os'] != 'linux' or config['architecture'] != 'amd64': _fail()
    labels = config.get('config', {}).get('Labels') or {}
    if ('org.opencontainers.image.revision' in labels and labels['org.opencontainers.image.revision'] != sha
            or 'org.opencontainers.image.source' in labels and labels['org.opencontainers.image.source'].removesuffix('.git') != 'https://github.com/' + REPO): _fail()
    return REGISTRY + '/' + name + '@' + digest


def capture_source(token, run_id, source_sha, destination):
    """Capture current master passive bytes; resolve authorized same-CI tags.

    Digests are independently resolved at capture, not recorded by the CI tag
    artifact. Trust assumes authorized project GHCR package publishers. No
    caller image map, metadata URL or downloaded source code is executed.
    """
    try:
        if type(run_id) is not int or run_id <= 0 or not isinstance(source_sha, str) or not re.fullmatch(r'[a-f0-9]{40}', source_sha): _fail()
        destination = _destination(destination); headers = _headers(token); deadline = time.monotonic() + 600
        repo = _head(headers, source_sha, deadline)
        run, jobs = _ci(headers, run_id, source_sha, repo, deadline)
        commit = _get(API + '/git/commits/' + source_sha, headers, deadline)
        if commit['sha'] != source_sha or not re.fullmatch(r'[a-f0-9]{40}', commit['tree']['sha']): _fail()
        tree = _get(API + '/git/trees/' + commit['tree']['sha'] + '?recursive=1', headers, deadline)
        if tree['sha'] != commit['tree']['sha']: _fail()
        if shutil.disk_usage(tempfile.gettempdir()).free < MAX_ARCHIVE: _fail()
        with tempfile.TemporaryFile() as archive:
            _fetch(API + '/tarball/' + source_sha, headers, MAX_ARCHIVE, deadline, target=archive)
            files = _capture_tar(archive, destination, tree, source_sha, deadline)
        artifacts = _pages(API + '/actions/runs/' + str(run_id) + '/artifacts', 'artifacts', headers, deadline)
        candidates = [artifact for artifact in artifacts if artifact['name'] == 'staging-stack-lock']
        if len(candidates) != 1 or type(candidates[0]['id']) is not int: _fail()
        artifact_id = candidates[0]['id']
        artifact = _get(API + '/actions/artifacts/' + str(artifact_id), headers, deadline)
        workflow = artifact['workflow_run']; job = jobs['staging-stack-lock']
        if (artifact['id'] != artifact_id or artifact['name'] != 'staging-stack-lock' or artifact['expired'] is not False
                or workflow['id'] != run_id or workflow['head_sha'] != source_sha or workflow['repository_id'] != repo['id']
                or workflow['head_repository_id'] != repo['id']
                or not transport._timestamp(job['started_at']) <= transport._timestamp(artifact['created_at']) <= transport._timestamp(job['completed_at'])
                or transport._timestamp(artifact['expires_at']) <= transport.datetime.now(transport.timezone.utc)): _fail()
        with tempfile.TemporaryFile() as archive:
            count, _ = _fetch(API + '/actions/artifacts/' + str(artifact_id) + '/zip', headers, 4 * 1024 ** 2, deadline, target=archive)
            if type(artifact.get('size_in_bytes')) is not int or count != artifact['size_in_bytes']: _fail()
            archive.seek(0)
            with zipfile.ZipFile(archive) as zipped:
                members = zipped.infolist()
                if len(members) != 1 or members[0].filename != 'stack.lock.yaml' or members[0].is_dir() or members[0].file_size > 1024 * 1024: _fail()
                if stat.S_IFMT(members[0].external_attr >> 16) not in (0, stat.S_IFREG) or members[0].flag_bits & 1: _fail()
                lock = zipped.read(members[0])
        tags = _lock(lock, source_sha, destination)
        images = {name: _resolve_image(name, tag, deadline) for name, tag in sorted(tags.items())}
        _head(headers, source_sha, deadline)
        return {'schema': 'voice-source-authority-v1', 'repository': REPO, 'source_sha': source_sha,
            'workflow_id': WORKFLOW, 'run_id': run_id, 'run_attempt': run['run_attempt'], 'tree_sha': tree['sha'],
            'source_files': dict(sorted(files.items())), 'images': images, 'lock_artifact_id': artifact_id,
            'lock_sha256': hashlib.sha256(lock).hexdigest(), 'image_provenance': 'independently resolved trusted CI tag',
            'verified': True}
    except Exception:
        raise SourceError('source_authority_rejected') from None
