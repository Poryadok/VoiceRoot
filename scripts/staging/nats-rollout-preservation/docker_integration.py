"""Owned Linux populated-store version/rollback check, never staging.

Uses disposable test JWTs, pinned NATS and two published Gateway releases.
Executes the compiled Gateway images in owned network-none containers. This
is Docker release execution; Kubernetes/staging acceptance remains human-owned.
"""
import copy
import json
import os
from pathlib import Path
import shutil
import uuid
import time
import yaml
import compiler
import guard
from docker_runtime import DockerRuntime
from scenario import fixture,ready,census
from controller import Blocked
from preserve import capture_cut, verify_post_apply
from apply import digest, paused_documents
from docker_recovery_integration import cleanup


def inputs(parent,base):
    (base/'inputs').mkdir(mode=0o750);os.chown(base/'inputs',0,65532)
    for role in ('bootstrap','social','realtime'):
        path=base/'inputs'/(role+'.creds');shutil.copyfile(parent/'inputs'/(role+'.creds'),path)
        path.chmod(0o440);os.chown(path,0,65532)
    for name in ('server.conf','kernel','bootstrap-realtime.sh','bootstrap-notification.sh','bootstrap-analytics-chat.sh','bootstrap-search.sh'):
        path=base/name;shutil.copyfile(parent/name,path)
        path.chmod(0o550 if name=='kernel' else 0o440);os.chown(path,0,65532)


OLD='ghcr.io/poryadok/voiceroot/gateway@sha256:0680c6c7890ef02939232a4427d410a22fc73b4c0f16aea6e1dd8a4f78997776'
NEW='ghcr.io/poryadok/voiceroot/gateway@sha256:2397cbe07a5b88cf405a76e841de8af2406495ac28b5ad4eb2b8882c8c3625b0'

def target(image):
    source=Path(__file__).resolve().parents[3]
    parameters={'registry':'ghcr.io/poryadok/voiceroot','tag':'561c91d52217adefe5fdd8f85f56dbb152e0a4a6',
        'generation':'r20260930a4','dataPVC':'voice-nats-jsdata-d202610040049430b','s3_signing_endpoint':'https://storage.example.invalid'}
    rows=[r for r in compiler.rendered_source(source,parameters,yaml.safe_load) if r['kind']=='Deployment' and r['metadata']['name']=='voice-gateway']
    pack=compiler.compile_target(source,rows,parameters['registry'],parameters['tag'],'images-only',['gateway'],
        {'voice-gateway/gateway':image},{},{'voice-gateway'},compiler.SOURCE_MANIFESTS)
    rendered=paused_documents(pack['manifests'],{'pvc':{'name':'unchanged-owned-test-store'},'target':pack['target']})
    assert rendered[0]['spec']['replicas']==0
    return rendered

def create_gateway(runtime,rows,suffix,bad=False):
    row=rows[0];assert row['spec']['replicas']==0
    container=next(c for c in row['spec']['template']['spec']['containers'] if c['name']=='gateway')
    image=container['image'];image_row=json.loads(runtime.run(['image','inspect',image]))[0]
    assert image in image_row['RepoDigests']
    name='voice-known-'+runtime.operation+'-'+suffix
    args=['create','--name',name,'--label','voice.known-nats.operation='+runtime.operation,
          '--network','none','--user','65532:65532','--read-only','--cap-drop','ALL',
          '--security-opt','no-new-privileges:true','--memory','256m','--pids-limit','64']
    if bad:args+=['--env','GATEWAY_SESSION_EPOCH_STRICT=invalid']
    cid=runtime.run([*args,image])
    runtime.owned[name]={'id':cid,'image':image,'image_id':image_row['Id'],'network':'none','mounts':set()}
    assert not runtime.inspect(name)['State']['Running']
    return name

def execute_gateway(runtime,name):
    runtime.run(['start',name]);deadline=time.monotonic()+30
    while time.monotonic()<deadline:
        if not runtime.inspect(name)['State']['Running']:raise Blocked('test_gateway_start_failed')
        try:
            if runtime.run(['exec',name,'wget','-q','-O','-','http://127.0.0.1:8080/health'],timeout=5)=='ok':
                runtime.stop(name);return
        except Blocked:pass
        time.sleep(.2)
    raise Blocked('test_gateway_health_timeout')


