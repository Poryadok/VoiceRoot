"""C2 source floors and observations; external DEV data is synthetic."""
import copy
from contextlib import ExitStack
import hashlib
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import full_cohort_c2_fresh_baseline as subject
import full_cohort_supervisor as supervisor_module
import media_backup_upload_runner
import media_operation_guard as guard
import media_operation_journal
import frontend_runtime_executor
import resource_fencing
import full_cohort_schema_metadata as metadata


def database_oids():
    result={name:100+index for index,name in enumerate(sorted(set(metadata.OWNERS.values()) | {'gateway_db'}))}
    result['game_integration_db']=22
    return result


def db_observations(engine,start):
    workloads={}
    oids=database_oids()
    for name in metadata.CONTAINERS:
        resolved=name in metadata.OWNERS or name=='gateway'
        result={'binding':{'workload':name,'state':'resolved' if resolved else 'not_observed',
                           'database':metadata.OWNERS.get(name,'gateway_db') if resolved else None}}
        if resolved:
            database=result['binding']['database']
            result.update(metadata={'database':database,'engine':copy.deepcopy(engine),
                                   'session':{'server_start':start,'database_oid':oids[database]}},secondary=[])
        workloads[name]=[{'status':'observed','result':result}]
    return {'workloads':workloads}


class ObservationTests(unittest.TestCase):
    def fixture(self):
        images = {'voice-'+name+'/'+container:'registry/'+name+'@sha256:'+'c'*64
                  for name,container in metadata.CONTAINERS.items()}
        old = {'revision':0, 'state':'held', 'phase':'deploy', 'owner':'scheduled_devops',
               'claim':'old-claim', 'window':'old-window', 'generation':1, 'operation':1,
               'candidate':{'state':'known','commit':'a'*40,'images':images}}
        deployed = copy.deepcopy(old)
        deployed.update(revision=1,phase='verify',observed={'state':'known','actor':'scheduled_devops',
            'claim':'old-claim','window':'old-window','operation':1,'commit':'a'*40,
            'images':images,'deployment':'old-deployment'})
        current = copy.deepcopy(deployed)
        current.update(code='DEV',environment='dev',revision=2,generation=2,state='held',phase='deploy',
                       owner='scheduled_devops',claim='new-claim',window='new-window',operation=3,
                       candidate={'state':'known','commit':'b'*40,'images':images},
                       observed={'state':'known','actor':'scheduled_devops','claim':'new-claim',
                                 'window':'new-window','operation':2,'images':images})
        history = current | {'events':[{'id':'r1','kind':'deployment','actor':'scheduled_devops',
                                       'before':old,'after':deployed},
                                      {'id':'r2','kind':'observe','before':copy.deepcopy(deployed),
                                       'after':copy.deepcopy(current)}]}
        context = {'actor':'scheduled_devops','claim':'new-claim','window':'new-window'}
        return context,current,{'namespace_uid':'ns','images':copy.deepcopy(images)},history,{'previous_commit':'a'*40}

    def test_image_only_remains_unknown_and_original_source_floor_is_retained(self):
        context,card,actual,history,predecessor = self.fixture()
        floors = subject._observation_floors(context,card,actual,history,predecessor)
        self.assertEqual(floors,['a'*40])
        self.assertNotIn('commit',card['observed'])
        card['observed']['commit'] = 'e'*40
        history['events'][-1]['after']['observed']['commit'] = 'e'*40
        self.assertEqual(subject._observation_floors(context,card,actual,history,predecessor),['a'*40,'e'*40])

    def test_mismatch_unknown_and_missing_predecessor_fail_closed(self):
        for fault in ('claim','operation','images','missing-old-floor','unknown','history-gap','tip','bad-commit',
                      'unattributed-image','partial-deployment'):
            with self.subTest(fault=fault):
                context,card,actual,history,predecessor = self.fixture()
                if fault == 'claim': card['observed']['claim']='foreign'
                if fault == 'operation': card['observed']['operation']=3
                if fault == 'images': actual['images']['voice-auth/auth']='different'
                if fault == 'missing-old-floor': predecessor['previous_commit']='f'*40
                if fault == 'unknown': card['observed']['state']='unknown'
                if fault == 'history-gap': history['events'][1]['id']='r3'
                if fault == 'tip': history['events'][1]['after']['window']='foreign'
                if fault == 'bad-commit': card['observed']['commit']='unverified'
                if fault == 'unattributed-image':
                    actual['images']['voice-auth/auth']='registry/auth@sha256:'+'e'*64
                    card['observed']['images']=copy.deepcopy(actual['images'])
                    history['events'][-1]['after']['observed']['images']=copy.deepcopy(actual['images'])
                if fault == 'partial-deployment':
                    del history['events'][0]['after']['observed']['images']['voice-auth/auth']
                    del history['events'][0]['after']['candidate']['images']['voice-auth/auth']
                    history['events'][1]['before']=copy.deepcopy(history['events'][0]['after'])
                with self.assertRaises(subject.C2BaselineError):
                    subject._observation_floors(context,card,actual,history,predecessor)

    def test_later_qualified_deployment_adds_floor_without_requiring_old_image(self):
        context,card,actual,history,predecessor=self.fixture()
        # A genuine later deployment has its own source and new image; its
        # event is continuous and the fresh 23-app map follows that event.
        updated=copy.deepcopy(actual['images']); updated['voice-auth/auth']='registry/auth@sha256:'+'e'*64
        card['candidate']['images']=copy.deepcopy(updated)
        history['events'][-1]['after']['candidate']['images']=copy.deepcopy(updated)
        before=copy.deepcopy(history['events'][-1]['after'])
        deployed=copy.deepcopy(before)
        deployed.update(revision=3,phase='verify',observed={'state':'known','actor':context['actor'],
            'claim':context['claim'],'window':context['window'],'operation':3,'commit':'b'*40,
            'images':copy.deepcopy(updated),'deployment':'later-deployment'})
        history['events'].append({'id':'r3','kind':'deployment','actor':context['actor'],
                                  'before':before,'after':deployed})
        current=copy.deepcopy(deployed); current.update(revision=4,phase='deploy',operation=4,
            observed={'state':'known','actor':context['actor'],'claim':context['claim'],
                      'window':context['window'],'operation':3,'images':copy.deepcopy(updated)})
        history['events'].append({'id':'r4','kind':'observe','before':copy.deepcopy(deployed),'after':copy.deepcopy(current)})
        history['revision']=4; actual['images']=updated
        self.assertEqual(subject._observation_floors(context,current,actual,history,predecessor),['a'*40,'b'*40])

    def test_current_catalog_is_not_compared_to_original_c1_database_oid(self):
        engine = {'namespace_uid':'ns', 'system_identifier':'new-system'}
        projection = {name:{'uid':'uid-'+name, 'spec_sha256':'d'*64} for name in metadata.CONTAINERS}
        actual = {'namespace_uid':'ns', 'workload_uids':{'Deployment/voice-'+name:'uid-'+name for name in projection},
                  'workload_spec_sha256':{'Deployment/voice-'+name:'d'*64 for name in projection}}
        inventory = {'projection':projection,'database_observations':db_observations(engine,'new-start')}
        catalog = {'namespace_uid':'ns','engine':engine,'server_start':'new-start',
                   'all_databases':database_oids(),'databases':database_oids()}
        subject._join_current('ns',actual,inventory,catalog)
        chat=inventory['database_observations']['workloads']['chat'][0]
        chat['result']['secondary']=[{'owner_consistency':'matched','metadata':{'database':'space_db',
            'engine':copy.deepcopy(engine),'session':{'server_start':'new-start','database_oid':database_oids()['space_db']}}}]
        inventory['database_observations']['workloads']['chat'].append(copy.deepcopy(chat))
        subject._join_current('ns',actual,inventory,catalog)
        for fault in ('uid','spec','engine','namespace','database','hold','unresolved-owner','secondary','observed-oid'):
            with self.subTest(fault=fault):
                a,i,c = copy.deepcopy(actual),copy.deepcopy(inventory),copy.deepcopy(catalog)
                if fault == 'uid': a['workload_uids']['Deployment/voice-auth']='foreign'
                if fault == 'spec': a['workload_spec_sha256']['Deployment/voice-auth']='f'*64
                if fault == 'engine': c['engine']['system_identifier']='foreign'
                if fault == 'namespace': c['namespace_uid']='foreign'
                if fault == 'database': c['databases']['game_integration_db']=23
                if fault == 'hold': i['database_observations']['workloads']['auth'][0]={'status':'hold','reason':'unknown'}
                if fault == 'unresolved-owner': i['database_observations']['workloads']['auth'][0]['result']['binding']['state']='not_observed'
                if fault == 'secondary': i['database_observations']['workloads']['chat'][0]['result']['secondary']=[{'owner_consistency':'hold'}]
                if fault == 'observed-oid': i['database_observations']['workloads']['auth'][0]['result']['metadata']['session']['database_oid']=999
                with self.assertRaises(subject.C2BaselineError): subject._join_current('ns',a,i,c)


