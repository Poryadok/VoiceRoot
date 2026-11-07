"""Hosted-only scoped enrollment and private NATS seed qualification.

This does not create a P2 Session, select a PVC, or authorize a rollout.
The caller retains the private workspace whenever any operation is uncertain.
"""
import hashlib
import json
import os
from pathlib import Path
import sys

import p2_seeded_fixture as fixture


def main(bundle, workspace):
    fixture.require_hosted()
    import guard
    bundle=Path(bundle);workspace=Path(workspace)
    fixture._root_directory(bundle)
    fixture._root_directory(workspace,private=True)
    base=workspace/'seed-runtime'
    base.mkdir(mode=0o700)
    # Never take over a previous run's root or its scratch mounts.
    Path(guard.ROOT).mkdir(mode=0o700)
    source=bundle/'source'
    roles=('realtime','notification','analytics-chat','search')
    paths={'bundle/'+name:bundle/name for name in ('helper','kernel','bootstrap-renewer','nats-9ae-predecessor.json')}
    paths.update({'source/'+role:source/'scripts/staging/nats-known-baseline/testdata/deployed-bootstrap-20261004'/(role+'-bootstrap.sh') for role in roles})
    pins={name:hashlib.sha256(path.read_bytes()).hexdigest() for name,path in paths.items()}
    # Pins bind this run's exact source/build; they are not external provenance.
    retained=[]
    def checkpoint(handle):
        retained[:] = [handle]
        public={'base':str(handle.base),'operations':[runtime.operation for runtime in handle.runtimes]}
        pending=workspace/'retained.tmp';target=workspace/'retained.json'
        fd=os.open(pending,os.O_WRONLY|os.O_CREAT|os.O_TRUNC|os.O_NOFOLLOW,0o600)
        with os.fdopen(fd,'w') as out:
            json.dump(public,out,sort_keys=True);out.flush();os.fsync(out.fileno())
        os.replace(pending,target)
    handle=fixture.prepare_owned_identity(base,bundle_root=bundle,source_root=source,
        input_sha256=pins,on_retained=checkpoint)
    observation=fixture.prepare_private_seed(handle)
    handle.verify_identity()
    snapshot=observation['census']
    if sum(row['state']['messages'] for row in snapshot['streams'])!=9 or len(snapshot['consumers'])!=45:
        raise RuntimeError('owned_fixture_census_invalid')
    # Every create in the successful sequential path returned and was recorded;
    # a failure or ambiguous reply never reaches this reconciliation callback.
    fixture.reconcile_containers(handle,lambda current: current is handle)
    Path(guard.ROOT).rmdir()
    print('OWNED_SEED_ENROLLMENT=PASS')


if __name__=='__main__':
    if len(sys.argv)!=3:raise SystemExit('expected bundle and retained workspace')
    main(*sys.argv[1:])
