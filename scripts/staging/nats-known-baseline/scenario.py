"""Owned fixture and actual new-staging-PVC closed backup verification."""
import json
import os
from pathlib import Path
import shutil
import time

from controller import Blocked, verify_archive


def ready(runtime, broker):
    for attempt in range(20):
        try:
            runtime.inspect(broker)
            runtime.run(['exec',broker,'/bin/busybox','wget','-q','-O','-',
                         'http://127.0.0.1:8222/healthz'],timeout=2)
            return
        except Blocked:
            if attempt==19: raise
            time.sleep(0.1)


def census(runtime, broker):
    result=runtime.kernel(broker,'census')/'census.json'
    row=json.loads(result.read_bytes())
    if row.get('schema')!='known-nats-census-v1' or len(row['streams'])!=15 or len(row['consumers'])!=42:
        raise Blocked('canonical_census_failed')
    return row


def zero(row):
    return (all(s['messages']==0 and s['last_seq']==0 for s in row['streams']) and
            all(all(c[k]==0 for k in ('ack_consumer','ack_stream','delivered_consumer','delivered_stream','ack_pending','pending')) for c in row['consumers']))


def closed_backup(runtime, broker, store, label, before, messages):
    runtime.stop(broker)
    archive=runtime.base/(label+'.tar')
    manifest=runtime.cold_archive(broker,store,archive)
    manifest_path=runtime.base/(label+'-manifest.json')
    manifest_path.write_text(json.dumps(manifest,sort_keys=True)); manifest_path.chmod(0o600)
    separate=runtime.base/(label+'-node-copy.tar')
    shutil.copyfile(archive,separate); separate.chmod(0o600)
    if not verify_archive(separate,manifest): raise Blocked('node_copy_failed')
    target=runtime.base/(label+'-restored')
    runtime.restore(separate,target,manifest)
    restored=runtime.start_broker(label+'-restore',target); ready(runtime,restored)
    after=census(runtime,restored)
    if before!=after: raise Blocked('restored_census_changed')
    return restored, {'messages':messages,'streams':15,'consumers':42,
        'archive':str(archive),'node_copy':str(separate),'manifest':str(manifest_path),
        'archive_sha256':manifest['archive_sha256'],'restore_verified':True,
        'node_copy_verified':True,'off_node_copy_verified':False,
        'mechanism':manifest['mechanism'],'capture':'broker-stopped-before-read'}


def fixture(runtime):
    store=runtime.base/'fixture-store'; store.mkdir(mode=0o700); os.chown(store,65532,65532)
    broker=runtime.start_broker('fixture',store); ready(runtime,broker)
    runtime.bootstrap(broker)
    initial=census(runtime,broker)
    if not zero(initial): raise Blocked('fixture_not_clean')
    seeded=runtime.kernel(broker,'seed')/'fixture.json'
    snapshot=runtime.base/'inputs'/'fixture.json'
    snapshot.write_bytes(seeded.read_bytes()); snapshot.chmod(0o440); os.chown(snapshot,0,65532)
    before=census(runtime,broker)
    restored, receipt=closed_backup(runtime,broker,store,'fixture',before,3)
    runtime.kernel(restored,'verify-closed'); runtime.kernel(restored,'drain')
    runtime.stop(restored); runtime.restart(restored); ready(runtime,restored)
    runtime.kernel(restored,'verify-drained'); runtime.stop(restored)
    receipt['record_ledger']=json.loads(snapshot.read_bytes())
    return receipt


def staging_baseline(runtime, store):
    store=Path(store).resolve(strict=True)
    if store.is_relative_to(runtime.base):
        # Local integration uses a fresh directory in its owned test volume.
        # Production receives only the UID-bound path from Staging.new_claim.
        if any(store.iterdir()): raise Blocked('owned_baseline_not_empty')
        os.chmod(store,0o700); os.chown(store,65532,65532)
    else:
        runtime.allow_new_store(store)
    broker=runtime.start_broker('final-baseline',store); ready(runtime,broker)
    runtime.bootstrap(broker)
    before=census(runtime,broker)
    if not zero(before): raise Blocked('staging_baseline_not_zero')
    restored,receipt=closed_backup(runtime,broker,store,'staging-baseline',before,0)
    runtime.stop(restored)
    receipt['closed_census']=before
    return receipt
