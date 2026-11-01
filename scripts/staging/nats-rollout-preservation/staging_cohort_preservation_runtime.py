"""Fixed STAGING cut adapter; no caller-supplied transport or authority."""
import copy
import json
import re
import time
import hashlib
import io
import tarfile
from pathlib import Path
from docker_runtime import DockerRuntime,NATS_IMAGE,LABEL
import guard
import staging_cohort_preservation as owner
from controller import Blocked
from apply import digest
from stage_runtime import Kube, HUB
from runtime_stage import RolloutStage, terminated_hub


class CohortKube(Kube):
    def __init__(self,session):
        self.session=session
        super().__init__()

    def _run(self,args,body=None,timeout=30):
        if args==['version','-o','json'] and body is None:return owner.version_read(self.session)
        return Kube._run(args,body=body,timeout=timeout)

    def cas(self,kind,row,changes):
        return owner.cut_cas(self.session,kind,row,changes)


class CohortStage(RolloutStage):
    @classmethod
    def from_captured(cls,session,stage,journal):
        if type(stage) is not RolloutStage:raise Blocked('cohort_actual_stage_required')
        obj=cls(CohortKube(session),stage.operation,copy.deepcopy(stage.expected),journal)
        obj.session=session
        for name in ('snapshots','original_snapshots','old_images','marker','service','final_claim','final_pv','final_path'):
            setattr(obj,name,copy.deepcopy(getattr(stage,name)))
        return obj

    def shutdown_original_hub(self):
        """Same retained-Pod orderly-exit proof as RolloutStage, fixed transport."""
        from docker_runtime import DockerRuntime,NATS_IMAGE
        from rollout_census import census
        version=self.kube.run(['version','-o','json'])['serverVersion']
        if int(version['major'])!=1 or int(version['minor'].rstrip('+'))<27:
            raise Blocked('rollout_retained_pod_version_unsupported')
        hub=self.kube.get('deployment',HUB);expected=self.snapshots[HUB]
        if hub['metadata']['uid']!=expected['metadata']['uid'] or hub['spec']['template']!=expected['spec']['template']:
            raise Blocked('rollout_original_hub_changed')
        pods=self.kube.run(['get','pods','-o','json'])['items']
        selected=[p for p in pods if any(v.get('persistentVolumeClaim',{}).get('claimName')==self.expected['source_claim']
            for v in p['spec'].get('volumes',[]))]
        if len(selected)!=1:raise Blocked('rollout_original_hub_inventory_invalid')
        pod=selected[0];meta=pod['metadata'];spec=pod['spec'];statuses=pod['status'].get('containerStatuses',[])
        refs=meta.get('ownerReferences',[])
        if (meta.get('deletionTimestamp') or pod['status'].get('phase')!='Running' or len(refs)!=1
            or refs[0].get('kind')!='ReplicaSet' or refs[0].get('controller') is not True
            or len(spec['containers'])!=1 or spec['containers'][0]['name']!='nats'
            or spec['containers'][0]['image']!=NATS_IMAGE or len(statuses)!=1):
            raise Blocked('rollout_original_hub_inventory_invalid')
        rs=self.kube.get('replicaset',refs[0]['name'])
        if (rs['metadata']['uid']!=refs[0]['uid'] or not any(o.get('kind')=='Deployment'
            and o.get('uid')==hub['metadata']['uid'] and o.get('controller') is True for o in rs['metadata'].get('ownerReferences',[]))):
            raise Blocked('rollout_original_hub_owner_changed')
        current=statuses[0];runtime_image=current.get('imageID','').removeprefix('docker-pullable://')
        fixed_images={NATS_IMAGE,'docker.io/library/nats@'+NATS_IMAGE.split('@',1)[1]}
        if (current.get('name')!='nats' or current.get('ready') is not True
            or not re.fullmatch('containerd://[a-f0-9]{64}',current.get('containerID','')) or runtime_image not in fixed_images
            or type(current.get('restartCount')) is not int or set(current.get('state',{}))!={'running'}):
            raise Blocked('rollout_original_hub_container_changed')
        node=self.kube.get('node',spec['nodeName'])
        if node['metadata'].get('deletionTimestamp') or not any(c.get('type')=='Ready' and c.get('status')=='True' for c in node.get('status',{}).get('conditions',[])):
            raise Blocked('rollout_original_hub_node_not_ready')
        def identity():
            self.owned_marker();live=self.kube.get('pod',meta['name'])
            matches=[s for s in live['status'].get('containerStatuses',[]) if s['name']=='nats']
            if live['metadata']['uid']!=meta['uid'] or live['metadata'].get('deletionTimestamp') or matches!=statuses:
                raise Blocked('rollout_original_hub_changed')
        identity();tree=json.loads(owner.monitor_capture(self.session,meta['name']));identity()
        class Snapshot:
            def monitor_jsz(self,name):return tree
        self.original_live_census=census(Snapshot(),'original',DockerRuntime(self.runtime_base,self.operation).account_id())
        finalizer='voice.io/nats-preservation-'+self.operation;existing=meta.get('finalizers',[])
        if finalizer in existing:raise Blocked('rollout_original_hub_finalizer_already_present')
        self.save({'kind':'original_hub_shutdown_intent','pod_uid':meta['uid'],'container_id':current['containerID'],
            'finalizer':finalizer,'live_census_sha256':digest(self.original_live_census)})
        self.kube.cas('pod',pod,[{'op':'add','path':'/metadata/finalizers','value':existing+[finalizer]}])
        self.scale(HUB,0);deadline=time.monotonic()+90
        while time.monotonic()<deadline:
            self.owned_marker();retained=self.kube.get('pod',meta['name'])
            if retained['metadata']['uid']!=meta['uid'] or finalizer not in retained['metadata'].get('finalizers',[]):
                raise Blocked('rollout_original_hub_retained_pod_changed')
            status=[s for s in retained['status'].get('containerStatuses',[]) if s['name']=='nats']
            if len(status)!=1 or status[0].get('containerID')!=current['containerID'] or status[0].get('restartCount')!=current['restartCount']:
                raise Blocked('rollout_original_hub_container_changed')
            if 'terminated' in status[0].get('state',{}):
                proof=terminated_hub(pod,retained)
                self.save({'kind':'original_hub_orderly_exit_verified','proof':proof})
                finals=retained['metadata']['finalizers'];index=finals.index(finalizer)
                self.kube.cas('pod',retained,[{'op':'test','path':'/metadata/finalizers','value':finals},
                    {'op':'remove','path':'/metadata/finalizers/'+str(index)}])
                self.save({'kind':'original_hub_owned_finalizer_removed','pod_uid':meta['uid']});return proof
            time.sleep(.2)
        raise Blocked('rollout_original_hub_shutdown_timeout')


