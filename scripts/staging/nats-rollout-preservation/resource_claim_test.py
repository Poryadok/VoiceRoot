"""Pure host tests: no tracker writes, subprocesses or stand access."""
import copy
import json
import os
import tempfile
import sys
from pathlib import Path
import unittest
from unittest.mock import Mock, patch

sys.path.insert(0,str(Path(__file__).resolve().parents[1]/'nats-known-baseline'))

import resource_claim as claim


def context():
    return dict(project='voice',resource='STAGING',actor='operator',
        claim='11111111-1111-4111-8111-111111111111',generation=7,
        window='22222222-2222-4222-8222-222222222222',revision=12,
        operation_id='stg-run-17',commit='c'*40,
        images={'voice-user/user':'registry/user@sha256:'+'c'*64})


def card(c):
    return dict(code='STAGING',environment='stg',state='held',phase='deploy',owner=c['actor'],
        claim=c['claim'],generation=c['generation'],window=c['window'],revision=c['revision'],operation=2,
        candidate=dict(state='known',commit=c['commit'],images=copy.deepcopy(c['images'])),
        observed=dict(state='known',claim=c['claim'],window=c['window'],operation=1,
            images={'voice-user/user':'registry/user@sha256:'+'b'*64}))


def scoped_target(c):
    return {'registry':'registry','tag':c['commit'],'mode':'full','changed_services':[],
        'source_hashes':{'source':'1'*64},'template_hashes':{'voice-user':'2'*64},
        'input_template_hashes':{'voice-user':'3'*64},'images':copy.deepcopy(c['images']),
        'manifest_sha256':'4'*64,'migration_sha256':'5'*64,'nonnats_sha256':'6'*64,
        'installation_sha256':claim.production_installation().sha256}


class MemoryJournal:
    def __init__(self): self.row=None; self.events=[];self.supersessions=[]
    def read(self): return copy.deepcopy(self.row)
    def save(self,row): self.row=copy.deepcopy(row);self.events.append(copy.deepcopy(row))
    def supersede(self,*args):self.supersessions.append(copy.deepcopy(args))


