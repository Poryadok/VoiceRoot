"""Synthetic protected-disk reader regressions; not actual P2 qualification."""
import copy
import hashlib
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import commands
import guard
import resource_claim
import staging_cohort_preservation as cohort
import story_witness as witness
from resource_claim_test import context
import story_preserved_witness_test as structural


class PreservedHistoryTests(unittest.TestCase):
    def fixture(self, root, *, target8=True):
        operation='a'*12;c=context();helper={'nats-rollout-preservation/resource_claim.py':'b'*64}
        helper_sha=cohort.digest(helper);image='ghcr.io/poryadok/voiceroot/story@sha256:'+'c'*64
        prepared={'request':{'gis_first_install':True}}
        approved={'source_files':{'actual/source':'d'*64},'images':{'story':image}}
        execution={'head_sha':c['commit']}
        source={'schema':'voice-staging-source-capture-v1','operation':operation,'context':c,
            'helper_sha256':helper_sha,'approved_sha256':cohort.digest(approved),'execution_sha256':cohort.digest(execution)}
        files={'000005.up.sql':'e'*64,'000008.up.sql':'f'*64};migrations={'story_db':{'from':4,'to':8,'engine':'golang-migrate','files':files}}
        profile={'schema':'voice-story-baseline4-target8-composition-v1','retained_image':image,
            'baseline':{'postgres_user_sha256':hashlib.sha256(b'actual_runtime').hexdigest()},
            'candidate':{'migration_files_sha256':cohort.digest(files)},
            'binding':{'migration_plan_sha256':cohort.digest(migrations)}}
        preflight={'actor':{'story_schema':profile if target8 else {'schema':'voice-story-current4-v1'}}}
        precut={'protected':'cut'};stores={'protected':'all-stores'}
        plan={key:'1'*64 for key in cohort.PLAN_FIELDS}
        plan.update(schema=cohort.SCHEMA,operation=operation,context=c,helper_sha256=helper_sha,
            candidate={'commit':c['commit'],'images':c['images']},actual_before={'voice-story/story':image},namespace_uid='owned-namespace',
            prerequisite_inputs_sha256=cohort.digest(prepared),approved_sha256=cohort.digest(approved),execution_sha256=cohort.digest(execution),
            source_capture_sha256=cohort.digest(source),source_inventory_sha256=cohort.digest(approved['source_files']),
            precut_capture_sha256=cohort.digest(precut),stores_sha256=cohort.digest(stores),preflight_sha256=cohort.digest(preflight))
        fence={'schema':'voice-staging-resource-fence-v1','operation':operation,'context':c,'helper_sha256':helper_sha,'target_sha256':cohort.digest(plan)}
        dump={'status':'DONE','binding':fence,'receipt':{'protected':'actual-dump-receipt'},'plaintext_sha256':'2'*64}
        archive={'manifest':{'protected':'archive-members'},'binding':{'dump_receipts':{'story_db':cohort.digest(dump['receipt'])}}}
        cipher=b'owned ciphertext';cipher_sha=hashlib.sha256(cipher).hexdigest()
        binding={'operation':operation,'challenge':'3'*32,'run_id':17,'head_sha':c['commit'],
            'cipher_sha256':cipher_sha,'cipher_bytes':len(cipher)}
        custody={'schema':'voice-nats-custody-v1','destination':'github:Poryadok/VoiceRoot',**binding,'artifact_id':18,'verified':True}
        base=root/'installed'/'cohort-preservation'/operation
        parent=root/'installed'/'prerequisites'/operation
        def write(path, value):
            directory=root
            for part in path.parent.relative_to(root).parts:
                directory=directory/part;cohort.private._private_directory(directory)
            cohort._write(path,value)
        proofs={};restores={}
        for phase,nonce in (('local','4'*32),('offnode','5'*32)):
            resources={'resources':{name:{'cid':format(index+6,'x')*64,'kind':kind,'status':'REMOVED'}
                for index,(name,kind) in enumerate((('pg','postgres'),('writer','story'),('positive','story-probe'),('negative','story-probe'),('invalid','story-probe')))}}
            proof=structural.PreservedWitnessTests().proof()
            proof['input']={'schema':'voice-story-preserved-target8-input-v1','operation':operation,
                'context_sha256':cohort.digest(c),'helper_sha256':helper_sha,'plan_sha256':cohort.digest(plan),
                'source_capture_sha256':cohort.digest(source),'profile_sha256':cohort.digest(profile),
                'migration_plan_sha256':cohort.digest(migrations),'migration_files_sha256':cohort.digest(files),
                'archive_sha256':cohort.digest(archive),'binding_sha256':cohort.digest(archive['binding']),
                'dump_receipt_sha256':cohort.digest(dump['receipt']),'dump_plaintext_sha256':dump['plaintext_sha256'],
                'restore_attempt':nonce,'phase':phase,'postgres':resources['resources']['pg']['cid'],
                'runtime_user':'actual_runtime','images':{'story0':image},'caller_sha256':__import__('story_fixture').PRESERVED_CALLER_SHA256}
            proof['writer_results'][0]['image']=image
            restored={'nonce':nonce,'phase':phase,'status':'COMPLETE','plan_sha256':cohort.digest(plan),
                'resources_sha256':cohort.digest(resources),'proof':{'cleanup':'verified','manifest_sha256':cohort.digest(archive['manifest'])}}
            if target8:restored['proof']['story_target8']=proof
            write(base/'restore'/nonce/'resources.json',resources);write(base/'restore'/nonce/'receipt.json',restored)
            write(base/(phase+'-restore.json'),restored);proofs[phase]=proof;restores[phase]=restored
        decrypted={'schema':'voice-nats-decrypted-restore-v1','cipher_sha256':cipher_sha,'cipher_bytes':len(cipher),
            'decrypted_members_verified':True,'remote_equivalence':'FULL_CIPHERTEXT_SHA256_AND_SIZE',
            'cohort_readback':{'schema':'voice-staging-cohort-offnode-restore-v1','operation':operation,
                'restore_sha256':cohort.digest(restores['offnode']),'manifest_sha256':cohort.digest(archive['manifest']),
                'cipher_members_verified':True,'cleanup':'verified'}}
        record={'schema':'voice-story-preserved-target8-witness-v1','operation':operation,'profile':profile,
            'plan_sha256':cohort.digest(plan),'archive_sha256':cohort.digest(archive),**proofs}
        key=witness._preserved_key(record);raw=witness.encoded(record);sha=hashlib.sha256(raw).hexdigest()
        authority={'schema':'voice-story-preserved-target8-authority-v1','key':key,'witness_sha256':sha,'registry_revision':1}
        if target8:
            registry=root/'installed'/'story-preserved-witnesses';cohort.private._private_directory(registry)
            witness.write_new(registry/(sha+'.json'),raw)
            write(registry/'selector.json',{'schema':'voice-story-preserved-target8-selector-v1','revision':1,
                'entries':{key:{'witness_sha256':sha,'enabled':True,'revision':1}}})
        records={'paused.json':{'protected':'all-paused'},'archive-record.json':archive,'native-cut.json':{'manifest':{'archive_sha256':'a'*64},'manifest_sha256':'b'*64,'census_sha256':'c'*64},
            'local-restore.json':restores['local'],'offnode-restore.json':restores['offnode'],
            'offnode-custody.json':custody,'decrypted-restore.json':decrypted,'cipher-binding.json':binding,
            'precut.json':precut,'stores.json':stores,'preflight.json':preflight,
            'metadata.json':{'source':{'migrations':migrations,'roles':{'roles':[{'name':'actual_runtime','login':True}]}}}}
        runtime=root/('rollout-'+operation)
        write(runtime/'copy-checkpoint.json',cohort._copy_checkpoint(operation,records['native-cut.json'],resource_claim.production_installation(),False))
        decrypted['members']={'copy-checkpoint.json':hashlib.sha256((runtime/'copy-checkpoint.json').read_bytes()).hexdigest()}
        for name,value in records.items():
            if not (base/name).exists():write(base/name,value)
        write(base/'effects'/'dump-story_db.json',dump)
        if target8:
            write(base/'auth'/('6'*32+'.json'),{'schema':'voice-staging-cohort-auth-scratch-v1',
                'operation':operation,'plan_sha256':cohort.digest(plan),'kind':'actor',
                'base':str(root/('service-actor-auth-'+'7'*32)),'nonce':'6'*32,
                'inputs':{'protected':'captured-auth-inputs'},'resources':{'owned-broker':{'id':'8'*64,'status':'REMOVED'}},'status':'COMPLETE'})
        receipt={'schema':'voice-staging-cohort-preservation-target8-receipt-v1' if target8 else 'voice-staging-cohort-preservation-receipt-v1',
            'operation':operation,'context':c,'helper_sha256':helper_sha,'plan_sha256':cohort.digest(plan),
            'prerequisite_inputs_sha256':cohort.digest(prepared),'actual_before':plan['actual_before'],'namespace_uid':plan['namespace_uid'],
            'resource_fence':fence,'auxiliary_successor_sha256':None}
        for name,file in {'paused_sha256':'paused.json','archive_sha256':'archive-record.json','native_cut_sha256':'native-cut.json',
            'local_restore_sha256':'local-restore.json','offnode_restore_sha256':'offnode-restore.json',
            'custody_sha256':'offnode-custody.json','decrypted_restore_sha256':'decrypted-restore.json','cipher_binding_sha256':'cipher-binding.json'}.items():
            receipt[name]=cohort.digest(records[file])
        if target8:receipt['story_target8']=authority
        successor={'plan':{'schema':'voice-staging-prerequisites-plan-v2','preservation_sha256':cohort.digest(receipt)}}
        captured={'plan_sha256':cohort.digest(successor['plan'])}
        target={'tag':c['commit'],'images':c['images'],'prerequisites_sha256':cohort.digest(captured),
            'cohort_preservation_sha256':cohort.digest(receipt)}
        rollout={'schema':'nats-rollout-root-v1','operation':operation,'status':'PASS','phase':'POST_RESTART','fence_status':'RELEASED','target':target,
            'migrations':migrations,'code_capture':helper,'prerequisites':captured,'resource_fence':{**fence,'target_sha256':cohort.digest(target)}}
        write(base/'checkpoint.json',{'status':'QUALIFIED','plan':plan,'initial_resource_fence':fence})
        write(base/'qualified.json',receipt)
        for name,value in {'prepared.json':prepared,'checkpoint.json':successor,'capture.json':captured,'approved.json':approved,'execution.json':execution}.items():write(parent/name,value)
        write(root/'installed'/'source-authority'/(operation+'.json'),source)
        runtime=root/('rollout-'+operation);write(runtime/'checkpoint.json',rollout)
        cohort.private._private_write(runtime/'rollout-backup.cms',cipher)
        return operation,receipt,base

    def test_target8_historical_pass_validates_protected_witness_without_session_or_effects(self):
        with tempfile.TemporaryDirectory(dir='/root') as directory:
            root=Path(directory)
            with patch.object(guard,'ROOT',root):
                operation,receipt,_=self.fixture(root)
                before={str(p):p.read_bytes() for p in root.rglob('*') if p.is_file()}
                with patch.object(commands,'capture',side_effect=AssertionError('history ran external command')),\
                    patch.object(resource_claim.Session,'__init__',side_effect=AssertionError('history revived Session')):
                    self.assertEqual(cohort.validate_historical_capture(operation),receipt)
                self.assertEqual({str(p):p.read_bytes() for p in root.rglob('*') if p.is_file()},before)

    def test_legacy_receipt_keeps_its_exact_historical_shape(self):
        with tempfile.TemporaryDirectory(dir='/root') as directory:
            root=Path(directory)
            with patch.object(guard,'ROOT',root):
                operation,receipt,base=self.fixture(root,target8=False)
                self.assertEqual(cohort.validate_historical_capture(operation),receipt)
                changed={**receipt,'story_target8':{}}
                cohort.save(base/'qualified.json',changed)
                with self.assertRaises(Exception):cohort.validate_historical_capture(operation)

    def test_target8_historical_drift_revocation_and_open_shapes_are_refused(self):
        for fault in ('revoked','missing_registry','witness','profile','dump_binding','cleanup','pending','auth_pending','auth_extra','auth_resource','missing_proof','extra','legacy_tag','final_plan','final_preservation'):
            with self.subTest(fault=fault),tempfile.TemporaryDirectory(dir='/root') as directory:
                root=Path(directory)
                with patch.object(guard,'ROOT',root):
                    operation,receipt,base=self.fixture(root)
                    if fault=='missing_registry':(root/'installed'/'story-preserved-witnesses'/'selector.json').unlink()
                    elif fault=='revoked':
                        path=root/'installed'/'story-preserved-witnesses'/'selector.json';row=cohort._read(path)
                        next(iter(row['entries'].values()))['enabled']=False;cohort.save(path,row)
                    elif fault=='witness':
                        path=root/'installed'/'story-preserved-witnesses'/(receipt['story_target8']['witness_sha256']+'.json')
                        row=cohort._read(path);row['local']['table_count']=7;cohort.save(path,row)
                    elif fault.startswith('auth_'):
                        path=base/'auth'/('6'*32+'.json');row=cohort._read(path)
                        if fault=='auth_pending':row['status']='RUNNING'
                        elif fault=='auth_extra':row['trusted_without_proof']=True
                        else:row['resources']['owned-broker']['id']='foreign'
                        cohort.save(path,row)
                    elif fault in ('profile','dump_binding','cleanup','pending'):
                        path=base/({'profile':'preflight.json','dump_binding':'effects/dump-story_db.json','cleanup':'restore/'+('4'*32)+'/resources.json','pending':'effects/dump-story_db.json'}[fault]);row=cohort._read(path)
                        if fault=='profile':row['actor']['story_schema']['retained_image']='foreign'
                        elif fault=='dump_binding':row['binding']['target_sha256']='0'*64
                        elif fault=='cleanup':row['resources']['writer']['status']='RUNNING'
                        else:row['status']='PENDING'
                        cohort.save(path,row)
                    elif fault.startswith('final_'):
                        path=root/('rollout-'+operation)/'checkpoint.json';row=cohort._read(path)
                        if fault=='final_plan':row['migrations']['story_db']['to']=4
                        else:
                            row['target']['cohort_preservation_sha256']='0'*64
                            row['resource_fence']['target_sha256']=cohort.digest(row['target'])
                        cohort.save(path,row)
                    else:
                        changed=copy.deepcopy(receipt)
                        if fault=='missing_proof':del changed['story_target8']
                        elif fault=='extra':changed['accepted_without_validation']=True
                        else:changed['schema']='voice-staging-cohort-preservation-receipt-v1'
                        cohort.save(base/'qualified.json',changed)
                    expected=('cohort_history_changed' if fault in ('profile','missing_proof','extra','legacy_tag')
                        else 'cohort_restore_cleanup_changed' if fault=='cleanup'
                        else 'cohort_history_incomplete' if fault=='pending' or fault.startswith('auth_')
                        else 'story_compatibility_witness_invalid')
                    with self.assertRaisesRegex(Exception,expected):cohort.validate_historical_capture(operation)


if __name__=='__main__':unittest.main()
