"""Independent bounded ciphertext artifact readback for the fixed repository.

GitHub REST artifact metadata and download redirect are documented at
https://docs.github.com/en/rest/actions/artifacts . No caller URL is consumed.
"""
from datetime import datetime, timezone
import hashlib
import http.client
import io
import json
import os
from pathlib import Path
import re
import shutil
import ssl
import stat
import struct
import tempfile
import time
from urllib.parse import urlsplit
import zipfile

API = 'https://api.github.com/repos/Poryadok/VoiceRoot/actions/artifacts/'
MAX_CIPHER_BYTES = 64 * 1024 ** 3
TIME_LIMIT = 600
CHUNK = 1024 * 1024

class CustodyError(ValueError):
    pass


def _fail():
    raise CustodyError('artifact_custody_rejected')


def _url(url, api=False):
    parsed = urlsplit(url)
    host = parsed.hostname or ''
    # The Azure prefix is deliberately narrower than GitHub's documented
    # *.blob.core.windows.net artifact network requirement.
    allowed = host == 'api.github.com' if api else (
        bool(re.fullmatch(r'productionresultssa[a-z0-9]+\.blob\.core\.windows\.net', host))
        or host == 'results-receiver.actions.githubusercontent.com')
    if (not allowed or parsed.scheme != 'https' or parsed.port not in (None, 443)
            or parsed.username or parsed.password or parsed.fragment): _fail()
    return parsed


def _request(url, headers, limit, deadline):
    parsed = urlsplit(url)
    remaining = deadline - time.monotonic()
    if remaining <= 0: _fail()
    # http.client does not consult environment proxy configuration or follow
    # redirects. The default SSL context verifies certificates and hostname.
    connection = http.client.HTTPSConnection(parsed.hostname, timeout=min(30, remaining), context=ssl.create_default_context())
    try:
        path = parsed.path + ('?' + parsed.query if parsed.query else '')
        connection.request('GET', path, headers=headers)
        response = connection.getresponse()
        response._custody_connection = connection
        return response
    except Exception:
        connection.close()
        _fail()


def _copy(response, target, limit, deadline):
    declared = response.headers.get('Content-Length')
    if declared is not None and (not declared.isdecimal() or int(declared) > limit): _fail()
    if response.headers.get('Content-Encoding', 'identity') != 'identity': _fail()
    count = 0
    while True:
        remaining = deadline - time.monotonic()
        if remaining <= 0: _fail()
        connection = getattr(response, '_custody_connection', None)
        if connection is not None and connection.sock is not None:
            connection.sock.settimeout(min(30, remaining))
        chunk = response.read(min(CHUNK, limit - count + 1))
        if not chunk: break
        count += len(chunk)
        if count > limit: _fail()
        target.write(chunk)
    return count


def _private_cipher(path):
    path = Path(path)
    before = path.lstat()
    if not stat.S_ISREG(before.st_mode): _fail()
    fd = os.open(path, os.O_RDONLY | getattr(os, 'O_NOFOLLOW', 0) | getattr(os, 'O_NONBLOCK', 0) | getattr(os, 'O_BINARY', 0))
    try:
        observed = os.fstat(fd)
        if (not stat.S_ISREG(observed.st_mode) or (before.st_dev, before.st_ino) != (observed.st_dev, observed.st_ino)
                or os.name == 'posix' and (observed.st_uid != os.geteuid() or observed.st_mode & 0o077)):
            _fail()
        return os.fdopen(fd, 'rb')
    except Exception:
        os.close(fd)
        _fail()


def _close(response):
    response.close()
    connection = getattr(response, '_custody_connection', None)
    if connection: connection.close()


def _download(url, headers, target, limit, deadline, redirects=False):
    for attempt in range(4):
        _url(url, api=attempt == 0)
        response = _request(url, headers if attempt == 0 else {}, limit, deadline)
        try:
            if response.status == 200:
                return _copy(response, target, limit, deadline)
            if not redirects or response.status not in (301, 302, 303, 307, 308) or attempt == 3: _fail()
            url = response.headers.get('Location', '')
            _url(url)
        finally:
            _close(response)
    _fail()


def _timestamp(value):
    parsed = datetime.fromisoformat(value.replace('Z', '+00:00'))
    if parsed.tzinfo is None: _fail()
    return parsed.astimezone(timezone.utc)


