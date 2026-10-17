"""Passive root source capture for the fixed Voice repository and CI workflow."""
import hashlib
from contextlib import contextmanager
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
MAX_API_PAGE_SIZE = 20
MAX_API_ITEMS = 10000
MAX_API_PAGES = (MAX_API_ITEMS + MAX_API_PAGE_SIZE - 1) // MAX_API_PAGE_SIZE
SOURCE_CACHE_DIR = Path('/var/lib/voice-nats-preservation/installed/source-archives')
IMAGE_NAMES = frozenset(('analytics', 'bot', 'chat', 'file', 'gateway', 'matchmaking', 'messaging',
    'moderation', 'notification', 'realtime', 'role', 'search', 'social', 'space', 'story', 'subscription',
    'user', 'voice', 'auth', 'web', 'developer-portal', 'admin', 'gameintegration', 'nats-hub-config-renderer'))
_DIAGNOSTIC_PHASES = frozenset((
    'source-preflight', 'repository-pin', 'source-equivalence', 'source-ci', 'source-tree',
    'source-archive', 'stack-lock-artifact', 'stack-lock-read', 'stack-lock-parse',
    'master-recheck', 'master-ref', 'source-capture',
    'registry-token', 'registry-index', 'registry-platform-manifest', 'registry-config', 'registry-image',
    *('registry-image-' + name for name in IMAGE_NAMES),
))

class SourceError(ValueError):
    def __init__(self, message='source_authority_rejected', *, phase=None, code='validation_rejected', failure_class='SourceError', http_status=None):
        super().__init__(message)
        self.phase = phase if isinstance(phase, str) and phase in _DIAGNOSTIC_PHASES else 'unknown'
        self.code = code if code in ('validation_rejected', 'transport_rejected', 'response_schema_invalid', 'archive_rejected', 'http_error', 'http_status', 'network_error') else 'internal_error'
        self.failure_class = failure_class if failure_class in ('SourceError', 'CustodyError', 'KeyError', 'TypeError', 'ValueError', 'OSError', 'BadZipFile', 'HTTPError', 'HTTPResponse', 'URLError', 'TimeoutError', 'Other') else 'Other'
        self.http_status = http_status if type(http_status) is int and 100 <= http_status <= 599 else None


def _source_diagnostic(error, phase):
    if isinstance(error, SourceError):
        return SourceError(phase=error.phase or phase, code=error.code,
            failure_class=error.failure_class, http_status=error.http_status)
    if isinstance(error, transport.CustodyError):
        return SourceError(phase=phase, code='transport_rejected', failure_class='CustodyError')
    if isinstance(error, KeyError):
        return SourceError(phase=phase, code='response_schema_invalid', failure_class='KeyError')
    if isinstance(error, TypeError):
        return SourceError(phase=phase, code='response_schema_invalid', failure_class='TypeError')
    if isinstance(error, ValueError):
        return SourceError(phase=phase, code='response_schema_invalid', failure_class='ValueError')
    if isinstance(error, OSError):
        return SourceError(phase=phase, code='transport_rejected', failure_class='OSError')
    if isinstance(error, zipfile.BadZipFile):
        return SourceError(phase=phase, code='archive_rejected', failure_class='BadZipFile')
    return SourceError(phase=phase, code='internal_error', failure_class='Other')


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
    for page in range(1, MAX_API_PAGES + 1):
        body = _get(url + '?per_page=' + str(MAX_API_PAGE_SIZE) + '&page=' + str(page), headers, deadline)
        observed = body['total_count']
        if type(observed) is not int or not 0 <= observed <= MAX_API_ITEMS or total is not None and observed != total: _fail()
        total = observed
        rows = body[field]
        if not isinstance(rows, list) or len(rows) > MAX_API_PAGE_SIZE: _fail()
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


