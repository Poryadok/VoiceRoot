"""Build only the distinct source-qualified restored-data8-v1 test executable.

Usage: python3 staging_story_target8_helper.py --root GIT_ROOT --output NEW_DIR
The result supplies caller bytes only; it never authenticates P2 backup/profile
lineage or executes the compiled probe. Legacy three-helper inputs are unchanged.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tarfile
import tempfile
import uuid

import staging_qualified_helpers as legacy

SOURCE = 'cf450c370ad9b8947a3a96081549aa549bfdc7da'
ARCHIVE_SHA = 'ea1a0b7084d20827fbd826ddfd8be9af6d99da6230e4b72d46404c67ae53ab76'
ORIGINAL_SHA = 'e947f9e9b771d8fd7be668c647f61d24b89eb06048b540a74e265bd5a31718e9'
VARIANT_SHA = 'bbb7dc68ef4f2d4a97a2de665fb94a71b213382b6c544f490c0837c573663df9'
MAIN = 'src/backend/story/cmd/owned-story-probe/main.go'
NAME = 'story-restored8-probe'
BINARY_SHA = '458e4e3e5e940137cddddaf42dda1c22505f92de5792600364907a343b209be0'
GO_IMAGE = 'docker.io/library/golang@sha256:8ac98ca534ac3f51e1f420a1dd2c15e74c75cfa0f23f3ad27eb5d7236c349a0c'
GO_CONFIG = 'sha256:c5fe2bbc8331458475b45686d82e0c28b1161addfe33f1ebac0a715a29587701'
LABEL = 'voice.ci.story-target8-build'
BUILD = """set -eu
mkdir -p /tmp/canonical
tar -xf /input/source.tar -C /tmp/canonical
cp /input/main.go /tmp/canonical/src/backend/story/cmd/owned-story-probe/main.go
cd /tmp/canonical/src/backend/story
go build -p=2 -mod=readonly -trimpath -buildvcs=false -o /out/story-restored8-probe ./cmd/owned-story-probe
"""


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def prepare_sources(root, inputs, variant):
    if Path(variant).is_symlink() or not Path(variant).is_file():
        raise ValueError('target8 variant must be a regular file')
    raw = Path(variant).read_bytes()
    if sha(raw) != VARIANT_SHA:
        raise ValueError('target8 variant pin mismatch')
    digest = legacy.archive(root, SOURCE, 'src/backend', inputs / 'source.tar')
    if digest != ARCHIVE_SHA or sha((inputs / 'source.tar').read_bytes()) != ARCHIVE_SHA:
        raise ValueError('target8 archive pin mismatch')
    with tarfile.open(inputs / 'source.tar', 'r:') as archive:
        matches = [member for member in archive.getmembers() if member.name == MAIN]
        if len(matches) != 1 or not matches[0].isfile():
            raise ValueError('target8 original main inventory mismatch')
        if sha(archive.extractfile(matches[0]).read()) != ORIGINAL_SHA:
            raise ValueError('target8 original main pin mismatch')
    (inputs / 'main.go').write_bytes(raw)
    return {'commit': SOURCE, 'subtree': 'src/backend', 'archive_sha256': digest,
            'overlay': {'path': MAIN, 'original_sha256': ORIGINAL_SHA,
                        'variant_sha256': VARIANT_SHA}}


def command(argv, records, check=True):
    result = subprocess.run(argv, capture_output=True)
    records.append({'argv': list(argv), 'exit': result.returncode,
                    'stdout_sha256': sha(result.stdout), 'stderr_sha256': sha(result.stderr)})
    if check and result.returncode:
        raise RuntimeError('target8 command failed: ' + ' '.join(argv[:3]) + '\n' +
                           result.stderr.decode('utf-8', errors='replace')[-4000:])
    return result


def authenticate_image(records):
    inspected = command(['docker', 'image', 'inspect', GO_IMAGE], records, check=False)
    if inspected.returncode:
        command(['docker', 'pull', '--platform', 'linux/amd64', GO_IMAGE], records)
        inspected = command(['docker', 'image', 'inspect', GO_IMAGE], records)
    image = json.loads(inspected.stdout)
    digest = 'golang@' + GO_IMAGE.split('@')[1]
    if (len(image) != 1 or image[0].get('Id') != GO_CONFIG or
            image[0].get('Os') != 'linux' or image[0].get('Architecture') != 'amd64' or
            not any(value in (digest, 'docker.io/library/' + digest)
                    for value in image[0].get('RepoDigests', [])) or
            'GOLANG_VERSION=1.26.8' not in image[0].get('Config', {}).get('Env', [])):
        raise ValueError('target8 Go image identity mismatch')
    return {'reference': GO_IMAGE, 'config_id': GO_CONFIG, 'os': 'linux',
            'architecture': 'amd64', 'go_version': '1.26.8'}


def cleanup_container(name, token, records):
    found = command(['docker', 'container', 'ls', '--all', '--filter', 'name=^/' + name + '$',
                     '--format', '{{.ID}}'], records)
    ids = found.stdout.decode().split()
    if not ids:
        return {'name': name, 'absent': True}
    if len(ids) != 1:
        raise RuntimeError('target8 cleanup inventory uncertain')
    value = json.loads(command(['docker', 'container', 'inspect', name], records).stdout)
    if (len(value) != 1 or value[0].get('Name') != '/' + name or
            value[0].get('Config', {}).get('Labels', {}).get(LABEL) != token or
            not value[0].get('Id', '').startswith(ids[0])):
        raise RuntimeError('target8 cleanup ownership mismatch; container preserved')
    owned_id = value[0]['Id']
    command(['docker', 'container', 'rm', '--force', owned_id], records)
    remaining = command(['docker', 'container', 'ls', '--all', '--filter', 'id=' + owned_id,
                         '--format', '{{.ID}}'], records)
    if remaining.stdout.strip():
        raise RuntimeError('target8 cleanup absence unproven')
    return {'name': name, 'id': owned_id, 'label': token, 'absent': True}


def publish(built, work, output, provenance):
    raw = (built / NAME).read_bytes()
    if not raw.startswith(b'\x7fELF') or sha(raw) != BINARY_SHA:
        raise ValueError('target8 binary pin mismatch: ' + sha(raw))
    stage = work / 'publish'
    stage.mkdir(mode=0o700)
    # Copy into files created by the invoking host UID; never chmod uid0 Docker files.
    shutil.copyfile(built / NAME, stage / NAME)
    (stage / NAME).chmod(0o400)
    (stage / 'provenance.json').write_text(json.dumps(provenance, sort_keys=True, indent=2) + '\n',
                                           encoding='utf-8')
    (stage / 'provenance.json').chmod(0o400)
    if output.exists() or output.is_symlink():
        raise ValueError('target8 output must be new')
    stage.rename(output)


def build(root, output):
    root = Path(root).resolve(strict=True)
    output = Path(output).absolute()
    if output.exists() or output.is_symlink():
        raise ValueError('target8 output must be new')
    output.parent.mkdir(parents=True, exist_ok=True)
    records = []
    with tempfile.TemporaryDirectory(prefix='voice-target8-build-', dir=output.parent) as directory:
        work = Path(directory)
        inputs, built = work / 'inputs', work / 'built'
        inputs.mkdir(mode=0o700)
        built.mkdir(mode=0o700)
        source = prepare_sources(root, inputs, Path(__file__).resolve().parents[1] /
                                 'staging' / 'story-restored8-probe-main.go')
        image = authenticate_image(records)
        token = uuid.uuid4().hex
        name = 'voice-story-target8-' + token
        print('TARGET8_CONTAINER=' + name, flush=True)
        uid = str(os.getuid()) if hasattr(os, 'getuid') else '1000'
        gid = str(os.getgid()) if hasattr(os, 'getgid') else '1000'
        argv = ['docker', 'container', 'create', '--name', name, '--label', LABEL + '=' + token,
                '--platform', 'linux/amd64', '--user', uid + ':' + gid,
                '--read-only', '--cap-drop=ALL', '--security-opt=no-new-privileges',
                '--pids-limit', '256', '--memory', '2g', '--cpus', '2',
                '--tmpfs', '/tmp:mode=1777,size=2g',
                '--mount', 'type=bind,source=' + str(inputs.resolve()) + ',target=/input,readonly',
                '--mount', 'type=bind,source=' + str(built.resolve()) + ',target=/out']
        environment = {'CGO_ENABLED': '0', 'GOWORK': 'off', 'GOTOOLCHAIN': 'local',
                       'GOCACHE': '/tmp/cache', 'GOMODCACHE': '/tmp/mod'}
        for key, value in environment.items():
            argv += ['--env', key + '=' + value]
        argv += [GO_IMAGE, 'sh', '-eu', '-c', BUILD]
        try:
            command(argv, records)
            command(['docker', 'container', 'start', '--attach', name], records)
        finally:
            cleanup = cleanup_container(name, token, records)
        provenance = {'schema': 'voice-ci-story-target8-helper-v1',
                      'contract': 'restored-data8-v1', 'source': source, 'go_image': image,
                      'environment': environment, 'build_script': BUILD,
                      'binaries': {NAME: BINARY_SHA}, 'container_user': uid + ':' + gid,
                      'cleanup': cleanup, 'commands': records}
        publish(built, work, output, provenance)
    print('TARGET8_HELPER=' + BINARY_SHA, flush=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', required=True, type=Path)
    parser.add_argument('--output', required=True, type=Path)
    args = parser.parse_args()
    build(args.root, args.output)


if __name__ == '__main__':
    main()
