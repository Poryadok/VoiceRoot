"""Disposable-host PostgreSQL transport for public cohort composition.

Only PostgreSQL is real. Kubernetes observations are fixture data; this helper
does not manufacture P2 receipts or bypass production qualification. The caller
owns the Session, the public action and all restore/custody assertions.
"""
import copy
import json
import os
import secrets
import subprocess
import sys
import time


DOCKER=['/usr/bin/docker','--host=unix:///var/run/docker.sock']


class HostedPostgres:
    def __init__(self, namespace_uid):
        if sys.platform!='linux' or os.geteuid()!=0 or os.environ.get('RUNNER_ENVIRONMENT')!='github-hosted':
            raise RuntimeError('disposable_hosted_linux_required')
        self.namespace_uid=namespace_uid;self.name='voice-cohort-source-'+secrets.token_hex(12)
        self.cid=None;self.original_popen=subprocess.Popen
        self.rows,self.slices=self._objects()

    def _docker(self,*args,body=None):
        result=subprocess.run(DOCKER+list(args),input=body,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=120)
        if result.returncode:raise RuntimeError('isolated_postgres_command_failed')
        return result.stdout

    def __enter__(self):
        from auth_user_adoption_runtime import QUALIFIED_TOOLS
        self.cid=self._docker('create','--name',self.name,'--network','none','--label','voice.test.owner='+self.name,
            '-e','POSTGRES_PASSWORD=isolated-test-only','-e','POSTGRES_USER=postgres',QUALIFIED_TOOLS['postgres']).decode().strip()
        try:
            self._docker('start',self.cid)
            deadline=time.monotonic()+60
            while time.monotonic()<deadline:
                result=subprocess.run(DOCKER+['exec',self.cid,'pg_isready','-h','127.0.0.1','-U','postgres'],
                    stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,timeout=10)
                if result.returncode==0:break
                time.sleep(.2)
            else:raise RuntimeError('isolated_postgres_not_ready')
            self._docker('exec','-i',self.cid,'psql','-X','-q','-v','ON_ERROR_STOP=1','-U','postgres','-d','postgres',
                body=b"CREATE TABLE public.preservation_fixture(id integer PRIMARY KEY, value text); INSERT INTO public.preservation_fixture VALUES(1,'kept');")
            return self
        except BaseException:
            self.__exit__(None,None,None)
            raise

    def __exit__(self,*_args):
        if self.cid is not None:
            cid,self.cid=self.cid,None
            self._docker('rm','-f','-v',cid)

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
        import commands,staging_cohort_preservation_pg as pg
        if argv[:len(pg.EXEC)]==pg.EXEC:
            commands.authority_guard(argv)
            return self._docker('exec','-i',self.cid,*argv[len(pg.EXEC):],body=body)
        identity=pg.owner.PREFIX+pg.IDENTITY_ARGS
        if argv==identity:
            commands.authority_guard(argv)
            return self._docker('exec',self.cid,*argv[argv.index('--')+1:])
        if argv[-5:]==['endpointslices','-l','kubernetes.io/service-name=voice-postgres','-o','json']:
            commands.authority_guard(argv)
            return json.dumps(self.slices).encode()
        return fallback(argv,body=body,**kwargs)

    def popen(self,argv,*args,**kwargs):
        import staging_cohort_preservation_pg as pg
        if argv[:len(pg.EXEC)]==pg.EXEC:
            argv=DOCKER+['exec','-i',self.cid,*argv[len(pg.EXEC):]]
        return self.original_popen(argv,*args,**kwargs)
