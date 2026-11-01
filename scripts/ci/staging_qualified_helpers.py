#!/usr/bin/env python3
"""Build mandatory isolated-test executables from canonical pinned Git sources.

This runs Go only in the pinned Linux Docker image. Generated executables are
hash-qualified inputs to source-custody fixtures; this tool never executes them.
"""
import argparse
import hashlib
import json
import shutil
import os
from pathlib import Path
import subprocess
import tempfile
import uuid

GO_IMAGE = 'golang@sha256:8ac98ca534ac3f51e1f420a1dd2c15e74c75cfa0f23f3ad27eb5d7236c349a0c'
KERNEL_SOURCE = 'f8624086f71d2d4baf1d1ae6cf6461246254960f'
HELPER_SOURCE = 'cf450c370ad9b8947a3a96081549aa549bfdc7da'
PINS = {
    'kernel': '9fced2b6eb7ca33d744ceceb1550774ee7a3a6f423c1a3561ab2d3afba107548',
    'bootstrap-renewer': '618f1f78e650d19c4948c1d08741ed625c6f9aa706159219dbf11c785a56237c',
    'story-current4-probe': '88b6c89b9e57d028c5371db3faaea9eec4624223f54204b59056525f6971f53a',
}
BUILD = """set -eu
mkdir /canonical-kernel /canonical-helpers
tar -xf /input/kernel-source.tar -C /canonical-kernel
tar -xf /input/helper-source.tar -C /canonical-helpers
export CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOMAXPROCS=2 GOFLAGS=-p=2
cd /canonical-kernel/scripts/staging/nats-known-baseline
go build -trimpath -o /out/kernel .
cd /canonical-helpers/src/backend/pkg
go build -trimpath -o /out/bootstrap-renewer ./cmd/nats-jwt-issuer
cd /canonical-helpers/src/backend/story/cmd/owned-story-probe
go build -trimpath -ldflags='-s -w' -o /out/story-current4-probe .
"""


def archive(root, commit, subtree, destination):
    # actions/checkout may be shallow. Fetch only the explicit public historical
    # source object when absent, before the private CI snapshot loses .git.
    # Git archive also honors checkout EOL policy. Ignore host autocrlf so
    # source bytes remain canonical LF on Windows and ordinary Linux CI alike.
    git = ['git', '-c', 'core.autocrlf=false', '-c', 'core.eol=lf',
           '--no-replace-objects', '-C', str(root)]
    present = subprocess.run([*git, 'cat-file', '-e', commit + '^{commit}'],
                             stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    if present.returncode:
        subprocess.run([*git, 'fetch', '--no-tags', 'origin', commit], check=True)
    actual = subprocess.check_output([*git, 'rev-parse', commit + '^{commit}'], text=True).strip()
    if actual != commit:
        raise ValueError('qualified helper source commit changed')
    with destination.open('xb') as stream:
        subprocess.run([*git, 'archive', '--format=tar', commit, subtree],
                       stdout=stream, check=True)
    return hashlib.sha256(destination.read_bytes()).hexdigest()


def build(root, output):
    root = Path(root).resolve(strict=True)
    output = Path(output)
    if output.exists() or output.is_symlink():
        raise ValueError('qualified helper output must be new')
    output.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix='voice-qualified-build-', dir=output.parent) as directory:
        work = Path(directory)
        inputs = work / 'inputs'
        built = work / 'built'
        inputs.mkdir()
        built.mkdir()
        sources = {
            'kernel': {'commit': KERNEL_SOURCE, 'subtree': 'scripts/staging/nats-known-baseline'},
            'helpers': {'commit': HELPER_SOURCE, 'subtree': 'src/backend'},
        }
        for name, record in sources.items():
            archive_name = 'kernel-source.tar' if name == 'kernel' else 'helper-source.tar'
            record['archive_sha256'] = archive(root, record['commit'], record['subtree'], inputs / archive_name)
        container = 'voice-qualified-helpers-' + uuid.uuid4().hex
        print('QUALIFIED_HELPER_CONTAINER=' + container, flush=True)
        try:
            subprocess.run([
                'docker', 'run', '--rm', '--name', container, '--pids-limit', '128', '--memory', '2g',
                '--mount', 'type=bind,source=' + str(inputs.resolve()) + ',target=/input,readonly',
                '--mount', 'type=bind,source=' + str(built.resolve()) + ',target=/out',
                GO_IMAGE, 'sh', '-eu', '-c', BUILD,
            ], check=True)
        finally:
            # --rm handles ordinary completion. A disconnected/interrupted CLI
            # may leave its container running; settle only this exact owned name.
            cleanup = subprocess.run(['docker', 'container', 'rm', '--force', container],
                                     capture_output=True, text=True)
            if cleanup.returncode and ('No such container: ' + container) not in cleanup.stderr:
                raise RuntimeError('qualified helper container cleanup uncertain: ' + container)
        observed = {}
        for name, wanted in PINS.items():
            raw = (built / name).read_bytes()
            actual = hashlib.sha256(raw).hexdigest()
            if not raw.startswith(b'\x7fELF') or actual != wanted:
                raise ValueError('qualified helper pin mismatch: ' + name + ': ' + actual)
            observed[name] = actual
        # Assemble the complete immutable directory privately. Failed chmod,
        # provenance write or final rename must never publish a partial output.
        # Docker writes uid0 files on a Linux bind mount. Read verified bytes
        # into a separate runner-owned stage before chmod and atomic publish.
        publish = work / 'publish'
        publish.mkdir(mode=0o700)
        for name in PINS:
            shutil.copyfile(built / name, publish / name)
            (publish / name).chmod(0o400)
        provenance = {'schema': 'voice-ci-qualified-helpers-v1', 'go_image': GO_IMAGE,
                      'sources': sources, 'build_script': BUILD, 'binaries': observed}
        (publish / 'provenance.json').write_text(json.dumps(provenance, sort_keys=True, indent=2) + '\n',
                                               encoding='utf-8')
        (publish / 'provenance.json').chmod(0o400)
        publish.rename(output)
        print('QUALIFIED_HELPERS=' + json.dumps(observed, sort_keys=True), flush=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', required=True, type=Path)
    parser.add_argument('--output', required=True, type=Path)
    args = parser.parse_args()
    build(args.root, args.output)


if __name__ == '__main__':
    main()
