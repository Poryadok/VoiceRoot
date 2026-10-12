"""Explicit Linux Docker acceptance for complete dynamic NATS inventory.

Run with Python on the development host. Only a fresh labeled test volume and
owned network-none containers are accepted; no staging inputs or URLs exist.
The embedded issuer makes disposable JWTs inside the private Linux test volume.
"""
import ast
import copy
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile
import time
import uuid

HELPER = r'''
package main
import (
 "fmt"; "os"; "path/filepath"; "time"
 "github.com/nats-io/jwt/v2"; "github.com/nats-io/nkeys"; "github.com/nats-io/nats.go"
)
func must(e error) { if e!=nil { panic("disposable_census_fixture_failed") } }
func write(base,name,value string) { must(os.WriteFile(filepath.Join(base,name),[]byte(value),0440)) }
func prepare(base string) {
 op,e:=nkeys.CreateOperator();must(e);defer op.Wipe()
 app,e:=nkeys.CreateAccount();must(e);defer app.Wipe()
 sys,e:=nkeys.CreateAccount();must(e);defer sys.Wipe()
 opPub,e:=op.PublicKey();must(e); appPub,e:=app.PublicKey();must(e);sysPub,e:=sys.PublicKey();must(e)
 oc:=jwt.NewOperatorClaims(opPub);oc.SystemAccount=sysPub;operator,e:=oc.Encode(op);must(e)
 ac:=jwt.NewAccountClaims(appPub)
 ac.Limits=jwt.OperatorLimits{AccountLimits:jwt.AccountLimits{Conn:64},NatsLimits:jwt.NatsLimits{Subs:1024,Data:64<<20,Payload:1<<20},JetStreamLimits:jwt.JetStreamLimits{MemoryStorage:8<<20,DiskStorage:32<<20,Streams:64,Consumer:512,MaxAckPending:4096}}
 account,e:=ac.Encode(op);must(e);system,e:=jwt.NewAccountClaims(sysPub).Encode(op);must(e)
 must(os.Mkdir(filepath.Join(base,"inputs"),0750))
 write(base,"inputs/account.public",appPub)
 write(base,"server.conf",fmt.Sprintf("host: 127.0.0.1\nport: 4222\nhttp: 127.0.0.1:8222\nmax_control_line: 32768\noperator: %q\nsystem_account: %q\nresolver: MEMORY\nresolver_preload: { %q: %q, %q: %q }\njetstream: { store_dir: /data, max_file_store: 64MB, max_memory_store: 16MB }\n",operator,sysPub,appPub,account,sysPub,system))
 // Broad authority belongs solely to this disposable, network-none realm.
 user,e:=nkeys.CreateUser();must(e);defer user.Wipe();pub,e:=user.PublicKey();must(e)
 uc:=jwt.NewUserClaims(pub);uc.Pub.Allow=[]string{">"};uc.Sub.Allow=[]string{">"}
 token,e:=uc.Encode(app);must(e);seed,e:=user.Seed();must(e)
 creds:="-----BEGIN NATS USER JWT-----\n"+token+"\n------END NATS USER JWT------\n\n-----BEGIN USER NKEY SEED-----\n"+string(seed)+"\n------END USER NKEY SEED------\n"
 for _,role:=range []string{"bootstrap","social","realtime"} {write(base,"inputs/"+role+".creds",creds)}
}
func extra() {
 nc,e:=nats.Connect("nats://127.0.0.1:4222",nats.UserCredentials("/inputs/bootstrap.creds"),nats.Timeout(2*time.Second),nats.NoReconnect());must(e);defer nc.Close()
 js,e:=nc.JetStream();must(e)
 _,e=js.AddStream(&nats.StreamConfig{Name:"rollout_extra",Subjects:[]string{"rollout.extra"},Storage:nats.FileStorage,Retention:nats.LimitsPolicy});must(e)
 _,e=js.Publish("rollout.extra",[]byte("known-extra-one"));must(e)
 _,e=js.Publish("rollout.extra",[]byte("known-extra-two"));must(e)
 _,e=js.AddConsumer("rollout_extra",&nats.ConsumerConfig{Durable:"rollout_extra_durable",AckPolicy:nats.AckExplicitPolicy,AckWait:5*time.Minute,DeliverPolicy:nats.DeliverAllPolicy});must(e)
 _,e=js.AddConsumer("rollout_extra",&nats.ConsumerConfig{Name:"rollout_extra_ephemeral",InactiveThreshold:5*time.Minute,AckPolicy:nats.AckExplicitPolicy,AckWait:5*time.Minute,DeliverPolicy:nats.DeliverAllPolicy});must(e)
 durable,e:=js.PullSubscribe("rollout.extra","rollout_extra_durable",nats.Bind("rollout_extra","rollout_extra_durable"));must(e)
 messages,e:=durable.Fetch(2,nats.MaxWait(2*time.Second));must(e);if len(messages)!=2 {panic("record_count")}
 must(messages[0].AckSync());must(messages[1].Nak())
 messages,e=durable.Fetch(1,nats.MaxWait(2*time.Second));must(e);if len(messages)!=1 {panic("redelivery_count")}
 ephemeral,e:=js.PullSubscribe("rollout.extra","",nats.Bind("rollout_extra","rollout_extra_ephemeral"));must(e)
 messages,e=ephemeral.Fetch(1,nats.MaxWait(2*time.Second));must(e);if len(messages)!=1 {panic("ephemeral_delivery_count")}
 must(nc.Flush())
}
func verify() {
 nc,e:=nats.Connect("nats://127.0.0.1:4222",nats.UserCredentials("/inputs/bootstrap.creds"),nats.Timeout(2*time.Second),nats.NoReconnect());must(e);defer nc.Close()
 js,e:=nc.JetStream();must(e)
 for index,payload:=range []string{"known-extra-one","known-extra-two"} {
  message,e:=js.GetMsg("rollout_extra",uint64(index+1));must(e)
  if string(message.Data)!=payload {panic("known_payload_changed")}
 }
}
func main() { if len(os.Args)==3 && os.Args[1]=="prepare" {prepare(os.Args[2]);return};if len(os.Args)==2 && os.Args[1]=="extra" {extra();return};if len(os.Args)==2 && os.Args[1]=="verify" {verify();return};panic("fixed_fixture_phase_required") }
'''


