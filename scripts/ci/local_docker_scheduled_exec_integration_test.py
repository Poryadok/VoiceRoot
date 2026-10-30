"""Opt-in process-kill proof for an owned, journalled Go test execution."""
from pathlib import Path
import json
import io
import os
import subprocess
import sys
import tempfile
import tarfile
import time
import uuid

ROOT = Path(__file__).resolve().parents[2]
GO_IMAGE = 'golang@sha256:23ce71337b726608afb1bc00986e630a05034a87a87ea1fc2cd1ae5cf33387eb'
sys.path.insert(0, str(ROOT / 'scripts/ci'))
from local_docker_owned import DockerCLI, OwnedRun
from local_docker_scheduled import ScheduledRun
import local_go_execution
import local_source_snapshot


def setup(owner, image):
    net = owner.create_resource('network')
    volume = owner.create_resource('volume')
    name = owner.create_container(image, net, {'/workspace': volume}, [], command=['sleep', 'infinity'])
    owner.start_container(name)
    return name


def worker(base, slots, image):
    scheduled = ScheduledRun.start(base, slots, DockerCLI(), 0)
    print('OWNER ' + json.dumps({'directory': str(scheduled.run.directory), 'slot': scheduled.slot}), flush=True)
    name = setup(scheduled.run, image)
    with (base / 'go.stdout').open('wb') as out, (base / 'go.stderr').open('wb') as err:
        files = {'go.mod': b'module probe\n\ngo 1.26\n',
                 'probe_test.go': b'package probe\nimport ("testing";"time")\nfunc TestLong(t *testing.T){time.Sleep(2*time.Minute)}\n'}
        with tempfile.TemporaryFile() as archive:
            with tarfile.open(fileobj=archive, mode='w') as tar:
                for filename, data in files.items():
                    info = tarfile.TarInfo(filename)
                    info.size = len(data)
                    info.mode = 0o644
                    tar.addfile(info, io.BytesIO(data))
            archive.seek(0)
            if scheduled.run.execute(name, ['tar', '-x', '-C', '/workspace'], out, err, 30, stdin=archive) != 0:
                raise RuntimeError('Go fixture preparation failed')
        print('READY', flush=True)
        scheduled.run.execute(name, ['sh', '-c', 'cd /workspace && GOMAXPROCS=2 go test -v'], out, err, 180)