class ResourceTests(unittest.TestCase):
    def test_captured_prerequisites_allow_only_postgres_endpoint_slice_observation(self):
        session,current,journal=self.fixture();session.admit(start=True)
        journal.row.update(phase='prerequisites-captured',target_sha256='d'*64)
        session.expected_target='d'*64
        prefix=['/usr/local/bin/k3s','kubectl','--namespace','voice-staging']
        query=prefix+['get','endpointslices','-l','kubernetes.io/service-name=voice-postgres','-o','json']
        session.check(query)
        foreign=query.copy();foreign[3]='voice-other'
        for changed in (query[:-3]+['kubernetes.io/service-name=other','-o','json'],query+['extra'],query[1:],foreign):
            with self.assertRaisesRegex(RuntimeError,'staging_prerequisite_command_forbidden'):
                session.check(changed)
        journal.row['phase']='prerequisites'
        with self.assertRaisesRegex(RuntimeError,'staging_prerequisite_command_forbidden'):session.check(query)
        journal.row.update(phase='prerequisites-captured',target_sha256=None);session.expected_target=None
        with self.assertRaisesRegex(RuntimeError,'staging_executor_fence_closed_or_changed'):session.check(query)

    def test_cohort_target_requires_exact_root_qualified_receipt(self):
        import sys
        from unittest.mock import Mock
        c=context();session=claim.Session(c,'a'*12,'b'*64,lambda:card(c),MemoryJournal())
        receipt={'qualified':'fixture'};adapter=Mock();adapter.load_qualified.return_value=receipt
        with patch.dict(sys.modules,{'staging_cohort_preservation':adapter}):
            row={'cohort_preservation_sha256':'c'*64}
            session._validate_cohort_target(row,{'cohort_preservation_sha256':claim.digest(receipt)})
            with self.assertRaisesRegex(RuntimeError,'staging_cohort_target_changed'):
                session._validate_cohort_target(row,{'cohort_preservation_sha256':'f'*64})
            with self.assertRaisesRegex(RuntimeError,'staging_cohort_target_changed'):
                session._validate_cohort_target({}, {'cohort_preservation_sha256':claim.digest(receipt)})

    def test_cohort_binding_rejects_unbound_effects_and_target_replacement(self):
        import sys
        from unittest.mock import Mock
        c=context();authority=card(c);journal=MemoryJournal()
        session=claim.Session(c,'a'*12,'b'*64,lambda:copy.deepcopy(authority),journal)
        session.admit(start=True)
        plan={'actual_before':copy.deepcopy(authority['observed']['images']),
              'candidate':{'commit':c['commit'],'images':copy.deepcopy(c['images'])}}
        adapter=Mock()
        identity=Mock();identity.IDENTITY_ARGS=['exec','voice-postgres-0','-c','postgres','--','fixed-identity']
        with patch.dict(sys.modules,{'staging_cohort_preservation':adapter,'staging_cohort_preservation_pg':identity}):
            bad=copy.deepcopy(plan);bad['candidate']['commit']='d'*40
            before=copy.deepcopy(journal.row)
            with self.assertRaisesRegex(RuntimeError,'staging_cohort_baseline_changed'):
                session.bind_cohort_preservation(bad,plan['actual_before'])
            self.assertEqual(journal.row,before)
            binding=session.bind_cohort_preservation(plan,plan['actual_before'])
            self.assertEqual(binding['target_sha256'],claim.digest(plan))
            self.assertEqual(session.check()['phase'],'cohort-preservation')
            self.assertEqual(session.check()['cohort_preservation_sha256'],claim.digest(plan))
            saved=copy.deepcopy(journal.row);del journal.row['cohort_preservation_sha256']
            with self.assertRaisesRegex(RuntimeError,'staging_cohort_lineage_invalid'):session.check()
            journal.row=saved
            argv=['/usr/local/bin/k3s','kubectl','--namespace','voice-staging','scale','deployment/voice-user','--replicas=0']
            with self.assertRaisesRegex(RuntimeError,'staging_prerequisite_command_forbidden'):session.check(argv)
            session._cohort_permit={'effect_id':'scale-user','argv':argv,'body_sha256':'invalid'}
            with self.assertRaisesRegex(RuntimeError,'staging_prerequisite_command_forbidden'):session.check(argv)
            session._cohort_permit={'effect_id':'scale-user','argv':argv,'body_sha256':'c'*64}
            with self.assertRaisesRegex(RuntimeError,'staging_prerequisite_command_forbidden'):
                session.check(argv[:-1]+['--replicas=1'])
            session.check(argv);self.assertIsNone(session._cohort_permit)
            with self.assertRaisesRegex(RuntimeError,'staging_prerequisite_command_forbidden'):session.check(argv)
            auth=['docker','inspect','owned-fixture']
            session._cohort_auth_permit={'effect_id':'actor-inspect','argv':auth,'body_sha256':'d'*64}
            with self.assertRaisesRegex(RuntimeError,'staging_cohort_auth_permit_invalid'):session.check(auth+['other'])
            session.check(auth);self.assertIsNone(session._cohort_auth_permit)
            with self.assertRaisesRegex(RuntimeError,'staging_prerequisite_command_forbidden'):session.check(auth)
            prefix=['/usr/local/bin/k3s','kubectl','--namespace','voice-staging','get']
            for kind in ('Secret','ConfigMap','Deployment','StatefulSet','Service','PersistentVolumeClaim','Job','Pod'):
                session.check(prefix+[kind,'voice-test','-o','json'])
            for bad in (prefix+['ClusterRole','voice-test','-o','json'],
                        prefix+['Secret','voice-test','-o','yaml'],
                        prefix+['Secret','voice-test','-o','json','--context=other'],
                        prefix+['clusterroles','voice-test','-o','json'],
                        prefix+['secrets','-o','json'],prefix+['clusterroles','-o','json']):
                with self.assertRaisesRegex(RuntimeError,'staging_prerequisite_command_forbidden'):session.check(bad)
            for kind in ('namespace','configmap','deployment','pvc','pv','node','pod','replicaset','service','secret'):
                session.check(prefix+[kind,'voice-test','-o','json'])
            for kind,name in (('statefulset','voice-postgres'),('persistentvolumeclaim','pgdata-voice-postgres-0'),
                              ('persistentvolume','pvc-1234'),('endpoints','voice-postgres')):
                session.check(prefix+[kind,name,'-o','json'])
            endpoint_slice=prefix+['endpointslices','-l','kubernetes.io/service-name=voice-postgres','-o','json']
            session.check(endpoint_slice)
            for bad in (prefix+['statefulset','unrelated','-o','json'],
                        prefix+['persistentvolumeclaim','unrelated','-o','json'],
                        prefix+['endpoints','unrelated','-o','json'],
                        endpoint_slice[:-3]+['kubernetes.io/service-name=other','-o','json']):
                with self.assertRaisesRegex(RuntimeError,'staging_prerequisite_command_forbidden'):session.check(bad)
            for kind in ('pods','hpa,jobs,deployments'):
                session.check(prefix+[kind,'-o','json'])
            from runtime_stage import IMAGE_TEMPLATE
            from stage_runtime import FENCE_TEMPLATE
            for template in (IMAGE_TEMPLATE,FENCE_TEMPLATE):
                session.check(prefix+['pods,replicasets','-o','go-template='+template])
            raw='/api/v1/namespaces/voice-staging/secrets?fieldSelector=metadata.name%3Dvoice-app-secrets&limit=2'
            session.check(prefix+['--raw',raw])
            for bad in (prefix+['pods,replicasets','-o','go-template={{.}}'],
                        prefix+['--raw',raw.replace('limit=2','limit=200')],
                        prefix+['--raw',raw.replace('voice-staging','other')]):
                with self.assertRaisesRegex(RuntimeError,'staging_prerequisite_command_forbidden'):session.check(bad)
            saved=copy.deepcopy(journal.row);journal.row['target_sha256']=None
            for unbound in (prefix+['secret','voice-test','-o','json'],prefix+['--raw',raw],
                            prefix+['pods,replicasets','-o','go-template='+IMAGE_TEMPLATE]):
                with self.assertRaises(RuntimeError):session.check(unbound)
            journal.row=saved
            import source_plan
            for ch_prefix in (['/usr/local/bin/k3s','kubectl','--namespace','voice-staging'],
                              ['kubectl','--namespace','voice-staging']):
                ch=ch_prefix+list(source_plan.CLICKHOUSE_READ_ARGS)
                session.check(ch)
                for index in range(len(ch)):
                    bad=ch.copy();bad[index]+='-altered'
                    with self.subTest(argv=ch,index=index),self.assertRaisesRegex(RuntimeError,'staging_prerequisite_command_forbidden'):
                        session.check(bad)
                for bad in (ch+['--context=other'],ch[:-1],ch_prefix+['exec','other-pod','--','sh']):
                    with self.assertRaisesRegex(RuntimeError,'staging_prerequisite_command_forbidden'):session.check(bad)
            for pg_prefix in (['/usr/local/bin/k3s','kubectl','--namespace','voice-staging'],
                              ['kubectl','--namespace','voice-staging']):
                for database in ('postgres','gateway_db'):
                    pg=pg_prefix+source_plan.postgres_read_args(database)
                    session.check(pg)
                    for index in range(len(pg)):
                        bad=pg.copy();bad[index]+='-altered'
                        with self.subTest(argv=pg,index=index),self.assertRaisesRegex(RuntimeError,'staging_prerequisite_command_forbidden'):
                            session.check(bad)
                    for bad in (pg+['--context=other'],pg[:-1],pg_prefix+['exec','other-pod','--','sh']):
                        with self.assertRaisesRegex(RuntimeError,'staging_prerequisite_command_forbidden'):session.check(bad)
            fixed=['/usr/local/bin/k3s','kubectl','--namespace','voice-staging']+identity.IDENTITY_ARGS
            session.check(fixed)
            with self.assertRaisesRegex(RuntimeError,'staging_prerequisite_command_forbidden'):session.check(fixed+['altered'])
            with self.assertRaisesRegex(RuntimeError,'advance_required'):
                session.bind_target(c['commit'],c['images'],plan['actual_before'],'e'*64)
            with self.assertRaisesRegex(RuntimeError,'advance_required'):session.ready()
            with self.assertRaisesRegex(RuntimeError,'actual_finish_required'):
                session.complete('PASS',c['images'])
            receipt={'schema':'fixture-qualified'};adapter.load_qualified.return_value=receipt
            parent=Mock();next_plan={'schema':'voice-staging-prerequisites-plan-v4','installation_sha256':session.installation.sha256,
                'actual_before':plan['actual_before'],'preservation_sha256':claim.digest(receipt)}
            with patch.dict(sys.modules,{'staging_prerequisites':parent}):
                altered=dict(next_plan,preservation_sha256='f'*64)
                with self.assertRaisesRegex(RuntimeError,'baseline_changed'):
                    session.bind_prerequisites(altered,plan['actual_before'])
                self.assertEqual(session.check()['phase'],'cohort-preservation')
                result=session.bind_prerequisites(next_plan,plan['actual_before'])
                self.assertEqual(result['target_sha256'],claim.digest(next_plan))
                self.assertEqual(session.check()['phase'],'prerequisites')

    def fixture(self):
        c=context(); current=card(c); journal=MemoryJournal()
        session=claim.Session(c,'a'*12,'b'*64,lambda:copy.deepcopy(current),journal)
        return session,current,journal

    def test_prerequisite_phase_denies_untyped_commands_and_consumes_one_permit(self):
        for phase in ('prerequisites','prerequisites-captured'):
            session,current,journal=self.fixture();session.admit(start=True)
            journal.row.update(phase=phase,target_sha256='d'*64);session.expected_target='d'*64
            session.check(['/usr/local/bin/k3s','kubectl','--namespace','voice-staging','get','deployment','voice-web','-o','json'])
            session.check(['kubectl','create','--dry-run=client','--validate=false','-f','/dev/stdin','-o','json'])
            session.check(['/usr/local/bin/k3s','kubectl','--namespace','voice-staging','create','--dry-run=server','-f','/dev/stdin','-o','json'])
            session.check(['/usr/local/bin/k3s','kubectl','--namespace','voice-staging','get','pods','-l','app=voice-nats-pvc-candidate','-o','json'])
            session.check(['/usr/local/bin/k3s','kubectl','--namespace','voice-staging','get','--raw',
                '/api/v1/namespaces/voice-staging/pods/voice-nats-pvc-candidate-abc:8222/proxy/jsz?accounts=true&streams=true&consumers=true&config=true&limit=2048'])
            for args in (['/usr/local/bin/k3s','kubectl','--namespace','voice-staging','scale','deployment','voice-web'],
                         ['/usr/local/bin/k3s','kubectl','--namespace','voice-staging','exec','voice-pod','--','sh'],
                         ['docker','run','anything'],['sh','-c','anything'],
                         ['kubectl','--namespace','voice-staging','get','secret','voice-app-secrets','-o','json','--namespace','voice-dev'],
                         ['kubectl','--namespace','voice-staging','get','pods','-o','json','--kubeconfig','/tmp/other'],
                         ['kubectl','--namespace','voice-staging','get','pods','-o','json','-A'],
                         ['kubectl','--namespace','voice-staging','get','--raw','/api/v1/namespaces/other/secrets'],
                         ['kubectl','--namespace','voice-staging','get','pods','-l','app=other','-o','json'],
                         ['kubectl','--namespace','voice-staging','apply','--dry-run=server','--force','-f','/dev/stdin','-o','json']):
                with self.subTest(phase=phase,args=args),self.assertRaisesRegex(RuntimeError,'prerequisite_command_forbidden'):
                    session.check(args)
            with self.assertRaisesRegex(RuntimeError,'advance_required'):session.ready()
            if phase=='prerequisites':
                effect=['/usr/local/bin/k3s','kubectl','--namespace','voice-staging','create','-f','/dev/stdin','-o','json']
                session._prerequisite_permit=effect.copy();session.check(effect)
                self.assertIsNone(session._prerequisite_permit)
                with self.assertRaisesRegex(RuntimeError,'prerequisite_command_forbidden'):session.check(effect)
            else:
                import commands
                readonly=['/usr/local/bin/k3s','kubectl','--namespace','voice-staging','exec','-i','voice-postgres-0']
                launch=commands.launch_snapshot(readonly,b'',25,1024,{},())
                session._prerequisite_readonly_permit=launch
                with patch.object(session,'readonly_current'):session.check(readonly,launch=launch)
                self.assertIsNone(session._prerequisite_readonly_permit)
                with self.assertRaisesRegex(RuntimeError,'prerequisite_command_forbidden'):session.check(readonly)

    def test_bound_cohort_prerequisites_allow_only_named_store_projection_reads(self):
        import source_plan, source_plan_test
        session,current,journal=self.fixture();session.admit(start=True)
        journal.row.update(phase='prerequisites',target_sha256='d'*64,cohort_preservation_sha256='c'*64)
        session.expected_target='d'*64;session._cohort_lineage='c'*64
        prefix=['/usr/local/bin/k3s','kubectl','--namespace','voice-staging','get']
        for mode in ('app-only','full'):
            kube,params=source_plan_test.SourceTests().canonical_fixture(mode)
            original=kube.get
            def guarded_get(kind,name):
                session.check(prefix+[kind,name,'-o','json'])
                return original(kind,name)
            kube.get=guarded_get
            self.assertIsInstance(source_plan.preservation_projection(source_plan_test.SOURCE,params,
                source_plan_test.decoder,kube),dict)
        for bad in (prefix+['Secret','unrelated','-o','json'],prefix+['Secret','-o','json'],
                    prefix+['ClusterRole','voice-app-secrets','-o','json'],
                    prefix+['Secret','voice-app-secrets','-o','yaml'],
                    prefix+['Secret','voice-app-secrets','-o','json','--context=other'],
                    ['kubectl','--namespace','voice-staging','get','Secret','voice-app-secrets','-o','json']):
            with self.subTest(argv=bad),self.assertRaisesRegex(RuntimeError,'staging_prerequisite_command_forbidden'):
                session.check(bad)
        for kind in ('ConfigMap','Deployment','StatefulSet','PersistentVolumeClaim','Service','Job','Pod'):
            with self.subTest(kind=kind),self.assertRaisesRegex(RuntimeError,'staging_prerequisite_command_forbidden'):
                session.check(prefix+[kind,'voice-unrelated','-o','json'])
        journal.row.pop('cohort_preservation_sha256');session._cohort_lineage=None
        with self.assertRaisesRegex(RuntimeError,'staging_prerequisite_command_forbidden'):
            session.check(prefix+['Secret','voice-app-secrets','-o','json'])
        journal.row['cohort_preservation_sha256']='c'*64;session._cohort_lineage='c'*64
        journal.row['target_sha256']=None;session.expected_target=None
        with self.assertRaises(RuntimeError):session.check(prefix+['Secret','voice-app-secrets','-o','json'])

    def test_successor_native_permit_is_one_shot_and_bound_to_current_fence(self):
        session,current,journal=self.fixture();session.admit(start=True)
        journal.row.update(phase='prerequisites-captured',target_sha256='d'*64,cohort_preservation_sha256='c'*64)
        session.expected_target='d'*64;session._cohort_lineage='c'*64
        capability=object();session._cohort_native_capability=capability
        argv=['/usr/bin/docker','--host=unix:///var/run/docker.sock','image','inspect','fixture']
        def permit(**changes):
            values=dict(capability=capability,phase='prerequisites-captured',
                binding_sha256=claim.digest(session.binding()),lineage_sha256='c'*64,
                effect_id='observe-native-0123456789abcdef',argv=tuple(argv),body_sha256='e'*64)
            values.update(changes);return claim._CohortNativePermit(**values)
        for bad in (permit(capability=object()),permit(phase='prepare'),permit(binding_sha256='a'*64),
                    permit(lineage_sha256='a'*64),permit(argv=tuple(argv+['other'])),permit(body_sha256='bad')):
            session._cohort_native_permit=bad
            with self.assertRaisesRegex(RuntimeError,'staging_cohort_native_permit_invalid'):session.check(argv)
        session._cohort_native_permit=permit();session.check(argv)
        self.assertIsNone(session._cohort_native_permit)
        with self.assertRaisesRegex(RuntimeError,'staging_cohort_native_permit_invalid'):session.check(argv)

    def test_prerequisite_binding_requires_baseline_and_durable_capture(self):
        session,current,journal=self.fixture();session.admit(start=True)
        actual=current['observed']['images'];qualified={'fixture':'qualified-p2'}
        plan={'schema':'voice-staging-prerequisites-plan-v4','actual_before':copy.deepcopy(actual),
              'preservation_sha256':claim.digest(qualified)}
        record={'schema':'fixture-capture','actual_after':copy.deepcopy(actual)}
        module=Mock();module.capture.return_value=record;module.load_capture.return_value=record
        # Unit boundary: exercise the real journal transitions; P2 validation
        # and its qualified receipt are deliberately synthetic here.
        cohort=Mock();cohort.load_qualified.return_value=qualified
        p2={'actual_before':copy.deepcopy(actual),'candidate':{'commit':context()['commit'],'images':context()['images']}}
        with patch.dict(sys.modules,{'staging_prerequisites':module,'staging_cohort_preservation':cohort}):
            session.bind_cohort_preservation(p2,actual)
            binding=session.bind_prerequisites(plan,actual)
            self.assertEqual(binding['target_sha256'],claim.digest(plan))
            self.assertEqual(journal.row['phase'],'prerequisites')
            with self.assertRaisesRegex(RuntimeError,'advance_required'):
                session.bind_target(context()['commit'],context()['images'],actual,'d'*64)
            module.load_capture.return_value={'different':True}
            with self.assertRaisesRegex(RuntimeError,'capture_changed'):session.capture_prerequisites(object())
            self.assertEqual(journal.row['phase'],'prerequisites')
            module.load_capture.return_value=record
            binding=session.capture_prerequisites(object())
            self.assertEqual(binding['target_sha256'],claim.digest(record))
            self.assertEqual(journal.row['phase'],'prerequisites-captured')
            with self.assertRaisesRegex(RuntimeError,'advance_required'):session.ready()

    def test_typed_prerequisite_successors_preserve_parent_binding(self):
        for mode in ('cold','existing'):
            session,current,journal=self.fixture();session.admit(start=True)
            before=copy.deepcopy(current['observed']['images']);after=dict(before,**{'voice-p3-principal-jwks/jwks':'pinned'})
            parent={'actual_before':before,'actual_after':after};parent_hash=claim.digest(parent)
            journal.row.update(phase='prerequisites-captured',target_sha256=parent_hash);session.expected_target=parent_hash
            prerequisites=Mock();prerequisites.load_capture.return_value=parent
            realtime=Mock();existing=Mock();receipt={'typed':'receipt','prerequisites_sha256':parent_hash}
            realtime.load_completed.return_value=receipt;existing.load_existing.return_value=receipt
            target={'tag':context()['commit'],'images':context()['images'],'prerequisites_sha256':parent_hash,
                'realtime_bootstrap_sha256':claim.digest(receipt),'installation_sha256':session.installation.sha256}
            with patch.dict(sys.modules,{'staging_prerequisites':prerequisites,'realtime_bootstrap':realtime,
                                        'realtime_bootstrap_existing':existing}):
                if mode=='cold':
                    plan={'actual_images':after,'prerequisites_sha256':parent_hash};realtime.load_plan.return_value=plan
                    session.bind_realtime_bootstrap(plan,after)
                    with self.assertRaisesRegex(RuntimeError,'capture_changed'):
                        session.advance_realtime_bootstrap({k:v for k,v in target.items() if k!='prerequisites_sha256'},after)
                    result=session.advance_realtime_bootstrap(target,after)
                else:
                    with self.assertRaisesRegex(RuntimeError,'target_changed'):
                        session.advance_prerequisites_target({k:v for k,v in target.items() if k!='prerequisites_sha256'},after)
                    result=session.advance_prerequisites_target(target,after)
                self.assertEqual(result['target_sha256'],claim.digest(target));self.assertEqual(journal.row['phase'],'prepare')

    def test_realtime_bootstrap_requires_typed_transition_and_exact_receipt(self):
        session,current,journal=self.fixture();session.admit(start=True)
        actual=current['observed']['images'];plan={'actual_images':actual,'fixture':'validated separately'}
        receipt={'fixture':'root validated'}
        module=Mock();module.load_completed.return_value=receipt;module.load_plan.return_value={}
        with patch.dict(sys.modules,{'realtime_bootstrap':module}):
            before=copy.deepcopy(journal.row)
            with self.assertRaisesRegex(RuntimeError,'baseline_changed'):
                session.bind_realtime_bootstrap({'actual_images':actual,'schema':'voice-staging-realtime-bootstrap-v2',
                    'prerequisites_sha256':'0'*64},actual)
            self.assertEqual(journal.row,before)
            first=session.bind_realtime_bootstrap(plan,actual)
            self.assertEqual(first['target_sha256'],claim.digest(plan))
            self.assertEqual(journal.row['phase'],'realtime-bootstrap')
            with self.assertRaisesRegex(RuntimeError,'advance_required'):
                session.bind_target(context()['commit'],context()['images'],actual,'d'*64)
            with self.assertRaisesRegex(RuntimeError,'advance_required'):session.ready()
            bad={'tag':context()['commit'],'images':context()['images'],'realtime_bootstrap_sha256':'0'*64,
                 'installation_sha256':session.installation.sha256}
            with self.assertRaises(RuntimeError):session.advance_realtime_bootstrap(bad,actual)
            self.assertEqual(journal.row['phase'],'realtime-bootstrap')
            target=dict(bad,realtime_bootstrap_sha256=claim.digest(receipt))
            result=session.advance_realtime_bootstrap(target,actual)
            self.assertEqual(result['target_sha256'],claim.digest(target))
            self.assertEqual(journal.row['phase'],'prepare')
            saved=len(journal.events)
            session.bind_target(context()['commit'],context()['images'],actual,claim.digest(target))
            self.assertEqual(len(journal.events),saved)
            with self.assertRaisesRegex(RuntimeError,'target_changed'):
                session.bind_target(context()['commit'],context()['images'],actual,'f'*64)
            module.load_completed.assert_called_with(session,actual=actual)
            with self.assertRaises(RuntimeError):session.advance_realtime_bootstrap(target,actual)

    def test_realtime_advance_rechecks_journal_after_receipt_load(self):
        session,current,journal=self.fixture();session.admit(start=True)
        actual=current['observed']['images'];plan={'actual_images':actual};receipt={'fixture':'root validated'}
        module=Mock()
        def changed(*args,**kwargs):
            journal.row['status']='uncertain'
            return receipt
        module.load_completed.side_effect=changed
        with patch.dict(sys.modules,{'realtime_bootstrap':module}):
            session.bind_realtime_bootstrap(plan,actual)
            target={'tag':context()['commit'],'images':context()['images'],'realtime_bootstrap_sha256':claim.digest(receipt),
                    'installation_sha256':session.installation.sha256}
            with self.assertRaises(RuntimeError):session.advance_realtime_bootstrap(target,actual)
            self.assertEqual(journal.row['status'],'uncertain')
            self.assertEqual(journal.row['phase'],'realtime-bootstrap')

    def test_identity_is_typed_and_exact_not_actor_label(self):
        for key in context():
            c=context();del c[key]
            with self.subTest(key=key),self.assertRaises(ValueError):
                claim.Session(c,'a'*12,'b'*64,Mock(),MemoryJournal())
        for key in ('claim','generation','window','revision','owner','state','phase'):
            session,current,journal=self.fixture();current[key]='stale'
            with self.subTest(key=key),self.assertRaises(RuntimeError):session.admit(start=True)
            self.assertIsNone(journal.row)

    def test_journal_precedes_commands_and_survives_stage_boundaries(self):
        session,current,journal=self.fixture();session.admit(start=True)
        session.check(['kubectl','get','deployments'])
        session.bind_target('c'*40,context()['images'],current['observed']['images'],'d'*64)
        binding=session.binding()
        self.assertEqual(journal.row['status'],'active')
        resumed=claim.Session(context(),'a'*12,'b'*64,lambda:current,journal)
        resumed.admit(start=False);resumed.require_binding(binding)
        resumed.ready();resumed.check(['kubectl','apply','-f','-'])
        self.assertEqual(journal.row['status'],'active')

    def test_new_generation_cannot_replace_active_or_uncertain_operation(self):
        for uncertain in (False,True):
            session,current,journal=self.fixture();session.admit(start=True)
            if uncertain:session.uncertain('partial apply')
            newer=context();newer.update(generation=8,claim='33333333-3333-4333-8333-333333333333')
            other=claim.Session(newer,'e'*12,'b'*64,lambda:card(newer),journal)
            with self.subTest(uncertain=uncertain),self.assertRaises(RuntimeError):other.admit(start=True)
            self.assertEqual(journal.row['context']['generation'],7)

    def test_paused_executor_rechecks_live_card_and_journal_before_spawn(self):
        session,current,journal=self.fixture();session.admit(start=True)
        current['claim']='33333333-3333-4333-8333-333333333333'
        with self.assertRaises(RuntimeError):session.check(['kubectl','patch','deployment'])
        current['claim']=context()['claim'];journal.row['status']='uncertain'
        with self.assertRaises(RuntimeError):session.check(['kubectl','delete','pod'])

    def test_unbound_target_allows_only_exact_client_source_decode(self):
        session,current,journal=self.fixture();session.admit(start=True)
        exact=['kubectl','create','--dry-run=client','--validate=false','-f','/dev/stdin','-o','json']
        session.check(exact)
        for changed in (exact+['--save-config'],exact[:-1],['kubectl','create','-f','/dev/stdin'],
                        ['kubectl','create','--dry-run=client','-f','/dev/stdin','-o','json']):
            with self.subTest(changed=changed),self.assertRaisesRegex(RuntimeError,'target_unbound'):
                session.check(changed)

    def test_legacy_or_changed_helper_binding_and_baseline_are_rejected(self):
        session,current,journal=self.fixture()
        with self.assertRaises(RuntimeError):session.admit(start=False)
        session.admit(start=True)
        with self.assertRaises(RuntimeError):session.bind_target('c'*40,context()['images'],{},'d'*64)
        session.bind_target('c'*40,context()['images'],current['observed']['images'],'d'*64)
        for changed in ({},dict(session.binding(),helper_sha256='e'*64)):
            with self.assertRaises(RuntimeError):session.require_binding(changed)

    def test_only_actual_finish_closes_and_old_generation_cannot_reopen(self):
        session,current,journal=self.fixture();session.admit(start=True)
        session.bind_target('c'*40,context()['images'],current['observed']['images'],'d'*64);session.ready()
        with self.assertRaises(RuntimeError):session.complete('WAITING',context()['images'])
        session.complete('PASS',context()['images'])
        with self.assertRaises(RuntimeError):session.admit(start=True)
        with self.assertRaises(RuntimeError):session.check(['kubectl','patch'])
        newer=context();newer.update(generation=8,claim='33333333-3333-4333-8333-333333333333',operation_id='next')
        claim.Session(newer,'e'*12,'b'*64,lambda:card(newer),journal).admit(start=True)
        with self.assertRaises(RuntimeError):session.check(['kubectl','apply'])

    def test_new_start_refuses_unknown_or_incomplete_closed_journal(self):
        for field,value in (('schema','unknown-v2'),('phase','prepare'),('actual',{}),
                            ('target_sha256',None),('helper_sha256','bad'),('operation','bad')):
            old,current,journal=self.prepared();old.complete('PASS',context()['images'])
            journal.row[field]=value;before=journal.read()
            newer=context();newer.update(generation=8,claim='33333333-3333-4333-8333-333333333333',operation_id='next')
            new=claim.Session(newer,'e'*12,'b'*64,lambda:card(newer),journal)
            with self.subTest(field=field),self.assertRaises(RuntimeError):new.admit(start=True)
            self.assertEqual(journal.read(),before)

    def test_bridge_admission_precedes_cached_result_and_recovery(self):
        import bridge
        request=dict(action='finish',operation='a'*12,claim_rv='17',nonce='b'*64)
        with tempfile.TemporaryDirectory() as directory:
            calls=[]
            bridge.dispatch(Path(directory),request,lambda row:{'status':'PASS'},lambda row:None,
                admit=lambda row:calls.append('check'))
            def stale(row):raise RuntimeError('claim rotated')
            with self.assertRaises(RuntimeError):
                bridge.dispatch(Path(directory),request,lambda row:self.fail('rerun'),lambda row:self.fail('recover'),admit=stale)
            self.assertEqual(calls,['check'])

    def test_actual_command_boundary_checks_before_popen_and_cleanup_has_same_guard(self):
        import commands
        session,current,journal=self.fixture();session.admit(start=True)
        session.bind_target('c'*40,context()['images'],current['observed']['images'],'d'*64)
        current['claim']='33333333-3333-4333-8333-333333333333'
        previous=commands.authority_guard
        with patch.object(commands.subprocess,'Popen') as spawn:
            with claim.command_scope(session.check):
                for argv in (['kubectl','patch','deployment'],['kubectl','scale','deployment','--replicas=0']):
                    with self.assertRaises(RuntimeError):commands.capture(argv)
            spawn.assert_not_called()
        self.assertIs(commands.authority_guard,previous)

    def test_failed_baseline_cannot_fall_through_to_refence(self):
        import commands
        session,current,journal=self.fixture();session.admit(start=True)
        with self.assertRaises(RuntimeError):session.bind_target('c'*40,context()['images'],{},'d'*64)
        with claim.command_scope(session.check),patch.object(commands.subprocess,'Popen') as spawn:
            with self.assertRaises(RuntimeError):commands.capture(['kubectl','patch','deployment','--patch={}'])
            spawn.assert_not_called()

    def test_runner_reads_live_journal_each_stage_and_never_closes_it(self):
        session,current,journal=self.fixture();session.admit(start=True)
        target=dict(tag='c'*40,images=context()['images'],installation_sha256=session.installation.sha256)
        session.bind_target('c'*40,context()['images'],current['observed']['images'],claim.digest(target));session.ready()
        binding=session.binding();helper={'fixture':'b'*64}
        journal.row['helper_sha256']=claim.digest(helper);binding['helper_sha256']=claim.digest(helper)
        receipt=dict(schema='nats-rollout-preservation-v2',installation_sha256=session.installation.sha256,operation='a'*12,target=target,resource_fence=binding)
        check=Mock(side_effect=lambda:copy.deepcopy(current));check.uncertain=Mock()
        with patch.object(claim,'require_helper'),patch.object(claim,'RootJournal',return_value=journal),patch.object(claim,'Check',return_value=check):
            with self.assertRaises(RuntimeError):
                with claim.runner_scope(receipt,Path('/unused'),helper) as runner:
                    journal.row['target_sha256']='e'*64
                    runner.check(['kubectl','apply'])
        self.assertEqual(journal.row['status'],'active')
        check.uncertain.assert_called_once()

    def test_mutating_client_requires_context_and_binds_it_into_nonce(self):
        import bridge_client
        environment=dict(GITHUB_TOKEN='private',GITHUB_RUN_ATTEMPT='1',GITHUB_RUN_ID='17',GITHUB_SHA='c'*40)
        with self.assertRaises(Exception):bridge_client.request(['prepare','images-only','user','c'*40,'17'],environment)
        environment['VOICE_TTENREC_CONTEXT']=__import__('json').dumps(context())
        first=bridge_client.request(['prepare','images-only','user','c'*40,'17'],environment)
        self.assertEqual(first['resource_context'],context())
        changed=context();changed['generation']+=1
        environment['VOICE_TTENREC_CONTEXT']=__import__('json').dumps(changed)
        second=bridge_client.request(['prepare','images-only','user','c'*40,'17'],environment)
        self.assertNotEqual(first['nonce'],second['nonce'])

    def prepared(self):
        session,current,journal=self.fixture();session.admit(start=True)
        session.bind_target('c'*40,context()['images'],current['observed']['images'],'d'*64);session.ready()
        return session,current,journal

    def test_provisional_finish_remains_active_through_actual_acceptance(self):
        session,current,journal=self.prepared();session.complete('PROVISIONAL_READY',context()['images'])
        self.assertEqual(journal.row['status'],'active')
        self.assertEqual(journal.row['phase'],'awaiting-ci')
        session.check(['kubectl','get','deployments'])
        actions=Mock();actions.state.return_value=(Path('/unused'),dict(status='PASS',target=dict(images=context()['images'])))
        claim.finish_request(session,actions,dict(action='accept-provisional',operation='a'*12),dict(status='PASS'))
        self.assertEqual(journal.row['status'],'complete')

    def test_cached_authorize_after_completion_does_not_reopen_or_mark_uncertain(self):
        session,current,journal=self.prepared();session.complete('PASS',context()['images'])
        before=copy.deepcopy(journal.row);events=len(journal.events)
        claim.finish_request(session,Mock(),dict(action='authorize'),dict(status='WAITING'))
        self.assertEqual(journal.row,before);self.assertEqual(len(journal.events),events)

    def test_nested_runner_and_apply_have_one_uncertain_notification_owner(self):
        session,current,journal=self.prepared();target=dict(tag='c'*40,images=context()['images'],installation_sha256=session.installation.sha256)
        helper={'fixture':'b'*64};journal.row.update(helper_sha256=claim.digest(helper),target_sha256=claim.digest(target))
        binding={k:copy.deepcopy(journal.row[k]) for k in ('schema','installation_sha256','context','operation','helper_sha256','target_sha256')}
        receipt=dict(schema='nats-rollout-preservation-v2',installation_sha256=session.installation.sha256,operation='a'*12,target=target,resource_fence=binding)
        check=Mock(side_effect=lambda:copy.deepcopy(current));check.uncertain=Mock(side_effect=RuntimeError('lost POST response'))
        with patch.object(claim,'require_helper'),patch.object(claim,'RootJournal',return_value=journal),patch.object(claim,'Check',return_value=check):
            with self.assertRaisesRegex(RuntimeError,'apply response lost'):
                with claim.runner_scope(receipt,Path('/unused'),helper):
                    with claim.runner_scope(receipt,Path('/unused'),helper):raise RuntimeError('apply response lost')
        self.assertEqual(check.uncertain.call_count,1)
        self.assertEqual(journal.row['status'],'active')

    def test_rejected_admission_cannot_mark_some_other_executor_uncertain(self):
        session,current,journal=self.prepared();session.complete('PASS',context()['images'])
        c=context();c.update(generation=8,claim='33333333-3333-4333-8333-333333333333')
        check=Mock(return_value=card(c));check.uncertain=Mock()
        actions=Mock();actions.binding={'fixture':'b'*64}
        request=dict(action='authorize',operation='a'*12,nonce='e'*64,resource_context=c)
        before=copy.deepcopy(journal.row)
        with patch.object(claim,'require_helper'),patch.object(claim,'RootJournal',return_value=journal),patch.object(claim,'Check',return_value=check):
            with self.assertRaises(RuntimeError):
                with claim.root_scope(actions,request) as active:active.admit_request(request)
        check.uncertain.assert_not_called();self.assertEqual(journal.row,before)

    def test_historical_parent_binding_does_not_reuse_its_old_resource_authority(self):
        session,current,journal=self.prepared();target=scoped_target(context())
        helper={'nats-rollout-preservation/resource_claim.py':'b'*64}
        session.helper=claim.digest(helper);journal.row.update(helper_sha256=session.helper,target_sha256=claim.digest(target))
        session.expected_target=claim.digest(target);binding=session.binding()
        state=dict(schema='nats-rollout-root-v2',installation_sha256=session.installation.sha256,operation='a'*12,target=target,code_capture=helper,resource_fence=binding)
        self.assertEqual(claim.historical_binding(state),binding)
        changed_target=dict(target,tag='e'*40)
        changed_state=dict(state,target=changed_target,resource_fence=dict(binding,target_sha256=claim.digest(changed_target)))
        for invalid in ({k:v for k,v in state.items() if k!='resource_fence'},changed_state):
            with self.assertRaises(RuntimeError):claim.historical_binding(invalid)

    def repair(self):
        old,current,journal=self.prepared();old.complete('PROVISIONAL_READY',context()['images'])
        newer=context();newer.update(generation=8,revision=14,claim='33333333-3333-4333-8333-333333333333',operation_id='repair-18')
        new=claim.Session(newer,'e'*12,'b'*64,lambda:card(newer),journal)
        new.repair_guard=Mock(return_value=dict(operation=old.operation,checkpoint_sha256='f'*64))
        return old,new,journal,dict(action='prepare-repair',operation=old.operation)

    def test_repair_archives_verified_provisional_before_replacing_and_denies_old_writer(self):
        old,new,journal,request=self.repair();before=journal.read()
        new.admit_request(request)
        self.assertEqual(journal.supersessions[0][0],before)
        self.assertEqual(new.repair_guard.call_count,2)
        self.assertEqual(journal.row['context'],new.context)
        with self.assertRaises(RuntimeError):old.check(['kubectl','apply'])
        new.uncertain('lost response')
        self.assertEqual(journal.row['status'],'uncertain')
        self.assertEqual(journal.supersessions[0][0],before)

    def test_repair_requires_fresh_identity_exact_phase_and_durable_history(self):
        for change in ('phase','status','actual','generation','claim','operation_id','action','proof','archive','reread'):
            old,new,journal,request=self.repair();before=journal.read()
            if change=='phase':journal.row['phase']='apply-ready'
            elif change=='status':journal.row['status']='uncertain'
            elif change=='actual':journal.row['actual']={}
            elif change in ('generation','claim','operation_id'):new.context[change]=old.context[change]
            elif change=='action':request['action']='prepare'
            elif change=='proof':new.repair_guard.side_effect=RuntimeError('bad protected parent')
            elif change=='archive':journal.supersede=Mock(side_effect=RuntimeError('fsync failed'))
            elif change=='reread':new.repair_guard.side_effect=[{'checkpoint_sha256':'f'*64},{'checkpoint_sha256':'d'*64}]
            expected=journal.read()
            with self.subTest(change=change),self.assertRaises(RuntimeError):new.admit_request(request)
            self.assertEqual(journal.row,expected)

    def test_repair_proof_reads_exact_protected_parent(self):
        old,current,journal=self.prepared();target=scoped_target(context())
        helper={'nats-rollout-preservation/resource_claim.py':'b'*64}
        old.helper=claim.digest(helper);journal.row.update(helper_sha256=old.helper,target_sha256=claim.digest(target))
        old.expected_target=claim.digest(target)
        state=dict(schema='nats-rollout-root-v2',installation_sha256=old.installation.sha256,operation=old.operation,target=target,code_capture=helper,resource_fence=old.binding(),
            status='PROVISIONAL_READY',phase='POST_RESTART',fence_status='RELEASED')
        old.complete('PROVISIONAL_READY',context()['images'])
        import json
        actions=Mock();actions.installation=old.installation;actions.state.return_value=(Path('/private'),state)
        request=dict(operation=old.operation)
        with patch('operator_retirement.private_bytes',return_value=json.dumps(state).encode()):
            proof=claim.repair_proof(actions,request,journal.row)
            self.assertEqual(proof['operation'],old.operation)
            state['fence_status']='UNKNOWN'
            with self.assertRaises(RuntimeError):claim.repair_proof(actions,request,journal.row)

    @unittest.skipUnless(os.name=='posix' and os.geteuid()==0,'isolated root Linux custody fixture')
    def test_root_archive_and_actual_protected_checkpoint_survive_failed_replacement(self):
        import root_main
        old,new,memory,request=self.repair()
        target=scoped_target(context())
        helper={'nats-rollout-preservation/resource_claim.py':'b'*64}
        previous=memory.read();previous.update(helper_sha256=claim.digest(helper),target_sha256=claim.digest(target))
        binding={k:previous[k] for k in ('schema','installation_sha256','context','operation','helper_sha256','target_sha256')}
        state=dict(schema='nats-rollout-root-v2',installation_sha256=old.installation.sha256,operation=old.operation,target=target,code_capture=helper,resource_fence=binding,
            status='PROVISIONAL_READY',phase='POST_RESTART',fence_status='RELEASED')
        # The disposable CI image has no host operator account. Keep the real
        # chown/mode/custody path, with a fixture-local group identity only.
        with tempfile.TemporaryDirectory(dir='/root') as folder,patch('grp.getgrnam',return_value=Mock(gr_gid=os.getgid())):
            root=Path(folder);(root/'installed').mkdir(mode=0o700)
            base=root/('rollout-'+old.operation);base.mkdir(mode=0o700)
            root_main.save(base/'checkpoint.json',state)
            journal=claim.RootJournal(root);journal.save(previous)
            import bridge_root,guard,root_cli
            actions=bridge_root.Actions.__new__(bridge_root.Actions);actions._installation=old.installation;actions.tt_session=new;actions.binding=helper
            new.journal=journal;new.repair_guard=lambda req,prior:claim.repair_proof(actions,req,prior)
            with patch.object(guard,'ROOT',root),patch.object(root_cli,'ROOT',root):
                with patch.object(journal,'save',side_effect=OSError('replace interrupted')):
                    with self.assertRaises(OSError):new.admit_request(request)
                self.assertEqual(journal.read(),previous)
                archive=root/'installed/tt-resource-supersessions'/('e'*12+'.json')
                record=claim.trusted_json(archive)
                self.assertEqual(record['previous'],previous)
                self.assertEqual(record['parent']['checkpoint_sha256'],__import__('hashlib').sha256((base/'checkpoint.json').read_bytes()).hexdigest())
                # Lost replacement response retries only the identical archive;
                # changed checkpoint bytes cannot overwrite durable history.
                new.admit_request(request)
                self.assertEqual(journal.read()['context'],new.context)
                self.assertEqual(claim.trusted_json(archive),record)
                archive.chmod(0o660)
                with self.assertRaises(RuntimeError):claim.trusted_json(archive)


if __name__=='__main__':unittest.main()
