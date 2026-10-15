import copy
import hashlib
import json
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import operator_retirement as r
from controller import Blocked


class GatewayRecoveryTests(unittest.TestCase):
    def test_exact_restart_boundary_rejects_changed_history_and_failed_refence(self):
        with tempfile.TemporaryDirectory() as td:
            base=Path(td);state,before=self.fixture(base)
            prefix=r.MEDIA_RECONCILED_PREFIX
            original={'events':[{'kind':'historical'}],'context':copy.deepcopy(state['context'])}
            final={'kind':'target_baseline_blocked','stage':'target-restart','class':'Blocked',
                'failure':'MEDIA_TARGET_BLOCKED stage=target-restart error=restart_readiness_timeout',
                'refence':{'verified':True,'hub_zero_pods':True,'recovery_failures':[]}}
            progress={'phase':'BLOCKED','context':state['context'],'events':[final]}
            state.update(events=original['events']+[final],fence_status='VERIFIED')
            prepared=json.loads((base/(prefix+'-prepared.json')).read_text())
            prepared.update(failed_apply={},reconciliation={'bound':True},outside_owned={'admin':'fixed'},
                source_sha='ca7304dcaa613bba4d1e7d08087de2839e0a3b66',
                binding_sha256='8fae034af1c05abed5373f17beba4633e6922ad389e8d7ccb68353a2d3ebaa4e')
            values={r.MEDIA_GATEWAY_PREFIX+'-blocked.json':state,prefix+'-progress.json':progress,
                prefix+'-prepared.json':prepared,prefix+'-source-authority.json':
                    {'source_sha':prepared['source_sha'],'run_id':38698269484}}
            values[prefix+'-normalized.json']=json.loads((base/(prefix+'-normalized.json')).read_text())
            pins={}
            def write(name,value):
                raw=json.dumps(value).encode();path=base/name;path.write_bytes(raw);path.chmod(0o600)
                pins[name]={'bytes':len(raw),'sha256':hashlib.sha256(raw).hexdigest()}
            for name,value in values.items():write(name,value)
            with patch.object(r,'MEDIA_FAILED_RESTART',pins),patch.object(r,'MEDIA_FAILED_APPLY',{}),\
                 patch.object(r,'media_failed_apply',return_value=original),\
                 patch.object(r,'media_failed_reconciliation',return_value={'bound':True}),\
                 patch.object(r,'media_failed_admin',return_value={'admin':'fixed'}):
                self.assertEqual(r.media_restart_failed(base),state)
                for name in pins:
                    path=base/name;raw=path.read_bytes();path.write_bytes(b'{}')
                    with self.subTest(pin=name),self.assertRaises(Blocked):r.media_restart_failed(base)
                    path.write_bytes(raw)
                for mutation in ('refence','hub','user','source'):
                    changed=copy.deepcopy(values)
                    if mutation=='refence':changed[prefix+'-progress.json']['events'][-1]['refence']['verified']=False
                    elif mutation=='source':changed[prefix+'-prepared.json']['source_sha']='f'*40
                    else:
                        name='voice-nats-pvc-candidate' if mutation=='hub' else 'voice-user'
                        changed[r.MEDIA_GATEWAY_PREFIX+'-blocked.json']['context']['snapshots'][name]['spec']['template']['changed']=True
                        changed[prefix+'-progress.json']['context']=copy.deepcopy(changed[r.MEDIA_GATEWAY_PREFIX+'-blocked.json']['context'])
                    for name,value in changed.items():write(name,value)
                    with self.subTest(mutation=mutation),self.assertRaises(Blocked):r.media_restart_failed(base)
                    for name,value in values.items():write(name,value)

    def fixture(self, base):
        services = ('analytics','auth','bot','chat','file','gateway','matchmaking',
                    'messaging','moderation','notification','realtime','role','search',
                    'social','space','story','subscription','user','voice')
        keys = ['Service/voice-'+s for s in services]
        keys += ['NetworkPolicy/voice-'+s for s in ('social-privacy-consumers',
            'social-principal-jwks','file-user-principal-listener','file-principal-jwks',
            'search-user-projection','search-projection-jwks')]
        keys += ['Deployment/voice-'+s for s in (*services,'web','developer-portal')]
        rows = []
        for key in keys:
            kind, name = key.split('/')
            row = {'kind':kind,'metadata':{'name':name,'uid':key},'spec':{'value':'target'}}
            if kind == 'Deployment':
                row['spec'] = {'replicas':0,'template':{'metadata':{'annotations':{}},
                    'spec':{'containers':[{'name':name,'image':'target'}]}}}
            rows.append(row)
        before = {row['kind']+'/'+row['metadata']['name']:copy.deepcopy(row) for row in rows}
        snapshots = {row['metadata']['name']:copy.deepcopy(row) for row in rows if row['kind']=='Deployment'}
        snapshots['voice-nats-pvc-candidate'] = {'kind':'Deployment','metadata':{'name':'voice-nats-pvc-candidate','uid':'hub'},
            'spec':{'replicas':0,'template':{'spec':{'containers':[{'name':'nats','image':'fixed-hub'}]}}}}
        state = {'context':{'snapshots':snapshots,'original_snapshots':copy.deepcopy(snapshots)}}
        prepared = {'before':before,'normalized_sha256':r.canonical(rows)}
        for suffix,value in (('normalized',rows),('prepared',prepared)):
            path=base/('operator-media-target-reconciled-'+suffix+'.json')
            path.write_text(json.dumps(value));path.chmod(0o600)
        return state,before

    def test_all_target_reconciliation_rejects_mixtures_uid_and_inventory_drift(self):
        with tempfile.TemporaryDirectory() as td:
            base=Path(td);state,before=self.fixture(base)
            with patch.object(r,'media_restart_failed',return_value=state):
                result=r.media_restart_reconciliation(base,before)
                self.assertEqual(len(result),46)
                for key in before:
                    changed=copy.deepcopy(before);changed[key]['metadata']['uid']='replacement'
                    with self.subTest(uid=key),self.assertRaises(Blocked):r.media_restart_reconciliation(base,changed)
                for key in ('Service/voice-auth','Deployment/voice-gateway','Deployment/voice-user'):
                    changed=copy.deepcopy(before);changed[key]['spec']['drift']='old-before'
                    with self.subTest(mixed=key),self.assertRaises(Blocked):r.media_restart_reconciliation(base,changed)
                for mutate in (lambda x:x.pop('Service/voice-auth'),lambda x:x.update(extra={})):
                    changed=copy.deepcopy(before);mutate(changed)
                    with self.assertRaises(Blocked):r.media_restart_reconciliation(base,changed)

    def test_workloads_bind_hub_separately_and_refuse_user_override(self):
        with tempfile.TemporaryDirectory() as td:
            base=Path(td);state,before=self.fixture(base)
            with patch.object(r,'media_restart_failed',return_value=state):
                r.media_restart_workloads(base,state['context']['snapshots'])
                for name in ('voice-nats-pvc-candidate','voice-user'):
                    changed=copy.deepcopy(state['context']['snapshots'])
                    changed[name]['spec']['template'].setdefault('metadata',{}).setdefault('annotations',{})['voice.io/nats-user-space-bootstrap']='operation'
                    with self.subTest(name=name),self.assertRaises(Blocked):r.media_restart_workloads(base,changed)


if __name__=='__main__':unittest.main()