def main():
    if os.environ.get('VOICE_LOCAL_DOCKER_TEST') != '1':
        raise RuntimeError('VOICE_LOCAL_DOCKER_TEST=1 is required')
    if len(sys.argv) == 5 and sys.argv[1] == '--worker':
        worker(Path(sys.argv[2]), Path(sys.argv[3]), sys.argv[4])
        return
    base = ROOT / 'tmp/ci-owned-proof' / ('killed-go-' + uuid.uuid4().hex)
    base.mkdir(parents=True)
    slots = base / 'slots'
    engine = DockerCLI()
    image = engine._call('image', 'inspect', GO_IMAGE, '--format', '{{.Id}}')
    other = ScheduledRun.start(base, slots, engine, 0)
    sentinel = OwnedRun.create(base, engine)
    child = None
    locator = None
    directory = None
    recovered = False
    try:
        other_name = setup(other.run, image)
        setup(sentinel, image)
        with (base / 'worker.log').open('wb') as log, (base / 'worker.stdout').open('wb') as worker_out:
            child = subprocess.Popen([sys.executable, '-u', __file__, '--worker', str(base), str(slots), image],
                                     stdout=worker_out, stderr=log)
            def wait_worker_line(prefix):
                deadline = time.monotonic() + 120
                while time.monotonic() < deadline:
                    for line in (base / 'worker.stdout').read_text(encoding='utf-8').splitlines():
                        if line.startswith(prefix):
                            return line
                    if child.poll() is not None:
                        raise RuntimeError('worker exited before ' + prefix)
                    time.sleep(.1)
                raise RuntimeError('worker timed out before ' + prefix)
            locator = json.loads(wait_worker_line('OWNER ')[6:])
            candidate = Path(locator['directory']).resolve()
            if candidate.parent != (base / 'tmp/ci-local').resolve() or type(locator['slot']) is not int:
                raise RuntimeError('invalid worker identity')
            directory = candidate
            wait_worker_line('READY')
            deadline = time.monotonic() + 120
            while time.monotonic() < deadline:
                state = json.loads((directory / 'ownership.json').read_text())
                if (any(x['state'] == 'executing' for x in state['resources']) and
                        b'=== RUN   TestLong' in (base / 'go.stdout').read_bytes()):
                    break
                if child.poll() is not None:
                    raise RuntimeError('worker exited before test execution')
                time.sleep(.1)
            else:
                raise RuntimeError('Go execution was not journalled')
            child.kill()
            child.wait(timeout=15)
            records = [(x['kind'], x['id']) for x in state['resources']]
            ScheduledRun.recover(directory, slots, engine, locator['slot'])
            recovered = True
            if any(engine.exists(kind, identity) for kind, identity in records):
                raise RuntimeError('recovered run left owned resources')
            with tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
                if other.run.execute(other_name, ['go', 'version'], out, err, 30) != 0:
                    raise RuntimeError('other run cannot execute after recovery')
            fixture = base / 'source-repository'
            fixture.mkdir()
            def git(*args):
                return subprocess.check_output(['git', '-C', str(fixture), *args], stderr=subprocess.PIPE)
            git('init', '-q')
            git('config', 'user.name', 'CI Fixture')
            git('config', 'user.email', 'ci-fixture@example.invalid')
            (fixture / 'module').mkdir()
            (fixture / 'module/go.mod').write_text('module example.test/module\n\ngo 1.26\n', encoding='utf-8')
            (fixture / 'module/probe_test.go').write_text(
                'package probe\nimport "testing"\nfunc TestOwnedGoDiagnostic(t *testing.T) {}\n', encoding='utf-8')
            (fixture / 'module/no_tests').mkdir()
            (fixture / 'module/no_tests/probe.go').write_text('package no_tests\n', encoding='utf-8')
            git('add', '.')
            git('commit', '-qm', 'fixture')
            head = git('rev-parse', 'HEAD').decode('ascii').strip()
            (fixture / 'module/dirty.txt').write_bytes(b'captured dirty source\r\n')
            source = local_source_snapshot.capture(fixture, head)
            source_copy = local_source_snapshot.materialize(fixture, source, base / 'source-copy')
            digest = local_go_execution.transfer_source(other.run, other_name, fixture, source, source_copy,
                                                        base / 'source.tar')
            inventory = local_go_execution.discover_packages(other.run, other_name, fixture, source, source_copy,
                digest, 'module', base / 'go-discovery')
            if (inventory['packages_with_tests'] != ['example.test/module'] or
                    inventory['packages_without_tests'] != ['example.test/module/no_tests']):
                raise RuntimeError('Go discovery missed a fixture package')
            report = local_go_execution.run_package_diagnostic(other.run, other_name, fixture, source, source_copy,
                digest, 'module', inventory['packages_with_tests'] + inventory['packages_without_tests'],
                base / 'go-diagnostic', inventory['packages_without_tests'])
            if report['status'] != 'incomplete' or report['tests']['passed'] != 1 or report['tests']['skipped']:
                raise RuntimeError('unexpected owned Go diagnostic result')
            with tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
                if other.run.execute(other_name, ['touch', '/workspace/source/module/injected.go'], out, err, 30) != 0:
                    raise RuntimeError('cannot inject extra source')
            try:
                local_go_execution._verify_workspace(other.run, other_name, fixture, source, digest)
            except ValueError as error:
                if 'workspace changed' not in str(error):
                    raise
            else:
                raise RuntimeError('extra container source was not rejected')
            for item in other.run.state['resources']:
                other.run._inspect(item)
            for item in sentinel.state['resources']:
                sentinel._inspect(item)
            if not all(sentinel._inspect(x)['running'] for x in sentinel.state['resources'] if x['kind'] == 'container'):
                raise RuntimeError('sentinel container stopped during recovery')
            print('killed Go worker recovered; other run and sentinel survived; journals:', base)
    finally:
        if child is not None and child.poll() is None:
            child.kill()
            child.wait(timeout=15)
        try:
            if directory is not None and not recovered:
                ScheduledRun.recover(directory, slots, engine, locator['slot'])
        finally:
            try:
                other.close()
            finally:
                try:
                    sentinel.cleanup()
                finally:
                    sentinel.close()


if __name__ == '__main__':
    main()
