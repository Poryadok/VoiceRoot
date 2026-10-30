"""Run planner-selected Go service short-test diagnostics; never issue PASS."""
import json
import os
from pathlib import Path
import re
import shutil
import subprocess

from local_docker_owned import DockerCLI, OwnershipError
from local_docker_scheduled import ScheduledRun
import local_go_dependencies
import local_go_execution
import local_source_snapshot


GO_IMAGE = 'golang@sha256:23ce71337b726608afb1bc00986e630a05034a87a87ea1fc2cd1ae5cf33387eb'
GO_VERSION = 'go1.26.10'


def _check_go_toolchain(go_binary):
    executable = shutil.which(go_binary)
    if not executable:
        raise ValueError('Go executable is unavailable; provide --go with ' + GO_VERSION)
    environment = {key: value for key, value in os.environ.items() if not key.upper().startswith('GO')}
    environment.update(GOENV='off', GOTOOLCHAIN='local')
    result = subprocess.run([executable, 'version'], env=environment, capture_output=True, timeout=30, check=False)
    version = result.stdout.decode('utf-8', errors='replace').strip()
    if result.returncode != 0 or not re.fullmatch(r'go version ' + GO_VERSION.replace('.', r'\.') + r' [a-z0-9]+/[a-z0-9]+', version):
        raise ValueError('Go executable must be ' + GO_VERSION + ' to match the pinned image; provide --go')
    return executable


def run(root, source, services, output, go_binary='go'):
    """Run the complete short-test inventory for each selected service.

    This does not run the non-short shared pkg job, integrations, race, lint,
    or image checks, and cannot authorize a PR handoff.
    """
    root, output = Path(root).resolve(), Path(output).resolve()
    if (not isinstance(services, list) or not services or
            any(not isinstance(name, str) or not re.fullmatch(r'[a-z][a-z0-9]*', name) for name in services) or
            len(set(services)) != len(services)):
        raise ValueError('invalid selected Go services')
    if output.exists() or output == root or root in output.parents:
        raise ValueError('Go diagnostic output must be new and outside the source tree')
    if local_source_snapshot.capture(root, source['head']) != source:
        raise ValueError('source changed before Go diagnostics')
    output.mkdir(parents=True)
    try:
        engine = DockerCLI()
        image = engine._call('image', 'inspect', GO_IMAGE, '--format', '{{.Id}}')
    except OwnershipError as error:
        raise OwnershipError('Cannot inspect the pinned Go image; ensure Docker is reachable and pull ' + GO_IMAGE) from error
    go_binary = _check_go_toolchain(go_binary)
    source_copy = local_source_snapshot.materialize(root, source, output / 'source')
    results = {}
    for service in services:
        module_dir = 'src/backend/' + service
        directory = output / service
        directory.mkdir()
        scheduled = ScheduledRun.start_host(directory, engine, 300)
        try:
            owner = scheduled.run
            network = owner.create_resource('network')
            volume = owner.create_resource('volume')
            name = owner.create_container(image, network, {'/workspace': volume}, [], command=['sleep', 'infinity'])
            owner.start_container(name)
            acquisition = local_go_dependencies.acquire(root, source, source_copy, module_dir,
                                                         directory / 'dependencies', go_binary)
            source_digest = local_go_execution.transfer_source(owner, name, root, source, source_copy,
                                                                directory / 'source.tar')
            transfer = local_go_execution.transfer_dependencies(owner, name, root, source, source_copy,
                source_digest, acquisition, directory / 'dependencies/download-cache.tar')
            inventory = local_go_execution.discover_packages(owner, name, root, source, source_copy,
                                                             source_digest, module_dir, directory / 'inventory')
            packages = sorted(inventory['packages_with_tests'] + inventory['packages_without_tests'])
            diagnostic = local_go_execution.run_package_diagnostic(owner, name, root, source, source_copy,
                source_digest, module_dir, packages, directory / 'tests', inventory['packages_without_tests'])
            results[service] = {'acquisition': acquisition, 'transfer': transfer,
                                'source_archive_sha256': source_digest, 'slot': scheduled.slot,
                                'ownership_journal': str((owner.directory / 'ownership.json').relative_to(output)),
                                'inventory': inventory, 'diagnostic': diagnostic}
        finally:
            scheduled.close()
    if local_source_snapshot.capture(root, source['head']) != source:
        raise ValueError('source changed during Go diagnostics')
    result = {'schema': 'voice-local-go-service-diagnostic-v1', 'status': 'incomplete',
              'source': source, 'image': {'reference': GO_IMAGE, 'id': image}, 'services': results}
    (output / 'diagnostic.json').write_text(json.dumps(result, sort_keys=True, indent=2) + '\n', encoding='utf-8')
    return result