DOCKER=['/usr/bin/docker','--host=unix:///var/run/docker.sock']


def _resources(session):
    owner._guard(session)
    nonce=owner.restore_attempt(session)
    path=owner._base(session)/'restore'/nonce/'resources.json'
    value=owner._read(path)
    if value.get('operation')!=session.operation or value.get('attempt')!=nonce:
        raise Blocked('cohort_restore_journal_changed')
    return path,value


class RestoreJournal:
    def __init__(self,session):self.session=session

    def begin(self,kind,name,labels):
        path,value=_resources(self.session)
        if (kind not in ('postgres','migrate','story','story-probe') or not re.fullmatch('voice-media-restore-[a-f0-9]{32}',name)
            or labels!={'voice.test.owner':name} or name in value['resources']
            or any(v['status'] in ('PENDING','UNCERTAIN') for v in value['resources'].values())):
            raise Blocked('cohort_restore_resource_invalid')
        nonce=owner.secrets.token_hex(32)[:32]
        value['resources'][name]={'kind':kind,'nonce':nonce,'name':name,'labels':labels,'status':'PENDING','cid':None}
        owner.save(path,value);return nonce

    def created(self,kind,nonce,cid,labels):
        path,value=_resources(self.session);rows=[r for r in value['resources'].values() if r['nonce']==nonce]
        if (len(rows)!=1 or rows[0]['kind']!=kind or rows[0]['cid']!=cid or rows[0]['labels']!=labels
            or rows[0]['status']!='OBSERVED' or not rows[0].get('inspected')):
            raise Blocked('cohort_restore_resource_unverified')
        rows[0]['status']='ACTIVE';owner.save(path,value)

    def begin_remove(self,kind,cid):
        path,value=_resources(self.session);rows=[r for r in value['resources'].values() if r['cid']==cid]
        if len(rows)!=1 or rows[0]['kind']!=kind or rows[0]['status']!='ACTIVE':raise Blocked('cohort_restore_cleanup_invalid')
        rows[0]['status']='REMOVING';owner.save(path,value)

    def removed(self,kind,cid,verified):
        path,value=_resources(self.session);rows=[r for r in value['resources'].values() if r['cid']==cid]
        if len(rows)!=1 or rows[0]['kind']!=kind or rows[0]['status']!='REMOVING' or verified is not True:
            raise Blocked('cohort_restore_cleanup_invalid')
        remaining=restore_command(self.session,DOCKER+['ps','-a','--no-trunc','--format','{{.ID}}']).decode().splitlines()
        volumes=restore_command(self.session,DOCKER+['volume','ls','--format','{{.Name}}']).decode().splitlines()
        if cid in remaining or set(rows[0].get('volumes',[]))&set(volumes):raise Blocked('cohort_restore_cleanup_incomplete')
        rows[0]['status']='REMOVED';owner.save(path,value)

    def uncertain(self,kind,nonce):
        path,value=_resources(self.session)
        for row in value['resources'].values():
            if row['kind']==kind and (nonce is None or row['nonce']==nonce):row['status']='UNCERTAIN'
        owner.save(path,value)


