"""Opt-in real-Docker proof of scheduled, explicitly owned fixtures."""
import json
import os
from pathlib import Path
import subprocess
import sys
import uuid

from local_docker_owned import DockerCLI, OwnedRun, OwnershipError
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


def crash_worker(checkout, slots, image):
    owner = ScheduledRun.start(checkout, slots, DockerCLI(), 0)
    print('OWNER ' + json.dumps({'directory': str(owner.run.directory), 'slot': owner.slot}), flush=True)
    try:
        fixture(owner.run, image)
        verify_live(owner.run)
        print('READY', flush=True)
        sys.stdin.readline()
    finally:
        owner.close()


def verify_host_slots(checkout, engine, image):
    """Exercise the actual host location with two checkouts and a sentinel."""
    first_dir, second_dir, sentinel_dir = (checkout / name for name in ('host-one', 'host-two', 'host-sentinel'))
    for directory in (first_dir, second_dir, sentinel_dir):
        directory.mkdir()
    first = second = sentinel = None
    records = []
    try:
        first = ScheduledRun.start_host(first_dir, engine, 0)
        second = ScheduledRun.start_host(second_dir, engine, 0)
        sentinel = OwnedRun.create(sentinel_dir, engine)
        for run in (first.run, second.run, sentinel):
            fixture(run, image)
            verify_live(run)
        records = [(item['kind'], item['id']) for run in (first.run, second.run, sentinel)
                   for item in run.state['resources']]
        try:
            unexpected = ScheduledRun.start_host(sentinel_dir, engine, 0)
        except TimeoutError:
            pass
        else:
            unexpected.close()
            raise RuntimeError('third host-wide fixture acquired an occupied slot')
        first.close()
        first = None
        verify_live(second.run)
        verify_live(sentinel)
    finally:
        errors = []
        for owner in (first, second):
            if owner is not None:
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
            raise RuntimeError('host fixture cleanup requires reconciliation') from errors[0]
    if any(engine.exists(kind, identifier) for kind, identifier in records):
        raise RuntimeError('host-wide fixture left owned resources')
    print('host-wide scheduler preserved the other checkout and sentinel; journals:', checkout)


def verify_lost_create_response(checkout, engine, image):
    """Recover a real created volume without touching a live peer or sentinel."""
    class LostResponse:
        def __init__(self, underlying):
            self.underlying = underlying

        def __getattr__(self, name):
            return getattr(self.underlying, name)

        def create(self, kind, name, labels):
            self.underlying.create(kind, name, labels)
            raise OwnershipError('simulated lost create response')

    slots = checkout / 'lost-create-slots'
    first = second = sentinel = None
    directory = slot = None
    recovered = False
    try:
        sentinel = OwnedRun.create(checkout, engine)
        fixture(sentinel, image)
        second = ScheduledRun.start(checkout, slots, engine, 0)
        fixture(second.run, image)
        first = ScheduledRun.start(checkout, slots, LostResponse(engine), 0)
        try:
            first.run.create_resource('volume')
        except OwnershipError:
            pass
        else:
            raise RuntimeError('lost response was not simulated')
        item = first.run.state['resources'][-1]
        if item['kind'] != 'volume' or item['state'] != 'creating' or item['id']:
            raise RuntimeError('create intent was not retained')
        name, directory, slot = item['name'], first.run.directory, first.slot
        first.abandon()
        first = None
        ScheduledRun.recover(directory, slots, engine, slot)
        recovered = True
        if engine.exists('volume', name):
            raise RuntimeError('recovered volume remains')
        verify_live(second.run)
        verify_live(sentinel)
        print('lost create response recovered; other fixture and sentinel remained live')
    finally:
        errors = []
        if first is not None:
            try:
                first.close()
            except Exception as error:
                errors.append(error)
        if directory is not None and not recovered:
            try:
                ScheduledRun.recover(directory, slots, engine, slot)
            except Exception as error:
                errors.append(error)
        if second is not None:
            try:
                second.close()
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
            raise RuntimeError('lost-create fixture cleanup requires reconciliation') from errors[0]


def main():
    if os.environ.get('VOICE_LOCAL_DOCKER_TEST') != '1':
        print('VOICE_LOCAL_DOCKER_TEST=1 is required', file=sys.stderr)
        return 2
    if len(sys.argv) == 5 and sys.argv[1] == '--worker':
        crash_worker(Path(sys.argv[2]), Path(sys.argv[3]), sys.argv[4])
        return 0
    if len(sys.argv) != 1:
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
        with (checkout / 'worker.log').open('wb') as log:
            child = subprocess.Popen([sys.executable, '-u', __file__, '--worker', str(checkout), str(slots), image],
                                     stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=log, text=True)
            locator = None
            directory = None
            recovered = False
            try:
                line = child.stdout.readline().strip()
                if not line.startswith('OWNER '):
                    raise RuntimeError('worker failed before registering ownership')
                locator = json.loads(line.removeprefix('OWNER '))
                candidate = Path(locator['directory']).resolve()
                if candidate.parent != (checkout / 'tmp/ci-local').resolve() or type(locator['slot']) is not int:
                    raise RuntimeError('worker returned an invalid recovery identity')
                directory = candidate
                if child.stdout.readline().strip() != 'READY':
                    raise RuntimeError('worker did not reach a settled fixture')
                child.kill()
                child.wait(timeout=15)
                orphan = OwnedRun.load(directory, engine)
                try:
                    verify_live(orphan)
                    records = orphan.state['resources'][:]
                finally:
                    orphan.close()
                verify_live(owners[1].run)
                verify_live(sentinel)
                ScheduledRun.recover(directory, slots, engine, locator['slot'])
                recovered = True
                if any(engine.exists(item['kind'], item['id']) for item in records):
                    raise RuntimeError('recovered worker left resources behind')
                verify_live(owners[1].run)
                verify_live(sentinel)
                replacement = ScheduledRun.start(checkout, slots, engine, 0)
                replacement.close()
                print('crashed journal-bound worker recovered; other fixture and sentinel remained live')
            finally:
                if child.poll() is None:
                    child.kill()
                    child.wait(timeout=15)
                child.stdin.close()
                child.stdout.close()
                if directory is not None and not recovered:
                    ScheduledRun.recover(directory, slots, engine, locator['slot'])
        limited = ScheduledRun.start(checkout, checkout / 'one-slot', engine, 0, capacity=1)
        try:
            fixture(limited.run, image)
            try:
                unexpected = ScheduledRun.start(checkout, checkout / 'one-slot', engine, 0, capacity=1)
            except TimeoutError:
                pass
            else:
                unexpected.close()
                raise RuntimeError('one-slot capacity allowed a second fixture')
            verify_live(owners[1].run)
            verify_live(sentinel)
        finally:
            limited.close()
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
    verify_host_slots(checkout, engine, image)
    verify_lost_create_response(checkout, engine, image)
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
