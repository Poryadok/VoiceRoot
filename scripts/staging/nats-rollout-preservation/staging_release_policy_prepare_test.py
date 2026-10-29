"""Finite alpha policy creation; fixture paths are not environment authority."""
import copy
import hashlib
import json
import os
from pathlib import Path
import sys
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import Mock, patch

import release_policy_wiring as wiring
import staging_release_policy_prepare as prepare


def owners():
    return {name:{'policy_directory':'/run/fixture/'+name,'snapshot_file':'/tmp/'+name+'-snapshot.json'}
            for name in wiring.OWNERS}


class DescriptorTests(unittest.TestCase):
    def test_same_candidate_preserved_policy_requires_protected_history_and_exact_live_objects(self):
        import guard
        with tempfile.TemporaryDirectory() as folder:
            root=Path(folder);operation='b'*12;source=root/'installed'/'sources'/operation
            path=source/'deploy/staging/services.yaml';path.parent.mkdir(parents=True)
            raw=(Path(__file__).resolve().parents[3]/'deploy/staging/services.yaml').read_bytes();path.write_bytes(raw)
            approved={'source_sha':'a'*40,'source_files':{'deploy/staging/services.yaml':hashlib.sha256(raw).hexdigest()}}
            value=prepare.staging_descriptor('a'*40)
            old=prepare._canonical({'release_policy':value,'unrelated':{'keep':'exact'}})
            policy=root/'installed'/'policy.json';policy.write_bytes(old);policy.chmod(0o600)
            expected=value['policy_object'];live=copy.deepcopy(expected)
            live['metadata'].update(uid='12345678-aaaa-aaaa-aaaa-123456789abc',resourceVersion='456')
            ref={'operation':'a'*12,'receipt_sha256':'d'*64}
            cm={'kind':'ConfigMap','name':expected['metadata']['name'],'namespace':'voice-staging',
                'uid':live['metadata']['uid'],'resourceVersion':'456','semantic_sha256':hashlib.sha256(prepare._canonical(expected)).hexdigest()}
            root_observation={'kind':'RootPolicy','name':'installed-policy','sha256':hashlib.sha256(old).hexdigest()}
            preserved={'create-policy-configmap':{'authority':ref,'observed':cm},
                       'commit-root-policy':{'authority':ref,'observed':root_observation}}
            build=getattr(prepare,'build_preserved_plan',None)
            self.assertTrue(callable(build),'protected same-candidate policy reuse is required')
            record,payloads=build(approved,source,old,value,operation=operation,
                namespace_uid='12345678-1234-1234-1234-123456789abc',source_capture_sha256='b'*64,preserved=preserved)
            self.assertEqual(record['effects'],[]);self.assertEqual(payloads,{})
            for fault in ('missing','unrelated_origin','same_operation','foreign_name'):
                malformed=copy.deepcopy(preserved)
                if fault=='missing':malformed.pop('commit-root-policy')
                elif fault=='unrelated_origin':malformed['commit-root-policy']['authority']={'operation':'c'*12,'receipt_sha256':'d'*64}
                elif fault=='same_operation':
                    for row in malformed.values():row['authority']['operation']=operation
                else:malformed['create-policy-configmap']['observed']['name']='foreign'
                with self.subTest(fault=fault),self.assertRaises(ValueError):
                    build(approved,source,old,value,operation=operation,namespace_uid=record['namespace_uid'],
                          source_capture_sha256='b'*64,preserved=malformed)
            namespace={'metadata':{'name':'voice-staging','uid':record['namespace_uid']}}
            kube=SimpleNamespace(get=Mock(side_effect=lambda kind,name:namespace if kind=='namespace' else live),run=Mock())
            qualified={'create-policy-configmap':{'authority':ref,'semantic':expected,'uid':cm['uid']},
                       'commit-root-policy':{'authority':ref,'semantic':json.loads(old),'sha256':root_observation['sha256']}}
            parent=SimpleNamespace(require_child=Mock(return_value=record),load_authority=Mock(return_value={
                'approved':approved,'execution':{'fixture':'protected'},'source':str(source)}),
                load_preserved=Mock(side_effect=lambda _s,_slot,effect:copy.deepcopy(qualified[effect])),
                load_payload=Mock(),submit_effect=Mock(),record_pending=Mock(),record_done=Mock())
            session=SimpleNamespace(operation=operation,check=Mock())
            with patch.dict(sys.modules,{'staging_prerequisites':parent}),patch.object(prepare,'Kube',return_value=kube),patch.object(guard,'ROOT',root):
                self.assertTrue(prepare.preflight_payloads(session))
                prepare.execute(session)
                self.assertEqual(prepare.verify_effect(session,'create-policy-configmap',cm),cm)
                self.assertEqual(prepare.verify_effect(session,'commit-root-policy',root_observation),root_observation)
                parent.submit_effect.assert_not_called();parent.record_pending.assert_not_called();parent.record_done.assert_not_called()
                parent.load_payload.assert_not_called();kube.run.assert_not_called()
                for fault in ('uid','rv','data','namespace','root_bytes','authority'):
                    with self.subTest(fault=fault):
                        saved_live=copy.deepcopy(live);saved_namespace=copy.deepcopy(namespace)
                        if fault=='uid':live['metadata']['uid']='12345678-bbbb-bbbb-bbbb-123456789abc'
                        elif fault=='rv':live['metadata']['resourceVersion']='457'
                        elif fault=='data':live['data']['policy.json']+=' '
                        elif fault=='namespace':namespace['metadata']['uid']='other'
                        elif fault=='root_bytes':policy.write_bytes(old+b' ')
                        else:qualified['create-policy-configmap']['authority']={'operation':'c'*12,'receipt_sha256':'e'*64}
                        with self.assertRaises(ValueError):prepare.preflight_payloads(session)
                        live.clear();live.update(saved_live);namespace.clear();namespace.update(saved_namespace)
                        policy.write_bytes(old);qualified['create-policy-configmap']['authority']=ref
                        parent.submit_effect.assert_not_called();parent.record_pending.assert_not_called()
            with self.assertRaises(ValueError):
                build(dict(approved,source_sha='c'*40),source,old,value,operation=operation,
                    namespace_uid=record['namespace_uid'],source_capture_sha256='b'*64,preserved=preserved)

    def test_coordinator_preflight_revalidates_source_payloads_and_live_preimages_before_effects(self):
        import guard
        with tempfile.TemporaryDirectory() as folder:
            root=Path(folder);source=root/'installed'/'sources'/('a'*12);path=source/'deploy/staging/services.yaml';path.parent.mkdir(parents=True)
            raw=(Path(__file__).resolve().parents[3]/'deploy/staging/services.yaml').read_bytes();path.write_bytes(raw)
            approved={'source_sha':'a'*40,'source_files':{'deploy/staging/services.yaml':hashlib.sha256(raw).hexdigest()}}
            value=prepare.staging_descriptor('a'*40)
            installed=root/'installed';old=b'{"s3_signing_endpoint":"fixture","unrelated":{"keep":true}}'
            policy=installed/'policy.json';policy.write_bytes(old);policy.chmod(0o600)
            plan,payloads=prepare.plan(approved,source,old,value,operation='a'*12,
                namespace_uid='12345678-1234-1234-1234-123456789abc',source_capture_sha256='b'*64,list_rv='123')
            namespace={'metadata':{'name':'voice-staging','uid':plan['namespace_uid']}}
            listing={'apiVersion':'v1','kind':'ConfigMapList','metadata':{'resourceVersion':'456'},'items':[]}
            kube=SimpleNamespace(get=Mock(return_value=namespace),run=Mock(return_value=listing))
            parent=SimpleNamespace(require_child=Mock(return_value=plan),
                load_payload=Mock(side_effect=lambda _session,_slot,effect:payloads[effect]),
                load_authority=Mock(return_value={'approved':approved,'execution':{'fixture':'protected'},'source':str(source)}),
                submit_effect=Mock(),record_pending=Mock())
            function=getattr(prepare,'preflight_payloads',None)
            self.assertTrue(callable(function),'fixed release-policy preflight is required')
            session=SimpleNamespace(operation='a'*12)
            with patch.dict(sys.modules,{'staging_prerequisites':parent}),patch.object(prepare,'Kube',return_value=kube),patch.object(guard,'ROOT',root):
                self.assertTrue(function(session))
                parent.submit_effect.assert_not_called();parent.record_pending.assert_not_called()
                parent.load_authority.assert_called_with(session)
                expected_path=('/api/v1/namespaces/voice-staging/configmaps?fieldSelector=metadata.name%3D'+
                               value['policy_object']['metadata']['name']+'&limit=2')
                kube.run.assert_called_with(['get','--raw',expected_path])
                def replace_inode(_argv):
                    replacement=installed/'replacement.json';replacement.write_bytes(old);replacement.chmod(0o600)
                    os.replace(replacement,policy)
                    return listing
                kube.run.side_effect=replace_inode
                with self.assertRaises(ValueError):function(session)
                kube.run.side_effect=None
                for fault in ('payload','source','old','namespace','present','continue','list_rv','list_type','authority_path','session_op'):
                    with self.subTest(fault=fault):
                        if fault=='payload':payloads['commit-root-policy']+=b' '
                        elif fault=='source':path.write_bytes(raw+b'\n')
                        elif fault=='old':policy.write_bytes(old+b' ')
                        elif fault=='namespace':namespace['metadata']['uid']='other'
                        elif fault=='present':listing['items']=[{'metadata':{'name':'foreign'}}]
                        elif fault=='continue':listing['metadata']['continue']='more'
                        elif fault=='list_rv':listing['metadata']['resourceVersion']=''
                        elif fault=='list_type':listing['kind']='List'
                        elif fault=='authority_path':parent.load_authority.return_value['source']=str(root/'foreign')
                        else:session.operation='b'*12
                        with self.assertRaises(ValueError):function(session)
                        payloads['commit-root-policy']=prepare._canonical(prepare.policy_successor(json.loads(old),value,approved['source_sha']))
                        path.write_bytes(raw);policy.write_bytes(old);namespace['metadata']['uid']=plan['namespace_uid']
                        parent.load_authority.return_value['source']=str(source);session.operation='a'*12
                        listing.update(kind='ConfigMapList',metadata={'resourceVersion':'456'},items=[])
                        parent.submit_effect.assert_not_called();parent.record_pending.assert_not_called()

    def test_documented_alpha_uses_explicit_environment_placement(self):
        value=prepare.staging_descriptor('a'*40)
        self.assertEqual(value['policy_object']['metadata']['name'],'voice-release-policy-'+('a'*12))
        self.assertEqual(set(value['policy_object']['data']),{'policy.json'})
        self.assertEqual(value['owners'],{name:{'policy_directory':'/etc/voice/release',
            'snapshot_file':'/tmp/voice-release-snapshot.json'} for name in wiring.OWNERS})
        with self.assertRaises(ValueError):prepare.staging_descriptor('0'*40)
    def test_finite_alpha_descriptor_is_source_bound_and_validated(self):
        candidate='a'*40
        result=prepare.descriptor(candidate,'voice-release-policy-fixture','policy.json',owners())
        wiring.validate(result,candidate,'voice-staging')
        self.assertEqual(result['policy_sha256'],hashlib.sha256(result['policy_object']['data']['policy.json'].encode()).hexdigest())
        policy=json.loads(result['policy_object']['data']['policy.json'])
        self.assertEqual(set(policy['capabilities'].values()),{'disabled'})
        for item in wiring.BASELINE:self.assertEqual(policy['rich_intents'][item],'qualified')
        for item in wiring.RESTRICTED:self.assertEqual(policy['rich_intents'][item],'disabled')
        self.assertEqual(result,prepare.descriptor(candidate,'voice-release-policy-fixture','policy.json',owners()))

    def test_no_guessed_owner_input_or_cross_candidate_binding(self):
        for candidate,name,key,value in (('', 'fixture','policy.json',owners()),('0'*40,'fixture','policy.json',owners()),
            ('a'*40,'../escape','policy.json',owners()),('a'*40,'fixture','../policy',owners()),
            ('a'*40,'fixture','policy.json',{}),('a'*40,'fixture','policy.json',dict(owners(),auth={'policy_directory':'/tmp','snapshot_file':'/tmp/a'}))):
            with self.subTest(candidate=candidate,name=name,key=key),self.assertRaises((ValueError,TypeError,KeyError)):
                prepare.descriptor(candidate,name,key,value)
        value=prepare.descriptor('a'*40,'fixture','policy.json',owners())
        with self.assertRaises(ValueError):wiring.validate(copy.deepcopy(value),'b'*40,'voice-staging')
        bad=owners();bad['auth']['policy_directory']='//etc/voice/release'
        with self.assertRaises(ValueError):prepare.descriptor('a'*40,'fixture','policy.json',bad)

    def test_only_release_policy_changes_and_inputs_are_not_mutated(self):
        old={'s3_signing_endpoint':'https://fixture.invalid','unrelated':{'opaque':['do-not-change']},
             'release_policy':{'previous':'retained-in-original'}}
        candidate=prepare.descriptor('a'*40,'fixture','policy.json',owners())
        before=copy.deepcopy(old);planned=prepare.policy_successor(old,candidate,'a'*40)
        self.assertEqual(old,before)
        self.assertEqual({k:v for k,v in planned.items() if k!='release_policy'},
                         {k:v for k,v in old.items() if k!='release_policy'})
        self.assertEqual(planned['release_policy'],candidate)
        planned['unrelated']['opaque'].append('change-copy')
        self.assertEqual(old,before)
        with self.assertRaises(ValueError):prepare.policy_successor(old,candidate,'b'*40)

    def test_protected_source_descriptor_requires_exact_approved_template_bytes(self):
        with tempfile.TemporaryDirectory() as folder:
            source=Path(folder);path=source/'deploy/staging/services.yaml';path.parent.mkdir(parents=True)
            # This fixture uses the real checked-in placeholder template.
            template=Path(__file__).resolve().parents[3]/'deploy/staging/services.yaml'
            raw=template.read_bytes();path.write_bytes(raw)
            value=prepare.staging_descriptor('a'*40)
            approved={'source_sha':'a'*40,'source_files':{'deploy/staging/services.yaml':hashlib.sha256(raw).hexdigest()}}
            self.assertEqual(prepare.validate_source(approved,source,value),value)
            changed=copy.deepcopy(approved);changed['source_files']['deploy/staging/services.yaml']='b'*64
            with self.assertRaises(ValueError):prepare.validate_source(changed,source,value)
            with self.assertRaises(ValueError):prepare.validate_source(dict(approved,source_sha='b'*40),source,value)
            collided=raw.replace(b'- {name: jwt-key, mountPath: /run/jwt.pem',b'- {name: jwt-key, mountPath: /etc/voice')
            path.write_bytes(collided)
            changed=copy.deepcopy(approved);changed['source_files']['deploy/staging/services.yaml']=hashlib.sha256(collided).hexdigest()
            with self.assertRaisesRegex(ValueError,'release_policy_mount_collision'):
                prepare.validate_source(changed,source,value)
            escaped=raw.replace(b'- {name: jwt-key, mountPath: /run/jwt.pem',
                                b'- {name: jwt-key, mountPath: "/etc/voice\\x2frelease\\x2fpolicy.json"')
            path.write_bytes(escaped)
            changed['source_files']['deploy/staging/services.yaml']=hashlib.sha256(escaped).hexdigest()
            with self.assertRaisesRegex(ValueError,'release_policy_mount_syntax_invalid'):
                prepare.validate_source(changed,source,value)
            for token in (b'"mountPath": /etc/voice/release',b'mountPath: *shared',
                          b'mountPath: /etc/voice/./release/policy.json',
                          b'mountPath: /etc/voice/other/../release/policy.json',
                          b'mountPath: //etc/voice/release/policy.json'):
                malformed=raw.replace(b'mountPath: /run/jwt.pem',token)
                path.write_bytes(malformed)
                changed['source_files']['deploy/staging/services.yaml']=hashlib.sha256(malformed).hexdigest()
                with self.assertRaisesRegex(ValueError,'release_policy_mount_syntax_invalid'):
                    prepare.validate_source(changed,source,value)
            path.unlink();path.symlink_to(template)
            with self.assertRaises(ValueError):prepare.validate_source(approved,source,value)

    def test_two_fixed_effects_bind_source_and_exact_policy_payloads(self):
        with tempfile.TemporaryDirectory() as folder:
            source=Path(folder);path=source/'deploy/staging/services.yaml';path.parent.mkdir(parents=True)
            raw=(Path(__file__).resolve().parents[3]/'deploy/staging/services.yaml').read_bytes();path.write_bytes(raw)
            approved={'source_sha':'a'*40,'source_files':{'deploy/staging/services.yaml':hashlib.sha256(raw).hexdigest()}}
            value=prepare.descriptor('a'*40,'fixture','policy.json',owners())
            old=b'{"s3_signing_endpoint":"https://fixture.invalid","opaque":{"keep":true}}'
            plan,payloads=prepare.plan(approved,source,old,value,operation='a'*12,
                namespace_uid='12345678-1234-1234-1234-123456789abc',source_capture_sha256='b'*64,list_rv='123')
            self.assertEqual([e['id'] for e in plan['effects']],['create-policy-configmap','commit-root-policy'])
            self.assertEqual(plan['source_sha'],'a'*40)
            self.assertEqual(plan['policy_before_sha256'],hashlib.sha256(old).hexdigest())
            for effect in plan['effects']:
                self.assertEqual(effect['payload_sha256'],hashlib.sha256(payloads[effect['id']]).hexdigest())
            after=json.loads(payloads['commit-root-policy'])
            self.assertEqual(after['opaque'],{'keep':True});self.assertEqual(after['release_policy'],value)
            self.assertEqual(json.loads(payloads['create-policy-configmap']),value['policy_object'])
            self.assertEqual(plan['policy_after_sha256'],hashlib.sha256(payloads['commit-root-policy']).hexdigest())
            self.assertTrue(prepare.validate_plan(plan,payloads,approved,source,old))
            altered=copy.deepcopy(plan);altered['effects'][1]['before']['sha256']='f'*64
            with self.assertRaises(ValueError):prepare.validate_plan(altered,payloads,approved,source,old)
            with self.assertRaises(ValueError):prepare.validate_plan(plan,dict(payloads,foreign=b'x'),approved,source,old)
            with self.assertRaises(ValueError):prepare.validate_plan(plan,payloads,approved,source,old+b' ')
            with self.assertRaises(ValueError):prepare.plan(approved,source,b'{"same":1,"same":2}',value,
                operation='a'*12,namespace_uid='12345678-1234-1234-1234-123456789abc',source_capture_sha256='b'*64,list_rv='123')
            with self.assertRaises(ValueError):prepare.plan(approved,source,old,value,operation='a'*12,
                namespace_uid='12345678-1234-1234-1234-123456789abc',source_capture_sha256='b'*64,list_rv='')

    def test_execute_uses_only_ordered_coordinator_effects(self):
        calls=[];session=object()
        parent=SimpleNamespace(require_child=lambda current,slot:calls.append(('require',current,slot)),
            record_pending=lambda current,slot,effect:calls.append(('pending',current,slot,effect)),
            submit_effect=lambda current,slot,effect:(calls.append(('submit',current,slot,effect)) or {'effect':effect}),
            record_done=lambda current,slot,effect,observation:calls.append(('done',current,slot,effect,observation)))
        with patch.dict(sys.modules,{'staging_prerequisites':parent}):prepare.execute(session)
        self.assertEqual([row[0] for row in calls],['require','pending','submit','done','pending','submit','done'])
        self.assertEqual([row[3] for row in calls if row[0]=='submit'],['create-policy-configmap','commit-root-policy'])
        self.assertTrue(all(row[1] is session and row[2]=='release_policy' for row in calls))

    def test_uncertain_first_effect_never_submits_policy_write(self):
        calls=[]
        def fail(*args):calls.append(args);raise RuntimeError('unknown-create-result')
        parent=SimpleNamespace(require_child=lambda *a:None,record_pending=lambda *a:None,submit_effect=fail,
                               record_done=lambda *a:self.fail('must not mark unknown effect done'))
        with patch.dict(sys.modules,{'staging_prerequisites':parent}),self.assertRaisesRegex(RuntimeError,'unknown-create-result'):
            prepare.execute(object())
        self.assertEqual(len(calls),1);self.assertEqual(calls[0][2],'create-policy-configmap')

    def test_create_readback_must_be_fresh_exact_uid_rv_and_policy(self):
        with tempfile.TemporaryDirectory() as folder:
            source=Path(folder);path=source/'deploy/staging/services.yaml';path.parent.mkdir(parents=True)
            raw=(Path(__file__).resolve().parents[3]/'deploy/staging/services.yaml').read_bytes();path.write_bytes(raw)
            approved={'source_sha':'a'*40,'source_files':{'deploy/staging/services.yaml':hashlib.sha256(raw).hexdigest()}}
            value=prepare.descriptor('a'*40,'fixture','policy.json',owners())
            plan,payloads=prepare.plan(approved,source,b'{"s3_signing_endpoint":"fixture"}',value,operation='a'*12,
                namespace_uid='12345678-1234-1234-1234-123456789abc',source_capture_sha256='b'*64,list_rv='123')
            desired=value['policy_object'];live=copy.deepcopy(desired)
            live['metadata'].update(uid='12345678-aaaa-aaaa-aaaa-123456789abc',resourceVersion='456')
            observation={'kind':'ConfigMap','name':'fixture','namespace':'voice-staging',
                         'uid':live['metadata']['uid'],'resourceVersion':'456',
                         'semantic_sha256':plan['effects'][0]['desired_sha256']}
            namespace={'metadata':{'name':'voice-staging','uid':plan['namespace_uid']}}
            kube=SimpleNamespace(get=lambda kind,name:namespace if kind=='namespace' else copy.deepcopy(live))
            parent=SimpleNamespace(require_child=lambda *a:copy.deepcopy(plan),
                                   load_payload=lambda current,slot,effect:payloads[effect])
            with patch.dict(sys.modules,{'staging_prerequisites':parent}),patch.object(prepare,'Kube',return_value=kube):
                self.assertEqual(prepare.verify_effect(object(),'create-policy-configmap',observation),observation)
                for field,value2 in (('uid','other'),('resourceVersion','457'),('semantic_sha256','0'*64),('name','other')):
                    bad=dict(observation);bad[field]=value2
                    with self.assertRaises(ValueError):prepare.verify_effect(object(),'create-policy-configmap',bad)
                live['metadata']['ownerReferences']=[{'uid':'foreign'}]
                with self.assertRaises(ValueError):prepare.verify_effect(object(),'create-policy-configmap',observation)
                del live['metadata']['ownerReferences']
                namespace['metadata']['uid']='other'
                with self.assertRaises(ValueError):prepare.verify_effect(object(),'create-policy-configmap',observation)
                namespace['metadata']['uid']=plan['namespace_uid']
                live['data']['policy.json']+=' '
                with self.assertRaises(ValueError):prepare.verify_effect(object(),'create-policy-configmap',observation)
            live=copy.deepcopy(desired);live['metadata'].update(uid=observation['uid'],resourceVersion='456')
            count=0
            def changing(*args):
                nonlocal count
                count+=1;value=copy.deepcopy(plan)
                if count>1:value['operation']='b'*12
                return value
            parent.require_child=changing
            with patch.dict(sys.modules,{'staging_prerequisites':parent}),patch.object(prepare,'Kube',return_value=kube):
                with self.assertRaisesRegex(ValueError,'release_policy_child_plan_changed'):
                    prepare.verify_effect(object(),'create-policy-configmap',observation)

    def test_root_policy_readback_requires_exact_private_payload_and_receipt(self):
        import guard
        with tempfile.TemporaryDirectory() as folder:
            root=Path(folder);source=root/'source';path=source/'deploy/staging/services.yaml';path.parent.mkdir(parents=True)
            raw=(Path(__file__).resolve().parents[3]/'deploy/staging/services.yaml').read_bytes();path.write_bytes(raw)
            approved={'source_sha':'a'*40,'source_files':{'deploy/staging/services.yaml':hashlib.sha256(raw).hexdigest()}}
            value=prepare.descriptor('a'*40,'fixture','policy.json',owners())
            plan,payloads=prepare.plan(approved,source,b'{"s3_signing_endpoint":"fixture"}',value,operation='a'*12,
                namespace_uid='12345678-1234-1234-1234-123456789abc',source_capture_sha256='b'*64,list_rv='123')
            installed=root/'installed';installed.mkdir();policy=installed/'policy.json'
            policy.write_bytes(payloads['commit-root-policy']);policy.chmod(0o600)
            parent=SimpleNamespace(require_child=lambda *a:copy.deepcopy(plan),load_payload=lambda current,slot,effect:payloads[effect])
            observation={'kind':'RootPolicy','name':'installed-policy','sha256':plan['policy_after_sha256']}
            with patch.dict(sys.modules,{'staging_prerequisites':parent}),patch.object(guard,'ROOT',root):
                self.assertEqual(prepare.verify_effect(object(),'commit-root-policy',observation),observation)
                with self.assertRaises(ValueError):prepare.verify_effect(object(),'commit-root-policy',dict(observation,sha256='0'*64))
                policy.write_bytes(b'changed')
                with self.assertRaises(ValueError):prepare.verify_effect(object(),'commit-root-policy',observation)


if __name__=='__main__':unittest.main()