STORY_HEALTH='i=0; while [ "$i" -lt 45 ]; do if wget -q -O - http://127.0.0.1:8080/health; then exit 0; fi; i=$((i+1)); sleep 1; done; exit 1'


def story_create(name,postgres,image,kind,dsn,baseline=0,unrelated=0,mode='positive'):
    """A closed executable envelope; no host paths, mounts or arbitrary commands."""
    if (not re.fullmatch('voice-media-restore-[a-f0-9]{32}',name)
        or not re.fullmatch('[a-f0-9]{64}',postgres) or not re.fullmatch('sha256:[a-f0-9]{64}',image)
        or not isinstance(dsn,str) or not re.fullmatch(r'postgres://[a-z_][a-z0-9_]{0,31}:[a-f0-9]{48}@127\.0\.0\.1:5432/story_db\?sslmode=disable',dsn)
        or kind not in ('story','story-probe') or type(baseline) is not int or not 0<=baseline<2**63
        or type(unrelated) is not int or not 0<=unrelated<2**63 or mode not in ('positive','invalid','wrong')):
        raise Blocked('cohort_story_command_invalid')
    args=['create','--name',name,'--network','container:'+postgres,'--label','voice.test.owner='+name,
        '--user','65532:65532','--read-only','--cap-drop=ALL','--security-opt=no-new-privileges',
        '--pids-limit=128','--memory=512m','--cpus=1']
    if kind=='story':
        if baseline or unrelated or mode!='positive':raise Blocked('cohort_story_command_invalid')
        return args+['-e','DATABASE_URL='+dsn,'-e','LISTEN_ADDR=:8080','-e','STORY_GRPC_LISTEN=:9090',
            '-e','USER_GRPC_ADDR=127.0.0.1:9091','--entrypoint','/service',image]
    return args+['-e','FIXTURE_STORY='+dsn,'-e','FIXTURE_BASELINE_STORIES='+('invalid' if mode=='invalid' else str(baseline)),
        '-e','FIXTURE_BASELINE_UNRELATED='+str(unrelated),'--entrypoint','/tmp/voice-story-restored8-probe',image]+(['wrong'] if mode=='wrong' else [])


