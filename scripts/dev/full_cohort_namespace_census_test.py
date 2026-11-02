import copy
import hashlib
from pathlib import Path
import sys
import types
import unittest
from unittest import mock
import yaml

import full_cohort_namespace_census as census


class InfraTest(unittest.TestCase):
    def test_mail_command_and_source_bound_volume(self):
        pod = {'containers': [{'name': 'mail', 'image': 'node:24-alpine',
                               'command': ['node', '/stub/server.mjs'],
                               'volumeMounts': [{'name': 'script', 'mountPath': '/stub', 'readOnly': True}]}],
               'volumes': [{'name': 'script', 'configMap': {'name': 'voice-dev-mail'}}]}
        census.validate_infra('voice-dev-mail', pod, {'server.mjs': 'trusted'}, 'trusted')
        with self.assertRaises(census.CensusError): census.validate_infra('voice-dev-mail', pod, {'server.mjs': 'other'}, 'trusted')
        bad = copy.deepcopy(pod); bad['containers'][0]['command'] = ['sh', '-c', 'unknown']
        with self.assertRaises(census.CensusError): census.validate_infra('voice-dev-mail', bad, {'server.mjs': 'trusted'}, 'trusted')

    def test_unknown_sidecar_init_and_host_network_are_refused(self):
        base = {'containers': [{'name': 'redis', 'image': 'docker.io/library/redis@sha256:' + 'a'*64,
                                'args': ['redis-server', '--appendonly', 'yes']}]}
        census.validate_infra('voice-redis', base)
        for fault in ('sidecar', 'init', 'host'):
            bad = copy.deepcopy(base)
            if fault == 'sidecar': bad['containers'].append({'name': 'foreign', 'image': 'foreign'})
            if fault == 'init': bad['initContainers'] = [{'name': 'foreign'}]
            if fault == 'host': bad['hostNetwork'] = True
            with self.subTest(fault=fault), self.assertRaises(census.CensusError): census.validate_infra('voice-redis', bad)

    def test_nats_rejects_unreviewed_renderer(self):
        pod = {'containers': [{'name':'nats','image':'nats:2.12.12-alpine@sha256:2ca98656a279b2d88cfdf2b8c3f0d5d7f3941ae9dc2ab12ebaa92d83e0f4ccdb',
                               'args':['-c','/etc/nats/nats.conf']}],
               'initContainers':[{'name':'nats-config-renderer','image':'unknown','args':['unsafe']}]}
        with self.assertRaises(census.CensusError): census.validate_infra('voice-nats-pvc-candidate', pod)

    def test_redis_rejects_extra_mount_and_environment(self):
        pod = {'containers':[{'name':'redis','image':'docker.io/library/redis@sha256:'+'a'*64,
                             'args':['redis-server','--appendonly','yes']}]}
        for fault in ('mount','env'):
            bad=copy.deepcopy(pod)
            if fault=='mount': bad['containers'][0]['volumeMounts']=[{'name':'evil','mountPath':'/usr/local/bin'}]
            if fault=='env': bad['containers'][0]['env']=[{'name':'DATABASE_URL','value':'foreign'}]
            with self.subTest(fault=fault), self.assertRaises(census.CensusError): census.validate_infra('voice-redis',bad)

    def test_fixed_namespace_identity_includes_zero_replica_controllers(self):
        known = {'Deployment/voice-' + name: 'app-' + name for name in census.APP_NAMES}
        known.update({kind + '/' + name: 'infra-' + name for kind, name in census.INFRA_NAMES})
        census.validate_workload_names(known)
        bad = dict(known, **{'Deployment/voice-unknown': 'unknown-uid'})
        with self.assertRaises(census.CensusError): census.validate_workload_names(bad)
        bad = dict(known); bad['Deployment/voice-user'] = ''
        with self.assertRaises(census.CensusError): census.validate_workload_names(bad)

    def test_cut_only_allows_source_owned_target_specs(self):
        source = {'workload_uids': {'Deployment/voice-user':'uid'},
                  'workload_spec_sha256': {'Deployment/voice-user':'old'}}
        target = {'replicas': 0}
        current = copy.deepcopy(source)
        current['workload_spec_sha256']['Deployment/voice-user'] = census.digest(target)
        proof = {'writers':[{'kind':'Deployment','name':'voice-user','uid':'uid','spec':target}]}
        census.validate_cut_baseline(source,current,proof)
        current['workload_spec_sha256']['Deployment/voice-user'] = 'foreign'
        with self.assertRaises(census.CensusError): census.validate_cut_baseline(source,current,proof)
        proof['writers'][0]['uid'] = 'foreign'
        with self.assertRaises(census.CensusError): census.validate_cut_baseline(source,current,proof)


