"""Cohort inputs are authenticated; fixtures are synthetic and never DEV evidence."""
import copy
import hashlib
import json
import pathlib
import tempfile
import unittest

import story_media_backup_cohort as cohort
import story_media_backup_cohort_finalize as finalize
from auth_flyway_upgrade_qualification_cohort import validate_cohort
from auth_user_adoption_runtime import QUALIFIED_TOOLS


def digest(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(',', ':'), ensure_ascii=True).encode()).hexdigest()


class CohortInputs(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(); self.addCleanup(self.temp.cleanup)
        self.root = pathlib.Path(self.temp.name); self.key = bytes(range(32)); self.dbs = ['auth_db']
        eng = {'namespace_uid':'33333333-3333-3333-3333-333333333333',
               'pod_uid':'44444444-4444-4444-4444-444444444444', 'pvc_uid':'55555555-5555-5555-5555-555555555555',
               'pv_uid':'66666666-6666-6666-6666-666666666666', 'container_id':'containerd://' + 'd'*64, 'restart_count':7}
        history = [{'version':'26','script':'V26__authority_source_revision.sql','checksum':123,'success':True}]
        catalog = {'database_owner':'voice','schema_owner':'voice','schema_acl':None,'objects':[], 'defaults':[], 'history':history}
        plans = {'auth_db':{'engine':'flyway','from':26,'to':26,'files':{'V26__authority_source_revision.sql':'e'*64}}}
        self.metadata = {'catalog':{'auth_db':catalog},'roles':{'roles':[],'memberships':[]},'migrations':plans}
        self.binding = {'operation_id':'media-c1-'+'a'*32,'snapshot_id':'capture-'+'b'*32,
            'source_sha':'c'*40,'project':'voice','resource':'DEV','actor':'scheduled_devops',
            'claim':'11111111-1111-1111-1111-111111111111','window':'22222222-2222-2222-2222-222222222222',
            'generation':3,'revision':4,'databases':self.dbs,
            'sources':{'auth_db':{'engine':eng,'server_start':'fixture','database_oid':1},
                       'roles':{'engine':eng,'server_start':'fixture'}},
            'restore_inputs':{'catalog_sha256':digest(self.metadata['catalog']), 'roles_catalog_sha256':digest(self.metadata['roles']),
                'migration_catalog_sha256':digest(plans),'postgres_image':QUALIFIED_TOOLS['postgres'],
                'migrate_image':QUALIFIED_TOOLS['migrate'],'migrate_binary_sha256':'6'*64,'postgres_major':16}}
        self.manifest = cohort.export(self.root,self.key,self.binding,self.dbs,lambda member,out:out.write(member.encode()))
        receipts = {}
        for i, member in enumerate(('auth_db','roles')):
            source=self.binding['sources'][member]
            receipts[member] = {'operation_id':self.binding['operation_id'],'snapshot_id':self.binding['snapshot_id'],
                'member':member, 'pod_uid':eng['pod_uid'],'pvc_uid':eng['pvc_uid'],'pv_uid':eng['pv_uid'],
                'container_id':eng['container_id'],'restart_count':7,'server_start':'fixture',
                'database_oid':source.get('database_oid'),'attempt_nonce':f'{i+1:032x}','exit':0,'scope':'dump-terminal-only'}
        self.binding,self.manifest=finalize.finalize(self.root,self.key,self.binding,self.dbs,self.manifest,receipts)
        self.custody = {'schema':'voice-cohort-custody-v1','destination':'github:Poryadok/VoiceRoot',
             'artifact_id':23,'operation_id':self.binding['operation_id'],'snapshot_id':self.binding['snapshot_id'],
             'challenge':'f'*32,'run_id':42,'head_sha':self.binding['source_sha'],'bundle_bytes':100,'bundle_sha256':'f'*64}
        self.restore = {'operation_id':self.binding['operation_id'],'snapshot_id':self.binding['snapshot_id'],
            'source_sha':self.binding['source_sha'],'manifest_sha256':digest(self.manifest),
            'databases':{'auth_db':{'mode':'restore-only','from':26,'to':26}},
            'scope':'selected-cohort-roles-catalog-and-ledger-only','cleanup':'verified'}
        self.admission={'source_sha':self.binding['source_sha'],'target':32,'flyway_enabled':False,
            'selector_mode':'absent','persistence':'jdbc','plan_sha256':'a'*64,
            'custody_sha256':digest(self.custody),'restore_sha256':digest(self.restore)}

    def check(self, **overrides):
        args = dict(root=self.root,key=self.key,binding=self.binding,databases=self.dbs,manifest=self.manifest,
                    metadata=self.metadata,custody=self.custody,restore=self.restore,admission=self.admission)
        args.update(overrides)
        return validate_cohort(**args)

    def test_valid_and_wrong_operation_source_hmac_or_restore_rejected(self):
        self.check()
        for kind in ('source','operation','hmac','restore','custody','target','metadata'):
            args = copy.deepcopy(dict(binding=self.binding,manifest=self.manifest,metadata=self.metadata,
                                      custody=self.custody,restore=self.restore,admission=self.admission))
            if kind == 'source': args['admission']['source_sha']='f'*40
            if kind == 'operation': args['custody']['operation_id']='media-c1-'+'f'*32
            if kind == 'hmac': args['manifest']['mac']='0'*64
            if kind == 'restore': args['restore']['databases']['auth_db']['mode']='migrate'
            if kind == 'custody': args['custody']['bundle_sha256']='0'*64
            if kind == 'target': args['admission']['target']=26
            if kind == 'metadata': args['metadata']['catalog']['auth_db']['history'][0]['checksum']=999
            with self.subTest(kind=kind), self.assertRaises(Exception): self.check(**args)


if __name__ == '__main__': unittest.main()
