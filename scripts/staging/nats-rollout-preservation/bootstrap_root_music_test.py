"""Exercise the explicit activation consumer, preserving its prior receipts."""
import base64,copy,hashlib,tempfile,unittest
from pathlib import Path
from unittest.mock import patch
import bootstrap_root as root
from bootstrap_selection_test import owned_fixture
from nats_contract_plan import DECLARATIONS,V3_DECLARATIONS,digest
from controller import Blocked

class MusicEnrollment(unittest.TestCase):
    def test_v4_root_activation_keeps_old_receipt_and_is_create_only(self):
        for legacy in (False,True):
            with self.subTest(legacy_single_consumer=legacy):self.activation(legacy)

    def activation(self,legacy):
        original,previous,rows,installation=owned_fixture()
        old=rows[previous['binding']['old_secret']['name']]
        old['metadata'].update(uid='cd1f24fe-cbf1-40f2-9043-f8affd24127d',resourceVersion='2864382')
        previous['binding']['old_secret'].update(uid=old['metadata']['uid'],resourceVersion=old['metadata']['resourceVersion'])
        previous['binding']['marker_uid']='cbafb90c-6974-4642-86d6-09b2eef1c843'
        normalized=[{'stream':s,'durable':d['durable_name'],'config':dict(d,max_ack_pending=1000)} for s,d in V3_DECLARATIONS]
        previous['auth_proof'].update(normalized_consumer=normalized[0]['config'],normalized_consumer_sha256=digest(normalized[0]['config']),normalized_consumers=normalized,normalized_consumers_sha256=digest(normalized))
        full=copy.deepcopy(previous['auth_proof'])
        if legacy:
            previous['auth_proof'].pop('normalized_consumers')
            previous['auth_proof'].pop('normalized_consumers_sha256')
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
        stream,config=DECLARATIONS[-1]
        full['normalized_consumers'].append({'stream':stream,'durable':config['durable_name'],'config':dict(config,max_ack_pending=1000)})
        full.update(schema='voice-bootstrap-auth-v4',normalized_consumers_sha256=digest(full['normalized_consumers']))
        def prove(credentials,operator,unchanged):
            self.assertEqual(credentials,b'renewed-music-private-creds');unchanged();return copy.deepcopy(full)
        with tempfile.TemporaryDirectory() as td:
            code=Path(td);binary=code/'nats-rollout-preservation/bootstrap-renewer';binary.parent.mkdir();binary.write_bytes(b'captured')
            binding={'nats-rollout-preservation/bootstrap-renewer':hashlib.sha256(b'captured').hexdigest()}
            with patch.object(root,'protected_seed',return_value=b'private-seed') as seed,patch.object(root,'invoke',return_value=b'renewed-music-private-creds') as invoke:
                with self.assertRaises(Blocked):root.enroll_current(Kube(),code,binding,lambda:None,read,save,prove)
                self.assertEqual((writes,created),([],[]))
                # A forged legacy shape cannot turn the owned account into the
                # installed authority either, and must fail before any effects.
                scoped_receipt=receipts['bootstrap-enrollment.json']
                forged=copy.deepcopy(scoped_receipt)
                forged['schema']='voice-nats-bootstrap-enrollment-v1';del forged['installation_sha256']
                receipts['bootstrap-enrollment.json']=forged
                with self.assertRaises(Blocked):root.enroll_current(Kube(),code,binding,lambda:None,read,save,prove)
                receipts['bootstrap-enrollment.json']=scoped_receipt
                self.assertEqual((writes,created),([],[]));seed.assert_not_called();invoke.assert_not_called()
                result=root._enroll_current(Kube(),code,binding,lambda:None,read,save,prove,installation=installation)
                again=root._enroll_current(Kube(),code,binding,lambda:None,read,save,prove,installation=installation)
                self.assertEqual(result,again)
                self.assertEqual(result['schema'],'voice-nats-bootstrap-enrollment-scoped-v4')
                self.assertEqual(result['installation_sha256'],installation.sha256)
                self.assertEqual(result['previous_enrollment'],saved)
                self.assertEqual(receipts['bootstrap-enrollment.json'],saved)
                self.assertEqual(receipts['bootstrap-renewal-draft.json'],{'immutable':'old-draft'})
                self.assertEqual(writes,['bootstrap-renewal-draft-v4.json','bootstrap-enrollment-v4.json'])
                self.assertEqual(len(created),1)
                # A changed old normalized default cannot be adopted by merely
                # refreshing the current proof or retaining a valid new Secret.
                changed_row=0 if legacy else 1
                full['normalized_consumers'][changed_row]['config']['max_ack_pending']=99
                full['normalized_consumers_sha256']=digest(full['normalized_consumers'])
                if legacy:
                    full['normalized_consumer']=copy.deepcopy(full['normalized_consumers'][0]['config'])
                    full['normalized_consumer_sha256']=digest(full['normalized_consumer'])
                with self.assertRaises(Blocked):root._enroll_current(Kube(),code,binding,lambda:None,read,save,prove,installation=installation)
                self.assertEqual(receipts['bootstrap-enrollment.json'],saved)

if __name__=='__main__':unittest.main()