_SAFE_FAILURE = re.compile(r'(?:^|:\s)(AssertionError|Blocked|CensusError|FileNotFoundError|ImportError|KeyError|ModuleNotFoundError|OSError|PermissionError|RuntimeError|TypeError|ValueError|panic)(?:: ([a-z][a-z0-9_]{0,95}))?')
_SAFE_FRAME = re.compile(r'^File "([^"]+)", line ([0-9]+)(?:, in [A-Za-z0-9_.<>]+)?$')
_SAFE_SOURCE_FILES = frozenset({
    'bootstrap_auth.py', 'bootstrap_enrollment.py', 'bootstrap_renewal.py', 'bootstrap_root.py',
    'commands.py', 'contract_fixture.py', 'controller.py', 'docker_runtime.py', 'guard.py',
    'nats_contract_actor.py', 'nats_contract_integration.py', 'nats_contract_plan.py',
    'nats_migration.py', 'native_store.py', 'preserve.py', 'rollout_census_integration.py',
    'rollout_census.py', 'scenario.py', 'stage_runtime.py',
})
_SAFE_FAILURE_CODES = frozenset({
    'baseline_record_changed', 'broker_not_running', 'broker_stop_failed', 'chat_predecessor_subjects',
    'container_identity_invalid', 'container_isolation_invalid', 'container_mount_invalid',
    'container_spec_invalid', 'disposable_census_fixture_failed', 'ephemeral_delivery_count',
    'fixed_fixture_phase_required', 'fixture_interrupted_after_update_before_info',
    'fixture_interruption_not_exercised', 'image_digest_invalid', 'kernel_failed', 'kernel_phase_invalid',
    'known_payload_changed', 'known_record_changed', 'known_record_count', 'nats_census_incomplete_or_invalid',
    'owned_census_fixture_command_failed', 'post_release_record_changed', 'record_count',
    'redelivery_count', 'reviewed_predecessor_shape',
})


