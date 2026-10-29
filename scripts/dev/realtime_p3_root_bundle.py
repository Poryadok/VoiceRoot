"""Make a reviewable fixed-source human-root P3 issuer launcher."""
import hashlib
import io
import json
from pathlib import Path
import re
import subprocess
import sys
import tarfile

FILES = ('credentials.py', 'realtime_p3_missing_issuer.py', 'realtime_p3_root_export.py', 'reset_credentials.py')


def build(source, output):
    source, output = Path(source), Path(output)
    contents = {}
    for name in FILES:
        path = source / name
        if path.is_symlink():
            raise ValueError('source symlink')
        contents[name] = path.read_bytes()
    template = Path(__file__).with_name('realtime_p3_root_launch.template.sh').read_text(encoding='utf-8')
    return _build(contents, template, output)


def _build(contents, template, output):
    output = Path(output)
    contents = dict(contents)
    contents['capture-manifest.json'] = json.dumps(
        {name: hashlib.sha256(data).hexdigest() for name, data in contents.items()},
        sort_keys=True, separators=(',', ':')).encode()
    buffer = io.BytesIO()
    with tarfile.open(fileobj=buffer, mode='w', format=tarfile.USTAR_FORMAT) as archive:
        for name, data in sorted(contents.items()):
            member = tarfile.TarInfo(name)
            member.size, member.mode = len(data), 0o400
            archive.addfile(member, io.BytesIO(data))
    raw = buffer.getvalue()
    digest = hashlib.sha256(raw).hexdigest()
    launcher = template.replace('__BUNDLE_SHA256__', digest).replace('__BUNDLE_BYTES__', str(len(raw)))
    output.mkdir(parents=True, exist_ok=True)
    (output / 'p3-source.tar').write_bytes(raw)
    (output / 'p3-root-launch.sh').write_text(launcher, encoding='utf-8', newline='\n')
    return digest


def build_git(repository, commit, output):
    """Use exact Git blobs; caller independently authenticates the commit."""
    if not re.fullmatch(r'[a-f0-9]{40}', commit):
        raise ValueError('full source commit required')
    def blob(name):
        result = subprocess.run(['git', '-C', str(repository), 'show', commit + ':scripts/dev/' + name],
                                stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, check=True, timeout=20)
        if not 0 < len(result.stdout) <= 1 << 20:
            raise ValueError('invalid source blob')
        return result.stdout
    contents = {name: blob(name) for name in FILES}
    template = blob('realtime_p3_root_launch.template.sh').decode('utf-8')
    return _build(contents, template, output)


if __name__ == '__main__':
    if len(sys.argv) != 4:
        raise SystemExit('usage: realtime_p3_root_bundle.py GIT_REPOSITORY FULL_COMMIT OUTPUT_DIR')
    print(build_git(sys.argv[1], sys.argv[2], sys.argv[3]))