def _story_create_check(args,resource,targets,tools):
    try:
        postgres=args[4].removeprefix('container:')
        if (postgres not in targets or targets[postgres]['kind']!='postgres' or targets[postgres]['status']!='ACTIVE'):
            raise ValueError()
        kind=resource['kind'];env={args[i+1].split('=',1)[0]:args[i+1].split('=',1)[1]
            for i,v in enumerate(args[:-1]) if v=='-e'}
        image=args[-2] if args[-1]=='wrong' else args[-1]
        allowed={tools['ids'][key] for key in tools['images'] if re.fullmatch('story[0-9]+',key)}
        if image not in allowed:raise ValueError()
        mode='wrong' if args[-1]=='wrong' else ('invalid' if env.get('FIXTURE_BASELINE_STORIES')=='invalid' else 'positive')
        baseline=0 if mode=='invalid' else int(env.get('FIXTURE_BASELINE_STORIES','0'))
        unrelated=int(env.get('FIXTURE_BASELINE_UNRELATED','0'))
        expected=story_create(args[2],postgres,image,kind,env['DATABASE_URL' if kind=='story' else 'FIXTURE_STORY'],baseline,unrelated,mode)
        if args!=expected:raise ValueError()
    except (IndexError,KeyError,TypeError,ValueError):raise Blocked('cohort_story_create_invalid') from None