class JournalTest(unittest.TestCase):
    def test_persisted_receipt_and_live_snapshot_drift(self):
        self.enterContext(mock.patch.object(census,'APP_NAMES',frozenset()))
        class Journal:
            def __init__(self): self.state={'steps':{}}
            def save(self): pass
        class Ledger:
            def __init__(self): self.row={'status':'active'}; self.rev=1
            def write(self,revision,row):
                if revision!=('uid',str(self.rev)): raise ValueError()
                self.row=copy.deepcopy(row); self.rev+=1
        class Fence:
            def __init__(self): self.context={'operation_id':'op','commit':'source'}; self.ledger=Ledger(); self.uncertain_called=False
            def uncertain(self,reason): self.uncertain_called=True
        class Inventory:
            def verify_current(self,token): return {'package_sha256':'p','projection':{}}
            def verify_receipt(self,token): return {'package_sha256':'p','projection':{}}
        phase=census.NamespaceCensus.__new__(census.NamespaceCensus)
        phase.journal=Journal(); phase.fence=Fence(); phase.inventory=Inventory(); phase.poisoned=False
        sample={'package_sha256':'p','baseline':{},'namespace_uid':'ns','application_pods':{}}
        phase._guard=lambda:(copy.deepcopy(phase.fence.ledger.row),('uid',str(phase.fence.ledger.rev)))
        phase._snapshot=lambda token:copy.deepcopy(sample)
        receipt=phase.run('token')
        self.assertEqual(phase.verify_current('token'),receipt)
        sample['baseline']['changed']=True
        with self.assertRaises(census.CensusError): phase.verify_current('token')

    def test_cut_target_and_pending_lost_ack(self):
        self.enterContext(mock.patch.object(census,'APP_NAMES',frozenset()))
        class Journal:
            def __init__(self): self.state={'steps':{}}
            def save(self): pass
        class Ledger:
            def __init__(self): self.row={'status':'active'}; self.rev=1; self.lose=False
            def read(self): return copy.deepcopy(self.row),('uid',str(self.rev))
            def write(self,revision,row):
                if revision!=('uid',str(self.rev)): raise ValueError()
                self.row=copy.deepcopy(row); self.rev+=1
                if self.lose: raise TimeoutError()
        class Fence:
            def __init__(self): self.context={'operation_id':'op','commit':'source'}; self.ledger=Ledger(); self.uncertain_called=False
            def uncertain(self,reason): self.uncertain_called=True
        class Inventory:
            def verify_current(self,token): return {'package_sha256':'p','projection':{}}
            def verify_receipt(self,token): return {'package_sha256':'p','projection':{}}
        class Cut:
            def __init__(self,fence):
                self.fence=fence; self.lock={}; self.namespace_uid='ns'; self.calls=0
                self.journal=types.SimpleNamespace(poisoned=False,dirfd=1,lockfd=1)
            def current_cut_proof(self):
                self.calls+=1
                return {'writers':[{'kind':'Deployment','name':'voice-user','uid':'u','generation':2,'spec':{'replicas':0}}]}
        phase=census.NamespaceCensus.__new__(census.NamespaceCensus)
        phase.journal=Journal(); phase.fence=Fence(); phase.inventory=Inventory(); phase.poisoned=False
        phase.lock={}; phase.namespace_uid='ns'
        original={'package_sha256':'p','baseline':{'workload_uids':{'Deployment/voice-user':'u','Deployment/voice-web':'w'},
            'workload_spec_sha256':{'Deployment/voice-user':'old','Deployment/voice-web':'old-web'}},
            'generations':{'Deployment/voice-user':1,'Deployment/voice-web':1},'namespace_uid':'ns',
            'application_pods':{'Deployment/voice-user':[{'uid':'user-pod'}],
                                'Deployment/voice-web':[{'uid':'web-pod','node':'node','container_id':'original','restart_count':0}]},
            'configmaps':{'web':{'uid':'config-uid','resourceVersion':'1'}},'jobs':[]}
        sample=copy.deepcopy(original)
        phase._guard=lambda:phase.fence.ledger.read()
        phase._snapshot=lambda token:copy.deepcopy(sample)
        receipt=phase.run('token')
        sample['baseline']['workload_spec_sha256']['Deployment/voice-user']=census.digest({'replicas':0})
        sample['generations']['Deployment/voice-user']=2
        sample['application_pods']['Deployment/voice-user']=[]
        cut=Cut(phase.fence)
        with mock.patch.dict(sys.modules,{'writer_cut':types.SimpleNamespace(WriterCut=Cut)}):
            self.assertEqual(phase.verify_current('token',cut),receipt)
            self.assertGreaterEqual(cut.calls,3)
            web=sample['application_pods']['Deployment/voice-web'][0]
            for field,value in (('uid','foreign'),('node','other'),('container_id','changed'),('restart_count',1)):
                old=web[field]; web[field]=value
                with self.subTest(field=field), self.assertRaises(census.CensusError): phase.verify_current('token',cut)
                web[field]=old
            sample['configmaps']['web']['resourceVersion']='2'
            with self.assertRaises(census.CensusError): phase.verify_current('token',cut)
            sample['configmaps']['web']['resourceVersion']='1'
            sample['jobs'].append({'uid':'new-migration-job'})
            with self.assertRaises(census.CensusError): phase.verify_current('token',cut)
            sample['jobs'].clear()
            sample['generations']['Deployment/voice-user']=3
            with self.assertRaises(census.CensusError): phase.verify_current('token',cut)
        phase2=census.NamespaceCensus.__new__(census.NamespaceCensus)
        phase2.journal=Journal(); phase2.fence=Fence(); phase2.inventory=Inventory(); phase2.poisoned=False
        phase2._guard=lambda:phase2.fence.ledger.read(); phase2._snapshot=lambda token:copy.deepcopy(original)
        phase2.fence.ledger.lose=True
        with self.assertRaises(census.CensusError): phase2.run('token')
        self.assertTrue(phase2.fence.uncertain_called)
        with self.assertRaises(census.CensusError): phase2.run('token')


