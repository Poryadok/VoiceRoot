"""Nonreset cold-cut proof: the selected staging store is never broker-mounted.

The caller owns the UID-bound physical fence and captured runtime. All INFO
queries run against restored private copies; no bootstrap, publish or ACK runs
against the selected staging claim. The release controller must retain the
fence until post-apply proof and its target-template checks have succeeded.
"""
import hashlib
import json
from pathlib import Path
import tempfile
import guard
from controller import Blocked
from native_store import archive_closed_store,verify_archive
from scenario import ready
from rollout_census import census, semantic


def canonical(value):
    return hashlib.sha256(json.dumps(value,sort_keys=True,separators=(',',':')).encode()).hexdigest()


def native_inventory(manifest):
    # Deterministic archive timestamps are absent; compare every native byte,
    # file path and directory, including records not delivered by a consumer.
    return {key:manifest[key] for key in ('mechanism','file_count','bytes','dirs','files')}


def isolated_census(runtime, archive, manifest, label):
    if not verify_archive(archive,manifest):raise Blocked('rollout_archive_changed')
    parent=Path(tempfile.mkdtemp(prefix=label+'-',dir=runtime.base))
    target=parent/'store'
    runtime.restore(archive,target,manifest)
    broker=runtime.start_broker(label,target)
    try:
        ready(runtime,broker)
        return census(runtime,broker,runtime.account_id())
    finally:
        runtime.stop(broker)


def capture_cut(runtime, source, verify_fence):
    verify_fence()
    runtime.no_operation_containers(running_only=True)
    archive=runtime.base/'rollout-before.tar'
    manifest=archive_closed_store(source,archive)
    verify_fence()
    row=isolated_census(runtime,archive,manifest,'rollout-before')
    return {'manifest':manifest,'census':row,'census_sha256':canonical(row)}


def verify_post_apply(runtime, source, cut, verify_fence):
    # No cold cut may be replaced by an empty/new claim or silently reseeded.
    verify_fence()
    runtime.no_operation_containers(running_only=True)
    attempt=Path(tempfile.mkdtemp(prefix='post-apply-',dir=runtime.base))
    archive=attempt/'rollout-after.tar'
    manifest=archive_closed_store(source,archive)
    if native_inventory(manifest)!=native_inventory(cut['manifest']):
        raise Blocked('rollout_native_store_changed')
    verify_fence()
    row=isolated_census(runtime,archive,manifest,'rollout-after')
    if canonical(cut['census'])!=cut['census_sha256'] or semantic(row)!=semantic(cut['census']):
        raise Blocked('rollout_populated_census_changed')
    verify_fence()
    return {'archive':str(archive.relative_to(runtime.base)),'archive_sha256':manifest['archive_sha256'],
            'native_files_verified':True,'census_verified':True,
            'messages':sum(s['messages'] for s in row['streams']),
            'census_sha256':cut['census_sha256']}
