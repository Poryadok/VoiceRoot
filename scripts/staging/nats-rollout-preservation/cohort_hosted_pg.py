"""Disposable-host PostgreSQL transport for public cohort composition.

Only PostgreSQL is real. Kubernetes observations are fixture data; this helper
does not manufacture P2 receipts or bypass production qualification. The caller
owns the Session, the public action and all restore/custody assertions.
"""
import copy
import json
import os
import re
import secrets
import subprocess
import sys
import time


DOCKER=['/usr/bin/docker','--host=unix:///var/run/docker.sock']
ADMIN_USER='voice'
ADMIN_PASSWORD='isolated-test-only'


class HostedPostgres:
    def __init__(self, namespace_uid):
        if sys.platform!='linux' or os.geteuid()!=0 or os.environ.get('RUNNER_ENVIRONMENT')!='github-hosted':
            raise RuntimeError('disposable_hosted_linux_required')
        self.namespace_uid=namespace_uid;self.name='voice-cohort-source-'+secrets.token_hex(12)
        self.cid=None;self.observed_cid=None;self.create_uncertain=False;self.reconciliation_error=None
        self.original_popen=subprocess.Popen
        self.rows,self.slices=self._objects()

    def _docker(self,*args,body=None):
        result=subprocess.run(DOCKER+list(args),input=body,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=120)
        if result.returncode:raise RuntimeError('isolated_postgres_command_failed')
        return result.stdout

    def _reconciliation_required(self):
        return RuntimeError('isolated_postgres_reconciliation_required:'
            f'name={self.name}:cid={self.cid}:observed_cid={self.observed_cid}')

    def _ids(self,selector):
        raw=self._docker('ps','-a','--no-trunc','--filter',selector,'--format','{{.ID}}')
        ids=raw.decode('ascii').splitlines()
        if len(ids)>1 or any(not re.fullmatch('[a-f0-9]{64}',value) for value in ids):
            raise RuntimeError('isolated_postgres_inventory_ambiguous')
        return ids

    def _verify(self,cid):
        rows=json.loads(self._docker('inspect',cid))
        if (not isinstance(rows,list) or len(rows)!=1 or rows[0].get('Id')!=cid
            or rows[0].get('Name')!='/'+self.name
            or rows[0].get('Config',{}).get('Labels',{}).get('voice.test.owner')!=self.name):
            raise RuntimeError('isolated_postgres_identity_changed')

    def _find(self):
        ids=self._ids('name=^/'+self.name+'$')
        if not ids:return None
        self.observed_cid=ids[0]
        self._verify(ids[0])
        return ids[0]

    def _cleanup(self):
        if self.create_uncertain:
            raise RuntimeError('isolated_postgres_create_terminality_unknown')
        named=self._find()
        if self.cid is not None:
            by_id=self._ids('id='+self.cid)
            if by_id and (by_id!=[self.cid] or named!=self.cid):
                raise RuntimeError('isolated_postgres_identity_changed')
            if not by_id and named is not None:
                raise RuntimeError('isolated_postgres_identity_changed')
        elif named is not None:self.cid=named
        if self.cid is not None and named is not None:
            try:self._docker('rm','-f','-v',self.cid)
            except BaseException:
                if not self._ids('id='+self.cid) and self._find() is None:
                    self.cid=None;self.observed_cid=None
                    return
                raise
        if ((self.cid is not None and self._ids('id='+self.cid)) or self._find() is not None):
            raise RuntimeError('isolated_postgres_cleanup_unconfirmed')
        self.cid=None;self.observed_cid=None

    def __enter__(self):
        from auth_user_adoption_runtime import QUALIFIED_TOOLS
        try:
            created=self._docker('create','--name',self.name,'--network','none','--label','voice.test.owner='+self.name,
                '-e','POSTGRES_PASSWORD='+ADMIN_PASSWORD,'-e','POSTGRES_USER='+ADMIN_USER,
                '-e','POSTGRES_INITDB_ARGS=--auth-host=scram-sha-256',QUALIFIED_TOOLS['postgres']).decode().strip()
        except BaseException as error:
            # An empty scan does not prove the daemon stopped processing a
            # lost create request. Preserve the unique name for reconciliation.
            self.create_uncertain=True
            try:self._find()
            except BaseException as scan_error:self.reconciliation_error=scan_error
            raise self._reconciliation_required() from error
        try:
            if not re.fullmatch('[a-f0-9]{64}',created):
                raise RuntimeError('isolated_postgres_create_identity_invalid')
            self.cid=created
            if self._find()!=self.cid:
                raise RuntimeError('isolated_postgres_create_identity_invalid')
            self._docker('start',self.cid)
            deadline=time.monotonic()+60
            while time.monotonic()<deadline:
                result=subprocess.run(DOCKER+['exec',self.cid,'pg_isready','-h','127.0.0.1','-U',ADMIN_USER],
                    stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,timeout=10)
                if result.returncode==0:break
                time.sleep(.2)
            else:raise RuntimeError('isolated_postgres_not_ready')
            self._docker('exec','-i',self.cid,'psql','-X','-q','-v','ON_ERROR_STOP=1','-U',ADMIN_USER,'-d','postgres',
                body=b"CREATE TABLE public.preservation_fixture(id integer PRIMARY KEY, value text); INSERT INTO public.preservation_fixture VALUES(1,'kept');")
            return self
        except BaseException:
            try:self._cleanup()
            except BaseException as cleanup:
                raise self._reconciliation_required() from cleanup
            raise

    def __exit__(self,*_args):
        try:self._cleanup()
        except BaseException as cleanup:
            raise self._reconciliation_required() from cleanup

    def _objects(self):
        def metadata(name,uid):return {'name':name,'namespace':'voice-staging','uid':uid,'resourceVersion':'1'}
        sts='00000000-0000-4000-8000-000000000021';pod='00000000-0000-4000-8000-000000000022'
        pvc='00000000-0000-4000-8000-000000000023';pv='00000000-0000-4000-8000-000000000024'
        rows={('statefulset','voice-postgres'):{'metadata':metadata('voice-postgres',sts)},
            ('pod','voice-postgres-0'):{'metadata':{**metadata('voice-postgres-0',pod),'labels':{'app':'voice-postgres'},
                'ownerReferences':[{'kind':'StatefulSet','uid':sts,'controller':True}]},
                'spec':{'containers':[{'name':'postgres','volumeMounts':[{'name':'data','mountPath':'/var/lib/postgresql/data'}]}],
                    'volumes':[{'name':'data','persistentVolumeClaim':{'claimName':'pgdata-voice-postgres-0'}}]},
                'status':{'podIP':'10.0.0.2','containerStatuses':[{'name':'postgres','state':{'running':{}},
                    'containerID':'containerd://'+'d'*64,'restartCount':0}]}},
            ('persistentvolumeclaim','pgdata-voice-postgres-0'):{'metadata':metadata('pgdata-voice-postgres-0',pvc),
                'spec':{'volumeName':'postgres-pv'},'status':{'phase':'Bound'}},
            ('persistentvolume','postgres-pv'):{'metadata':metadata('postgres-pv',pv),
                'spec':{'claimRef':{'name':'pgdata-voice-postgres-0','namespace':'voice-staging','uid':pvc}},'status':{'phase':'Bound'}},
            ('service','voice-postgres'):{'metadata':metadata('voice-postgres','service'),
                'spec':{'clusterIP':'10.0.0.1','selector':{'app':'voice-postgres'},'ports':[{'port':5432,'targetPort':5432}]}},
            ('endpoints','voice-postgres'):{'metadata':metadata('voice-postgres','endpoints'),
                'subsets':[{'addresses':[{'ip':'10.0.0.2','targetRef':{'kind':'Pod','uid':pod}}],'ports':[{'port':5432}]}]}}
        slices={'items':[{'metadata':metadata('postgres-slice','slice'),'ports':[{'port':5432}],
            'endpoints':[{'addresses':['10.0.0.2'],'conditions':{'ready':True},'targetRef':{'kind':'Pod','uid':pod}}]}]}
        return rows,slices

    def has_get(self,kind,name):return (kind.lower(),name) in self.rows

    def get(self,kind,name):
        import commands
        commands.authority_guard(['/usr/local/bin/k3s','kubectl','--namespace','voice-staging','get',kind,name,'-o','json'])
        return copy.deepcopy(self.rows[(kind.lower(),name)])

    def capture(self,argv,*,body=b'',fallback,**kwargs):
        import commands,resource_claim,staging_cohort_preservation_pg as pg
        def authorize():
            guard=commands.authority_guard;session=getattr(guard,'__self__',None)
            bound=type(session) is resource_claim.Session and getattr(guard,'__func__',None) is resource_claim.Session.check
            privileged=bound and session._prerequisite_readonly_permit is not None
            if bound:
                launch=commands.launch_snapshot(argv,body,kwargs.get('timeout',30),kwargs.get('limit',2<<20),
                    kwargs.get('env'),kwargs.get('pass_fds',()))
                guard(argv,launch=launch)
            else:guard(argv)
            if privileged:
                if commands.authority_guard!=guard:raise RuntimeError('staging_resource_guard_changed')
                session.readonly_current()
        runtime_script=('IFS= read -r PGPASSWORD; export PGPASSWORD; '
            'export PGOPTIONS="-c statement_timeout=15000 -c lock_timeout=5000"; '
            'exec psql -h voice-postgres -X -qAt -v ON_ERROR_STOP=1 -U realtime -d realtime_db -f -')
        if argv==pg.EXEC+['sh','-ceu',runtime_script]:
            if not re.fullmatch(rb'[A-Za-z0-9_-]{32,128}\nSELECT current_user,current_database\(\);\n',body):
                raise RuntimeError('isolated_postgres_runtime_auth_invalid')
            authorize()
            # The private broker has no Kubernetes DNS: retain the exact SQL,
            # identity and credential, changing only the network endpoint.
            local=runtime_script.replace('-h voice-postgres ','-h 127.0.0.1 ')
            return self._docker('exec','-i',self.cid,'sh','-ceu',local,body=body)
        if argv[:len(pg.EXEC)]==pg.EXEC:
            authorize()
            return self._docker('exec','-i',self.cid,*argv[len(pg.EXEC):],body=body)
        identity=pg.owner.PREFIX+pg.IDENTITY_ARGS
        if argv==identity:
            authorize()
            return self._docker('exec',self.cid,*argv[argv.index('--')+1:])
        import realtime_bootstrap
        admin=pg.owner.PREFIX+['exec','voice-postgres-0','-c','postgres','--','sh','-ceu',realtime_bootstrap._ADMIN_IDENTITY]
        if argv==admin and body==b'':
            authorize()
            return self._docker('exec',self.cid,*argv[argv.index('--')+1:])
        if argv[-5:]==['endpointslices','-l','kubernetes.io/service-name=voice-postgres','-o','json']:
            authorize()
            return json.dumps(self.slices).encode()
        return fallback(argv,body=body,**kwargs)

    def popen(self,argv,*args,**kwargs):
        import staging_cohort_preservation_pg as pg
        if argv[:len(pg.EXEC)]==pg.EXEC:
            argv=DOCKER+['exec','-i',self.cid,*argv[len(pg.EXEC):]]
        return self.original_popen(argv,*args,**kwargs)
