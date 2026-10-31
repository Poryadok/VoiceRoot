"""Opt-in controlled-container crash and owned cleanup proof."""
import os
from pathlib import Path
import subprocess
import sys
import time
import uuid

from local_docker_owned import DockerCLI, OwnedRun, OwnershipError


def start_fixture(owner, image):
    network = owner.create_resource('network')
    volume = owner.create_resource('volume')
    container = owner.create_container(image, network, {'/data': volume}, [])
    owner.start_container(container)


def ping(owner, engine):
    record = next(item for item in owner.state['resources'] if item['kind'] == 'container')
    deadline = time.monotonic() + 20
    while True:
        try:
            if (owner._inspect(record).get('running') and
                    engine._call('container', 'exec', record['id'], 'redis-cli', 'ping') == 'PONG'):
                owner._inspect(record)
                return
        except OwnershipError:
            pass
        if time.monotonic() >= deadline:
            raise RuntimeError('owned Redis unavailable')
        time.sleep(0.2)


def worker(checkout, image):
    engine = DockerCLI()
    owner = OwnedRun.create(checkout, engine)
    print('OWNER ' + str(owner.directory), flush=True)
    try:
        start_fixture(owner, image)
        ping(owner, engine)
        print('READY', flush=True)
        if sys.stdin.readline().strip() == 'fail':
            raise RuntimeError('intentional fixture failure')
    finally:
        try:
            owner.cleanup()
        finally:
            owner.close()


def main():
    if os.environ.get('VOICE_LOCAL_DOCKER_TEST') != '1':
        print('VOICE_LOCAL_DOCKER_TEST=1 is required for owned Docker proof', file=sys.stderr)
        return 2
    if len(sys.argv) == 4 and sys.argv[1] == '--worker':
        worker(Path(sys.argv[2]), sys.argv[3])
        return 0
    if len(sys.argv) != 1:
        return 2
    root = Path(__file__).resolve().parents[2]
    checkout = root / 'tmp/ci-owned-proof' / ('cancel-' + uuid.uuid4().hex)
    checkout.mkdir(parents=True, exist_ok=False)
    engine = DockerCLI()
    image = engine._call('image', 'inspect', 'redis:7-alpine', '--format', '{{.Id}}')
    owners = []
    child = None
    child_directory = None
    child_owner = None
    failed = False
    log = (checkout / 'worker.log').open('wb')
    try:
        for _ in range(2):
            owner = OwnedRun.create(checkout, engine)
            owners.append(owner)
            start_fixture(owner, image)
            ping(owner, engine)
        def start_worker():
            nonlocal child, child_directory
            child = subprocess.Popen([sys.executable, '-u', __file__, '--worker', str(checkout), image],
                                     stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=log, text=True)
            line = child.stdout.readline().strip()
            if not line.startswith('OWNER '):
                raise RuntimeError('worker failed before registering ownership')
            candidate = Path(line.removeprefix('OWNER ')).resolve()
            if candidate.parent != (checkout / 'tmp/ci-local').resolve():
                raise RuntimeError('worker ownership is outside the probe directory')
            child_directory = candidate
            if child.stdout.readline().strip() != 'READY':
                raise RuntimeError('worker did not reach a settled fixture')

        start_worker()
        child.stdin.write('fail\n')
        child.stdin.flush()
        child.wait(timeout=60)
        if child.returncode == 0:
            raise RuntimeError('intentional failure was not reported')
        completed = OwnedRun.load(child_directory, engine)
        try:
            if any(item['state'] != 'removed' for item in completed.state['resources']):
                raise RuntimeError('failed worker did not finish its owned cleanup')
        finally:
            completed.close()
        child.stdin.close()
        child.stdout.close()
        child, child_directory = None, None
        for owner in owners:
            ping(owner, engine)

        start_worker()
        child.kill()
        child.wait(timeout=15)
        if child.returncode == 0:
            raise RuntimeError('worker did not terminate abnormally')
        child_owner = OwnedRun.load(child_directory, engine)
        ping(child_owner, engine)
        child_owner.cleanup()
        for owner in owners:
            for record in owner.state['resources']:
                owner._inspect(record)
            ping(owner, engine)
        print('failed and crashed worker cleanup preserved the other run and sentinel')
    finally:
        if child is not None:
            if child.poll() is None:
                child.kill()
                child.wait(timeout=15)
            child.stdin.close()
            child.stdout.close()
        if child_owner is None and child_directory is not None:
            try:
                child_owner = OwnedRun.load(child_directory, engine)
            except Exception as error:
                failed = True
                print('worker ownership requires reconciliation:', type(error).__name__, file=sys.stderr)
        if child_owner is not None:
            try:
                child_owner.cleanup()
            except Exception as error:
                failed = True
                print('worker cleanup requires reconciliation:', type(error).__name__, file=sys.stderr)
            finally:
                child_owner.close()
        for owner in reversed(owners):
            try:
                owner.cleanup()
            except Exception as error:
                failed = True
                print('owned cleanup requires reconciliation:', owner.run_id, type(error).__name__, file=sys.stderr)
            finally:
                owner.close()
        log.close()
        print('ownership journals:', checkout)
    return 1 if failed else 0


if __name__ == '__main__':
    raise SystemExit(main())
