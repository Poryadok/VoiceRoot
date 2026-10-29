"""Opt-in real-Docker proof of scheduled, explicitly owned fixtures."""
import os
from pathlib import Path
import sys
import uuid

from local_docker_owned import DockerCLI, OwnedRun
from local_docker_scheduled import ScheduledRun


def fixture(run, image):
    network = run.create_resource('network')
    volume = run.create_resource('volume')
    container = run.create_container(image, network, {'/data': volume}, [])
    run.start_container(container)
    return run.state['resources'][:]


def verify_live(run):
    for item in run.state['resources']:
        observed = run._inspect(item)
        if item['kind'] == 'container' and not observed.get('running'):
            raise RuntimeError('owned fixture container stopped')


def main():
    if os.environ.get('VOICE_LOCAL_DOCKER_TEST') != '1':
        print('VOICE_LOCAL_DOCKER_TEST=1 is required', file=sys.stderr)
        return 2
    root = Path(__file__).resolve().parents[2]
    checkout = root / 'tmp/ci-owned-proof' / ('scheduled-' + uuid.uuid4().hex)
    checkout.mkdir(parents=True, exist_ok=False)
    slots = checkout / 'slots'
    engine = DockerCLI()
    image = engine._call('image', 'inspect', 'redis:7-alpine', '--format', '{{.Id}}')
    owners = []
    sentinel = None
    try:
        sentinel = OwnedRun.create(checkout, engine)
        fixture(sentinel, image)
        for _ in range(2):
            owner = ScheduledRun.start(checkout, slots, engine, 0)
            owners.append(owner)
            fixture(owner.run, image)
        for owner in owners:
            verify_live(owner.run)
        verify_live(sentinel)
        try:
            unexpected = ScheduledRun.start(checkout, slots, engine, 0)
        except TimeoutError:
            pass
        else:
            unexpected.close()
            raise RuntimeError('third heavy fixture acquired an occupied slot')
        first_records = owners[0].run.state['resources'][:]
        owners[0].close()
        if any(engine.exists(item['kind'], item['id']) for item in first_records):
            raise RuntimeError('first owner left resources behind')
        verify_live(owners[1].run)
        verify_live(sentinel)
        try:
            with ScheduledRun.start(checkout, slots, engine, 0) as run:
                fixture(run, image)
                raise ValueError('intentional body failure')
        except ValueError as error:
            if str(error) != 'intentional body failure':
                raise
        verify_live(owners[1].run)
        verify_live(sentinel)
        print('scheduled cleanup preserved the second fixture and sentinel; failure released its cleaned slot')
        print('ownership journals:', checkout)
    finally:
        errors = []
        for owner in reversed(owners):
            try:
                owner.close()
            except Exception as error:
                errors.append(error)
        if sentinel is not None:
            try:
                sentinel.cleanup()
            except Exception as error:
                errors.append(error)
            finally:
                sentinel.close()
        if errors:
            raise RuntimeError('fixture cleanup requires reconciliation') from errors[0]
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