def _provisional_ci(headers, run_id, sha, repo, deadline):
    """Explicit staging only: exact immutable images, no known failed job."""
    run = _get(API + '/actions/runs/' + str(run_id), headers, deadline)
    if (run['id'] != run_id or run['workflow_id'] != WORKFLOW or run['head_sha'] != sha
            or run['head_branch'] != 'master' or run['event'] != 'push'
            or run['repository']['full_name'] != REPO or run['head_repository']['full_name'] != REPO
            or run['repository']['id'] != repo['id'] or run['head_repository']['id'] != repo['id']
            or run['path'].split('@', 1)[0] != '.github/workflows/ci.yml'
            or run['status'] not in ('in_progress', 'completed')
            or run['status'] == 'in_progress' and run['conclusion'] is not None
            or run['status'] == 'completed' and run['conclusion'] != 'success'): _fail()
    attempt = run['run_attempt']
    if type(attempt) is not int or attempt <= 0: _fail()
    jobs = _pages(API + '/actions/runs/' + str(run_id) + '/attempts/' + str(attempt) + '/jobs', 'jobs', headers, deadline)
    named = {}
    for job in jobs:
        name = job['name']
        if (job['run_id'] != run_id or job['head_sha'] != sha or job.get('run_attempt') != attempt
                or not isinstance(name, str) or not name or name in named
                or job['status'] not in ('queued', 'in_progress', 'completed')): _fail()
        if job['status'] == 'completed':
            if job['conclusion'] not in ('success', 'skipped'): _fail()
        elif job['conclusion'] is not None or run['status'] == 'completed': _fail()
        named[name] = job
    lock = named.get('staging-stack-lock'); gate = named.get('ci-gate')
    if lock is None or lock['status'] != 'completed' or lock['conclusion'] != 'success': _fail()
    # GitHub does not list a dependency-blocked job until it is scheduled.
    if gate is None and run['status']!='in_progress': _fail()
    if gate is not None and gate['status'] == 'completed' and gate['conclusion'] != 'success': _fail()
    return run, {'staging-stack-lock': lock, 'ci-gate': gate}


def revalidate_ci(token, authority, *, final=False):
    """Never choose a new run or attempt when revalidating bound admission."""
    headers = _headers(token); deadline = time.monotonic() + 60
    repo = _head(headers, authority['source_sha'], deadline)
    check = _provisional_ci if authority.get('provisional_staging') is True and not final else _ci
    run, _ = check(headers, authority['run_id'], authority['source_sha'], repo, deadline)
    if run['run_attempt'] != authority['run_attempt']: _fail()
    return run


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


@contextmanager
def cached_archive(sha):
    """Passive fixed-SHA bytes; all authority still comes from the live API."""
    if not isinstance(sha,str) or not re.fullmatch(r'[a-f0-9]{40}',sha):_fail()
    name=sha+'.tar.gz'
    parent=os.open('/',os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW)
    fd=None
    def identity(row):
        return (row.st_dev,row.st_ino,row.st_mode,row.st_uid,row.st_gid,row.st_nlink,
                row.st_size,row.st_mtime_ns,row.st_ctime_ns)
    try:
        for component in SOURCE_CACHE_DIR.parts[1:]:
            try:child=os.open(component,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW,dir_fd=parent)
            except FileNotFoundError:
                yield None;return
            row=os.fstat(child)
            if row.st_uid!=0 or row.st_mode&0o022:
                os.close(child);_fail()
            os.close(parent);parent=child
        try:fd=os.open(name,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK,dir_fd=parent)
        except FileNotFoundError:
            yield None;return
        before=os.fstat(fd)
        if (not stat.S_ISREG(before.st_mode) or before.st_uid!=0 or before.st_mode&0o022
            or before.st_nlink!=1 or not 1<=before.st_size<=MAX_ARCHIVE
            or identity(os.stat(name,dir_fd=parent,follow_symlinks=False))!=identity(before)):_fail()
        with os.fdopen(fd,'rb',closefd=False) as archive:
            yield archive
        if identity(os.fstat(fd))!=identity(before) or identity(os.stat(name,dir_fd=parent,follow_symlinks=False))!=identity(before):_fail()
    finally:
        if fd is not None:os.close(fd)
        os.close(parent)


def _capture_tar(archive, destination, tree, sha, deadline,*,cached=False):
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
    prefix = 'VoiceRoot-'+sha if cached else 'Poryadok-VoiceRoot-'+sha[:7]
    seen, hashes, items = set(), {}, 0
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
            if (member.size != row['size']
                or bool(member.mode&0o111)!=(row['mode']=='100755')): _fail()
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


def _resolve_image(name, sha, deadline, *, identity=False):
    if name not in IMAGE_NAMES or not re.fullmatch(r'sha256:[a-f0-9]{64}' if identity else r'[a-f0-9]{40}', sha): _fail()
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
    if not identity and ('org.opencontainers.image.revision' in labels and labels['org.opencontainers.image.revision'] != sha
            or 'org.opencontainers.image.source' in labels and labels['org.opencontainers.image.source'].removesuffix('.git') != 'https://github.com/' + REPO): _fail()
    if identity:
        return {'requested_image':REGISTRY+'/'+name+'@'+sha,
                'manifest_image':REGISTRY+'/'+name+'@'+digest,
                'config_sha256':descriptor['digest'].removeprefix('sha256:')}
    return REGISTRY + '/' + name + '@' + digest


