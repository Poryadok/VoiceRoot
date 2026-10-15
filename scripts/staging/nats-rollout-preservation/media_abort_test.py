import copy
import hashlib
import json
import tempfile
import unittest
from unittest.mock import patch
from pathlib import Path
import operator_retirement as retirement
import installer
from controller import Blocked


class MediaAbortTest(unittest.TestCase):
    def test_prepared_normalized_configmap_cannot_be_replaced(self):
        rows=[{'kind':'ConfigMap','metadata':{'name':'voice-app-config'},'data':{'safe':'true'}}]
        prepared={'normalized_sha256':retirement.canonical(rows)}
        retirement.media_target_prepared(rows,prepared)
        changed=copy.deepcopy(rows);changed[0]['data']['safe']='tampered'
        with self.assertRaises(Blocked):retirement.media_target_prepared(changed,prepared)
        with self.assertRaises(Blocked):retirement.media_target_prepared(rows,{})

    def test_target_baseline_binds_source_images_migrations_and_retained_hub(self):
        import source_authority
        names=retirement.CORE|{'voice-web','voice-admin'}
        source='b'*40
        images={name:source_authority.REGISTRY+'/'+name+'@sha256:'+'c'*64 for name in source_authority.IMAGE_NAMES}
        snapshots={n:{'metadata':{'uid':n},'spec':{'template':{'spec':{'containers':[
            {'name':n.removeprefix('voice-'),'image':images.get(n.removeprefix('voice-'),'hub@sha256:'+'d'*64)}]}}}} for n in names}
        snapshot={'context':{'snapshots':snapshots},'migrations':[{'kept':True}], 'active_contract':{'contract':{'kept':True}}}
        documents=[{'kind':'Deployment','metadata':{'name':n,'namespace':'voice-staging'},'spec':row['spec']}
                   for n,row in snapshots.items() if n!=retirement.HUB]
        used={r['metadata']['name']+'/'+c['name']:c['image'] for r in documents for c in r['spec']['template']['spec']['containers']}
        source_files={'deploy/staging/services.yaml':'e'*64}
        target={'tag':source,'mode':'app-only','changed_services':[],'images':used,'source_hashes':source_files,
            'template_hashes':{r['metadata']['name']:retirement.canonical(r['spec']['template']) for r in documents},
            'manifest_sha256':retirement.canonical(documents),'migration_sha256':retirement.canonical(snapshot['migrations'])}
        pack={'target':target,'manifests':documents,'migrations':snapshot['migrations'],'contract':snapshot['active_contract']['contract']}
        authority={'schema':'voice-source-authority-v1','repository':source_authority.REPO,'source_sha':source,
            'workflow_id':source_authority.WORKFLOW,'run_id':123,'run_attempt':1,'tree_sha':'f'*40,
            'source_files':source_files,'images':images,'lock_artifact_id':456,'lock_sha256':'a'*64,
            'image_provenance':'independently resolved trusted CI tag','verified':True}
        normalized=copy.deepcopy(documents)
        result=retirement.media_target_templates(snapshot,authority,pack,normalized)
        self.assertEqual(set(result),names)
        self.assertEqual(result[retirement.HUB],snapshots[retirement.HUB]['spec']['template'])
        for what in ('source','image','migration','hub','normalized_image','missing_deployment'):
            a,p,n=copy.deepcopy(authority),copy.deepcopy(pack),copy.deepcopy(normalized)
            if what=='source':a['source_files']['deploy/staging/services.yaml']='0'*64
            if what=='image':a['images']['voice']=images['auth']
            if what=='migration':p['migrations'].append({'unexpected':True})
            if what=='hub':n.append({'kind':'Deployment','metadata':{'name':retirement.HUB},'spec':snapshots[retirement.HUB]['spec']})
            if what=='normalized_image':n[0]['spec']['template']['spec']['containers'][0]['image']='floating:tag'
            if what=='missing_deployment':n.pop()
            with self.subTest(what=what),self.assertRaises(Blocked):retirement.media_target_templates(snapshot,a,p,n)

    def test_complete_receipt_and_retained_bytes_are_required_for_idle_admission(self):
        def encoded(value):return json.dumps(value,sort_keys=True,separators=(',',':')).encode()
        with tempfile.TemporaryDirectory() as td:
            root=Path(td);base=root/('rollout-'+retirement.MEDIA_ABORT_OPERATION);base.mkdir()
            prefix=retirement.MEDIA_ABORT_PREFIX
            names=retirement.CORE|{'voice-web','voice-admin'}
            originals={n:{'metadata':{'uid':n},'spec':{'template':{'spec':{'containers':[{'name':'app','image':'old'}]}}}} for n in names}
            context={'original_snapshots':originals,'snapshots':copy.deepcopy(originals),
                'old_images':{n+'/app':'example.invalid/app@sha256:'+'a'*64 for n in names},
                'marker':{'metadata':{'uid':'marker','resourceVersion':'1'},'data':{'phase':'rollout-verified','knownRolloutOperation':retirement.MEDIA_ABORT_OPERATION}},
                'expected':{'namespace_uid':'ns','source_claim':'pvc','source_claim_uid':'claim','source_pv_uid':'pv'},
                'final_claim':{'spec':{}},'final_pv':{'spec':{}},'final_path':'/private/pvc',
                'service':{'metadata':{'uid':'service'}}}
            snapshot={'operation':retirement.MEDIA_ABORT_OPERATION,'phase':'RESTART','status':'BLOCKED','fence_status':'VERIFIED',
                'error':'restart_readiness_timeout','events':[],'migration_jobs':[{'name':str(i)} for i in range(16)],
                'active_contract':{'contract':'retained'},'preservation':{'native_files_verified':True,'archive':'post/archive.tar'},
                'nats_migration':{'verified':True},'code_capture':{},'context':context}
            original=encoded(snapshot);active=retirement.canonical(snapshot['active_contract']);note=b'compatibility'
            def write(name,value):
                path=base/name;path.parent.mkdir(parents=True,exist_ok=True);path.write_bytes(value);path.chmod(0o600)
            write(prefix+'-snapshot.json',original)
            roles=set(retirement.LEAVES)|{'gateway'}
            setup={n:{'uid':n,'template_sha256':retirement.canonical(t),'replicas':1,'ready':True}
                for n,t in retirement.media_abort_templates(snapshot).items()}
            proof={'setup':setup,'storage':retirement.precut_retained_setup(snapshot),'active_contract_sha256':active,
                'migration_jobs_sha256':retirement.canonical(snapshot['migration_jobs']),'no_migration_replay':True,
                'actors':{'roles':sorted(roles),'proofs':{s:{'proof':{k:True for k in ('signed_chain_verified',
                    'server_authentication_verified','signed_effective_permissions_verified','broker_grant_compatibility_verified')}} for s in roles}},
                'leaf_connections':{s:{'connected':True,'remote_server_sha256':'a'*64} for s in retirement.LEAVES}}
            values={name:b'retained' for name in ('rollout-before.tar','rollout-before-manifest.json','copy-checkpoint.json',
                'rollout-backup.cms','decrypted-restore.json','apply-manifests.json','apply-authorization.json','post/archive.tar')}
            values.update({prefix+'-compatibility.md':note,prefix+'-active-contract.json':encoded(snapshot['active_contract']),prefix+'-verified.json':encoded(proof)})
            for name,value in values.items():write(name,value)
            receipt={'schema':'voice-media-original-restore-closure-v1','operation':retirement.MEDIA_ABORT_OPERATION,
                'waiver':retirement.WAIVER,'snapshot_sha256':hashlib.sha256(original).hexdigest(),'compatibility_sha256':hashlib.sha256(note).hexdigest(),
                'deployment_pass':False,'rollback_eligible':False,'configuration_basis':'ORIGINAL_APPLICATION_IMAGES_RETAIN_MIGRATED_NATS_AND_DATABASES',
                'migration_jobs_sha256':retirement.canonical(snapshot['migration_jobs']),'active_contract_sha256':active,'setup':setup,
                'released_marker':{'uid':'marker','resourceVersion':'2','data':{'phase':'active'}},
                'retained_files':{n:{'sha256':hashlib.sha256(v).hexdigest(),'bytes':len(v)} for n,v in values.items()},'journal':{'absent':True}}
            with patch.object(retirement.guard,'ROOT',root),patch.object(retirement,'MEDIA_ABORT_CHECKPOINT',receipt['snapshot_sha256']),\
                 patch.object(retirement,'MEDIA_ABORT_COMPATIBILITY',receipt['compatibility_sha256']),patch.object(retirement,'MEDIA_ABORT_ACTIVE',active),\
                 patch.object(retirement.root_cli,'code_binding',return_value={}),patch.object(retirement,'media_abort_journal',return_value={'absent':True}):
                write(prefix+'-receipt.json',encoded(receipt));terminal=retirement.media_abort_terminal(snapshot,receipt)
                self.assertEqual(retirement.verify_media_abort_terminal(base,terminal),receipt)
                for field,value in [('deployment_pass',True),('rollback_eligible',True),('configuration_basis','TARGET')]:
                    changed=copy.deepcopy(receipt);changed[field]=value;write(prefix+'-receipt.json',encoded(changed))
                    with self.subTest(field=field),self.assertRaises(Blocked):retirement.verify_media_abort_terminal(base,retirement.media_abort_terminal(snapshot,changed))
                write(prefix+'-receipt.json',encoded(receipt));write('apply-manifests.json',b'drift')
                with self.assertRaises(Blocked):retirement.verify_media_abort_terminal(base,terminal)
                # The failed old-image attempt cannot be relabeled. A new target
                # closure binds both failures and its own independent source proof.
                import source_authority
                write('apply-manifests.json',values['apply-manifests.json'])
                (base/(prefix+'-receipt.json')).unlink();(base/(prefix+'-verified.json')).unlink()
                snapshot['migrations']=[{'database':'already-applied'}]
                snapshot['active_contract']={'contract':{'retained':True}}
                original=encoded(snapshot);write(prefix+'-snapshot.json',original)
                retirement.MEDIA_ABORT_CHECKPOINT=hashlib.sha256(original).hexdigest()
                retirement.MEDIA_ABORT_ACTIVE=retirement.canonical(snapshot['active_contract'])
                blocked=copy.deepcopy(snapshot);blocked['phase']='OPERATOR_RETIRING'
                blocked_raw=encoded(blocked)
                progress={'phase':'BLOCKED','snapshot_sha256':retirement.MEDIA_ABORT_CHECKPOINT,'context':context}
                progress_raw=encoded(progress);new=retirement.MEDIA_TARGET_PREFIX
                write(new+'-blocked.json',blocked_raw);write(prefix+'-progress.json',progress_raw)
                source='b'*40
                images={n:source_authority.REGISTRY+'/'+n+'@sha256:'+'c'*64 for n in source_authority.IMAGE_NAMES}
                documents=[]
                for n,row in originals.items():
                    if n==retirement.HUB:continue
                    obj={'kind':'Deployment','metadata':{'name':n,'uid':n,'namespace':'voice-staging'},'spec':copy.deepcopy(row['spec'])}
                    obj['spec']['template']['spec']['containers'][0]['image']=images[n.removeprefix('voice-')];documents.append(obj)
                documents.append({'kind':'ConfigMap','metadata':{'name':'voice-app-config','uid':'config','namespace':'voice-staging'},'data':{'safe':'true'}})
                hashes={'deploy/staging/services.yaml':'e'*64}
                authority={'schema':'voice-source-authority-v1','repository':source_authority.REPO,'source_sha':source,
                    'workflow_id':source_authority.WORKFLOW,'run_id':123,'run_attempt':1,'tree_sha':'f'*40,'source_files':hashes,
                    'images':images,'lock_artifact_id':456,'lock_sha256':'a'*64,'image_provenance':'independently resolved trusted CI tag','verified':True}
                target={'tag':source,'mode':'app-only','changed_services':[],'source_hashes':hashes,
                    'images':{d['metadata']['name']+'/app':d['spec']['template']['spec']['containers'][0]['image'] for d in documents if d['kind']=='Deployment'},
                    'template_hashes':{d['metadata']['name']:retirement.canonical(d['spec']['template']) for d in documents if d['kind']=='Deployment'},
                    'manifest_sha256':retirement.canonical(documents),'migration_sha256':retirement.canonical(snapshot['migrations'])}
                pack={'target':target,'manifests':documents,'migrations':snapshot['migrations'],'contract':snapshot['active_contract']['contract']}
                templates=retirement.media_target_templates(blocked,authority,pack,documents)
                setup={n:{'uid':n,'template_sha256':retirement.canonical(t),'replicas':1,'ready':True} for n,t in templates.items()}
                proof.update(setup=setup,active_contract_sha256=retirement.MEDIA_ABORT_ACTIVE,resources=retirement.media_target_resources(documents))
                new_values={n:v for n,v in values.items() if not n.startswith(prefix+'-')}
                new_values.update({new+'-compatibility.md':note,new+'-active-contract.json':encoded(snapshot['active_contract']),
                    new+'-verified.json':encoded(proof),new+'-blocked.json':blocked_raw,prefix+'-progress.json':progress_raw,
                    prefix+'-snapshot.json':original,new+'-source-authority.json':encoded(authority),new+'-compiled.json':encoded(pack),
                    new+'-normalized.json':encoded(documents),new+'-parameters.json':encoded({'tag':source}),new+'-prepared.json':encoded({'normalized_sha256':retirement.canonical(documents)})})
                for name,value in new_values.items():write(name,value)
                receipt.update(schema='voice-media-target-baseline-closure-v1',configuration_basis=retirement.MEDIA_TARGET_BASIS,
                    snapshot_sha256=retirement.MEDIA_ABORT_CHECKPOINT,active_contract_sha256=retirement.MEDIA_ABORT_ACTIVE,setup=setup,
                    retained_files={n:{'sha256':hashlib.sha256(v).hexdigest(),'bytes':len(v)} for n,v in new_values.items()})
                with patch.object(retirement,'MEDIA_TARGET_BLOCKED',hashlib.sha256(blocked_raw).hexdigest()),\
                     patch.object(retirement,'MEDIA_TARGET_PROGRESS',hashlib.sha256(progress_raw).hexdigest()):
                    write(new+'-receipt.json',encoded(receipt));terminal=retirement.media_abort_terminal(blocked,receipt)
                    self.assertEqual(retirement.verify_media_abort_terminal(base,terminal),receipt)
                    self.assertEqual(terminal['status'],'ABORTED')
                    self.check_reconciled_terminal(base,blocked,documents,receipt,proof,authority,pack,encoded,write)
                    # Rehashing the changed normalized data and its live proof
                    # into a closure cannot replace the prepare-time authority.
                    changed_rows=copy.deepcopy(documents);changed_rows[-1]['data']['safe']='tampered'
                    changed_proof=copy.deepcopy(proof);changed_proof['resources']=retirement.media_target_resources(changed_rows)
                    changed=copy.deepcopy(receipt)
                    for name,value in ((new+'-normalized.json',changed_rows),(new+'-verified.json',changed_proof)):
                        value_raw=encoded(value);write(name,value_raw)
                        changed['retained_files'][name]={'sha256':hashlib.sha256(value_raw).hexdigest(),'bytes':len(value_raw)}
                    write(new+'-receipt.json',encoded(changed))
                    with self.assertRaises(Blocked):retirement.verify_media_abort_terminal(base,retirement.media_abort_terminal(blocked,changed))
                    write(new+'-normalized.json',encoded(documents));write(new+'-verified.json',encoded(proof));write(new+'-receipt.json',encoded(receipt))
                    bad=copy.deepcopy(proof);bad['resources'][next(iter(bad['resources']))]['semantic_sha256']='0'*64
                    bad_raw=encoded(bad);write(new+'-verified.json',bad_raw)
                    changed=copy.deepcopy(receipt);changed['retained_files'][new+'-verified.json']={'sha256':hashlib.sha256(bad_raw).hexdigest(),'bytes':len(bad_raw)}
                    write(new+'-receipt.json',encoded(changed))
                    with self.assertRaises(Blocked):retirement.verify_media_abort_terminal(base,retirement.media_abort_terminal(blocked,changed))
                    write(new+'-verified.json',encoded(proof));write(new+'-receipt.json',encoded(receipt))
                    for name in (new+'-blocked.json',prefix+'-progress.json',new+'-source-authority.json',new+'-normalized.json'):
                        prior=(base/name).read_bytes();write(name,b'{}')
                        with self.subTest(retained=name),self.assertRaises(Blocked):retirement.verify_media_abort_terminal(base,terminal)
                        write(name,prior)
                    write(prefix+'-receipt.json',b'{}')
                    with self.assertRaises(Blocked):retirement.verify_media_abort_terminal(base,terminal)

    def check_reconciled_terminal(self,base,blocked,documents,receipt,proof,authority,pack,encoded,write):
        old=retirement.MEDIA_TARGET_PREFIX;new=retirement.MEDIA_RECONCILED_PREFIX
        restore={path.name:path.read_bytes() for path in base.iterdir() if path.is_file()}
        try:
            # Represent the actual partial boundary: Auth CAS succeeded before
            # its event, other Deployment templates remain at their old values.
            ordered=sorted(copy.deepcopy(documents),key=lambda row:row['metadata']['name']!='voice-auth')
            before={row['kind']+'/'+row['metadata']['name']:copy.deepcopy(row) for row in ordered}
            for key,row in before.items():
                if row['kind']=='Deployment':row['spec']=copy.deepcopy(blocked['context']['snapshots'][row['metadata']['name']]['spec'])
            observed=copy.deepcopy(before);auth=ordered[0];self.assertEqual(auth['metadata']['name'],'voice-auth')
            observed['Deployment/voice-auth']=copy.deepcopy(auth)
            observed['Deployment/voice-auth']['metadata']['annotations']={'deployment.kubernetes.io/revision':'42'}
            failed=copy.deepcopy(blocked);failed['fence_status']='UNKNOWN'
            failed['context']['snapshots']['voice-auth']=copy.deepcopy(observed['Deployment/voice-auth'])
            progress={'phase':'BLOCKED','context':failed['context'],'events':[]}
            historical={new+'-blocked.json':encoded(failed),old+'-progress.json':encoded(progress),
                old+'-normalized.json':encoded(ordered),old+'-prepared.json':encoded({'before':before,
                    'normalized_sha256':retirement.canonical(ordered),'outside_owned':{},'preflight':{'objects':[]}})}
            for name,value in historical.items():write(name,value)
            for suffix in ('receipt','verified'):(base/(old+'-'+suffix+'.json')).unlink()
            pins={name:{'bytes':len(value),'sha256':hashlib.sha256(value).hexdigest()} for name,value in historical.items()}
            with patch.object(retirement,'MEDIA_FAILED_APPLY',pins),patch.object(retirement,'MEDIA_FAILED_APPLIED',()),\
                 patch.object(retirement,'MEDIA_FAILED_ADMIN',{}):
                reconciliation=retirement.media_failed_reconciliation(base,observed)
                prepared={'normalized_sha256':retirement.canonical(documents),'before':observed,
                    'failed_apply':pins,'reconciliation':reconciliation,'outside_owned':{}}
                newproof=dict(proof,outside_owned={})
                values={name:raw for name,raw in restore.items() if name in receipt['retained_files'] and not name.startswith(old+'-')}
                values.update(historical)
                values.update({new+'-compatibility.md':restore[old+'-compatibility.md'],
                    new+'-active-contract.json':restore[old+'-active-contract.json'],new+'-verified.json':encoded(newproof),
                    new+'-source-authority.json':encoded(authority),new+'-compiled.json':encoded(pack),
                    new+'-normalized.json':encoded(documents),new+'-parameters.json':b'{}',new+'-prepared.json':encoded(prepared)})
                # Archive paths may be nested, so use the existing canonical
                # receipt inventory rather than just the flat fixture files.
                for name in receipt['retained_files']:
                    if not name.startswith(old+'-') and name not in values:values[name]=(base/name).read_bytes()
                for name,value in values.items():write(name,value)
                closure=copy.deepcopy(receipt);closure.update(schema=retirement.MEDIA_RECONCILED_SCHEMA,
                    retained_files={name:{'bytes':len(value),'sha256':hashlib.sha256(value).hexdigest()} for name,value in values.items()})
                write(new+'-receipt.json',encoded(closure))
                terminal=retirement.media_abort_terminal(failed,closure)
                self.assertEqual(retirement.verify_media_abort_terminal(base,terminal),closure)
                self.assertEqual(terminal['operator_media_abort']['schema'],retirement.MEDIA_RECONCILED_REFERENCE)
                self.check_gateway_terminal(base,failed,closure,prepared,newproof,encoded,write)
                for schema in ('voice-media-target-baseline-reference-v1','voice-media-original-restore-reference-v1'):
                    forged=copy.deepcopy(terminal);forged['operator_media_abort']['schema']=schema
                    with self.subTest(schema=schema),self.assertRaises((Blocked,FileNotFoundError)):
                        retirement.verify_media_abort_terminal(base,forged)
                tampered=copy.deepcopy(prepared);tampered['reconciliation']={}
                raw=encoded(tampered);write(new+'-prepared.json',raw)
                forged=copy.deepcopy(closure);forged['retained_files'][new+'-prepared.json']={'bytes':len(raw),'sha256':hashlib.sha256(raw).hexdigest()}
                write(new+'-receipt.json',encoded(forged))
                with self.assertRaises(Blocked):retirement.verify_media_abort_terminal(base,retirement.media_abort_terminal(failed,forged))
        finally:
            for name,value in restore.items():write(name,value)

    def check_gateway_terminal(self,base,failed,closure,prepared,proof,encoded,write):
        old=retirement.MEDIA_RECONCILED_PREFIX;new=retirement.MEDIA_GATEWAY_PREFIX
        pins=copy.deepcopy(retirement.MEDIA_FAILED_APPLY)
        blocked=copy.deepcopy(failed);blocked['fence_status']='VERIFIED'
        blocked['events'].append({'kind':'gateway_failed_then_refenced'})
        blocked_raw=encoded(blocked);write(new+'-blocked.json',blocked_raw)
        pins[new+'-blocked.json']={'bytes':len(blocked_raw),'sha256':hashlib.sha256(blocked_raw).hexdigest()}
        progress_raw=encoded({'phase':'BLOCKED'});write(old+'-progress.json',progress_raw)
        pins[old+'-progress.json']={'bytes':len(progress_raw),'sha256':hashlib.sha256(progress_raw).hexdigest()}
        new_prepared=copy.deepcopy(prepared);new_prepared.pop('failed_apply')
        new_prepared['failed_restart']=pins
        values={name:(base/name).read_bytes() for name in closure['retained_files'] if not name.startswith(old+'-')}
        values.update({name:(base/name).read_bytes() for name in pins})
        for suffix in ('compatibility.md','active-contract.json','source-authority.json','compiled.json',
                       'normalized.json','parameters.json'):
            values[new+'-'+suffix]=(base/(old+'-'+suffix)).read_bytes()
        values[new+'-prepared.json']=encoded(new_prepared);values[new+'-verified.json']=encoded(proof)
        for name,value in values.items():write(name,value)
        receipt=copy.deepcopy(closure);receipt.update(schema=retirement.MEDIA_GATEWAY_SCHEMA,
            retained_files={name:{'bytes':len(value),'sha256':hashlib.sha256(value).hexdigest()} for name,value in values.items()})
        with patch.object(retirement,'MEDIA_FAILED_RESTART',pins),patch.object(retirement,'media_restart_pins',return_value=pins),\
             patch.object(retirement,'media_restart_failed',return_value=blocked),\
             patch.object(retirement,'media_restart_reconciliation',return_value=prepared['reconciliation']):
            write(new+'-receipt.json',encoded(receipt));terminal=retirement.media_abort_terminal(blocked,receipt)
            self.assertEqual(retirement.verify_media_abort_terminal(base,terminal),receipt)
            self.assertEqual(terminal['operator_media_abort']['schema'],retirement.MEDIA_GATEWAY_REFERENCE)
            for schema in (retirement.MEDIA_RECONCILED_REFERENCE,'voice-media-target-baseline-reference-v1'):
                forged=copy.deepcopy(terminal);forged['operator_media_abort']['schema']=schema
                with self.assertRaises((Blocked,FileNotFoundError)):retirement.verify_media_abort_terminal(base,forged)
            tampered=copy.deepcopy(new_prepared);tampered['failed_restart']={}
            raw=encoded(tampered);write(new+'-prepared.json',raw)
            forged=copy.deepcopy(receipt);forged['retained_files'][new+'-prepared.json']={'bytes':len(raw),'sha256':hashlib.sha256(raw).hexdigest()}
            write(new+'-receipt.json',encoded(forged))
            with self.assertRaises(Blocked):retirement.verify_media_abort_terminal(base,retirement.media_abort_terminal(blocked,forged))
        write(new+'-prepared.json',encoded(new_prepared));write(new+'-receipt.json',encoded(receipt))
        self.check_monitor_terminal(base,blocked,receipt,new_prepared,proof,encoded,write)

    def check_monitor_terminal(self,base,failed,closure,prepared,proof,encoded,write):
        old=retirement.MEDIA_GATEWAY_PREFIX;new=retirement.MEDIA_MONITOR_PREFIX
        pins=copy.deepcopy(closure['retained_files'])
        blocked=copy.deepcopy(failed);blocked['fence_status']='VERIFIED'
        blocked['events'].append({'kind':'gateway_failed_then_refenced'})
        blocked_raw=encoded(blocked);write(new+'-blocked.json',blocked_raw)
        pins[new+'-blocked.json']={'bytes':len(blocked_raw),'sha256':hashlib.sha256(blocked_raw).hexdigest()}
        progress_raw=encoded({'phase':'BLOCKED'});write(old+'-progress.json',progress_raw)
        pins[old+'-progress.json']={'bytes':len(progress_raw),'sha256':hashlib.sha256(progress_raw).hexdigest()}
        new_prepared=copy.deepcopy(prepared);new_prepared.pop('failed_restart')
        new_prepared['failed_monitor']=pins
        values={name:(base/name).read_bytes() for name in closure['retained_files'] if not name.startswith(old+'-')}
        values.update({name:(base/name).read_bytes() for name in pins})
        for suffix in ('compatibility.md','active-contract.json','source-authority.json','compiled.json',
                       'normalized.json','parameters.json'):
            values[new+'-'+suffix]=(base/(old+'-'+suffix)).read_bytes()
        values[new+'-prepared.json']=encoded(new_prepared);values[new+'-verified.json']=encoded(proof)
        for name,value in values.items():write(name,value)
        receipt=copy.deepcopy(closure);receipt.update(schema=retirement.MEDIA_MONITOR_SCHEMA,
            retained_files={name:{'bytes':len(value),'sha256':hashlib.sha256(value).hexdigest()} for name,value in values.items()})
        with patch.object(retirement,'MEDIA_FAILED_MONITOR',pins),patch.object(retirement,'media_monitor_pins',return_value=pins),\
             patch.object(retirement,'media_monitor_failed',return_value=blocked),\
             patch.object(retirement,'media_monitor_reconciliation',return_value=prepared['reconciliation']):
            write(new+'-receipt.json',encoded(receipt));terminal=retirement.media_abort_terminal(blocked,receipt)
            self.assertEqual(retirement.verify_media_abort_terminal(base,terminal),receipt)
            self.assertEqual(terminal['operator_media_abort']['schema'],retirement.MEDIA_MONITOR_REFERENCE)
            for schema in (retirement.MEDIA_GATEWAY_REFERENCE,'voice-media-target-baseline-reference-v1'):
                forged=copy.deepcopy(terminal);forged['operator_media_abort']['schema']=schema
                with self.assertRaises((Blocked,FileNotFoundError)):retirement.verify_media_abort_terminal(base,forged)
            tampered=copy.deepcopy(new_prepared);tampered['failed_monitor']={}
            raw=encoded(tampered);write(new+'-prepared.json',raw)
            forged=copy.deepcopy(receipt);forged['retained_files'][new+'-prepared.json']={'bytes':len(raw),'sha256':hashlib.sha256(raw).hexdigest()}
            write(new+'-receipt.json',encoded(forged))
            with self.assertRaises(Blocked):retirement.verify_media_abort_terminal(base,retirement.media_abort_terminal(blocked,forged))
        write(new+'-prepared.json',encoded(new_prepared));write(new+'-receipt.json',encoded(receipt))

    def test_exact_snapshot_hash_is_required_before_any_structural_authority(self):
        for raw in (b'{}', b'{"operation":"0e2be62a3326","status":"BLOCKED"}'):
            with self.assertRaises(Blocked): retirement.media_abort_snapshot(raw)

    def test_terminal_preserves_failed_evidence_and_never_grants_rollback(self):
        original={'events':[{'kind':'failed'}],'error':'restart_readiness_timeout',
                  'nats_migration':{'verified':True},'migration_jobs':[{'name':'kept'}]}
        terminal=retirement.media_abort_terminal(original,{'deployment_pass':False})
        self.assertEqual(terminal['status'],'ABORTED')
        self.assertEqual(terminal['error'],original['error'])
        self.assertEqual(terminal['nats_migration'],original['nats_migration'])
        self.assertEqual(terminal['migration_jobs'],original['migration_jobs'])
        self.assertEqual(original['events'],[{'kind':'failed'}])
        self.assertIs(terminal['events'][-1]['deployment_pass'],False)
        self.assertIs(terminal['events'][-1]['rollback_eligible'],False)

    def test_original_apps_are_digest_pinned_while_migrated_hub_is_retained(self):
        names=retirement.CORE|{'voice-web','voice-admin'}
        originals={n:{'spec':{'template':{'spec':{'containers':[{'name':'app','image':'old:tag'}]}}}} for n in names}
        current=copy.deepcopy(originals);current[retirement.HUB]['spec']['template']['spec']['containers'][0]['image']='migrated-hub'
        images={n+'/app':'example.invalid/app@sha256:'+'a'*64 for n in names}
        snapshot={'context':{'original_snapshots':originals,'snapshots':current,'old_images':images}}
        result=retirement.media_abort_templates(snapshot)
        self.assertEqual(result[retirement.HUB],current[retirement.HUB]['spec']['template'])
        self.assertEqual(result['voice-user']['spec']['containers'][0]['image'],images['voice-user/app'])
        images['voice-user/app']='floating:tag'
        with self.assertRaises(Blocked):retirement.media_abort_templates(snapshot)

    def test_absent_finish_response_is_preserved_and_any_new_response_vetoed(self):
        request={'action':'finish','operation':retirement.MEDIA_ABORT_OPERATION,'nonce':retirement.MEDIA_ABORT_FINISH[:-5]}
        row={'phase':'STARTED','request':request,'request_sha256':retirement.canonical(request)}
        raw=json.dumps(row).encode()
        with tempfile.TemporaryDirectory() as td:
            root=Path(td);(root/'responses').mkdir()
            with patch.object(retirement,'private_bytes',return_value=raw),patch.object(retirement,'MEDIA_ABORT_JOURNAL',hashlib.sha256(raw).hexdigest()):
                self.assertTrue(retirement.media_abort_journal(root)['response_absent'])
                (root/'responses'/retirement.MEDIA_ABORT_FINISH).write_text('{}')
                with self.assertRaises(Blocked):retirement.media_abort_journal(root)

    def test_pending_admission_requires_exact_verified_receipt_membership(self):
        root=Path('/private/installed');path=root/'journal'/retirement.MEDIA_ABORT_FINISH
        row={'phase':'STARTED','request':{'operation':retirement.MEDIA_ABORT_OPERATION}}
        with patch.object(retirement,'document',return_value=row),patch.object(retirement,'verify_media_abort_terminal') as verify:
            retirement.pending_request_accepted(root,path,{'journals':{}});verify.assert_called_once()
            with self.assertRaises(Blocked):retirement.pending_request_accepted(root,path.with_name('a'*64+'.json'),{'journals':{}})
        with patch.object(retirement,'document',return_value=row),patch.object(retirement,'verify_media_abort_terminal',side_effect=Blocked('bad')):
            with self.assertRaises(Blocked):retirement.pending_request_accepted(root,path,{'journals':{}})

    def test_installer_has_exact_predecessor_and_unique_history_path(self):
        with patch.object(installer, '_upgrade_idle_transition') as call:
            installer.upgrade_idle_media_v1('candidate')
        self.assertEqual(call.call_args.args[:4], ('candidate',
            '1295ba61d4603a18790c33cfb9b12ae5e3d337c799d9853bd0fca3a80cc00c20',
            'idle-media-v1', 'media-1295'))

    def test_exact_completed_original_restore_is_admitted_without_relabeling_pass(self):
        state = {'operation': '0e2be62a3326', 'status': 'ABORTED', 'operator_media_abort': {}}
        base = Path('/private/rollout-0e2be62a3326')
        # The terminal dispatcher must use the dedicated validator, never PASS.
        with patch.object(retirement, 'verify_media_abort_terminal', return_value={'deployment_pass': False}, create=True) as verify:
            self.assertTrue(retirement.idle_state(base, state))
            verify.assert_called_once_with(base, state)


if __name__ == '__main__': unittest.main()
