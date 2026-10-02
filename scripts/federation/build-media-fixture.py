"""Build the real-media protocol client from a minimal owned fixture context."""
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
for package in ['protocol', 'nodecache', 'mediaauthority']:
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
print(json.dumps({'context': str(context)}), flush=True)
raise SystemExit(subprocess.run(['rtk', 'docker', 'build', '-t', 'voice-sfu-media-fixture:game-sprint', str(context)], cwd=root).returncode)