class TypedPhaseTests(unittest.TestCase):
    def test_confirmed_c2_current_oid_and_lost_response(self):
        context,card,actual,history,old = ObservationTests().fixture()
        context.update(project='voice',resource='DEV',generation=2,revision=2,
                       operation_id='media-c2-'+'b'*32,commit='b'*40,
                       claim='11111111-1111-4111-8111-111111111111',
                       window='22222222-2222-4222-8222-222222222222',
                       images=copy.deepcopy(card['candidate']['images']))
        card.update(owner=context['actor'],claim=context['claim'],window=context['window'])
        card['observed'].update(claim=context['claim'],window=context['window'])
        history['events'][-1]['after']=copy.deepcopy(card)
        old['historical_database_oid']=11
        lock={'metadata':{'name':guard.LOCK,'namespace':'voice-dev','uid':'lock','resourceVersion':'4',
                          'labels':dict(guard.LABELS)},
              'data':{'resource_operation':context['operation_id'],'operation':'run',
                      'resource_context':json.dumps(context,sort_keys=True,separators=(',',':'))}}
        namespace={'metadata':{'name':'voice-dev','uid':'ns','labels':dict(guard.LABELS)}}
        captured={'previous':'controlled historical boundary'}
        descriptor={'snapshot_sha256':subject.predecessor_module.digest(captured)}
        admission={'status':'done','descriptor':descriptor,'old_revision':['ledger','1'],'new_revision':['ledger','2']}
        row=dict(context,status='active',step='admitted',predecessor=copy.deepcopy(descriptor))
        document={'metadata':{'name':resource_fencing.LEDGER,'namespace':'voice-dev','uid':'ledger','resourceVersion':'2'},
                  'data':{'executor':json.dumps(row)}}
        writes=[]
        def call(*args,**kwargs):
            if args[0]=='patch':
                body=kwargs['body']; writes.append(copy.deepcopy(body))
                if body[0]['value']!='ledger' or body[1]['value']!=document['metadata']['resourceVersion']:
                    raise RuntimeError('CAS')
                document['data']=copy.deepcopy(body[2]['value'])
                document['metadata']['resourceVersion']=str(int(document['metadata']['resourceVersion'])+1)
                return copy.deepcopy(document)
            if args[1]=='namespace': return copy.deepcopy(namespace)
            if args[2]==guard.LOCK: return copy.deepcopy(lock)
            return copy.deepcopy(document)
        nodes=[{'name':'node','uid':'node-uid','hostname':'node','architecture':'amd64'}]
        # Kubernetes, TT, historic custody and SQL observations are controlled.
        # Production phase types, Supervisor admission, receipt verification,
        # journal durability and the ledger CAS are exercised.
        with tempfile.TemporaryDirectory() as root, patch.object(media_backup_upload_runner,'ROOT',Path(root)), \
             patch.object(resource_fencing.ResourceCheck,'__call__',return_value=card), \
             patch.object(subject.predecessor_module,'_no_tt_proxy'):
            op_root=Path(root)/context['operation_id']; op_root.mkdir(mode=0o700)
            root_intent={'resource_context':context,'namespace_uid':'ns','cohort_route_mode':'full-cohort-v1'}
            with media_operation_journal.Journal(op_root/'journal',root_intent) as root_journal, \
                 media_operation_journal.Journal(op_root/'package',{'resource_context':context,'namespace_uid':'ns',
                     'full_cohort_package':{'run_id':1,'run_attempt':1,'nodes':nodes}}) as pj, \
                 media_operation_journal.Journal(op_root/'inventory',{'resource_context':context,'namespace_uid':'ns'}) as ij, \
                 media_operation_journal.Journal(op_root/'catalog',{'resource_context':context,'namespace_uid':'ns'}) as cj, \
                 media_operation_journal.Journal(op_root/'census',{'resource_context':context,'namespace_uid':'ns'}) as nj, \
                 media_operation_journal.Journal(op_root/'fresh',{'resource_context':context,'namespace_uid':'ns'}) as fj:
                check=resource_fencing.ResourceCheck(subject.predecessor_module.TT_ENDPOINT,context)
                def facade(journal):
                    client=frontend_runtime_executor.KubectlTransport.__new__(frontend_runtime_executor.KubectlTransport)
                    client.journal=journal; client.call=call
                    return client,resource_fencing.Fence(context,check,resource_fencing.KubeLedger(client),check.mark_uncertain)
                rc,rf=facade(root_journal); pc,pf=facade(pj); ic,inf=facade(ij); cc,cf=facade(cj); nc,nf=facade(nj); fc,ff=facade(fj)
                root_journal.state['steps']['predecessor-admission']=copy.deepcopy(admission)
                root_journal.state['steps']['full-cohort-predecessor']={'status':'captured','descriptor':descriptor,'snapshot':captured}
                root_journal.save()
                sup=supervisor_module.Supervisor(root_journal,rf,rc,lock,'ns')
                raw=b'{}'; (op_root/'canonical-application-package.json').write_bytes(raw)
                (op_root/'canonical-application-package.json').chmod(0o600)
                package_receipt={'operation_id':context['operation_id'],'source_sha':context['commit'],
                    'images':context['images'],'nodes':nodes,'run_id':1,'run_attempt':1,
                    'package_sha256':hashlib.sha256(raw).hexdigest()}
                package_marker={'status':'done','receipt_sha256':subject._digest(package_receipt)}
                pj.state['steps'][subject.full_cohort_package_phase.STEP]=package_marker | {'receipt':package_receipt}; pj.save()
                sup.package=subject.full_cohort_package_phase.PackagePhase(pj,pf,pc,lock,'ns')
                sup.inventory=subject.full_cohort_inventory_phase.InventoryPhase(ij,inf,ic,lock,'ns',sup.package)
                sup.catalog=subject.full_cohort_affected_plan.FullCatalogPhase(cj,cf,cc,lock,'ns',sup.inventory)
                sup.census=subject.full_cohort_namespace_census.NamespaceCensus(nj,nf,nc,lock,'ns',sup.package,sup.inventory)
                engine={'namespace_uid':'ns','system_identifier':'new-system'}; start='new-start'
                projection={name:{'uid':'uid-'+name,'spec_sha256':'d'*64} for name in metadata.CONTAINERS}
                actual.update(workload_uids={'Deployment/voice-'+name:'uid-'+name for name in projection},
                              workload_spec_sha256={'Deployment/voice-'+name:'d'*64 for name in projection})
                dbobs=db_observations(engine,start)
                inventory_receipt={'operation_id':context['operation_id'],'source_sha':context['commit'],
                    'package_sha256':package_receipt['package_sha256'],'pods':{},'projection':projection,
                    'database_observations':dbobs}
                inventory_marker={'status':'done','nonce':'1'*32,'operation_id':context['operation_id'],
                                  'source_sha':context['commit'],'receipt_sha256':subject._digest(inventory_receipt)}
                ij.state['steps'][subject.full_cohort_inventory_phase.STEP]=inventory_marker | {'receipt':inventory_receipt}; ij.save()
                catalog_receipt={'operation_id':context['operation_id'],'source_sha':context['commit'],'namespace_uid':'ns',
                    'scope':'all-databases-connectable-backup','engine':engine,'server_start':start,
                    'all_databases':database_oids(),'databases':database_oids(),'excluded':{},'properties':{}}
                catalog_marker={'status':'done','nonce':'2'*32,'operation_id':context['operation_id'],
                                'receipt_sha256':subject._digest(catalog_receipt)}
                cj.state['steps']['full-cohort-db-catalog']=catalog_marker | {'receipt':catalog_receipt}; cj.save()
                census_snapshot={'baseline':actual}
                census_receipt={'operation_id':context['operation_id'],'inventory_sha256':subject._digest(inventory_receipt),
                                **census_snapshot}
                census_marker={'status':'done','nonce':'3'*32,'operation_id':context['operation_id'],
                               'source_sha':context['commit'],'receipt_sha256':subject._digest(census_receipt)}
                nj.state['steps'][subject.full_cohort_namespace_census.STEP]=census_marker | {'receipt':census_receipt}; nj.save()
                current,rev=rf.ledger.read(); current.update(full_cohort_package=package_marker,
                    full_cohort_inventory=inventory_marker,full_cohort_db_catalog=catalog_marker,
                    full_cohort_namespace_census=census_marker); rf.ledger.write(rev,current)
                comparison={'base_commit':{'sha':'a'*40},'merge_base_commit':{'sha':'a'*40},'status':'ahead','ahead_by':1}
                with ExitStack() as stack:
                    stack.enter_context(patch.object(subject.full_cohort_package_phase,'ROOT',Path(root)))
                    stack.enter_context(patch.object(subject.predecessor_module,'load',return_value=old))
                    stack.enter_context(patch.object(subject.config_custody,'load_predecessor',return_value={'controlled':'config'}))
                    stack.enter_context(patch.object(subject.tool_custody,'reuse',return_value={'controlled':'tool'}))
                    stack.enter_context(patch.object(subject.full_cohort_namespace_census.NamespaceCensus,'_snapshot',return_value=census_snapshot))
                    stack.enter_context(patch.object(subject.image_only,'_read_history',return_value=history))
                    stack.enter_context(patch.object(subject.full_cohort_inventory_phase,'inventory',return_value=({}, {}, projection)))
                    stack.enter_context(patch.object(subject.metadata,'catalog_from_source',return_value={}))
                    stack.enter_context(patch.object(subject.metadata,'inspect_all',return_value=dbobs))
                    stack.enter_context(patch.object(subject.full_cohort_inventory_phase.InventoryPhase,'_admin_role',return_value='observer'))
                    stack.enter_context(patch.object(subject.full_cohort_affected_plan,'_full_catalog',return_value={
                        k:catalog_receipt[k] for k in ('all_databases','databases','excluded','properties')}))
                    head=stack.enter_context(patch.object(subject.canonical.source,'_head'))
                    compare=stack.enter_context(patch.object(subject.canonical.source,'_get',return_value=comparison))
                    stack.enter_context(patch.object(subject.full_cohort_package_phase.PackagePhase,'_nodes'))
                    phase=subject.C2FreshBaseline(sup,fj,ff,fc)
                    before=len(writes)
                    head.side_effect=ValueError('master moved')
                    with self.assertRaisesRegex(ValueError,'master moved'): phase.run('fixture-token')
                    head.side_effect=None
                    compare.return_value=dict(comparison,status='diverged')
                    with self.assertRaises(subject.C2BaselineError): phase.run('fixture-token')
                    compare.return_value=comparison
                    self.assertEqual(len(writes),before)
                    proof=phase.run('fixture-token')
                    self.assertEqual(proof['catalog']['databases']['game_integration_db'],22)
                    self.assertEqual(phase.verify_current('fixture-token'),proof)
                    self.assertEqual(old['historical_database_oid'],11)
                    root_journal.state['steps']['predecessor-admission']['status']='pending'
                    with self.assertRaises(supervisor_module.SupervisorError): phase.verify_current('fixture-token')
                    root_journal.state['steps']['predecessor-admission']['status']='done'; root_journal.save()
                    current,rev=ff.ledger.read(); del current[subject.MARKER]; ff.ledger.write(rev,current)
                    del fj.state['steps'][subject.STEP]; fj.save()
                    original_write=ff.ledger.write
                    def lose_done(revision,value):
                        original_write(revision,value)
                        if value.get(subject.MARKER,{}).get('status')=='done': raise OSError('lost done reply')
                    retry=subject.C2FreshBaseline(sup,fj,ff,fc)
                    with patch.object(ff.ledger,'write',side_effect=lose_done), \
                         patch.object(ff,'uncertain',side_effect=OSError('uncertainty failed')):
                        with self.assertRaisesRegex(OSError,'lost done reply'): retry.run('fixture-token')
                    self.assertTrue(retry.poisoned)
                    self.assertEqual(fj.state['steps'][subject.STEP]['marker']['status'],'pending')
                    reopened=subject.C2FreshBaseline(sup,fj,ff,fc)
                    with self.assertRaises(subject.C2BaselineError): reopened.verify_current('fixture-token')
                    moved=op_root/'package-moved'; pj.path.rename(moved)
                    try:
                        with self.assertRaises((OSError,subject.tool_identity.ToolIdentityError)):
                            reopened._local()
                    finally:
                        moved.rename(pj.path)


if __name__ == '__main__': unittest.main()