def _safe_failure_detail(stderr):
    frames = []
    failures = []
    for line in stderr.decode(errors='replace').splitlines()[-40:]:
        frame = _SAFE_FRAME.fullmatch(line.strip())
        if frame:
            filename, line_number = Path(frame.group(1)).name, int(frame.group(2))
            if filename in _SAFE_SOURCE_FILES and 1 <= line_number <= 2000:
                detail = filename + ':' + str(line_number)
                if detail not in frames:
                    frames.append(detail)
        for match in _SAFE_FAILURE.finditer(line.strip()):
            kind, code = match.groups()
            detail = kind + (': ' + code if code in _SAFE_FAILURE_CODES else '')
            if detail not in failures:
                failures.append(detail)
    parts = []
    if frames:
        parts.append('frames=' + ','.join(frames[-4:]))
    if failures:
        parts.append('errors=' + ';'.join(failures[:8]))
    return ' '.join(parts)


def call(args, **kwargs):
    result = subprocess.run(args, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=180, **kwargs)
    if result.returncode:
        detail = _safe_failure_detail(result.stderr)
        message = 'owned_census_fixture_command_failed'
        if detail:
            message += ': ' + detail
        raise RuntimeError(message) from None
    return result.stdout.decode().strip()


def _pinned_runtime_images():
    module_dir = Path(__file__).resolve().parents[1] / 'nats-known-baseline'
    tree = ast.parse((module_dir / 'docker_runtime.py').read_text(encoding='utf-8'))
    images = {}
    for node in tree.body:
        if not isinstance(node, ast.Assign) or not isinstance(node.value, ast.Constant):
            continue
        if len(node.targets) == 1 and isinstance(node.targets[0], ast.Name):
            if node.targets[0].id in ('NATS_IMAGE', 'BOX_IMAGE') and isinstance(node.value.value, str):
                images[node.targets[0].id] = node.value.value
    if set(images) != {'NATS_IMAGE', 'BOX_IMAGE'}:
        raise RuntimeError('fixture_runtime_image_source_invalid')
    return images['NATS_IMAGE'], images['BOX_IMAGE']


def _prefetch_pinned_images(images, command):
    for image in images:
        repository, separator, digest = image.partition('@sha256:')
        if (not repository or not separator or len(digest) != 64
                or any(char not in '0123456789abcdef' for char in digest)):
            raise RuntimeError('fixture_pinned_image_reference_invalid')
        command(['docker', 'pull', image])
        rows = json.loads(command(['docker', 'image', 'inspect', image]))
        if (not isinstance(rows, list) or len(rows) != 1
                or not isinstance(rows[0].get('RepoDigests'), list)
                or 'sha256:' + digest not in [row.split('@')[-1] for row in rows[0]['RepoDigests']
                                             if isinstance(row, str)]):
            raise RuntimeError('fixture_pinned_image_digest_invalid')