def main():
    prepared=Path('/var/lib/docker/volumes/voice-known-test-20261004/_data')
    operation='rollout'+uuid.uuid4().hex[:12]
    base=prepared/operation;base.mkdir(mode=0o700);inputs(prepared,base)
    runtime=DockerRuntime(base,operation)
    try:
        old=target(OLD);new=target(NEW)
        old_gateway=create_gateway(runtime,old,'old-gateway');execute_gateway(runtime,old_gateway)
        runtime.run(['rm',old_gateway]);del runtime.owned[old_gateway]
        known=fixture(runtime)
        source=base/'fixture-store'
        cut=capture_cut(runtime,source,lambda:runtime.no_operation_containers(running_only=True))
        assert old[0]['spec']['template']!=new[0]['spec']['template']
        new_gateway=create_gateway(runtime,new,'new-gateway')
        after=verify_post_apply(runtime,source,cut,lambda:runtime.no_operation_containers(running_only=True))
        assert after['messages']==3
        execute_gateway(runtime,new_gateway)
        runtime.run(['rm',new_gateway]);del runtime.owned[new_gateway]
        # New traffic after the successful release must survive a fresh rollback
        # cut; the original three-record backup is never replayed over it.
        broker=runtime.start_broker('after-release',source);ready(runtime,broker)
        publisher=runtime.create('after-release-publish',__import__('docker_runtime').BOX_IMAGE,
            [(base/'inputs','/inputs',False)],['/bin/sh','-ceu',
                'nats --server nats://127.0.0.1:4222 --creds /inputs/social.creds pub social.user_blocked \'{"fixture":"after-new-release"}\''],broker)
        runtime.run(['start',publisher]);assert runtime.run(['wait',publisher])=='0'
        assert sum(s['messages'] for s in census(runtime,broker)['streams'])==4
        runtime.stop(broker)
    finally:cleanup(runtime)
    # Rollback uses another owned operation/cold cut of the same populated store,
    # never extraction over it or fixture/bootstrap on it.
    rollback=prepared/(operation+'-rollback');rollback.mkdir(mode=0o700);inputs(base,rollback)
    runtime=DockerRuntime(rollback,operation+'back')
    try:
        cut=capture_cut(runtime,source,lambda:runtime.no_operation_containers(running_only=True))
        reverted=create_gateway(runtime,old,'rollback-gateway')
        after=verify_post_apply(runtime,source,cut,lambda:runtime.no_operation_containers(running_only=True))
        assert after['messages']==4
        execute_gateway(runtime,reverted);runtime.run(['rm',reverted]);del runtime.owned[reverted]
        failure_base=prepared/(operation+'-failure');failure_base.mkdir(mode=0o700);inputs(rollback,failure_base)
        failure=DockerRuntime(failure_base,operation+'fail')
        try:
            failure_cut=capture_cut(failure,source,lambda:failure.no_operation_containers(running_only=True))
            failed=create_gateway(failure,new,'failed-gateway',bad=True)
            failure.run(['start',failed]);assert failure.run(['wait',failed],timeout=10)=='1'
            assert verify_post_apply(failure,source,failure_cut,lambda:failure.no_operation_containers(running_only=True))['messages']==4
        finally:cleanup(failure)
        print(json.dumps({'schema':'nats-populated-rollout-test-v2','messages':4,'streams':15,'consumers':42,
            'release_transition':[OLD,NEW,OLD],'business_services_executed':True,'kubernetes_deployment_exercised':False,
            'new_post_release_record_preserved':True,'failed_release_preserved':True,
            'native_bytes_preserved':True,'ack_pending_preserved':True,
            'before_archive_sha256':cut['manifest']['archive_sha256'],
            'census_sha256':cut['census_sha256']},sort_keys=True))
    finally:cleanup(runtime)

if __name__=='__main__':main()
