"""CONNECT/PING/PONG only, on an empty pinned network-none scratch broker."""
import json,os,shutil,tempfile,time,uuid
from pathlib import Path
from bootstrap_root import secret_bytes,hash_bytes
from controller import Blocked
from docker_runtime import DockerRuntime,NATS_IMAGE
import guard

def prove(credentials,operator,binary,unchanged):
    unchanged()
    with tempfile.TemporaryDirectory(prefix='service-actor-auth-',dir=guard.ROOT) as td:
        base=Path(td);base.chmod(0o750);os.chown(base,0,65532)
        inputs=base/'inputs';inputs.mkdir(mode=0o750);os.chown(inputs,0,65532)
        p=inputs/'actor.creds';p.write_bytes(credentials);p.chmod(0o440);os.chown(p,0,65532)
        helper=base/'actor-helper';shutil.copyfile(binary,helper);helper.chmod(0o550);os.chown(helper,0,65532)
        tokens={k:secret_bytes(operator,k).decode().strip() for k in ('operator.jwt','account.jwt','system-account.jwt','account.public','system-account.public')}
        q=json.dumps
        # No JetStream/store/routes/gateways/leaf listener. Only original signed
        # resolver authority and fixed isolated CLIENT transport are retained.
        config='host: 127.0.0.1\nport: 4222\nmax_control_line: 32768\noperator: '+q(tokens['operator.jwt'])+'\nsystem_account: '+q(tokens['system-account.public'])+'\nresolver: MEMORY\nresolver_preload: { '+q(tokens['account.public'])+': '+q(tokens['account.jwt'])+', '+q(tokens['system-account.public'])+': '+q(tokens['system-account.jwt'])+' }\n'
        p=base/'server.conf';p.write_text(config);p.chmod(0o440);os.chown(p,0,65532)
        runtime=DockerRuntime(base,'actor'+uuid.uuid4().hex[:12])
        try:
            broker=runtime.create('broker',NATS_IMAGE,[(p,'/server.conf',False)],['/usr/local/bin/nats-server','-c','/server.conf'])
            runtime.run(['start',broker]);runtime.inspect(broker)
            deadline=time.monotonic()+15
            while True:
                client=runtime.create('client-'+str(len(runtime.owned)),NATS_IMAGE,
                    [(helper,'/kernel',False),(inputs,'/inputs',False)],['/kernel','--authenticate-existing-actor'],broker)
                runtime.run(['start',client])
                if runtime.run(['wait',client],timeout=10)=='0':break
                if time.monotonic()>=deadline:raise Blocked('existing_service_actor_server_authentication_refused')
                time.sleep(0.2)
            runtime.inspect(client);runtime.stop(broker);unchanged()
            return {'server_authentication_verified':True,'server_image':NATS_IMAGE,
                'account_sha256':hash_bytes(tokens['account.public'].encode()),
                'credential_sha256':hash_bytes(credentials),'empty_scratch':True,
                'publication_authorization_exercised':False,'business_delivery_exercised':False}
        finally:
            for name in reversed(list(runtime.owned)):
                row=runtime.inspect(name);runtime.run(['rm','-f',row['Id']])
                if runtime.run(['ps','-a','--filter','id='+row['Id'],'--format','{{.ID}}']):raise Blocked('existing_service_actor_cleanup_failed')
