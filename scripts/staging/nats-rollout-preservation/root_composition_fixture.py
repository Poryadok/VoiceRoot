"""Private synthetic cluster fixture with source-checked installed helpers."""
import hashlib
import os
from pathlib import Path
import shutil
import tarfile

import staging_prerequisites as prerequisites
from root_main import save


ARCHIVE_SHA256='a3b48489d83965fbd96512d501989551db063dcab6d46943dea413a2428d6a1a'
ARCHIVE_SIZE=38502400
SYNTHETIC_BINARIES={name:('synthetic installed binary: '+name+'\n').encode() for name in
    ('nats-known-baseline/kernel','nats-rollout-preservation/bootstrap-renewer',
     'nats-rollout-preservation/story-current4-probe')}
SYNTHETIC_PINS={name:hashlib.sha256(data).hexdigest() for name,data in SYNTHETIC_BINARIES.items()}


def helper_binding(source, code):
    """Use current source for Python helpers and independently pinned binary bytes."""
    archive_name=os.environ.get('VOICE_TEST_PINNED_ARCHIVE')
    archive=Path(archive_name) if archive_name else None
    if archive is not None:
        raw=archive.read_bytes()
        if len(raw)!=ARCHIVE_SIZE or hashlib.sha256(raw).hexdigest()!=ARCHIVE_SHA256:
            raise ValueError('pinned_fixture_archive_changed')
    code.mkdir(parents=True,mode=0o700)
    members=set(prerequisites.REQUIRED_HELPERS)|{'nats-rollout-preservation/realtime_bootstrap_existing.py'}
    binding={}
    for name in sorted(members):
        path=code/name;path.parent.mkdir(parents=True,exist_ok=True,mode=0o700)
        shutil.copyfile(source/'scripts'/'staging'/name,path);path.chmod(0o600)
        binding[name]=hashlib.sha256(path.read_bytes()).hexdigest()
    binaries=SYNTHETIC_BINARIES
    if archive is not None:
        binaries={}
        with tarfile.open(archive,'r:') as bundle:
            for name in prerequisites.BINARY_PINS:
                matched=[entry for entry in bundle if entry.name==name]
                if len(matched)!=1 or not matched[0].isfile():raise ValueError('pinned_fixture_member_invalid')
                binaries[name]=bundle.extractfile(matched[0]).read()
    for name,wanted in prerequisites.BINARY_PINS.items():
        data=binaries[name]
        if hashlib.sha256(data).hexdigest()!=wanted:raise ValueError('pinned_fixture_member_changed')
        path=code/name;path.parent.mkdir(parents=True,exist_ok=True,mode=0o700);path.write_bytes(data);path.chmod(0o600)
        binding[name]=wanted
    save(code/'capture-manifest.json',binding)
    return binding
