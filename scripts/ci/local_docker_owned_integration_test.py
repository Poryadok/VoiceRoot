"""Opt-in Docker proof for controlled owned fixture cleanup."""
import os
from pathlib import Path
import sys
import time
import uuid

from local_docker_owned import DockerCLI, OwnedRun, OwnershipError


def main():
    if os.environ.get('VOICE_LOCAL_DOCKER_TEST') != '1':
        print('VOICE_LOCAL_DOCKER_TEST=1 is required for owned Docker proof', file=sys.stderr)
        return 2
    root = Path(__file__).resolve().parents[2]
    checkout = root / 'tmp/ci-owned-proof' / ('probe-' + uuid.uuid4().hex)
    checkout.mkdir(parents=True, exist_ok=False)
    engine = DockerCLI()
    image = engine._call('image', 'inspect', 'redis:7-alpine', '--format', '{{.Id}}')
    owners = []
    failed = False
    try:
        for _ in range(3):
            owner = OwnedRun.create(checkout, engine)
            owners.append(owner)
            network = owner.create_resource('network')
            volume = owner.create_resource('volume')
            container = owner.create_container(image, network, {'/data': volume}, [])
            owner.start_container(container)
        def ping(owner):
            record = next(item for item in owner.state['resources'] if item['kind'] == 'container')
            deadline = time.monotonic() + 20
            while True:
                try:
                    owner._inspect(record)
                    if engine._call('container', 'exec', record['id'], 'redis-cli', 'ping') == 'PONG':
                        owner._inspect(record)
                        return
                except OwnershipError:
                    pass
                if time.monotonic() >= deadline:
                    raise RuntimeError('owned Redis did not respond inside its container')
                time.sleep(0.2)
        for owner in owners:
            ping(owner)
        first, second, sentinel = owners
        first.cleanup()
        for owner in (second, sentinel):
            for record in owner.state['resources']:
                owner._inspect(record)
                if record['kind'] == 'container' and not owner._inspect(record).get('running'):
                    raise RuntimeError('other owned container stopped')
            ping(owner)
        print('first run cleanup preserved the other run and sentinel containers, networks and volumes')
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
