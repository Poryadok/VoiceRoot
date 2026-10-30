"""Acquire a source-bound private Go cache for local diagnostics, not PASS."""
import base64
import hashlib
import json
import os
from pathlib import Path
import shutil
import stat
import subprocess
import tarfile

import local_go_execution
import local_source_snapshot as snapshot


def _sha256(path):
    digest = hashlib.sha256()
    with Path(path).open('rb') as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b''):
            digest.update(block)
    return digest.hexdigest()


def _gomod_sum(path):
    # Go's h1 hash for a module's go.mod is the hash of a single-file tree.
    line = (_sha256(path) + '  go.mod\n').encode('ascii')
    return 'h1:' + base64.b64encode(hashlib.sha256(line).digest()).decode('ascii')


def _json_stream(raw):
    text = raw.decode('utf-8')
    decoder = json.JSONDecoder()
    while text.strip():
        text = text.lstrip()
        item, end = decoder.raw_decode(text)
        yield item
        text = text[end:]


def acquire(root, source, source_copy, module_dir, output, go_binary='go'):
    """Download exact go.sum-backed modules to a new private diagnostic cache.

    This does not transfer the cache, bind the host toolchain to an approved
    image, supervise an entire preflight, or issue a successful receipt.
    """
    local_go_execution._module_path(module_dir)
    root, source_copy, output = Path(root).resolve(), Path(source_copy).resolve(), Path(output).resolve()
    if (output.exists() or output == root or root in output.parents or
            output == source_copy or source_copy in output.parents):
        raise ValueError('dependency output must be new and outside source trees')
    module = source_copy.joinpath(*module_dir.split('/'))
    for parent in (source_copy, *[source_copy.joinpath(*module_dir.split('/')[:i])
                                  for i in range(1, len(module_dir.split('/')) + 1)]):
        if (parent / 'go.work').exists():
            raise ValueError('Go workspace dependency acquisition is not supported')
    if not (module / 'go.mod').is_file():
        raise ValueError('selected Go module is missing')
    executable = shutil.which(go_binary)
    if not executable:
        raise ValueError('Go toolchain is unavailable')
    executable = Path(executable).resolve()
    toolchain_digest = _sha256(executable)
    snapshot.verify_materialized(root, source, source_copy)
    if snapshot.capture(root, source['head']) != source:
        raise ValueError('source changed before dependency acquisition')
    output.mkdir(parents=True)
    cache = output / 'modcache'
    environment = {key: value for key, value in os.environ.items()
                   if not key.upper().startswith(('GO', 'GIT_'))}
    environment.update(GOENV='off', GOWORK='off', GOFLAGS='-mod=readonly', GOTOOLCHAIN='local',
                       GOPROXY='https://proxy.golang.org', GOSUMDB='sum.golang.org', GOPRIVATE='',
                       GONOSUMDB='', GONOPROXY='', GOMODCACHE=str(cache), GOPATH=str(output / 'gopath'),
                       GOCACHE=str(output / 'build-cache'))
    version = subprocess.run([str(executable), 'version'], env=environment, stdout=subprocess.PIPE,
                             stderr=subprocess.PIPE, timeout=30, check=True).stdout.decode('utf-8').strip()
    command = [str(executable), 'mod', 'download', '-json', 'all']
    with (output / 'download.json').open('xb') as stdout, (output / 'download.stderr').open('xb') as stderr:
        result = subprocess.run(command, cwd=module, env=environment, stdout=stdout, stderr=stderr,
                                timeout=1800, check=False)
    if result.returncode != 0:
        raise ValueError('Go dependency acquisition failed')
    verify = subprocess.run([str(executable), 'mod', 'verify'], cwd=module, env=environment,
                            stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=1800, check=False)
    if verify.returncode != 0 or verify.stdout.strip() != b'all modules verified':
        raise ValueError('downloaded Go module cache failed verification')
    if _sha256(executable) != toolchain_digest:
        raise ValueError('Go toolchain changed during dependency acquisition')
    snapshot.verify_materialized(root, source, source_copy)
    if snapshot.capture(root, source['head']) != source:
        raise ValueError('source changed during dependency acquisition')
    sum_path = module / 'go.sum'
    sums = set()
    if sum_path.exists():
        for line in sum_path.read_text(encoding='utf-8').splitlines():
            parts = line.split()
            if len(parts) != 3:
                raise ValueError('invalid go.sum entry')
            sums.add(tuple(parts))
    entries, versions, mod_sums, zip_sums = {}, set(), {}, {}
    download_root = (cache / 'cache' / 'download').resolve()
    for item in _json_stream((output / 'download.json').read_bytes()):
        if not isinstance(item, dict) or item.get('Error'):
            raise ValueError('invalid Go module download result')
        module_name, version_name = item.get('Path'), item.get('Version')
        if (not isinstance(module_name, str) or not isinstance(version_name, str) or
                (module_name, version_name) in versions or
                (module_name, version_name, item.get('Sum')) not in sums or
                (module_name, version_name + '/go.mod', item.get('GoModSum')) not in sums):
            raise ValueError('downloaded Go module is not bound to go.sum')
        versions.add((module_name, version_name))
        paths = [Path(item[key]) for key in ('Info', 'GoMod', 'Zip')]
        paths.append(Path(str(paths[-1]) + 'hash'))
        if paths[-1].read_text(encoding='utf-8').strip() != item['Sum'] or _gomod_sum(paths[1]) != item['GoModSum']:
            raise ValueError('downloaded Go module hash differs from go.sum')
        mod_sums[paths[1]] = item['GoModSum']
        zip_sums[paths[-1]] = item['Sum']
        for path in paths:
            info = path.lstat()
            if (not stat.S_ISREG(info.st_mode) or download_root not in path.resolve().parents or
                    path.is_symlink()):
                raise ValueError('invalid Go module cache artifact')
            relative = path.relative_to(download_root).as_posix()
            if relative in entries:
                raise ValueError('duplicate Go module cache artifact')
            entries[relative] = path
    archive_path = output / 'download-cache.tar'
    with archive_path.open('xb') as raw, tarfile.open(fileobj=raw, mode='w') as tar:
        for relative, path in sorted(entries.items()):
            tar.add(path, arcname='cache/download/' + relative, recursive=False)
    verify = subprocess.run([str(executable), 'mod', 'verify'], cwd=module, env=environment,
                            stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=1800, check=False)
    if verify.returncode != 0 or verify.stdout.strip() != b'all modules verified':
        raise ValueError('downloaded Go module cache changed during archival')
    if (any(_gomod_sum(path) != value for path, value in mod_sums.items()) or
            any(path.read_text(encoding='utf-8').strip() != value for path, value in zip_sums.items())):
        raise ValueError('downloaded Go module hashes changed during archival')
    with tarfile.open(archive_path, mode='r:') as archive:
        members = archive.getmembers()
        expected = sorted(entries.items())
        if len(members) != len(expected):
            raise ValueError('Go cache archive entries changed')
        for member, (relative, path) in zip(members, expected):
            if not member.isfile() or member.name != 'cache/download/' + relative:
                raise ValueError('Go cache archive entries changed')
            digest = hashlib.sha256()
            with archive.extractfile(member) as stream:
                for block in iter(lambda: stream.read(1024 * 1024), b''):
                    digest.update(block)
            if not stat.S_ISREG(path.lstat().st_mode) or digest.hexdigest() != _sha256(path):
                raise ValueError('Go cache archive bytes changed')
    snapshot.verify_materialized(root, source, source_copy)
    if snapshot.capture(root, source['head']) != source or _sha256(executable) != toolchain_digest:
        raise ValueError('source or Go toolchain changed during cache archival')
    return {'status': 'incomplete', 'command': command, 'toolchain_version': version,
            'toolchain_sha256': toolchain_digest, 'archive_sha256': _sha256(archive_path),
            'module_count': len(versions), 'source': source}