class SnapshotTest(unittest.TestCase):
    def fixture(self):
        root=Path(__file__).resolve().parents[2]
        loaded=list(yaml.safe_load_all((root/'deploy/staging/infra.yaml').read_text(encoding='utf8')))
        loaded+=list(yaml.safe_load_all((root/'deploy/staging/minio.yaml').read_text(encoding='utf8')))
        infra=[]
        for obj in loaded:
            if (obj.get('kind'),obj.get('metadata',{}).get('name')) in census.INFRA_NAMES:
                infra.append(obj)
        mail={'kind':'Deployment','metadata':{'name':'voice-dev-mail'},'spec':{'replicas':1,'template':{'spec':{
            'containers':[{'name':'mail','image':'node:24-alpine','command':['node','/stub/server.mjs'],
                           'volumeMounts':[{'name':'script','mountPath':'/stub','readOnly':True}]}],
            'volumes':[{'name':'script','configMap':{'name':'voice-dev-mail'}}]}}}}
        infra.append(mail)
        images={'voice-postgres':'docker.io/library/postgres','voice-redis':'docker.io/library/redis',
            'voice-livekit':'docker.io/livekit/livekit-server','voice-clamav':'docker.io/clamav/clamav',
            'voice-clickhouse':'docker.io/clickhouse/clickhouse-server','voice-minio':'ghcr.io/poryadok/voiceroot/minio'}
        for obj in infra:
            name=obj['metadata']['name']; spec=obj['spec']['template']['spec']
            if name in images: spec['containers'][0]['image']=images[name]+'@sha256:'+'a'*64
            if name=='voice-nats-pvc-candidate':
                spec['initContainers'][0]['image']='ghcr.io/poryadok/voiceroot/nats-hub-config-renderer@sha256:'+'a'*64
            if name=='voice-minio': spec['containers'][0]['env'][0]['value']='https://example.invalid'
        apps=[]
        for index,name in enumerate(sorted(census.APP_NAMES)):
            containers=[{'name':name.replace('developer-portal','portal').replace('gameintegration','gameintegration'),
                         'image':'ghcr.io/test/app@sha256:'+'b'*64}]
            if name=='web': containers[0]['env']=[{'name':'WEB_FLAG','valueFrom':{'configMapKeyRef':{'name':'voice-web-config','key':'WEB_FLAG'}}}]
            if index<18: containers.append({'name':'nats-leaf','image':'docker.io/library/nats@sha256:'+'c'*64})
            apps.append({'kind':'Deployment','metadata':{'name':'voice-'+name},
                         'spec':{'replicas':1,'template':{'spec':{'containers':containers}}}})
        rows=apps+infra; deployments=[]; statefulsets=[]; replicas=[]; pods=[]
        def meta(name,uid=None,owner=None):
            m={'name':name,'uid':uid or name+'-uid','namespace':'voice-dev','resourceVersion':'1','generation':1}
            if owner: m['ownerReferences']=[{'controller':True,'kind':owner[0],'name':owner[1],'uid':owner[2]}]
            return m
        for obj in rows:
            kind=obj['kind']; name=obj['metadata']['name']; obj['metadata']=meta(name)
            (deployments if kind=='Deployment' else statefulsets).append(obj)
            if kind=='Deployment':
                rs=meta(name+'-rs',owner=('Deployment',name,obj['metadata']['uid']))
                replicas.append({'kind':'ReplicaSet','metadata':rs}); owner=('ReplicaSet',rs['name'],rs['uid'])
            else: owner=('StatefulSet',name,obj['metadata']['uid'])
            spec=copy.deepcopy(obj['spec']['template']['spec']); spec['nodeName']='node'
            for pvc,claim in {'voice-postgres':('pgdata','pgdata-voice-postgres-0'),
                    'voice-clickhouse':('chdata','chdata-voice-clickhouse-0'),
                    'voice-minio':('data','data-voice-minio-0')}.items():
                if name==pvc: spec.setdefault('volumes',[]).append({'name':claim[0],'persistentVolumeClaim':{'claimName':claim[1]}})
            states=[]
            for c in spec['containers']:
                image='docker.io/library/node@sha256:'+'d'*64 if name=='voice-dev-mail' else 'containerd://sha256:'+'e'*64
                states.append({'name':c['name'],'ready':True,'started':True,'restartCount':0,
                    'containerID':'containerd://'+'f'*64,'imageID':image,'state':{'running':{}}})
            inits=[{'name':c['name'],'restartCount':0,'containerID':'containerd://'+'1'*64,
                    'imageID':'containerd://sha256:'+'2'*64,'state':{'terminated':{'exitCode':0}}} for c in spec.get('initContainers',[])]
            pods.append({'kind':'Pod','metadata':meta(name+'-pod',owner=owner),'spec':spec,
                         'status':{'phase':'Running','containerStatuses':states,'initContainerStatuses':inits}})
        pvcs=[{'kind':'PersistentVolumeClaim','metadata':meta(n),'status':{'phase':'Bound'}} for n in
              ('pgdata-voice-postgres-0','data-voice-minio-0','chdata-voice-clickhouse-0','voice-nats-jsdata')]
        def listing(kind,items): return {'kind':kind+'List','metadata':{'resourceVersion':'2'},'items':items}
        docs={'namespace':{'metadata':meta('voice-dev')},'deployments':listing('Deployment',deployments),
            'statefulsets':listing('StatefulSet',statefulsets),'pods':listing('Pod',pods),
            'replicasets':listing('ReplicaSet',replicas),'pvcs':listing('PersistentVolumeClaim',pvcs),
            'jobs':listing('Job',[]),'daemonsets':listing('DaemonSet',[]),'cronjobs':listing('CronJob',[]),
            'horizontalpodautoscalers':listing('HorizontalPodAutoscaler',[])}
        return root,docs

    def test_real_collector_census_rejects_unknown_controller(self):
        import canonical_application_package as canonical
        import publish_candidate
        root,docs=self.fixture()
        phase=census.NamespaceCensus.__new__(census.NamespaceCensus)
        phase.fence=type('Fence',(),{'context':{'commit':'s'}})(); phase.namespace_uid='voice-dev-uid'
        phase._guard=lambda:None
        phase.package=type('Package',(),{'verify_current':lambda obj,token:{'package_sha256':'p'}})()
        phase._read=lambda resource:copy.deepcopy(docs[resource])
        mail=(root/'docker/verification-stub/server.mjs').read_text(encoding='utf8')
        phase._config=lambda name:{'kind':'ConfigMap','metadata':{'uid':'uid','resourceVersion':'1'},
                                   'data':{'server.mjs':mail} if name=='voice-dev-mail' else {'POSTGRES_USER':'value','nats.conf':'value','livekit.yaml':'value','WEB_FLAG':'value'}}
        phase._secret_metadata=lambda name:{'uid':'uid','resourceVersion':'1','keys':[
            'POSTGRES_PASSWORD','CLICKHOUSE_PASSWORD','MINIO_ROOT_USER','MINIO_ROOT_PASSWORD',
            'operator.jwt','account.jwt','system-account.jwt','account.public','system-account.public',
            'tls.crt','tls.key','ca.crt']}
        source={'source_sha':'s','files':{'docker/verification-stub/server.mjs':hashlib.sha256(mail.encode()).hexdigest()}}
        with mock.patch.object(canonical,'ROOT',root), mock.patch.object(publish_candidate,'source_snapshot',return_value=source):
            result=phase._snapshot('token')
            self.assertEqual(len(result['baseline']['workload_uids']),31)
            docs['daemonsets']['items'].append({'kind':'DaemonSet','metadata':{'name':'foreign','uid':'foreign','resourceVersion':'1'}})
            with self.assertRaises(census.CensusError): phase._snapshot('token')

    def test_real_journal_and_fence_capture_with_synthetic_transport(self):
        import canonical_application_package as canonical
        import frontend_runtime_executor
        import full_cohort_inventory_phase
        import full_cohort_package_phase
        import media_backup_phase_test
        import media_operation_journal
        import publish_candidate
        import resource_fencing
        import writer_cut
        import writer_cut_test
        base=media_backup_phase_test.PhaseTest(); base.setUp(); self.addCleanup(base.doCleanups)
        root,docs=self.fixture(); docs['namespace']['metadata']['uid']='ns'
        journal=media_operation_journal.Journal(Path(base.temp.name)/'census',
            {'resource_context':copy.deepcopy(base.context),'namespace_uid':'ns'})
        self.addCleanup(journal.close)
        package_journal=media_operation_journal.Journal(Path(base.temp.name)/'package',
            {'resource_context':copy.deepcopy(base.context),'namespace_uid':'ns'})
        inventory_journal=media_operation_journal.Journal(Path(base.temp.name)/'inventory',
            {'resource_context':copy.deepcopy(base.context),'namespace_uid':'ns'})
        self.addCleanup(package_journal.close); self.addCleanup(inventory_journal.close)
        client=writer_cut_test.Client(base.lock,journal); base.ledger.client=client
        class Package:
            def __init__(self): self.fence=base.fence; self.lock=base.lock; self.namespace_uid='ns'; self.journal=package_journal
            def verify_current(self,token): return {'package_sha256':'p'}
        package=Package()
        projection={}
        for name in census.APP_NAMES:
            pod=next(p for p in docs['pods']['items'] if p['metadata']['name']=='voice-'+name+'-pod')
            deployment=next(d for d in docs['deployments']['items'] if d['metadata']['name']=='voice-'+name)
            projection[name]={'uid':deployment['metadata']['uid'],'generation':deployment['metadata']['generation'],
                'spec_sha256':census.digest(deployment['spec']),'replicas':deployment['spec']['replicas'],
                'pods':[{'name':pod['metadata']['name'],'uid':pod['metadata']['uid'],
                'replicaset_uid':pod['metadata']['ownerReferences'][0]['uid'],
                'containers':sorted(({'name':s['name'],'container_id':s['containerID'],'image_id':s['imageID'],
                    'restart_count':s['restartCount']} for s in pod['status']['containerStatuses']),key=lambda s:s['name'])}]}
        class Inventory:
            def __init__(self):
                self.fence=base.fence; self.lock=base.lock; self.namespace_uid='ns'; self.package=package; self.journal=inventory_journal
            def verify_current(self,token): return {'package_sha256':'p','projection':copy.deepcopy(projection)}
            def verify_receipt(self,token): return {'package_sha256':'p','projection':copy.deepcopy(projection)}
        inventory=Inventory()
        mail=(root/'docker/verification-stub/server.mjs').read_text(encoding='utf8')
        source={'source_sha':base.context['commit'],
                'files':{'docker/verification-stub/server.mjs':hashlib.sha256(mail.encode()).hexdigest()}}
        with mock.patch.object(frontend_runtime_executor,'KubectlTransport',type(client)), \
             mock.patch.object(resource_fencing,'KubeLedger',type(base.ledger)), \
             mock.patch.object(full_cohort_package_phase,'PackagePhase',Package), \
             mock.patch.object(full_cohort_inventory_phase,'InventoryPhase',Inventory), \
             mock.patch.object(canonical,'ROOT',root), \
             mock.patch.object(publish_candidate,'source_snapshot',return_value=source):
            phase=census.NamespaceCensus(journal,base.fence,client,base.lock,'ns',package,inventory)
            phase._read=lambda resource:copy.deepcopy(docs[resource])
            config_rev={'voice-web-config':'1'}
            phase._config=lambda name:{'kind':'ConfigMap','metadata':{'uid':'uid','resourceVersion':config_rev.get(name,'1')},
                'data':{'server.mjs':mail} if name=='voice-dev-mail' else {'POSTGRES_USER':'value','nats.conf':'value','livekit.yaml':'value','WEB_FLAG':'value'}}
            phase._secret_metadata=lambda name:{'uid':'uid','resourceVersion':'1','keys':[
                'POSTGRES_PASSWORD','CLICKHOUSE_PASSWORD','MINIO_ROOT_USER','MINIO_ROOT_PASSWORD',
                'operator.jwt','account.jwt','system-account.jwt','account.public','system-account.public',
                'tls.crt','tls.key','ca.crt']}
            old_uid=projection['web']['uid']
            projection['web']['uid']='foreign-uid'
            with self.assertRaises(census.CensusError): phase.run('token')
            self.assertNotIn('full_cohort_namespace_census',base.ledger.row)
            projection['web']['uid']=old_uid
            receipt=phase.run('token')
            self.assertEqual(base.ledger.row['full_cohort_namespace_census']['status'],'done')
            self.assertEqual(phase.verify_current('token'),receipt)
            with self.assertRaises(census.CensusError): phase.run('token')
            source=copy.deepcopy(next(x for x in docs['deployments']['items'] if x['metadata']['name']=='voice-user'))
            admission={'controllers':[source],'guards':{'jobs':[],'cronjobs':[],'hpas':[]},
                       'linked':{'jobs':[],'cronjobs':[]},'session_scope':{'databases':['fixture']}}
            cut_journal=media_operation_journal.Journal(Path(base.temp.name)/'cut',
                {'resource_context':copy.deepcopy(base.context),'namespace_uid':'ns','writer_cut':copy.deepcopy(admission)})
            self.addCleanup(cut_journal.close)
            cut_client=writer_cut_test.Client(base.lock,cut_journal)
            cut_client.workloads={('Deployment','voice-user'):copy.deepcopy(source)}
            class LedgerView:
                def __init__(self): self.client=cut_client
                def read(self): return base.ledger.read()
                def write(self,revision,row): return base.ledger.write(revision,row)
            cut_fence=resource_fencing.Fence(base.context,lambda:copy.deepcopy(base.card),LedgerView())
            cut=writer_cut.WriterCut(cut_journal,cut_fence,cut_client,base.lock,'ns',admission,
                lambda scope:{'scope':scope,'active_sessions':0,'active_transactions':0})
            def patch_cut(kind,name,patch):
                current=cut_client.patch(kind,name,patch)
                selected=next(x for x in docs['deployments']['items'] if x['metadata']['name']==name)
                selected['spec']=copy.deepcopy(current['spec']); selected['metadata']=copy.deepcopy(current['metadata'])
                docs['pods']['items']=[p for p in docs['pods']['items'] if p['metadata']['name']!='voice-user-pod']
                return current
            with mock.patch.object(cut,'_patch',side_effect=patch_cut): cut.drain()
            self.assertEqual(phase.verify_current('token',cut),receipt)
            web=next(p for p in docs['pods']['items'] if p['metadata']['name']=='voice-web-pod')
            web['status']['containerStatuses'][0]['containerID']='containerd://'+'9'*64
            with self.assertRaises(census.CensusError): phase.verify_current('token',cut)
            web['status']['containerStatuses'][0]['containerID']='containerd://'+'f'*64
            self.assertEqual(phase.verify_current('token',cut),receipt)
            config_rev['voice-web-config']='2'
            with self.assertRaises(census.CensusError): phase.verify_current('token',cut)
            config_rev['voice-web-config']='1'
            self.assertEqual(phase.verify_current('token',cut),receipt)
            docs['daemonsets']['items'].clear()
            pg=next(p for p in docs['pods']['items'] if p['metadata']['name']=='voice-postgres-pod')
            claim=pg['spec']['volumes'][0]['persistentVolumeClaim']
            claim['claimName']='foreign'
            with self.assertRaises(census.CensusError): phase._snapshot('token')
            claim['claimName']='pgdata-voice-postgres-0'
            pg['spec']['containers'][0]['lifecycle']={'postStart':{'exec':{'command':['foreign']}}}
            with self.assertRaises(census.CensusError): phase._snapshot('token')
            del pg['spec']['containers'][0]['lifecycle']
            phase._config=lambda name:{'kind':'ConfigMap','metadata':{'uid':'uid','resourceVersion':'1'},
                                       'data':{'server.mjs':'foreign'} if name=='voice-dev-mail' else {'POSTGRES_USER':'value','nats.conf':'value','livekit.yaml':'value'}}
            with self.assertRaises(census.CensusError): phase._snapshot('token')
            phase._config=lambda name:{'kind':'ConfigMap','metadata':{'uid':'uid','resourceVersion':'1'},
                'data':{'server.mjs':mail} if name=='voice-dev-mail' else {'POSTGRES_USER':'value','nats.conf':'value','livekit.yaml':'value'},
                'binaryData':{'foreign':'AA=='} if name=='voice-dev-mail' else {}}
            with self.assertRaises(census.CensusError): phase._snapshot('token')
            inventory_journal.close()
            with self.assertRaises(census.CensusError): phase._guard()


if __name__ == '__main__': unittest.main()
