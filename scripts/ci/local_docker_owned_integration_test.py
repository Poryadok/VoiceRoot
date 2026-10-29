"""Opt-in Docker proof for owned network and volume cleanup only."""
import os
from pathlib import Path
import sys
import uuid

from local_docker_owned import DockerCLI, OwnedRun


def main():
    if os.environ.get('VOICE_LOCAL_DOCKER_TEST') != '1':
        print('VOICE_LOCAL_DOCKER_TEST=1 is required for owned Docker proof', file=sys.stderr)
        return 2
    root = Path(__file__).resolve().parents[2]
    checkout = root / 'tmp/ci-owned-proof' / ('probe-' + uuid.uuid4().hex)
    checkout.mkdir(parents=True, exist_ok=False)
    engine = DockerCLI()
    owners = []
    failed = False
    try:
        for _ in range(3):
            owner = OwnedRun.create(checkout, engine)
            owners.append(owner)
            owner.create_resource('network')
            owner.create_resource('volume')
        first, second, sentinel = owners
        first.cleanup()
        for owner in (second, sentinel):
            for record in owner.state['resources']:
                owner._inspect(record)
        print('first run cleanup preserved the other run and sentinel resources')
    finally:
        for owner in reversed(owners):
            try:
                owner.cleanup()
            except Exception as error:
                failed = True
                print('owned cleanup requires reconciliation:', owner.run_id, type(error).__name__, file=sys.stderr)
            finally:
                owner.close()
        print('ownership journals:', checkout)
    return 1 if failed else 0


if __name__ == '__main__':
    raise SystemExit(main())
