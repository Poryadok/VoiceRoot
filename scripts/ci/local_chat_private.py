"""A source-bound private Chat integration diagnostic; never a preflight pass."""

import go_integration_receipts as receipts
import hashlib
import json
from pathlib import Path
import local_rootless_profile as rootless
import local_source_snapshot as snapshot
import local_go_execution

PACKAGE = 'voice/backend/chat/internal/store'
CASE = 'TestSpaceMembersStore_IsAndList'


def validate_events(raw):
    events = receipts.read_events(raw.decode('utf-8'))
    summary = receipts.validate_test_run(events, [PACKAGE])
    if summary['skipped'] or receipts.verify_required(events, [(PACKAGE, CASE)], 0):
        raise ValueError('required Chat integration test did not pass without skips')
    return summary


def inspect_pair(owner, daemon_name, client_name, *, shared_workspace=False):
    """Inspect a locked journal's private daemon and client at one instant.

    The caller must enforce exclusive daemon, network and socket-volume access
    throughout execution; snapshots and the journal lock cannot provide it.
    """
    owner._validate()
    owner._check_daemon()
    resources = owner.state['resources']
    if type(shared_workspace) is not bool:
        raise ValueError('invalid shared workspace mode')
    profile = 'rootless-workspace-v1' if shared_workspace else 'rootless-unix-v1'
    daemons = [r for r in resources if r['kind'] == 'container' and r['name'] == daemon_name
               and r['state'] == 'ready' and r.get('profile') == profile]
    clients = [r for r in resources if r['kind'] == 'container' and r['name'] == client_name
               and r['state'] == 'ready' and 'command' in r]
    if len(daemons) != 1 or len(clients) != 1:
        raise ValueError('owned daemon and client are required')
    daemon, client = daemons[0], clients[0]
    if (daemon['network'] != client['network'] or set(client['mounts']) != {'/socket', '/workspace'} or
            client['mounts']['/socket'] != daemon['mounts']['/run/user/1000'] or
            (client['mounts']['/workspace'] != daemon['mounts'].get('/workspace') if shared_workspace
             else client['mounts']['/workspace'] in daemon['mounts'].values())):
        raise ValueError('private client socket or workspace differs')
    networks = [r for r in resources if r['kind'] == 'network' and r['id'] == daemon['network']
                and r['state'] == 'ready']
    names = set(daemon['mounts'].values()) | set(client['mounts'].values())
    volumes = [r for r in resources if r['kind'] == 'volume' and r['name'] in names and r['state'] == 'ready']
    if len(networks) != 1 or len(volumes) != len(names):
        raise ValueError('private network or volumes are not ready')
    network = networks[0]
    for item in [network, *volumes, daemon, client]:
        observed = owner._inspect(item)
        if item['kind'] == 'container' and observed.get('running') is not True:
            raise ValueError('private daemon and client must be running')
    def raw(kind, identifier):
        rows = json.loads(owner.engine._call(kind, 'inspect', identifier))
        if not isinstance(rows, list) or len(rows) != 1 or not isinstance(rows[0], dict):
            raise ValueError('invalid Docker inspection')
        return rows[0]
    net = raw('network', network['id'])
    daemon_raw = raw('container', daemon['id'])
    client_raw = raw('container', client['id'])
    labels = {'voice.ci.run': owner.run_id, 'voice.ci.resource': network['nonce'], 'voice.ci.kind': 'network'}
    rootless.verify_owned_network(net, network['id'], network['name'], labels,
                                  {daemon['id']: daemon['name'], client['id']: client['name']})
    address = rootless.verify_runtime_network(daemon_raw, network['id'])
    for item, value in ((daemon, daemon_raw), (client, client_raw)):
        attached = value['NetworkSettings']['Networks']
        if len(attached) != 1:
            raise ValueError('unexpected container network attachment')
        endpoint = next(iter(attached.values()))
        record = net['Containers'][item['id']]
        if (endpoint['NetworkID'] != network['id'] or endpoint['EndpointID'] != record['EndpointID'] or
                endpoint['IPAddress'] != record['IPv4Address'].split('/')[0]):
            raise ValueError('network endpoint identity differs')
    owner._check_daemon()
    return address


def run_selected(owner, daemon_name, client_name, root, source, source_copy, archive_digest, output):
    """Run one real Chat integration case against a pre-provisioned daemon.

    Caller must hold the ScheduledRun slot and enforce exclusive network/socket
    access. This diagnostic authenticates neither tools nor images and is not a
    preflight receipt. Owner retains responsibility for all cleanup and recovery.
    """
    root, output = Path(root).resolve(), Path(output).resolve()
    if output == root or root in output.parents or output.exists():
        raise ValueError('diagnostic output must be new and outside the source tree')
    module = 'src/backend/chat'
    def verify():
        snapshot.verify_materialized(root, source, source_copy)
        if snapshot.capture(root, source['head']) != source:
            raise ValueError('source changed during Chat diagnostic')
        local_go_execution._verify_modules(owner, client_name, module)
        local_go_execution._verify_workspace(owner, client_name, root, source, archive_digest)
    address = inspect_pair(owner, daemon_name, client_name)
    verify()
    if inspect_pair(owner, daemon_name, client_name) != address:
        raise ValueError('private daemon address changed')
    output.mkdir(parents=True)
    command = ['sh', '-c', 'cd /workspace/source/src/backend/chat && '
               'exec env GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly '
               'GOOS=linux GOARCH=amd64 CGO_ENABLED=0 GOMAXPROCS=2 '
               'GOCACHE=/workspace/go-build GOMODCACHE=/workspace/go-mod '
               'DOCKER_HOST=unix:///socket/docker.sock TESTCONTAINERS_RYUK_DISABLED=true '
               'TESTCONTAINERS_HOST_OVERRIDE="$1" '
               'go test -json -count=1 -p 2 -run "^TestSpaceMembersStore_IsAndList$" ./internal/store',
               'voice-private-chat', address]
    with (output / 'events.jsonl').open('xb') as stdout, (output / 'stderr.log').open('xb') as stderr:
        result = owner.execute(client_name, command, stdout, stderr, 1800)
    if result != 0:
        raise ValueError('Chat integration diagnostic failed')
    if inspect_pair(owner, daemon_name, client_name) != address:
        raise ValueError('private daemon address changed')
    verify()
    raw = (output / 'events.jsonl').read_bytes()
    summary = validate_events(raw)
    return {'status': 'incomplete', 'source': source, 'source_archive_sha256': archive_digest,
            'command': command, 'exit_code': result, 'events_sha256': hashlib.sha256(raw).hexdigest(),
            'tests': summary, 'unbound_inputs': ['image and tool provenance', 'exclusive daemon socket access',
                                                  'Go dependency acquisition and cached ziphash source binding']}