def _command(session,argv,body):
    if not isinstance(argv,(list,tuple)) or any(not isinstance(a,str) or '\0' in a for a in argv):
        raise Blocked('cohort_restore_command_invalid')
    args=list(argv)
    if args[:2]==DOCKER:args=args[2:]
    elif args[:1] in (['docker'],['/usr/bin/docker']):args=args[1:]
    else:raise Blocked('cohort_restore_command_invalid')
    path,state=_resources(session)
    if any(v['status']=='UNCERTAIN' for v in state['resources'].values()):raise Blocked('cohort_restore_uncertain')
    targets={r['cid']:r for r in state['resources'].values() if r['cid'] is not None and r['status']!='REMOVED'}
    tools=owner._read(owner._base(session)/'restore-tools.json')
    refs=set(tools['images'].values());image_ids=set(tools.get('ids',{}).values())
    kind=None;resource=None
    if args==['info','--format','{{.OSType}}'] or args==['ps','-a','--no-trunc','--format','{{.ID}}'] or args==['volume','ls','--format','{{.Name}}']:
        if body:raise Blocked('cohort_restore_command_invalid')
    elif len(args)==3 and args[:2]==['image','inspect'] and args[2] in refs:
        if body:raise Blocked('cohort_restore_command_invalid')
        kind='image'
    elif len(args)>=9 and args[0:2]==['create','--name']:
        name=args[2];resource=state['resources'].get(name)
        if resource is None or resource['status']!='PENDING' or resource['cid'] is not None or body:
            raise Blocked('cohort_restore_create_invalid')
        if resource['kind'] in ('story','story-probe'):
            _story_create_check(args,resource,targets,tools)
            archive=owner.load_backup_archive(session)
            metadata=owner.load_metadata(session,archive['metadata_ref'])['source']
            context,_=owner.story_restore_inputs(session,archive['binding'],metadata,args[4].removeprefix('container:'))
            dsn=[args[index+1].split('=',1)[1] for index,value in enumerate(args[:-1])
                if value=='-e' and args[index+1].split('=',1)[0] in ('DATABASE_URL','FIXTURE_STORY')]
            if len(dsn)!=1 or dsn[0].split('://',1)[1].split(':',1)[0]!=context['runtime_user']:
                raise Blocked('cohort_story_runtime_identity_changed')
        else:
            if args[3:7]!=['--network','none','--label','voice.test.owner='+name] or args[-1] not in image_ids:
                raise Blocked('cohort_restore_create_invalid')
            options=args[7:-1]
            if resource['kind']=='migrate':
                if options!=['--entrypoint','/bin/true'] or args[-1]!=tools['ids']['migrate']:raise Blocked('cohort_restore_create_invalid')
            elif (len(options)!=4 or options[::2]!=['-e','-e']
                or not re.fullmatch(r'POSTGRES_USER=voice_cohort_[a-f0-9]{24}',options[1])
                or not re.fullmatch(r'POSTGRES_PASSWORD_FILE=/tmp/voice-cohort-password-[a-f0-9]{32}',options[3])
                or args[-1]!=tools['ids']['postgres']):raise Blocked('cohort_restore_create_invalid')
        kind='create'
    elif len(args)==2 and args[0] in ('inspect','start','wait','logs') and args[1] in targets:
        resource=targets[args[1]];kind=args[0]
        if body or resource['status'] not in ('OBSERVED','ACTIVE','REMOVING'):raise Blocked('cohort_restore_command_invalid')
        if kind in ('wait','logs') and (resource['kind']!='story-probe' or resource['status']!='ACTIVE'):
            raise Blocked('cohort_story_command_invalid')
    elif len(args)==4 and args[:3]==['stop','--time','5'] and args[3] in targets:
        resource=targets[args[3]]
        if body or resource['kind']!='story' or resource['status']!='ACTIVE':raise Blocked('cohort_story_command_invalid')
    elif len(args)==4 and args[:3]==['rm','-f','-v'] and args[3] in targets:
        resource=targets[args[3]]
        if body or resource['status']!='REMOVING':raise Blocked('cohort_restore_cleanup_invalid')
    elif len(args)==3 and args[0]=='cp':
        if args[1]=='-':
            match=re.fullmatch(r'([a-f0-9]{64}):/tmp/',args[2]);cid=match.group(1) if match else None
            if cid not in targets or not body:raise Blocked('cohort_restore_copy_invalid')
            if targets[cid]['kind']=='story-probe':
                import story_fixture
                try:
                    with tarfile.open(fileobj=io.BytesIO(body),mode='r:') as archive:
                        members=archive.getmembers()
                        if (len(members)!=1 or members[0].name!='voice-story-restored8-probe' or not members[0].isfile()
                            or members[0].mode!=0o555 or not 0<members[0].size<=32<<20
                            or hashlib.sha256(archive.extractfile(members[0]).read()).hexdigest()!=story_fixture.PRESERVED_CALLER_SHA256):
                            raise ValueError()
                except (ValueError,tarfile.TarError):raise Blocked('cohort_story_caller_changed') from None
            elif targets[cid]['kind']!='postgres':raise Blocked('cohort_restore_copy_invalid')
        else:
            match=re.fullmatch(r'([a-f0-9]{64}):/usr/local/bin/migrate',args[1]);cid=match.group(1) if match else None
            if cid not in targets or targets[cid]['kind']!='migrate' or args[2]!='-' or body:raise Blocked('cohort_restore_copy_invalid')
        resource=targets[cid]
    elif args and args[0]=='exec':
        tail=args[1:]
        if tail[:1]==['-i']:tail=tail[1:]
        if tail[:2]==['-e','PGOPTIONS=-c search_path=pg_catalog,public']:tail=tail[2:]
        if len(tail)<2 or tail[0] not in targets or targets[tail[0]]['status']!='ACTIVE':raise Blocked('cohort_restore_exec_invalid')
        resource=targets[tail[0]]
        if resource['kind']=='story':
            if body or tail[1:] not in (['sha256sum','/service'],['sh','-ec',STORY_HEALTH]):raise Blocked('cohort_story_exec_invalid')
            return DOCKER+args,path,state,resource,kind
        if resource['kind']!='postgres' or tail[1] not in ('psql','pg_restore','sh','rm'):raise Blocked('cohort_restore_exec_invalid')
        # These commands run only inside a verified network-none anonymous-volume
        # clone; they cannot address a live workload, host path, or Docker socket.
        if tail[1]=='rm' and (len(tail)!=5 or tail[2:4]!=['-f','--'] or not re.fullmatch(r'/tmp/voice-cohort-password-[a-f0-9]{32}',tail[4])):
            # Existing fixed restore uses rm -f without --.
            if len(tail)!=4 or tail[2]!='-f' or not re.fullmatch(r'/tmp/voice-cohort-password-[a-f0-9]{32}',tail[3]):raise Blocked('cohort_restore_exec_invalid')
    else:raise Blocked('cohort_restore_command_invalid')
    return DOCKER+args,path,state,resource,kind


