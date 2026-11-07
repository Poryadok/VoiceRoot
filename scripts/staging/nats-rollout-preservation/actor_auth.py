"""CONNECT/PING/PONG only, on an empty pinned network-none scratch broker."""
import json,os,shutil,tempfile,time,uuid
from pathlib import Path
from bootstrap_root import secret_bytes,hash_bytes
from controller import Blocked
from docker_runtime import DockerRuntime,NATS_IMAGE
import guard

class AuthScratch:
    """Retain mounted inputs whenever Docker cleanup cannot be confirmed."""
    def __init__(self,prefix,root):
        self.path=Path(tempfile.mkdtemp(prefix=prefix,dir=root));self.held=False
    def __enter__(self):return self
    def hold(self):self.held=True
    def release(self):self.held=False
    def __exit__(self,*_):
        if not self.held:shutil.rmtree(self.path)

def prove(credentials,operator,binary,unchanged):
    unchanged()
    with AuthScratch('service-actor-auth-',guard.ROOT) as scratch:
        base=scratch.path;base.chmod(0o750);os.chown(base,0,65532)
        inputs=base/'inputs';inputs.mkdir(mode=0o750);os.chown(inputs,0,65532);inputs.chmod(0o750)
        p=inputs/'actor.creds';p.write_bytes(credentials);p.chmod(0o440);os.chown(p,0,65532)
        helper=base/'actor-helper';shutil.copyfile(binary,helper);helper.chmod(0o550);os.chown(helper,0,65532)
        tokens={k:secret_bytes(operator,k).decode().strip() for k in ('operator.jwt','account.jwt','system-account.jwt','account.public','system-account.public')}
        q=json.dumps
        # No JetStream/store/routes/gateways/leaf listener. Only original signed
        # resolver authority and fixed isolated CLIENT transport are retained.
        config='host: 127.0.0.1\nport: 4222\nmax_control_line: 32768\noperator: '+q(tokens['operator.jwt'])+'\nsystem_account: '+q(tokens['system-account.public'])+'\nresolver: MEMORY\nresolver_preload: { '+q(tokens['account.public'])+': '+q(tokens['account.jwt'])+', '+q(tokens['system-account.public'])+': '+q(tokens['system-account.jwt'])+' }\n'
        p=base/'server.conf';p.write_text(config);p.chmod(0o440);os.chown(p,0,65532)
        import commands,resource_claim
        session=getattr(commands.authority_guard,'__self__',None)
        cohort_runtime=isinstance(session,resource_claim.Session) and session.check().get('cohort_preservation_sha256') is not None
        if cohort_runtime:
            scratch.hold() # Constructor may persist a journal before it returns.
            import staging_cohort_preservation
            runtime=staging_cohort_preservation.auth_runtime(session,base,'actor')
        else:runtime=DockerRuntime(base,'actor'+uuid.uuid4().hex[:12])
        scratch.hold()
        completed=False
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
            completed=True
            return {'server_authentication_verified':True,'server_image':NATS_IMAGE,
                'account_sha256':hash_bytes(tokens['account.public'].encode()),
                'credential_sha256':hash_bytes(credentials),'empty_scratch':True,
                'publication_authorization_exercised':False,'business_delivery_exercised':False}
        finally:
            try:
                for name in reversed(list(runtime.owned)):
                    row=runtime.inspect(name);runtime.run(['rm','-f',row['Id']])
                    if runtime.run(['ps','-a','--filter','id='+row['Id'],'--format','{{.ID}}']):raise Blocked('existing_service_actor_cleanup_failed')
            finally:
                if cohort_runtime:runtime.finish_auth()
            if not cohort_runtime:runtime.no_operation_containers()
            if completed:scratch.release()