def immutable_image_identity(image, deadline):
    # Content identity only; this does not attest source revision or CI approval.
    match=re.fullmatch(re.escape(REGISTRY)+r'/([a-z0-9-]+)@(sha256:[a-f0-9]{64})',image)
    if match is None or match[1] not in IMAGE_NAMES:_fail()
    return _resolve_image(match[1],match[2],deadline,identity=True)


def same_image_content(actual, target, deadline):
    pattern=re.escape(REGISTRY)+r'/([a-z0-9-]+)@sha256:[a-f0-9]{64}'
    left=re.fullmatch(pattern,actual);right=re.fullmatch(pattern,target)
    if left is None or right is None or left[1]!=right[1] or left[1] not in IMAGE_NAMES:_fail()
    a=immutable_image_identity(actual,deadline);b=immutable_image_identity(target,deadline)
    return a['manifest_image']==b['manifest_image'] and a['config_sha256']==b['config_sha256']


def _path_tree_sha(headers, commit_sha, path, deadline, *, expected_type):
    commit = _get(API + '/git/commits/' + commit_sha, headers, deadline)
    if commit.get('sha') != commit_sha or not re.fullmatch(r'[a-f0-9]{40}', commit.get('tree', {}).get('sha', '')): _fail()
    tree_sha = commit['tree']['sha']
    parts = path.split('/')
    for index, part in enumerate(parts):
        tree = _get(API + '/git/trees/' + tree_sha, headers, deadline)
        if tree.get('sha') != tree_sha or tree.get('truncated') is not False or not isinstance(tree.get('tree'), list): _fail()
        matches = [row for row in tree['tree'] if isinstance(row, dict) and row.get('path') == part]
        if len(matches) != 1: _fail()
        row = matches[0]
        final = index == len(parts) - 1
        if (not re.fullmatch(r'[a-f0-9]{40}', str(row.get('sha', '')))
            or row.get('type') != (expected_type if final else 'tree')): _fail()
        tree_sha = row['sha']
    return tree_sha


def _equivalent_master_source(headers, source_sha, master_sha, deadline):
    if not re.fullmatch(r'[a-f0-9]{40}', source_sha) or not re.fullmatch(r'[a-f0-9]{40}', master_sha): _fail()
    comparison = _get(API + '/compare/' + source_sha + '...' + master_sha, headers, deadline)
    status = comparison.get('status')
    ahead_by = comparison.get('ahead_by')
    if (comparison.get('base_commit', {}).get('sha') != source_sha
        or comparison.get('merge_base_commit', {}).get('sha') != source_sha
        or type(ahead_by) is not int or ahead_by < 0
        or source_sha == master_sha and (status != 'identical' or ahead_by != 0)
        or source_sha != master_sha and (status != 'ahead' or ahead_by <= 0)): _fail()
    paths = ('src/backend', '.github/workflows/ci.yml',
        'scripts/ci/backend-docker-context.txt', 'scripts/ci/staging-image-catalog.json')
    bindings = {}
    for path in paths:
        expected_type = 'tree' if path == 'src/backend' else 'blob'
        source = _path_tree_sha(headers, source_sha, path, deadline, expected_type=expected_type)
        current = _path_tree_sha(headers, master_sha, path, deadline, expected_type=expected_type)
        if source != current: _fail()
        bindings[path] = {'git_object_sha1': source}
    return {'current_master_sha_before': master_sha, 'equivalent_inputs': bindings,
        'source_is_ancestor': True}