def restore_command(session,argv,*,body=b'',timeout=120):
    if not isinstance(body,bytes) or len(body)>64<<20 or not 0<timeout<=900:raise Blocked('cohort_restore_command_invalid')
    argv,path,state,resource,kind=_command(session,argv,body)
    readiness=('export PGPASSWORD="$POSTGRES_PASSWORD"; '
        'exec psql -h 127.0.0.1 -X -qAt -v ON_ERROR_STOP=1 -U "$1" -d postgres -c "SHOW server_version_num"')
    tail=argv[len(DOCKER):]
    if (len(tail)==7 and tail[:1]==['exec'] and tail[2:6]==['sh','-ec',readiness,'--']
        and resource is not None and resource['kind']=='postgres'
        and re.fullmatch('voice_cohort_[a-f0-9]{24}',tail[6]) and not body):
        # This is the existing shared readiness query, translated only for the
        # owned isolated clone's password-file initialization. A single bounded
        # effect waits for readiness; no failed effect is replayed or adopted.
        script=('export PGPASSWORD="$(cat "$POSTGRES_PASSWORD_FILE")"; i=0; '
            'while [ "$i" -lt 120 ]; do '
            'if v=$(PGCONNECT_TIMEOUT=1 psql -h 127.0.0.1 -X -qAt -v ON_ERROR_STOP=1 '
            '-U "$1" -d postgres -c "SHOW server_version_num" 2>/dev/null); then printf "%s" "$v"; exit 0; fi; '
            'i=$((i+1)); sleep 0.25; done; exit 1')
        argv=DOCKER+['exec',tail[1],'sh','-ec',script,'--',tail[6]];timeout=60
    if len(body)>2<<20:
        tools=owner._read(owner._base(session)/'restore-tools.json')
        fixed_tool=(len(tail)==6 and tail[:2]==['exec','-i'] and tail[3:]==['sh','-ec',
            'cat > /tmp/voice-migrate && chmod 0555 /tmp/voice-migrate'])
        fixed_probe=(len(tail)==3 and tail[:2]==['cp','-'] and resource is not None and resource['kind']=='story-probe')
        if not fixed_probe and (not fixed_tool or hashlib.sha256(body).hexdigest()!=tools.get('migrate_binary_sha256')):
            raise Blocked('cohort_restore_body_limit')
    result=owner._observation(session,'restore',argv,body,limit=80<<20,timeout=timeout)
    if owner._read(path)!=state:raise Blocked('cohort_restore_journal_changed')
    if kind=='image':
        rows=json.loads(result)
        if len(rows)!=1 or rows[0].get('Os')!='linux' or rows[0].get('Architecture')!='amd64' or not re.fullmatch('sha256:[a-f0-9]{64}',rows[0].get('Id','')) or argv[-1] not in rows[0].get('RepoDigests',[]):
            raise Blocked('cohort_restore_image_changed')
        target=owner._base(session)/'restore-tools.json';tools=owner._read(target)
        selected=[key for key,value in tools['images'].items() if value==argv[-1]]
        if len(selected)!=1:raise Blocked('cohort_restore_image_changed')
        tools.setdefault('ids',{})[selected[0]]=rows[0]['Id'];owner.save(target,tools)
    elif kind=='create':
        cid=result.decode('ascii').strip()
        if not re.fullmatch('[a-f0-9]{64}',cid):raise Blocked('cohort_restore_create_unknown')
        resource.update(cid=cid,status='OBSERVED',image=(argv[-2] if argv[-1]=='wrong' else argv[-1]));owner.save(path,state)
        if resource['kind'] in ('story','story-probe'):
            resource.update(network=tail[4],create_args=tail);owner.save(path,state)
    elif kind=='inspect':
        rows=json.loads(result)
        if (len(rows)!=1 or rows[0].get('Id')!=resource['cid'] or rows[0].get('Image')!=resource['image']
            or rows[0].get('Config',{}).get('Labels',{}).get('voice.test.owner')!=resource['name']
            or rows[0].get('HostConfig',{}).get('NetworkMode')!=resource.get('network','none') or rows[0].get('HostConfig',{}).get('Privileged')
            or rows[0].get('HostConfig',{}).get('PortBindings') or rows[0].get('HostConfig',{}).get('Binds')
            or any(v.get('Type')!='volume' for v in rows[0].get('Mounts',[]))):raise Blocked('cohort_restore_identity_changed')
        volumes=rows[0].get('Mounts',[])
        if resource['kind'] in ('story','story-probe'):
            host=rows[0].get('HostConfig',{});config=rows[0].get('Config',{})
            if (config.get('User')!='65532:65532' or host.get('ReadonlyRootfs') is not True
                or host.get('CapDrop')!=['ALL'] or 'no-new-privileges' not in host.get('SecurityOpt',[])):
                raise Blocked('cohort_story_isolation_changed')
        if resource['kind']=='postgres' and (len(volumes)!=1 or volumes[0].get('Destination')!='/var/lib/postgresql/data') or resource['kind']!='postgres' and volumes:
            raise Blocked('cohort_restore_volume_changed')
        resource.update(inspected=True,volumes=[v['Name'] for v in volumes]);owner.save(path,state)
    return result


