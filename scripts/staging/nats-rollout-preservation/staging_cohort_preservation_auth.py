"""Source-fixed actor/bootstrap scratch transport under preservation lineage.

Only disposable, empty, network-none auth realms and their owned clients are
accepted. No live storage, caller URL, Docker endpoint or host command exists.
"""
import hashlib
import json
import os
from pathlib import Path
import re
import stat

import commands
from bootstrap_root import secret_bytes
from controller import Blocked
from docker_runtime import DockerRuntime,NATS_IMAGE,BOX_IMAGE,LABEL
from stage_runtime import Kube
import staging_cohort_preservation as owner
from staging_cohort_preservation_runtime import DOCKER


def _guard(session):
    plan=owner.load_plan(session);owner._phase_continuity(session,plan)
    if (session.check().get('cohort_preservation_sha256')!=owner.digest(plan)
        or owner._state(session).get('status') not in ('PAUSED','BACKUP','AWAITING_OFF_NODE','QUALIFIED')):
        raise Blocked('cohort_auth_lineage_changed')
    return owner.load_preflight(session)


def _file(path,mode):
    fd=os.open(path,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK)
    try:
        row=os.fstat(fd)
        if (not stat.S_ISREG(row.st_mode) or row.st_uid!=0 or row.st_gid!=65532
            or stat.S_IMODE(row.st_mode)!=mode or row.st_nlink!=1 or not 0<row.st_size<=32<<20):
            raise Blocked('cohort_auth_input_custody_invalid')
        raw=os.read(fd,row.st_size+1);after=os.fstat(fd)
        fields=('st_dev','st_ino','st_size','st_mtime_ns','st_ctime_ns')
        if len(raw)!=row.st_size or any(getattr(row,k)!=getattr(after,k) for k in fields):
            raise Blocked('cohort_auth_input_changed')
        return raw
    finally:os.close(fd)


