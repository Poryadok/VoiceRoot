"""Build the maintained Voice SFU from a minimal context and verified upstream."""
import argparse
import hashlib
import json
import pathlib
import shutil
import subprocess
import tempfile
import urllib.request

root = pathlib.Path(__file__).resolve().parents[2]
parser = argparse.ArgumentParser()
parser.add_argument('--tag', default='voice-sfu:game-sprint')
parser.add_argument('--context-only', action='store_true')
args = parser.parse_args()
working = root / 'tmp/sfu-enforcement-20261002'
working.mkdir(parents=True, exist_ok=True)
context = pathlib.Path(tempfile.mkdtemp(prefix='sfu-build-', dir=working))
pins = {
    'livekit-v1.8.4.tar.gz': ('https://codeload.github.com/livekit/livekit/tar.gz/refs/tags/v1.8.4', '81e8b7c6ed90fe98f91bb0b1dd48bf254f564f3cc925ce5d25e335e2e03fd648'),
    'protocol-v1.34.0.tar.gz': ('https://codeload.github.com/livekit/protocol/tar.gz/refs/tags/v1.34.0', '05452d9d7fc6a036f752458d4f9506105e5f4f11596b7bb7cd5ef34b549a6c9c'),
}
for name, (url, digest) in pins.items():
    cached = working / name
    if not cached.exists():
        urllib.request.urlretrieve(url, cached)
    if hashlib.sha256(cached.read_bytes()).hexdigest() != digest:
        raise SystemExit('upstream digest mismatch: ' + name)
    shutil.copy2(cached, context / name)
(context / 'upstream.sha256').write_text(''.join(digest+'  '+name+'\n' for name, (_, digest) in pins.items()), encoding='utf-8', newline='\n')
for name in ['Dockerfile', 'prepare.py', 'voice_authority.go']:
    shutil.copy2(root / 'docker/voice-node/livekit' / name, context / name)
for package in ['protocol', 'nodecache', 'mediaauthority']:
    target = context / 'authority' / package
    target.mkdir(parents=True, exist_ok=True)
    for source in (root / 'src/backend/federation' / package).glob('*.go'):
        shutil.copy2(source, target / source.name)
inputs = {p.relative_to(context).as_posix(): hashlib.sha256(p.read_bytes()).hexdigest() for p in context.rglob('*') if p.is_file() and p.name != 'inputs.json'}
(context / 'inputs.json').write_text(json.dumps(inputs, indent=2), encoding='utf-8')
digest = hashlib.sha256(json.dumps(inputs, sort_keys=True).encode()).hexdigest()
print(json.dumps({'context': str(context), 'input_sha256': digest, 'files': len(inputs)}), flush=True)
if not args.context_only:
    raise SystemExit(subprocess.run(['rtk', 'docker', 'build', '--label', 'voice.sfu.input-sha256='+digest, '-t', args.tag, str(context)], cwd=root).returncode)
