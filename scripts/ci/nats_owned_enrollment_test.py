"""Control-flow tests only: crypto and scratch broker are deliberately mocked."""
import base64
import copy
import sys
import unittest
from pathlib import Path
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[1] / 'staging'
sys.path.insert(0, str(ROOT / 'nats-rollout-preservation'))
sys.path.insert(0, str(ROOT / 'nats-known-baseline'))
import nats_owned_enrollment as owned
from bootstrap_selection_test import owned_fixture
from controller import Blocked
from docker_runtime import NATS_IMAGE
from nats_contract_plan import DECLARATIONS, digest
from resource_claim import production_installation

ACL = (Path(__file__).resolve().parents[2] / 'deploy/nats/acl-intent.yaml').read_bytes()


class Kube:
    def __init__(self, fixture, rows):
        self.fixture, self.rows, self.events = fixture, rows, []
    def get(self, kind, name):
        self.events.append(('get', kind, name))
        return self.fixture.get(kind, name)
    def run(self, args, body=None):
        self.events.append(tuple(args))
        if args[0] == 'get':
            name = args[2].rsplit('=', 1)[1]
            return {'items': [copy.deepcopy(self.rows[name])] if name in self.rows else []}
        if args[0] == 'create':
            row = copy.deepcopy(body)
            row['metadata'].update(uid='created-uid', resourceVersion='1')
            self.rows[row['metadata']['name']] = row
            return row
        raise AssertionError(args)


class Tests(unittest.TestCase):
    def fixture(self):
        kube, receipt, rows, installation = owned_fixture()
        operator = rows['voice-nats-operator-' + installation.generation]
        operator['data'].update({'account.jwt':base64.b64encode(b'account-jwt').decode(),
                                 'operator.jwt':base64.b64encode(b'operator-jwt').decode()})
        binding = copy.deepcopy(receipt['binding'])
        rows.pop(receipt['secret']['name'])
        normalized = [{'stream':stream,'durable':config['durable_name'],'config':copy.deepcopy(config)}
                      for stream,config in DECLARATIONS]
        auth = {'schema':'voice-bootstrap-auth-v4','verified':True,'server_image':NATS_IMAGE,
                'account_sha256':installation.account_sha256,'reply_prefix':'_INBOX.voice.bootstrap.reply',
                'normalized_consumer':normalized[0]['config'],'normalized_consumer_sha256':digest(normalized[0]['config']),
                'normalized_consumers':normalized,'normalized_consumers_sha256':digest(normalized)}
        return Kube(kube, rows), binding, operator, installation, auth

    def test_real_enrollment_and_selection_under_owned_domain(self):
        kube,binding,operator,installation,auth=self.fixture()
        with patch.object(owned.actor_verification,'verify') as verify, patch.object(owned.bootstrap_auth,'prove',return_value=auth) as prove:
            row=owned.enroll_owned(kube,binding,b'new-creds',operator,installation,Path('/not-executed'),ACL,lambda:None)
        self.assertEqual(row['schema'],'voice-nats-bootstrap-enrollment-scoped-v1')
        self.assertEqual(row['installation_sha256'],installation.sha256)
        self.assertEqual(row['auth_proof'],{k:auth[k] for k in owned._LEGACY_AUTH_KEYS})
        self.assertEqual(verify.call_args.args[5:7],(installation.bootstrap_actor_sha256,installation.account_sha256))
        self.assertEqual(prove.call_args.kwargs['installation'],installation)
        self.assertEqual(sum(event[0]=='create' for event in kube.events),1)

    def test_foreign_authority_and_failed_auth_refuse_without_receipt(self):
        kube,binding,operator,installation,auth=self.fixture()
        with self.assertRaises(Blocked):
            owned.enroll_owned(kube,binding,b'new-creds',operator,production_installation(),None,ACL,lambda:None)
        self.assertEqual(kube.events,[])
        with self.assertRaisesRegex(Blocked,'owned_enrollment_acl_source_changed'):
            owned.enroll_owned(kube,binding,b'new-creds',operator,installation,None,ACL+b'\n# altered',lambda:None)
        self.assertEqual(kube.events,[])
        foreign=copy.deepcopy(operator);foreign['metadata']['uid']='foreign'
        with self.assertRaisesRegex(Blocked,'owned_enrollment_operator_changed'):
            owned.enroll_owned(kube,binding,b'new-creds',foreign,installation,None,ACL,lambda:None)
        self.assertEqual(kube.events,[])
        with patch.object(owned.actor_verification,'verify',side_effect=Blocked('bad-chain')),patch.object(owned.bootstrap_auth,'prove') as prove:
            with self.assertRaisesRegex(Blocked,'bad-chain'):
                owned.enroll_owned(kube,binding,b'new-creds',operator,installation,None,ACL,lambda:None)
        prove.assert_not_called()

    def test_successor_retains_real_created_predecessor_and_exact_domain(self):
        kube,binding,operator,installation,auth=self.fixture()
        with patch.object(owned.actor_verification,'verify'),patch.object(owned.bootstrap_auth,'prove',return_value=auth):
            prior=owned.enroll_owned(kube,binding,b'first-creds',operator,installation,None,ACL,lambda:None)
            successor_binding=dict(binding,previous_enrollment_sha256=digest(prior))
            successor=owned.enroll_owned(kube,successor_binding,b'second-creds',operator,installation,None,ACL,lambda:None,previous=prior)
        self.assertEqual(successor['schema'],'voice-nats-bootstrap-enrollment-scoped-v4')
        self.assertEqual(successor['previous_enrollment'],prior)
        self.assertEqual(successor['previous_enrollment_sha256'],digest(prior))
        self.assertEqual(successor['auth_proof'],auth)
        changed=copy.deepcopy(prior);changed['installation_sha256']='0'*64
        kube.events.clear()
        with self.assertRaisesRegex(Blocked,'owned_enrollment_predecessor_invalid'):
            owned.enroll_owned(kube,dict(binding,previous_enrollment_sha256=digest(changed)),b'third-creds',operator,installation,None,ACL,lambda:None,previous=changed)
        self.assertEqual(kube.events,[])


if __name__ == '__main__':
    unittest.main()