def _capture_source_at_master(token, run_id, source_sha, destination, master_sha, *, provisional_staging=False):
    phase = 'source-preflight'
    try:
        if type(run_id) is not int or run_id <= 0 or not isinstance(source_sha, str) or not re.fullmatch(r'[a-f0-9]{40}', source_sha): _fail()
        destination = _destination(destination); headers = _headers(token); deadline = time.monotonic() + 600
        phase = 'repository-pin'
        repo = _head(headers, master_sha, deadline)
        if type(provisional_staging) is not bool: _fail()
        equivalence = None
        if master_sha != source_sha:
            if provisional_staging: _fail()
            phase = 'source-equivalence'
            equivalence = _equivalent_master_source(headers, source_sha, master_sha, deadline)
        phase = 'source-ci'
        run, jobs = (_provisional_ci if provisional_staging else _ci)(headers, run_id, source_sha, repo, deadline)
        phase = 'source-tree'
        commit = _get(API + '/git/commits/' + source_sha, headers, deadline)
        if commit['sha'] != source_sha or not re.fullmatch(r'[a-f0-9]{40}', commit['tree']['sha']): _fail()
        tree = _get(API + '/git/trees/' + commit['tree']['sha'] + '?recursive=1', headers, deadline)
        if tree['sha'] != commit['tree']['sha']: _fail()
        if shutil.disk_usage(tempfile.gettempdir()).free < MAX_ARCHIVE: _fail()
        phase = 'source-archive'
        with cached_archive(source_sha) as cached:
            if cached is not None:
                files = _capture_tar(cached,destination,tree,source_sha,deadline,cached=True)
            else:
                with tempfile.TemporaryFile() as archive:
                    _fetch(API + '/tarball/' + source_sha, headers, MAX_ARCHIVE, deadline, target=archive)
                    files = _capture_tar(archive, destination, tree, source_sha, deadline)
        phase = 'stack-lock-artifact'
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
        phase = 'stack-lock-read'
        with tempfile.TemporaryFile() as archive:
            count, _ = _fetch(API + '/actions/artifacts/' + str(artifact_id) + '/zip', headers, 4 * 1024 ** 2, deadline, target=archive)
            if type(artifact.get('size_in_bytes')) is not int or count != artifact['size_in_bytes']: _fail()
            archive.seek(0)
            with zipfile.ZipFile(archive) as zipped:
                members = zipped.infolist()
                if len(members) != 1 or members[0].filename != 'stack.lock.yaml' or members[0].is_dir() or members[0].file_size > 1024 * 1024: _fail()
                if stat.S_IFMT(members[0].external_attr >> 16) not in (0, stat.S_IFREG) or members[0].flag_bits & 1: _fail()
                lock = zipped.read(members[0])
        phase = 'stack-lock-parse'
        tags = _lock(lock, source_sha, destination)
        images = {}
        for name, tag in sorted(tags.items()):
            phase = 'registry-image-' + name
            images[name] = _resolve_image(name, tag, deadline)
        phase = 'master-recheck'
        _head(headers, master_sha, deadline)
        if equivalence is not None:
            equivalence['current_master_sha_after'] = master_sha
        result = {'schema': 'voice-source-authority-v1', 'repository': REPO, 'source_sha': source_sha,
            'workflow_id': WORKFLOW, 'run_id': run_id, 'run_attempt': run['run_attempt'], 'tree_sha': tree['sha'],
            'source_files': dict(sorted(files.items())), 'images': images, 'lock_artifact_id': artifact_id,
            'lock_sha256': hashlib.sha256(lock).hexdigest(), 'image_provenance': 'independently resolved trusted CI tag',
            'verified': True}
        if equivalence is not None:
            result['source_equivalence'] = equivalence
        if provisional_staging:
            result['provisional_staging'] = True
            revalidate_ci(token, result)
        return result
    except Exception as error:
        raise _source_diagnostic(error, phase) from None


def capture_source(token, run_id, source_sha, destination, *, provisional_staging=False):
    """Capture only the currently pinned master source and its canonical CI."""
    return _capture_source_at_master(token, run_id, source_sha, destination, source_sha,
        provisional_staging=provisional_staging)


def capture_equivalent_master_source(token, run_id, source_sha, destination):
    """Capture a successful historical master CI only when current build inputs match.

    The source must be an actual ancestor of the pinned current master. The full
    backend tree and the CI workflow/context/catalog that select its build must
    be byte-identical. The current master ref is pinned before and after capture.
    """
    phase = 'master-ref'
    try:
        headers = _headers(token)
        deadline = time.monotonic() + 60
        ref = _get(API + '/git/ref/heads/master', headers, deadline)
        master_sha = ref.get('object', {}).get('sha') if ref.get('ref') == 'refs/heads/master' else None
        if not isinstance(master_sha, str) or not re.fullmatch(r'[a-f0-9]{40}', master_sha): _fail()
        phase = 'source-capture'
        return _capture_source_at_master(token, run_id, source_sha, destination, master_sha,
            provisional_staging=False)
    except SourceError:
        raise
    except Exception:
        raise SourceError(phase=phase) from None
