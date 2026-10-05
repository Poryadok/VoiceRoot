"""Explicit Linux Docker acceptance for complete dynamic NATS inventory.

Run with Python on the development host. Only a fresh labeled test volume and
owned network-none containers are accepted; no staging inputs or URLs exist.
The embedded issuer makes disposable JWTs inside the private Linux test volume.
"""
import copy
import json
import os
from pathlib import Path
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


def call(args, **kwargs):
    result = subprocess.run(args, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=180, **kwargs)
    if result.returncode:
        raise RuntimeError('owned_census_fixture_command_failed')
    return result.stdout.decode().strip()


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


def remote_run(bundle):
    import socket
    if sys.platform != 'linux' or os.getuid() != 1000 or socket.gethostname() != 'pmdebook':
        raise RuntimeError('approved_pmd_host_required')
    assert call(['docker', 'info', '--format', '{{.OSType}}']) == 'linux'
    operation = 'voice-nats-rollout-fixture-' + uuid.uuid4().hex[:16]
    label = 'voice.rollout.census=' + operation
    volume = call(['docker', 'volume', 'create', '--label', label, operation])
    assert volume == operation
    outer = operation + '-runner'; runner_id = None
    try:
        rows = json.loads(call(['docker', 'volume', 'inspect', volume]))
        assert len(rows) == 1 and rows[0]['Labels']['voice.rollout.census'] == operation
        base = rows[0]['Mountpoint']
        assert base == '/var/lib/docker/volumes/' + volume + '/_data'
        mounts = [('/usr/bin/docker','/usr/bin/docker'),
            ('/lib/x86_64-linux-gnu/libc.so.6','/lib/x86_64-linux-gnu/libc.so.6'),
            ('/lib64/ld-linux-x86-64.so.2','/lib64/ld-linux-x86-64.so.2'),
            (str(bundle / 'source'),'/source'), (str(bundle),'/bundle')]
        args = ['docker', 'run', '--name', outer, '--label', label, '--network', 'none',
            '--env', 'VOICE_CENSUS_APPROVED_HOST=pmdebook',
            '--mount', 'type=volume,source=' + volume + ',target=' + base,
            '--mount', 'type=bind,source=/var/run/docker.sock,target=/var/run/docker.sock']
        for origin,target in mounts:
            args += ['--mount','type=bind,source='+origin+',target='+target+',readonly']
        output = call([*args, 'python:3.12-alpine', 'python3', '/bundle/rollout_census_integration.py', '--inner', base])
    finally:
        rows = json.loads(call(['docker', 'inspect', outer]))
        assert len(rows) == 1 and rows[0]['Config']['Labels']['voice.rollout.census'] == operation
        runner_id = rows[0]['Id']; call(['docker', 'rm', '-f', runner_id])
        rows = json.loads(call(['docker', 'volume', 'inspect', volume]))
        assert rows[0]['Labels']['voice.rollout.census'] == operation
        call(['docker', 'volume', 'rm', volume])
        assert not call(['docker','ps','-a','--filter','label='+label,'--format','{{.ID}}'])
        assert not call(['docker','volume','ls','--filter','label='+label,'--format','{{.Name}}'])
        assert not call(['docker','network','ls','--filter','label='+label,'--format','{{.ID}}'])
    receipt = json.loads(output)
    receipt.update(removed_volume=volume, removed_runner_id=runner_id,
        execution_host='pmdebook', execution_uid=1000, execution_exit_code=0,
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
            and os.environ.get('VOICE_CENSUS_APPROVED_HOST') == 'pmdebook'):
        inner(sys.argv[2])
    elif len(sys.argv) == 1:
        main()
    else:
        raise SystemExit('owned_census_fixture_arguments_invalid')