def inner(base):
    sys.path[:0] = ['/bundle', '/source/scripts/staging/nats-known-baseline']
    from docker_runtime import DockerRuntime, NATS_IMAGE
    from scenario import ready
    from controller import verify_archive
    from rollout_census import census, semantic, CensusError

    class Runtime(DockerRuntime):
        def monitor_jsz(self, broker):
            self.inspect(broker)
            raw = self.run(['exec', broker, '/bin/busybox', 'wget', '-q', '-O', '-',
                'http://127.0.0.1:8222/jsz?accounts=true&streams=true&consumers=true&config=true&limit=2048'])
            if len(raw.encode()) > 64 * 1024 ** 2:
                raise RuntimeError('owned_census_fixture_monitor_bound')
            return json.loads(raw)

    base = Path(base)
    call(['/bundle/helper', 'prepare', str(base)])
    for name in ('kernel', 'helper'):
        shutil.copyfile('/bundle/' + name, base / name)
        os.chmod(base / name, 0o550); os.chown(base / name, 0, 65532)
    for role in ('realtime', 'notification', 'analytics-chat', 'search'):
        source = Path('/source/scripts/staging/nats-known-baseline/testdata/deployed-bootstrap-20261004') / (role + '-bootstrap.sh')
        target = base / ('bootstrap-' + role + '.sh')
        shutil.copyfile(source, target); os.chmod(target, 0o440); os.chown(target, 0, 65532)
    for path in [base / 'inputs', base / 'server.conf', *list((base / 'inputs').iterdir())]:
        os.chown(path, 0, 65532)
    os.chmod(base, 0o755)
    operation = 'census' + uuid.uuid4().hex[:12]
    runtime = Runtime(base, operation)
    cleaned = []
    try:
        store = base / 'store'; store.mkdir(mode=0o700); os.chown(store, 65532, 65532)
        broker = runtime.start_broker('original', store); ready(runtime, broker)
        runtime.bootstrap(broker)
        account = (base / 'inputs/account.public').read_text().strip()
        baseline = census(runtime, broker, account)
        assert (len(baseline['streams']), len(baseline['consumers'])) == (15, 42)
        seeded = runtime.kernel(broker, 'seed') / 'fixture.json'
        snapshot = base / 'inputs/fixture.json'; snapshot.write_bytes(seeded.read_bytes())
        snapshot.chmod(0o440); os.chown(snapshot, 0, 65532)
        extra = runtime.create('extra-seed', NATS_IMAGE,
            [(base / 'helper', '/kernel', False), (base / 'inputs', '/inputs', False)], ['/kernel', 'extra'], broker)
        runtime.run(['start', extra]); assert runtime.run(['wait', extra]) == '0'
        started = time.monotonic()
        before = census(runtime, broker, account)
        assert (len(before['streams']), len(before['consumers'])) == (16, 44)
        assert sum(row['state']['messages'] for row in before['streams']) == 5
        durable = next(row for row in before['consumers'] if row['name'] == 'rollout_extra_durable')
        ephemeral = next(row for row in before['consumers'] if row['name'] == 'rollout_extra_ephemeral')
        assert durable['num_ack_pending'] == 1 and durable['num_redelivered'] == 1
        assert durable['ack_floor']['stream_seq'] == 1 and durable['delivered']['stream_seq'] == 2
        assert not ephemeral['durable'] and ephemeral['num_ack_pending'] == 1
        assert ephemeral['inactive_threshold'] == 300_000_000_000
        raw = runtime.monitor_jsz(broker)
        omitted = copy.deepcopy(raw)
        selected = next(row for row in omitted['account_details'] if row['id'] == account)
        selected['stream_detail'] = [row for row in selected['stream_detail'] if row['name'] != 'rollout_extra']
        class Incomplete:
            def monitor_jsz(self, unused): return omitted
        try:
            census(Incomplete(), broker, account)
        except CensusError:
            pass
        else:
            raise AssertionError('omitted_extra_inventory_accepted')
        runtime.stop(broker)
        manifest = runtime.cold_archive(broker, store, base / 'cold.tar')
        copy_path = base / 'independent-copy.tar'; shutil.copyfile(base / 'cold.tar', copy_path)
        assert verify_archive(copy_path, manifest)
        restored = base / 'restored'; runtime.restore(copy_path, restored, manifest)
        # Re-archive before restart proves every native file byte copied exactly.
        from controller import archive_closed_store
        restored_manifest = archive_closed_store(restored, base / 'restored-native.tar')
        assert manifest['files'] == restored_manifest['files']
        restored_broker = runtime.start_broker('restored', restored); ready(runtime, restored_broker)
        after = census(runtime, restored_broker, account)
        assert semantic(before) == semantic(after)
        runtime.kernel(restored_broker, 'verify-closed')
        verifier = runtime.create('known-extra-verify', NATS_IMAGE,
            [(base / 'helper', '/kernel', False), (base / 'inputs', '/inputs', False)], ['/kernel', 'verify'], restored_broker)
        runtime.run(['start', verifier]); assert runtime.run(['wait', verifier]) == '0'
        elapsed = time.monotonic() - started
        assert elapsed < 60 and elapsed * 1_000_000_000 < ephemeral['inactive_threshold']
        runtime.stop(restored_broker)
        receipt = {'schema': 'voice-rollout-census-docker-v1', 'streams': 16, 'consumers': 44,
            'messages': sum(row['state']['messages'] for row in before['streams']),
            'extra_durable_ack_pending': durable['num_ack_pending'], 'extra_durable_redelivered': durable['num_redelivered'],
            'ephemeral_ack_pending': ephemeral['num_ack_pending'], 'ephemeral_inactive_threshold_ns': ephemeral['inactive_threshold'],
            'proof_elapsed_seconds': round(elapsed, 3), 'omitted_extra_rejected': True,
            'native_file_bytes_equal': True, 'cold_copy_semantic_equal': True,
            'known_record_payloads_verified': True,
            'account_identity_equal': before['account'] == after['account'], 'archive_sha256': manifest['archive_sha256']}
    finally:
        for name in reversed(list(runtime.owned)):
            row = runtime.inspect(name)
            runtime.run(['rm', '-f', row['Id']]); cleaned.append(row['Id'])
            assert not runtime.run(['ps', '-a', '--filter', 'id=' + row['Id'], '--format', '{{.ID}}'])
    receipt['removed_container_ids'] = cleaned
    print(json.dumps(receipt, sort_keys=True))


