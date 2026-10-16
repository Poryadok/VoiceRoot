"""Exercise the explicit activation consumer, preserving its prior receipts."""
import base64,copy,hashlib,tempfile,unittest
from pathlib import Path
from unittest.mock import patch
import bootstrap_root as root
import bootstrap_selection as selection
from bootstrap_selection_test import fixture
from nats_contract_plan import DECLARATIONS,V3_DECLARATIONS,digest
from controller import Blocked

class MusicEnrollment(unittest.TestCase):
    def test_v4_root_activation_keeps_old_receipt_and_is_create_only(self):
        original,previous,rows,account=fixture()
        old=rows[previous['binding']['old_secret']['name']]
        old['metadata'].update(uid='cd1f24fe-cbf1-40f2-9043-f8affd24127d',resourceVersion='2864382')
        previous['binding']['old_secret'].update(uid=old['metadata']['uid'],resourceVersion=old['metadata']['resourceVersion'])
        previous['binding']['marker_uid']='cbafb90c-6974-4642-86d6-09b2eef1c843'
        normalized=[{'stream':s,'durable':d['durable_name'],'config':dict(d,max_ack_pending=1000)} for s,d in V3_DECLARATIONS]
        previous['auth_proof'].update(normalized_consumer=normalized[0]['config'],normalized_consumer_sha256=digest(normalized[0]['config']),normalized_consumers=normalized,normalized_consumers_sha256=digest(normalized))
        saved=copy.deepcopy(previous)
        receipts={'bootstrap-enrollment.json':copy.deepcopy(previous),'bootstrap-renewal-draft.json':{'immutable':'old-draft'}}
        created=[];writes=[]
        class Kube:
            def get(self,kind,name):
                result=original.get(kind,name)
                if kind=='configmap':result['metadata']['uid']=previous['binding']['marker_uid']
                return result
            def run(self,args,body=None):
                if args[0]=='get':
                    name=args[2].split('=',2)[-1]
                    return {'items':[copy.deepcopy(rows[name])] if name in rows else []}
                if args[0]!='create':raise AssertionError('unexpected mutation')
                name=body['metadata']['name']
                if name in rows:raise AssertionError('overwrote Secret')
                created.append(name);rows[name]=copy.deepcopy(body)
                rows[name]['metadata'].update(uid='new-uid',resourceVersion='2')
                return copy.deepcopy(rows[name])
        def read(path):
            if path.name not in receipts:raise FileNotFoundError(path)
            return copy.deepcopy(receipts[path.name])
        def save(path,value):
            if path.name in receipts:raise AssertionError('overwrote prior receipt')
            writes.append(path.name);receipts[path.name]=copy.deepcopy(value)
        full=copy.deepcopy(previous['auth_proof'])
        stream,config=DECLARATIONS[-1]
        full['normalized_consumers'].append({'stream':stream,'durable':config['durable_name'],'config':dict(config,max_ack_pending=1000)})
        full.update(schema='voice-bootstrap-auth-v4',normalized_consumers_sha256=digest(full['normalized_consumers']))
        def prove(credentials,operator,unchanged):
            self.assertEqual(credentials,b'renewed-music-private-creds');unchanged();return copy.deepcopy(full)
        with tempfile.TemporaryDirectory() as td:
            code=Path(td);binary=code/'nats-rollout-preservation/bootstrap-renewer';binary.parent.mkdir();binary.write_bytes(b'captured')
            binding={'nats-rollout-preservation/bootstrap-renewer':hashlib.sha256(b'captured').hexdigest()}
            with patch.object(root,'ACCOUNT_SHA',account),patch.object(selection,'ACCOUNT_SHA',account),patch.object(root,'protected_seed',return_value=b'private-seed'),patch.object(root,'invoke',return_value=b'renewed-music-private-creds'):
                result=root.enroll_current(Kube(),code,binding,lambda:None,read,save,prove)
                again=root.enroll_current(Kube(),code,binding,lambda:None,read,save,prove)
                self.assertEqual(result,again)
                self.assertEqual(result['schema'],'voice-nats-bootstrap-enrollment-v4')
                self.assertEqual(result['previous_enrollment'],saved)
                self.assertEqual(receipts['bootstrap-enrollment.json'],saved)
                self.assertEqual(receipts['bootstrap-renewal-draft.json'],{'immutable':'old-draft'})
                self.assertEqual(writes,['bootstrap-renewal-draft-v4.json','bootstrap-enrollment-v4.json'])
                self.assertEqual(len(created),1)
                # A changed old normalized default cannot be adopted by merely
                # refreshing the current proof or retaining a valid new Secret.
                full['normalized_consumers'][1]['config']['max_ack_pending']=99
                full['normalized_consumers_sha256']=digest(full['normalized_consumers'])
                with self.assertRaises(Blocked):root.enroll_current(Kube(),code,binding,lambda:None,read,save,prove)

if __name__=='__main__':unittest.main()
