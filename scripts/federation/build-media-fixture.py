"""Build the real-media protocol client from a minimal owned fixture context."""
import hashlib
import json
import pathlib
import shutil
import subprocess
import tempfile

root = pathlib.Path(__file__).resolve().parents[2]
working = root / 'tmp/sfu-enforcement-20261002'
working.mkdir(parents=True, exist_ok=True)
context = pathlib.Path(tempfile.mkdtemp(prefix='media-build-', dir=working))
for module in ['federation', 'pkg']:
    target = context / 'src/backend' / module
    target.mkdir(parents=True, exist_ok=True)
    for name in ['go.mod', 'go.sum']:
        shutil.copy2(root / 'src/backend' / module / name, target / name)
for package in ['protocol', 'nodecache', 'mediaauthority', 'nodepublisher', 'cmd/node-authority']:
    target = context / 'src/backend/federation' / package
    target.mkdir(parents=True, exist_ok=True)
    for source in (root / 'src/backend/federation' / package).glob('*.go'):
        shutil.copy2(source, target / source.name)
target = context / 'tests/federation-media'
for source in (root / 'tests/federation-media').rglob('*'):
    if source.is_file() and source.suffix in {'.go', '.mod', '.sum'}:
        destination = target / source.relative_to(root / 'tests/federation-media')
        destination.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(source, destination)
shutil.copy2(root / 'tests/federation-media/Dockerfile', context / 'Dockerfile')
hasher = hashlib.sha256()
for source in sorted(context.rglob('*')):
    if source.is_file():
        hasher.update(source.relative_to(context).as_posix().encode() + b'\0' + source.read_bytes())
record = {'context': str(context), 'input_sha256': hasher.hexdigest(), 'image': 'voice-sfu-media-fixture:game-sprint'}
with (working / 'media-build.log').open('w', encoding='utf-8') as output:
    result = subprocess.run(['rtk', 'proxy', 'docker', 'build', '--label', 'voice.input.sha256=' + record['input_sha256'], '-t', record['image'], str(context)], cwd=root, stdout=output, stderr=subprocess.STDOUT)
record['exit'] = result.returncode
if not result.returncode:
    record['image_id'] = subprocess.check_output(['rtk', 'proxy', 'docker', 'image', 'inspect', '--format', '{{.Id}}', record['image']], cwd=root, text=True).strip()
(working / 'media-build-result.json').write_text(json.dumps(record, indent=2))
print(json.dumps(record))
raise SystemExit(result.returncode)