def verify_artifact(token, binding, cipher_path, artifact_id):
    """Verify remote archive metadata and all ciphertext bytes; never extract."""
    try:
        if (not isinstance(token, str) or not token or '\n' in token or '\r' in token
                or type(artifact_id) is not int or artifact_id <= 0
                or set(binding) != {'operation', 'challenge', 'run_id', 'head_sha', 'cipher_sha256', 'cipher_bytes', 'created_at'}): _fail()
        for key in ('operation', 'challenge'):
            if not isinstance(binding[key], str) or not re.fullmatch(r'[a-zA-Z0-9_-]{16,128}', binding[key]): _fail()
        if (type(binding['run_id']) is not int or binding['run_id'] <= 0
                or type(binding['cipher_bytes']) is not int or not 0 < binding['cipher_bytes'] <= MAX_CIPHER_BYTES
                or not re.fullmatch(r'[0-9a-f]{40}', binding['head_sha'])
                or not re.fullmatch(r'[0-9a-f]{64}', binding['cipher_sha256'])): _fail()
        start = _timestamp(binding['created_at'])
        now = datetime.now(timezone.utc)
        if start > now or (now - start).total_seconds() > TIME_LIMIT: _fail()
        deadline = time.monotonic() + TIME_LIMIT
        hasher, count = hashlib.sha256(), 0
        with _private_cipher(cipher_path) as local:
            while chunk := local.read(CHUNK):
                if time.monotonic() > deadline: _fail()
                count += len(chunk)
                if count > binding['cipher_bytes']: _fail()
                hasher.update(chunk)
        if count != binding['cipher_bytes'] or hasher.hexdigest() != binding['cipher_sha256']: _fail()
        headers = {'Authorization': 'Bearer ' + token, 'Accept': 'application/vnd.github+json',
            'X-GitHub-Api-Version': '2022-11-28', 'User-Agent': 'Voice-root-custody'}
        metadata_buffer = io.BytesIO()
        url = API + str(artifact_id)
        _download(url, headers, metadata_buffer, 65536, deadline)
        metadata = json.loads(metadata_buffer.getvalue())
        run = metadata['workflow_run']
        created = _timestamp(metadata['created_at'])
        archive_limit = binding['cipher_bytes'] + max(1024 * 1024, binding['cipher_bytes'] // 100)
        if (metadata['id'] != artifact_id or metadata['url'] != url
                or metadata['name'] != 'voice-nats-rollout-' + binding['operation'] + '-' + binding['challenge']
                or metadata['expired'] is not False or run['id'] != binding['run_id'] or run['head_sha'] != binding['head_sha']
                or created < start or created > now or _timestamp(metadata['expires_at']) <= now or type(metadata['size_in_bytes']) is not int
                or not 0 < metadata['size_in_bytes'] <= archive_limit): _fail()
        if shutil.disk_usage(tempfile.gettempdir()).free < metadata['size_in_bytes'] + CHUNK: _fail()
        with tempfile.TemporaryFile() as remote:
            downloaded = _download(url + '/zip', headers, remote, archive_limit, deadline, redirects=True)
            if downloaded != metadata['size_in_bytes']: _fail()
            # Bound central-directory parsing before ZipFile allocates entries.
            # Exactly one member means ZIP64 entry-count expansion is unnecessary;
            # ZIP64 individual sizes remain supported by ZipFile.
            remote.seek(-min(downloaded, 65557), 2)
            tail = remote.read(65557)
            offset = tail.rfind(b'PK\x05\x06')
            if offset < 0 or len(tail) != offset + 22: _fail()
            _, disk, directory_disk, disk_count, total_count, directory_bytes, _, comment_bytes = struct.unpack('<4s4H2LH', tail[offset:])
            if disk or directory_disk or disk_count != 1 or total_count != 1 or directory_bytes > 8192 or comment_bytes: _fail()
            remote.seek(0)
            with zipfile.ZipFile(remote) as archive:
                members = archive.infolist()
                if len(members) != 1: _fail()
                member = members[0]
                mode = member.external_attr >> 16
                if (member.filename != 'rollout-backup.cms' or member.is_dir() or member.flag_bits & 1
                        or stat.S_IFMT(mode) not in (0, stat.S_IFREG) or member.file_size != binding['cipher_bytes']
                        or member.header_offset != 0 or member.compress_size < binding['cipher_bytes'] * 0.95
                        or member.compress_type not in (zipfile.ZIP_STORED, zipfile.ZIP_DEFLATED)): _fail()
                hasher, count = hashlib.sha256(), 0
                with archive.open(member) as content:
                    while chunk := content.read(CHUNK):
                        if time.monotonic() > deadline: _fail()
                        count += len(chunk)
                        if count > binding['cipher_bytes']: _fail()
                        hasher.update(chunk)
                if count != binding['cipher_bytes'] or hasher.hexdigest() != binding['cipher_sha256']: _fail()
        return {'schema': 'voice-nats-custody-v1', 'destination': 'github:Poryadok/VoiceRoot',
            **{key: binding[key] for key in ('operation', 'challenge', 'run_id', 'head_sha', 'cipher_sha256', 'cipher_bytes')},
            'artifact_id': artifact_id, 'verified': True}
    except Exception:
        raise CustodyError('artifact_custody_rejected') from None
