"""Run one incomplete Auth diagnostic under a caller-controlled Docker boundary."""
import argparse
import json
import os
from pathlib import Path
import re

from local_docker_owned import DockerCLI
from local_docker_scheduled import ScheduledRun
import local_auth_private
import local_chat_orchestration as common
import local_source_snapshot as snapshot


def run(root, head, output, helper, helper_digest, archive, archive_digest, image_id, maven_image_id, cache):
    if os.name != 'nt':
        raise ValueError('Windows supervised diagnostic required; Linux is unsupported')
    root, output = Path(root).resolve(), Path(output).resolve()
    if output.exists() or output == root or root in output.parents:
        raise ValueError('output must be a new directory outside source tree')
    for value in (image_id, maven_image_id):
        if not isinstance(value, str) or not re.fullmatch(r'sha256:[0-9a-f]{64}', value):
            raise ValueError('invalid image identity')
    common._verify_file(helper, helper_digest)
    common._verify_file(archive, archive_digest)
    local_auth_private.cache_digest(cache)
    source = snapshot.capture(root, head)
    output.mkdir(parents=True)
    source_copy = snapshot.materialize(root, source, output / 'source')
    checkout = output / 'run'
    checkout.mkdir()
    engine = DockerCLI()
    if engine._call('image', 'inspect', maven_image_id, '--format', '{{.Id}}') != maven_image_id:
        raise ValueError('Maven image identity differs')
    scheduled = ScheduledRun.start_host(checkout, engine, 300)
    owner = scheduled.run
    failure, result = None, None
    try:
        network = owner.create_resource('network')
        mounts = {name: owner.create_resource('volume') for name in
                  ('/home/rootless/.local/share/docker', '/var/lib/docker', '/run/user/1000')}
        workspace = owner.create_resource('volume')
        client = owner.create_container(maven_image_id, network,
            {'/socket': mounts['/run/user/1000'], '/workspace': workspace}, [],
            command=['sleep', 'infinity'], clear_entrypoint=True)
        owner.start_container(client)
        import tempfile
        with tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
            if owner.execute(client, ['chown', '1000:1000', '/socket'], out, err, 30):
                raise ValueError('socket initialization failed')
        daemon = owner.create_rootless_container(network, mounts,
            lambda request: common._create_daemon(helper, helper_digest, engine.rootless_creator_env(), request))
        owner.start_container(daemon)
        common._wait_daemon(owner, daemon, client)
        copied = owner.directory / 'postgres-source.tar'
        with Path(archive).open('rb') as source_file, copied.open('xb') as target:
            for block in iter(lambda: source_file.read(1024 * 1024), b''):
                target.write(block)
            target.flush()
            os.fsync(target.fileno())
        common._verify_file(archive, archive_digest)
        common._verify_file(copied, archive_digest)
        with (output / 'image.stdout').open('xb') as out, (output / 'image.stderr').open('xb') as err:
            owner.load_canonical_rootless_image(daemon, copied, archive_digest, image_id,
                                                'postgres:16-alpine', out, err, 300)
        diagnostic = local_auth_private.run_selected(owner, daemon, client, root, source, source_copy,
                                                     cache, output / 'selected')
        result = {'schema': 'voice-local-auth-private-diagnostic-v1', 'status': 'incomplete',
                  'diagnostic': diagnostic, 'slot': scheduled.slot,
                  'ownership_journal': str((owner.directory / 'ownership.json').relative_to(output)),
                  'inputs': {'maven_image_id': maven_image_id, 'postgres_image_id': image_id,
                             'postgres_archive_sha256': archive_digest, 'creator_sha256': helper_digest}}
    except BaseException as error:
        failure = error
    common._finish(scheduled, failure)
    if snapshot.capture(root, head) != source:
        raise ValueError('source changed during Auth diagnostic cleanup')
    (output / 'diagnostic.json').write_text(json.dumps(result, sort_keys=True, indent=2) + '\n', encoding='utf-8')
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ('head', 'output', 'helper', 'helper-sha256', 'postgres-archive',
                 'postgres-archive-sha256', 'postgres-image-id', 'maven-image-id', 'maven-cache'):
        parser.add_argument('--' + name, required=True)
    args = parser.parse_args()
    result = run(Path(__file__).resolve().parents[2], args.head, args.output, args.helper,
                 args.helper_sha256, args.postgres_archive, args.postgres_archive_sha256,
                 args.postgres_image_id, args.maven_image_id, args.maven_cache)
    print(json.dumps(result, sort_keys=True))
    return 2


if __name__ == '__main__':
    raise SystemExit(main())
