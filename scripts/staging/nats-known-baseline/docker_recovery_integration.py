"""Owned Linux-only reconstruction test; disposable fixture JWTs, no staging."""
import json
import os
from pathlib import Path
import shutil
import uuid

from controller import file_sha
from docker_runtime import DockerRuntime
from scenario import fixture, census, zero, ready, recover_staging_baseline


def cleanup(runtime):
    for name in reversed(runtime.owned):
        runtime.inspect(name);runtime.run(['rm','-f',name])


if __name__=='__main__':
    prepared=Path('/var/lib/docker/volumes/voice-known-test-20261004/_data')
    operation='test'+uuid.uuid4().hex[:12]
    base=prepared/operation;base.mkdir(mode=0o700)
    (base/'inputs').mkdir(mode=0o750);os.chown(base/'inputs',0,65532)
    for role in ('bootstrap','social','realtime'):
        path=base/'inputs'/(role+'.creds');shutil.copyfile(prepared/'inputs'/(role+'.creds'),path)
        path.chmod(0o440);os.chown(path,0,65532)
    for name in ('server.conf','kernel','bootstrap-realtime.sh','bootstrap-notification.sh','bootstrap-analytics-chat.sh','bootstrap-search.sh'):
        path=base/name;shutil.copyfile(prepared/name,path)
        path.chmod(0o555 if name=='kernel' else 0o440);os.chown(path,0,65532)
    runtime=DockerRuntime(base,operation)
    try:
        fixture_receipt=fixture(runtime)
    finally:
        cleanup(runtime)
    store=base/'existing-final-store';store.mkdir(mode=0o700);os.chown(store,65532,65532)
    runtime=DockerRuntime(base,operation)
    try:
        broker=runtime.start_broker('initial-baseline',store);ready(runtime,broker)
        runtime.bootstrap(broker)
        assert zero(census(runtime,broker))
        runtime.stop(broker)
    finally:
        cleanup(runtime)
    old=base/'out-5';old.mkdir(mode=0o700);(old/'proof-sentinel').write_bytes(b'previous-proof')
    before=file_sha(old/'proof-sentinel')
    manifest=json.loads((base/'fixture-manifest.json').read_bytes())
    runtime=DockerRuntime(base,operation)
    try:
        runtime.no_operation_containers()
        receipt=recover_staging_baseline(runtime,store,manifest,lambda:runtime.no_operation_containers(running_only=True))
        assert receipt['messages']==0 and receipt['restore_verified'] and receipt['streams']==15 and receipt['consumers']==42
        assert file_sha(old/'proof-sentinel')==before
        assert receipt['archive_sha256']!=fixture_receipt['archive_sha256']
        print(json.dumps({'schema':'known-nats-reconstructed-runtime-test-v1','operation':operation,
                         'existing_store_no_bootstrap':True,'old_proof_preserved':True,
                         'fixture':fixture_receipt,'zero_baseline':receipt},sort_keys=True))
    finally:
        cleanup(runtime)