def remote_run(bundle, transport='pmdebook'):
    import socket
    if transport == 'pmdebook':
        if sys.platform != 'linux' or os.getuid() != 1000 or socket.gethostname() != 'pmdebook':
            raise RuntimeError('approved_pmd_host_required')
    elif (transport == 'github-hosted' and
          (sys.platform != 'linux' or os.environ.get('RUNNER_ENVIRONMENT') != 'github-hosted'
           or os.environ.get('GITHUB_ACTIONS') != 'true')):
        raise RuntimeError('github_hosted_fixture_runner_required')
    elif transport != 'github-hosted':
        raise RuntimeError('owned_census_fixture_transport_invalid')
    assert call(['docker', 'info', '--format', '{{.OSType}}']) == 'linux'
    if transport == 'github-hosted':
        _prefetch_pinned_images(_pinned_runtime_images(), call)
    operation = 'voice-nats-rollout-fixture-' + uuid.uuid4().hex[:16]
    label = 'voice.rollout.census=' + operation
    run_id = uuid.uuid4().hex[:12]
    volume = operation
    outer = operation + '-runner'; runner_id = None
    output = None; primary = None
    try:
        volume = call(['docker', 'volume', 'create', '--label', label, operation])
        assert volume == operation
        rows = json.loads(call(['docker', 'volume', 'inspect', volume]))
        assert len(rows) == 1 and rows[0]['Labels']['voice.rollout.census'] == operation
        base = rows[0]['Mountpoint']
        assert base == '/var/lib/docker/volumes/' + volume + '/_data'
        mounts = [('/usr/bin/docker','/usr/bin/docker'),
            ('/lib/x86_64-linux-gnu/libc.so.6','/lib/x86_64-linux-gnu/libc.so.6'),
            ('/lib64/ld-linux-x86-64.so.2','/lib64/ld-linux-x86-64.so.2'),
            (str(bundle / 'source'),'/source'), (str(bundle),'/bundle')]
        marker = 'pmdebook' if transport == 'pmdebook' else 'github-hosted'
        args = ['docker', 'run', '--name', outer, '--label', label, '--network', 'none',
            '--env', 'VOICE_CENSUS_APPROVED_HOST=' + marker,
            '--env', 'VOICE_CENSUS_FIXTURE_ID=' + run_id,
            '--mount', 'type=volume,source=' + volume + ',target=' + base,
            '--mount', 'type=bind,source=/var/run/docker.sock,target=/var/run/docker.sock']
        for origin,target in mounts:
            args += ['--mount','type=bind,source='+origin+',target='+target+',readonly']
        output = call([*args, 'python@sha256:1b668429b3511ab407d8e00648891631b0b1a4d7e15e3ca70f38ab5b91ad4ab4', 'python3', '/bundle/rollout_census_integration.py', '--inner', base])
    except BaseException as error:
        primary = error
        raise
    finally:
        cleanup_errors = []
        try:
            owned = call(['docker', 'ps', '-a', '--filter', 'label=' + label, '--format', '{{.ID}}'])
            for container_id in owned.splitlines():
                rows = json.loads(call(['docker', 'inspect', container_id]))
                if len(rows) != 1 or rows[0]['Config']['Labels'].get('voice.rollout.census') != operation:
                    raise RuntimeError('owned_fixture_container_identity_failed')
                if rows[0]['Name'].lstrip('/') != outer:
                    raise RuntimeError('owned_fixture_container_name_failed')
                runner_id = rows[0]['Id']
                call(['docker', 'rm', '-f', runner_id])
            if call(['docker', 'ps', '-a', '--filter', 'label=' + label, '--format', '{{.ID}}']):
                raise RuntimeError('owned_fixture_container_cleanup_failed')
            for child_id in ('contract' + run_id, 'freshrollback' + run_id):
                child_label = 'voice.known-nats.operation=' + child_id
                children = call(['docker', 'ps', '-a', '--filter', 'label=' + child_label, '--format', '{{.ID}}'])
                for container_id in children.splitlines():
                    rows = json.loads(call(['docker', 'inspect', container_id]))
                    if len(rows) != 1 or rows[0]['Config']['Labels'].get('voice.known-nats.operation') != child_id:
                        raise RuntimeError('owned_fixture_child_identity_failed')
                    call(['docker', 'rm', '-f', rows[0]['Id']])
                if call(['docker', 'ps', '-a', '--filter', 'label=' + child_label, '--format', '{{.ID}}']):
                    raise RuntimeError('owned_fixture_child_cleanup_failed')
            rows = json.loads(call(['docker', 'volume', 'inspect', volume]))
            if len(rows) != 1 or rows[0]['Labels'].get('voice.rollout.census') != operation:
                raise RuntimeError('owned_fixture_volume_identity_failed')
            call(['docker', 'volume', 'rm', volume])
            for kind, args in (('volume', ['docker', 'volume', 'ls', '--filter', 'label=' + label, '--format', '{{.Name}}']),
                               ('network', ['docker', 'network', 'ls', '--filter', 'label=' + label, '--format', '{{.ID}}'])):
                if call(args):
                    raise RuntimeError('owned_fixture_' + kind + '_cleanup_failed')
        except Exception:
            cleanup_errors.append('owned_fixture_cleanup_failed')
        if cleanup_errors and primary is None:
            raise RuntimeError(cleanup_errors[0])
        if cleanup_errors:
            print(cleanup_errors[0], file=sys.stderr, flush=True)
    receipt = json.loads(output)
    receipt.update(removed_volume=volume, removed_runner_id=runner_id,
        execution_host=transport, execution_uid=(1000 if transport == 'pmdebook' else os.getuid()), execution_exit_code=0,
        owned_container_volume_network_cleanup_verified=True)
    print(json.dumps(receipt, sort_keys=True))