class NativeRuntime(DockerRuntime):
    """Only private restored NATS copies, never mounting the live selected store."""
    def __init__(self,session,*,successor=None):
        self.session=session
        super().__init__(owner._runtime_base(session),session.operation)
        self.captured={}
        self.successor=successor
        self.unresolved_create=False
        self.unresolved_effect=False

    def _run(self,args,timeout=60,limit=1<<20):
        owner.revalidate_paused(self.session)
        known=set(self.captured)|{row['id'] for row in self.captured.values()}
        creating=None
        if args in (['ps','--filter','label='+LABEL+'='+self.operation,'--format','{{.ID}}'],
            ['ps','-a','--filter','label='+LABEL+'='+self.operation,'--format','{{.ID}}'],
            ['image','inspect',NATS_IMAGE]):pass
        elif len(args)==2 and args[0] in ('inspect','start','wait') and args[1] in known:pass
        elif len(args)==4 and args[:3]==['stop','--time','10'] and args[3] in known:pass
        elif len(args)==3 and args[:2]==['rm','-f'] and args[2] in known:pass
        elif len(args)==6 and args[:3]==['ps','-a','--filter'] and args[3].startswith('id=') and args[3][3:] in known and args[4:]==['--format','{{.ID}}']:pass
        elif (len(args)==8 and args[0]=='exec' and args[1] in known and args[2:7]==['/bin/busybox','wget','-q','-O','-']
            and args[7] in ('http://127.0.0.1:8222/healthz',
                'http://127.0.0.1:8222/jsz?accounts=true&streams=true&consumers=true&config=true&limit=2048')):
            if args[7]=='http://127.0.0.1:8222/healthz':
                # The shared ready() caller retries failed commands. Keep those
                # probes inside one bounded, owned-copy operation instead: a
                # failed/lost terminal response must never replay a PENDING
                # operation through the root transport.
                script=('i=0; while [ "$i" -lt 120 ]; do '
                    'if v=$(/bin/busybox wget -T 1 -q -O - http://127.0.0.1:8222/healthz 2>/dev/null); '
                    'then printf "%s" "$v"; exit 0; fi; i=$((i+1)); /bin/busybox sleep 0.25; done; exit 1')
                args=['exec',args[1],'/bin/busybox','sh','-ec',script];timeout=160
        elif args[:1]==['create']:
            if len(args)<31 or not re.fullmatch('voice-known-'+self.operation+'-[a-z0-9-]{1,32}',args[2]):raise Blocked('cohort_native_command_invalid')
            creating=args[2]
            if creating in self.captured:raise Blocked('cohort_native_replay_forbidden')
            network=args[6]
            if network!='none' and network not in {'container:'+row['id'] for row in self.captured.values()}:raise Blocked('cohort_native_network_invalid')
            prefix=['create','--name',creating,'--label',LABEL+'='+self.operation,'--network',network,
                '--user','65532:65532','--read-only','--env','HOME=/tmp','--env','XDG_CACHE_HOME=/tmp/cache',
                '--workdir','/tmp','--cap-drop','ALL','--security-opt','no-new-privileges:true','--memory','256m',
                '--pids-limit','64','--tmpfs','/tmp:rw,noexec,nosuid,size=16m']
            if args[:len(prefix)]!=prefix:raise Blocked('cohort_native_command_invalid')
            tail=args[len(prefix):]
            while tail[:1]==['--mount']:
                if len(tail)<2:raise Blocked('cohort_native_mount_invalid')
                match=re.fullmatch(r'type=bind,source=([^,]+),target=(/data|/inputs|/out|/kernel|/server.conf)(,readonly)?',tail[1])
                if not match:raise Blocked('cohort_native_mount_invalid')
                path=Path(match[1]);target=match[2];readonly=bool(match[3])
                if path.resolve(strict=True)!=path or not path.is_relative_to(self.base):raise Blocked('cohort_native_mount_invalid')
                if target in ('/inputs','/kernel','/server.conf') and (path!=self.base/target[1:] or not readonly):raise Blocked('cohort_native_mount_invalid')
                if target in ('/data','/out') and readonly:raise Blocked('cohort_native_mount_invalid')
                tail=tail[2:]
            if tail not in (['--entrypoint','/usr/local/bin/nats-server',NATS_IMAGE,'-c','/server.conf'],
                ['--entrypoint','/kernel',NATS_IMAGE,'--phase','census']):raise Blocked('cohort_native_command_invalid')
        else:raise Blocked('cohort_native_command_invalid')
        if creating:self.unresolved_create=True
        self.unresolved_effect=True
        if self.successor is None:
            raw=owner._observation(self.session,'native',DOCKER+list(args),limit=limit,timeout=timeout)
        else:
            raw=owner._successor_native_observation(self.session,self.successor,DOCKER+list(args),limit=limit,timeout=timeout)
        result=raw.decode('utf8').strip()
        if creating:
            if not re.fullmatch('[a-f0-9]{64}',result):raise Blocked('cohort_native_create_unknown')
            self.captured[creating]={'id':result,'name':creating}
            owner._write(owner._base(self.session)/('native-'+creating+'.json'),self.captured[creating])
            self.unresolved_create=False
        self.unresolved_effect=False
        owner.revalidate_paused(self.session)
        return result