class ScratchRuntime(DockerRuntime):
    def __init__(self,session,base,kind):
        preflight=_guard(session);owner._no_pending(session);base=Path(base)
        if session in owner._active_auth:raise Blocked('cohort_auth_operation_busy')
        prefix={'actor':'service-actor-auth-','bootstrap':'bootstrap-auth-'}.get(kind)
        if (prefix is None or base.parent!=owner.guard.ROOT or not base.name.startswith(prefix)
            or base.resolve(strict=True)!=base):raise Blocked('cohort_auth_directory_invalid')
        for folder in (base,base/'inputs'):
            row=folder.lstat()
            if not stat.S_ISDIR(row.st_mode) or row.st_uid!=0 or row.st_gid!=65532 or stat.S_IMODE(row.st_mode)!=0o750:
                raise Blocked('cohort_auth_directory_invalid')
        self.session=session;self.kind=kind;self.nonce=owner.secrets.token_hex(16)
        super().__init__(base,'proof'+self.nonce[:12]);self.captured={}
        self.path=owner._base(session)/'auth'/(self.nonce+'.json')
        owner.private._private_directory(self.path.parent)
        self.inputs=self._inputs(preflight)
        if kind=='bootstrap':
            store=base/'auth-store';row=store.lstat()
            if (not stat.S_ISDIR(row.st_mode) or row.st_uid!=65532 or stat.S_IMODE(row.st_mode)!=0o700
                or any(store.iterdir())):raise Blocked('cohort_auth_store_not_empty')
        owner._write(self.path,{'schema':'voice-staging-cohort-auth-scratch-v1','operation':session.operation,
            'plan_sha256':owner.digest(owner.load_plan(session)),'kind':kind,'base':str(base),'nonce':self.nonce,
            'inputs':self.inputs,'resources':{},'status':'RUNNING'})
        owner._active_auth[session]=self.nonce

    def _inputs(self,preflight):
        kube=Kube();generation=preflight['enrollment']['binding']['generation']
        operator=kube.get('secret','voice-nats-operator-'+generation)
        if (operator['metadata'].get('namespace')!='voice-staging' or operator['metadata'].get('deletionTimestamp')
            or any(operator['metadata'].get(key)!=preflight['enrollment']['binding']['operator_secret'][key]
                for key in ('name','uid','resourceVersion'))):
            raise Blocked('cohort_auth_operator_changed')
        if preflight['actor'] is not None:
            bound=preflight['actor']['binding']
            if owner.digest(operator)!=bound['operator_body_sha256']:raise Blocked('cohort_auth_operator_changed')
        tokens={k:secret_bytes(operator,k).decode().strip() for k in
            ('operator.jwt','account.jwt','system-account.jwt','account.public','system-account.public')}
        q=json.dumps
        config='host: 127.0.0.1\nport: 4222\n'+('http: 127.0.0.1:8222\n' if self.kind=='bootstrap' else '')
        config+='max_control_line: 32768\noperator: '+q(tokens['operator.jwt'])+'\nsystem_account: '+q(tokens['system-account.public'])+'\nresolver: MEMORY\nresolver_preload: { '+q(tokens['account.public'])+': '+q(tokens['account.jwt'])+', '+q(tokens['system-account.public'])+': '+q(tokens['system-account.jwt'])+' }\n'
        if self.kind=='bootstrap':config+='jetstream { store_dir: /data }\n'
        if _file(self.base/'server.conf',0o440)!=config.encode():raise Blocked('cohort_auth_config_changed')
        if self.kind=='actor':
            if preflight['actor'] is None:raise Blocked('cohort_auth_actor_missing')
            services=kube.get('secret',bound['services']['name'])
            if owner.digest(services)!=bound['services_body_sha256']:raise Blocked('cohort_auth_credentials_changed')
            raw=_file(self.base/'inputs'/'actor.creds',0o440)
            if raw not in [secret_bytes(services,role+'.creds') for role in preflight['actor']['roles']]:
                raise Blocked('cohort_auth_credentials_changed')
            helper=_file(self.base/'actor-helper',0o550)
            if hashlib.sha256(helper).hexdigest()!=owner.private.BINARY_PINS['nats-rollout-preservation/bootstrap-renewer']:
                raise Blocked('cohort_auth_helper_changed')
            expected={'inputs','server.conf','actor-helper'};files={'server.conf':config.encode(),'inputs/actor.creds':raw,'actor-helper':helper}
        else:
            enrollment=preflight['enrollment'];secret=kube.get('secret',enrollment['secret']['name'])
            if any(secret['metadata'].get(k)!=enrollment['secret'][k] for k in ('name','uid','resourceVersion')):
                raise Blocked('cohort_auth_credentials_changed')
            raw=_file(self.base/'inputs'/'bootstrap.creds',0o440)
            if raw!=secret_bytes(secret,'bootstrap.creds'):raise Blocked('cohort_auth_credentials_changed')
            expected={'inputs','server.conf','auth-store'};files={'server.conf':config.encode(),'inputs/bootstrap.creds':raw}
        if {p.name for p in self.base.iterdir()}!=expected or {p.name for p in (self.base/'inputs').iterdir()}!={self.kind+'.creds' if self.kind=='bootstrap' else 'actor.creds'}:
            raise Blocked('cohort_auth_inputs_changed')
        return {name:hashlib.sha256(raw).hexdigest() for name,raw in files.items()}

    def _check(self):
        preflight=_guard(self.session)
        if self._inputs(preflight)!=self.inputs:raise Blocked('cohort_auth_inputs_changed')
        row=owner._read(self.path)
        if row['inputs']!=self.inputs or row['resources']!=self.captured or row['nonce']!=self.nonce:
            raise Blocked('cohort_auth_journal_changed')
        return row

    def finish_auth(self):
        """Called by the fixed prove() finally, including exceptional exits."""
        owner._active_auth.pop(self.session,None)
        row=self._check()
        if (row.get('status')!='COMPLETE' or not self.captured
            or any(value['status']!='REMOVED' for value in self.captured.values())):
            raise Blocked('cohort_auth_cleanup_unqualified')
        owner._no_pending(self.session)

    def _run(self,args,timeout=60,limit=1<<20):
        row=self._check();owner._no_pending(self.session)
        known=set(self.captured)|{r['id'] for r in self.captured.values() if r['status']!='REMOVED'}
        known={name for name in known if name not in self.captured or self.captured[name]['status']!='REMOVED'}
        creating=None;removing=None
        if len(args)==3 and args[:2]==['image','inspect'] and args[2] in (NATS_IMAGE,BOX_IMAGE):pass
        elif len(args)==2 and args[0] in ('inspect','start','wait','logs') and args[1] in known:pass
        elif len(args)==4 and args[:3]==['stop','--time','10'] and args[3] in known:pass
        elif len(args)==3 and args[:2]==['rm','-f'] and args[2] in known:removing=args[2]
        elif (len(args)==6 and args[:3]==['ps','-a','--filter'] and args[3].startswith('id=')
            and args[3][3:] in {r['id'] for r in self.captured.values()} and args[4:]==['--format','{{.ID}}']):pass
        elif (len(args)==8 and args[0]=='exec' and args[1] in known and self.kind=='bootstrap'
            and args[2:7]==['/bin/busybox','wget','-q','-O','-']
            and args[7] in ('http://127.0.0.1:8222/healthz','http://127.0.0.1:8222/jsz?accounts=true&streams=true&consumers=true&config=true&limit=2048')):pass
        elif args[:1]==['create']:
            creating=self._create(args)
        else:raise Blocked('cohort_auth_command_invalid')
        if args[-1:] == ['http://127.0.0.1:8222/healthz']:
            args=['exec',args[1],'/bin/busybox','sh','-ec','i=0; while [ "$i" -lt 120 ]; do if v=$(/bin/busybox wget -T 1 -q -O - http://127.0.0.1:8222/healthz 2>/dev/null); then printf "%s" "$v"; exit 0; fi; i=$((i+1)); /bin/busybox sleep 0.25; done; exit 1'];timeout=160
        argv=DOCKER+list(args);ident='auth-'+self.nonce+'-'+owner.secrets.token_hex(8)
        folder=owner._base(self.session)/'effects';owner.private._private_directory(folder)
        intent={'schema':'voice-staging-cohort-effect-v1','operation':self.session.operation,'effect_id':ident,
            'binding':self.session.binding(),'argv':argv,'body_sha256':hashlib.sha256(b'').hexdigest(),'status':'PENDING'}
        path=folder/(ident+'.json');owner._write(path,intent);self._check()
        if owner._read(path)!=intent or getattr(self.session,'_cohort_auth_permit',None) is not None:
            raise Blocked('cohort_auth_effect_changed')
        self.session._cohort_auth_permit={k:intent[k] for k in ('effect_id','argv','body_sha256')}
        try:
            raw=commands.capture(argv,body=b'',timeout=timeout,limit=limit,env={'PATH':'/usr/bin:/bin','HOME':'/nonexistent'})
            if self.session._cohort_auth_permit is not None:raise Blocked('cohort_auth_permit_not_consumed')
            self._check()
            if owner._read(path)!=intent:raise Blocked('cohort_auth_effect_changed')
            result=raw.decode('utf8').strip()
            if creating:
                if not re.fullmatch('[a-f0-9]{64}',result):raise Blocked('cohort_auth_create_unknown')
                self.captured[creating]={'id':result,'status':'OWNED'}
            if removing:
                for name,value in self.captured.items():
                    if removing in (name,value['id']):value['status']='REMOVED'
            row['resources']=self.captured
            if args[:3]==['ps','-a','--filter'] and not result and self.captured and all(v['status']=='REMOVED' for v in self.captured.values()):
                row['status']='COMPLETE';owner._active_auth.pop(self.session,None)
            owner.save(self.path,row)
            owner.save(path,{**intent,'status':'DONE','response_sha256':hashlib.sha256(raw).hexdigest()})
            return result
        except BaseException:
            owner._active_auth.pop(self.session,None)
            raise
        finally:self.session._cohort_auth_permit=None

    def _create(self,args):
        if len(args)<29 or args[:2]!=['create','--name']:raise Blocked('cohort_auth_create_invalid')
        name=args[2];network=args[6]
        if not re.fullmatch('voice-known-'+self.operation+'-[a-z0-9-]{1,32}',name) or name in self.captured:
            raise Blocked('cohort_auth_create_invalid')
        allowed={'container:'+r['id'] for n,r in self.captured.items() if n.endswith(('-broker','-proof')) and r['status']=='OWNED'}
        if network!='none' and network not in allowed:raise Blocked('cohort_auth_network_invalid')
        prefix=['create','--name',name,'--label',LABEL+'='+self.operation,'--network',network,
            '--user','65532:65532','--read-only','--env','HOME=/tmp','--env','XDG_CACHE_HOME=/tmp/cache','--workdir','/tmp',
            '--cap-drop','ALL','--security-opt','no-new-privileges:true','--memory','256m','--pids-limit','64','--tmpfs','/tmp:rw,noexec,nosuid,size=16m']
        if args[:len(prefix)]!=prefix:raise Blocked('cohort_auth_create_invalid')
        tail=args[len(prefix):];mounts=[]
        while tail[:1]==['--mount'] and len(tail)>1:mounts.append(tail[1]);tail=tail[2:]
        bind=lambda source,target,writable=False:'type=bind,source='+str(self.base/source)+',target='+target+('' if writable else ',readonly')
        if tail==['--entrypoint','/usr/local/bin/nats-server',NATS_IMAGE,'-c','/server.conf']:
            expected=[bind('server.conf','/server.conf')] if self.kind=='actor' else [bind('auth-store','/data',True),bind('server.conf','/server.conf')]
            if mounts!=expected or network!='none':raise Blocked('cohort_auth_mount_invalid')
        elif self.kind=='actor' and tail==['--entrypoint','/kernel',NATS_IMAGE,'--authenticate-existing-actor']:
            if mounts!=[bind('actor-helper','/kernel'),bind('inputs','/inputs')] or network not in allowed:
                raise Blocked('cohort_auth_mount_invalid')
        elif self.kind=='bootstrap' and len(tail)==8 and tail[:4]==['--entrypoint','/bin/sh',BOX_IMAGE,'-c']:
            fixed='exec nats --server nats://127.0.0.1:4222 --creds /inputs/bootstrap.creds --inbox-prefix _INBOX.voice.bootstrap.reply --timeout 5s req --raw "$1" "$2"'
            if (tail[4:6]!=[fixed,'voice-contract'] or mounts!=[bind('inputs','/inputs')] or network not in allowed
                or not re.fullmatch(r'\$JS\.API\.(STREAM\.(CREATE|UPDATE|INFO|MSG.GET)|CONSUMER\.(CREATE|INFO))\.[A-Za-z0-9_.-]+',tail[6])
                or len(tail[7].encode())>64<<10):raise Blocked('cohort_auth_request_invalid')
            if tail[7] and not isinstance(json.loads(tail[7]),dict):raise Blocked('cohort_auth_request_invalid')
        else:raise Blocked('cohort_auth_create_invalid')
        return name