def main():
    import base64
    source = Path(__file__).resolve().parents[3]
    with tempfile.TemporaryDirectory(prefix='voice-census-build-') as directory:
        bundle=Path(directory); (bundle/'helper.go').write_text(HELPER)
        env=dict(os.environ,GOOS='linux',GOARCH='amd64',CGO_ENABLED='0')
        module=source/'scripts/staging/nats-known-baseline'
        call(['go','build','-o',str(bundle/'helper'),str(bundle/'helper.go')],cwd=module,env=env)
        call(['go','build','-o',str(bundle/'kernel'),'.'],cwd=module,env=env)
        records={name:base64.b64encode((bundle/name).read_bytes()).decode() for name in ('helper','kernel')}
        for name in ('rollout_census.py','rollout_census_integration.py'):
            records[name]=base64.b64encode((Path(__file__).parent/name).read_bytes()).decode()
        for name in ('docker_runtime.py','scenario.py','controller.py','commands.py'):
            records['source/scripts/staging/nats-known-baseline/'+name]=base64.b64encode((module/name).read_bytes()).decode()
        for role in ('realtime','notification','analytics-chat','search'):
            name='testdata/deployed-bootstrap-20261004/'+role+'-bootstrap.sh'
            records['source/scripts/staging/nats-known-baseline/'+name]=base64.b64encode((module/name).read_bytes()).decode()
        script = ('import base64,os,pathlib,tempfile,sys,socket\n'
            + 'assert sys.platform=='+repr('linux')+' and os.getuid()==1000 and socket.gethostname()=='+repr('pmdebook')+'\n'
            + 'with tempfile.TemporaryDirectory(prefix='+repr('voice-nats-rollout-fixture-')+') as directory:\n'
            + ' bundle=pathlib.Path(directory)\n'
            + ' for name,data in '+repr(records)+'.items():\n'
            + '  target=bundle/name; target.parent.mkdir(parents=True,exist_ok=True); target.write_bytes(base64.b64decode(data)); target.chmod(0o755 if name in ('+repr('helper')+','+repr('kernel')+') else 0o644)\n'
            + ' bundle.chmod(0o755); sys.path.insert(0,str(bundle)); import rollout_census_integration as fixture; fixture.remote_run(bundle)\n')
        print(call(['ssh','voice-staging','python3 -'],input=script.encode()))


if __name__ == '__main__':
    if (len(sys.argv) == 3 and sys.argv[1] == '--inner' and sys.platform == 'linux'
            and os.environ.get('VOICE_CENSUS_APPROVED_HOST') in ('pmdebook', 'github-hosted')):
        inner(sys.argv[2])
    elif len(sys.argv) == 1:
        main()
    else:
        raise SystemExit('owned_census_fixture_arguments_invalid')
