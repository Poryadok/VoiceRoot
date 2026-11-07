"""Synthetic isolated Kube inputs; no installed credentials or runtime claim."""
import base64,copy,hashlib,unittest,uuid
import actor_root,bootstrap_selection,renderer_root
from unittest.mock import patch
from controller import Blocked
from nats_contract_plan import DECLARATION,digest
from docker_runtime import NATS_IMAGE
from resource_claim import InstallationAuthority,production_installation


def fixture():
    account='A'+'A'*55
    sha=hashlib.sha256(account.encode()).hexdigest()
    domain=InstallationAuthority('owned-isolated-v1',str(uuid.uuid4()),'r20260930a4',sha,'b'*64,'c'*64)
    def secret(name,values):
        return {'type':'Opaque','immutable':True,'metadata':{'name':name,'uid':name+'-uid','resourceVersion':'1','namespace':'voice-staging'},
            'data':{k:base64.b64encode(v.encode()).decode() for k,v in values.items()}}
    generation=domain.generation
    old=secret('voice-nats-bootstrap-credentials-'+generation,{'bootstrap.creds':'old'})
    new=secret('voice-nats-bootstrap-contract-'+'a'*24,{'bootstrap.creds':'new'})
    new['metadata']['labels']={'voice-nats-generation':generation,'voice-nats-existing-bootstrap-renewal':'true'}
    operator=secret('voice-nats-operator-'+generation,{'account.public':account})
    identity=lambda row:{k:row['metadata'][k] for k in ('name','uid','resourceVersion')}
    binding={'generation':generation,'marker_uid':'marker','marker_rv':'1','account_sha256':sha,'actor_sha256':domain.bootstrap_actor_sha256,
        'old_secret':identity(old)|{'credential_sha256':bootstrap_selection.credential_hash(old)},'operator_secret':identity(operator)}
    auth={'verified':True,'server_image':NATS_IMAGE,'account_sha256':sha,'reply_prefix':'_INBOX.voice.bootstrap.reply',
        'normalized_consumer':copy.deepcopy(DECLARATION),'normalized_consumer_sha256':digest(DECLARATION)}
    receipt={'schema':'voice-nats-bootstrap-enrollment-scoped-v1','installation_sha256':domain.sha256,'verified':True,
        'binding':binding,'secret':identity(new),'credential_sha256':bootstrap_selection.credential_hash(new),'auth_proof':auth}
    class Kube:
        def __init__(self):self.calls=[];self.rows={r['metadata']['name']:r for r in (old,new,operator)}
        def get(self,kind,name):
            self.calls.append((kind,name))
            if kind=='configmap':return {'metadata':{'uid':'marker'},'data':{'generation':generation}}
            return copy.deepcopy(self.rows[name])
    return domain,receipt,Kube()


class DomainTests(unittest.TestCase):
    def test_initial_enrollment_normalization_versions(self):
        from nats_contract_plan import V3_DECLARATIONS,DECLARATIONS,V6_DECLARATIONS
        for declarations,schema in ((V3_DECLARATIONS,None),(DECLARATIONS,'voice-bootstrap-auth-v4'),(V6_DECLARATIONS,'voice-bootstrap-auth-v5')):
            domain,receipt,kube=fixture()
            rows=[{'stream':stream,'durable':config['durable_name'],'config':copy.deepcopy(config)} for stream,config in declarations]
            receipt['auth_proof'].update(normalized_consumer=rows[0]['config'],normalized_consumer_sha256=digest(rows[0]['config']),
                normalized_consumers=rows,normalized_consumers_sha256=digest(rows))
            if schema:receipt['auth_proof']['schema']=schema
            with self.subTest(schema=schema):
                self.assertEqual(bootstrap_selection.select(kube,receipt,domain.generation,installation=domain),receipt['secret']['name'])

    def test_scoped_successor_keeps_prior_binding_and_rejects_mixed_versions(self):
        from nats_contract_plan import DECLARATIONS,V3_DECLARATIONS
        domain,previous,kube=fixture()
        old_rows=[{'stream':stream,'durable':config['durable_name'],'config':dict(config,max_ack_pending=1000)} for stream,config in V3_DECLARATIONS]
        previous['auth_proof'].update(normalized_consumer=old_rows[0]['config'],normalized_consumer_sha256=digest(old_rows[0]['config']),normalized_consumers=old_rows,normalized_consumers_sha256=digest(old_rows))
        current=copy.deepcopy(previous)
        selected=copy.deepcopy(kube.rows[current['secret']['name']]);selected['metadata'].update(name='voice-nats-bootstrap-contract-'+'b'*24,uid='new-uid')
        selected['data']['bootstrap.creds']=base64.b64encode(b'new-successor').decode()
        kube.rows[selected['metadata']['name']]=selected
        current.update(schema='voice-nats-bootstrap-enrollment-scoped-v4',previous_enrollment=previous,previous_enrollment_sha256=digest(previous),
            secret={k:selected['metadata'][k] for k in ('name','uid','resourceVersion')},credential_sha256=bootstrap_selection.credential_hash(selected))
        current['binding']['previous_enrollment_sha256']=digest(previous)
        stream,config=DECLARATIONS[-1]
        all_rows=old_rows+[{'stream':stream,'durable':config['durable_name'],'config':dict(config,max_ack_pending=1000)}]
        current['auth_proof'].update(schema='voice-bootstrap-auth-v4',normalized_consumers=all_rows,normalized_consumers_sha256=digest(all_rows))
        self.assertEqual(bootstrap_selection.select(kube,current,domain.generation,installation=domain),selected['metadata']['name'])
        for fault in ('schema','digest','config','legacy-link'):
            altered=copy.deepcopy(current)
            if fault=='schema':altered['auth_proof']['schema']='foreign'
            elif fault=='digest':altered['auth_proof']['normalized_consumers_sha256']='0'*64
            elif fault=='config':
                altered['auth_proof']['normalized_consumers'][1]['durable']='foreign'
                altered['auth_proof']['normalized_consumers_sha256']=digest(altered['auth_proof']['normalized_consumers'])
            else:
                altered['auth_proof']['normalized_consumers'][0]['config']['max_ack_pending']=42
                altered['auth_proof']['normalized_consumers_sha256']=digest(altered['auth_proof']['normalized_consumers'])
            kube.calls.clear()
            with self.subTest(fault=fault),self.assertRaises(Blocked):bootstrap_selection.select(kube,altered,domain.generation,installation=domain)
            self.assertEqual(kube.calls,[])
        altered=copy.deepcopy(current);altered['previous_enrollment']['schema']='voice-nats-bootstrap-enrollment-v1';altered['previous_enrollment'].pop('installation_sha256')
        altered['previous_enrollment_sha256']=digest(altered['previous_enrollment']);altered['binding']['previous_enrollment_sha256']=altered['previous_enrollment_sha256']
        kube.calls.clear()
        with self.assertRaises(Blocked):bootstrap_selection.select(kube,altered,domain.generation,installation=domain)
        self.assertEqual(kube.calls,[])

    def test_owned_selection_and_closed_domain_guards_before_kube(self):
        domain,receipt,kube=fixture()
        self.assertEqual(bootstrap_selection.select(kube,receipt,domain.generation,installation=domain),receipt['secret']['name'])
        other=InstallationAuthority('owned-isolated-v1',str(uuid.uuid4()),domain.generation,domain.account_sha256,domain.bootstrap_actor_sha256,domain.genesis_sha256)
        for case in ('foreign','production','extra','nested-extra','legacy','mixed','generation'):
            row=copy.deepcopy(receipt);expected=domain;generation=domain.generation
            if case=='foreign':expected=other
            if case=='production':expected=production_installation()
            if case=='extra':row['unexpected']=True
            if case=='nested-extra':row['binding']['unexpected']=True
            if case=='legacy':row['schema']='voice-nats-bootstrap-enrollment-v1';row.pop('installation_sha256')
            if case=='mixed':row['schema']='voice-nats-bootstrap-enrollment-scoped-v4';row['previous_enrollment']={'schema':'voice-nats-bootstrap-enrollment-v1'};row['previous_enrollment_sha256']=digest(row['previous_enrollment'])
            if case=='generation':generation='r20260930b'
            kube.calls.clear()
            with self.subTest(case=case),self.assertRaises(Blocked):bootstrap_selection.select(kube,row,generation,installation=expected)
            self.assertEqual(kube.calls,[])

    def test_actor_and_renderer_reject_wrong_generation_before_effects(self):
        domain,_,kube=fixture()
        class Stage:
            expected={'generation':'r20260930b'}
        with self.assertRaises(Blocked):actor_root.preflight(kube,None,None,None,Stage(),['social'],None,installation=domain)
        with self.assertRaises(Blocked):renderer_root.revalidate(kube,Stage(),{},installation=domain)
        self.assertEqual(kube.calls,[])

    def test_foreign_actor_account_refused_before_story_or_sql(self):
        domain,_,kube=fixture()
        other=InstallationAuthority('owned-isolated-v1',str(uuid.uuid4()),domain.generation,'d'*64,domain.bootstrap_actor_sha256,domain.genesis_sha256)
        class Stage:expected={'generation':domain.generation}
        with patch.object(actor_root,'story_schema',side_effect=AssertionError('unexpected SQL')),patch.object(actor_root,'story_content',side_effect=AssertionError('unexpected Docker')):
            with self.assertRaises(Blocked):actor_root.preflight(kube,None,None,None,Stage(),['story'],None,installation=other)
        self.assertEqual(kube.calls,[('secret','voice-nats-operator-'+domain.generation)])

    def test_operator_drift_refused_during_story_witness_callback(self):
        import story_witness
        domain,_,kube=fixture()
        snapshot={'metadata':{'name':'voice-story','namespace':'voice-staging','uid':'story'},'spec':{}}
        class Stage:
            expected={'generation':domain.generation}
            snapshots={'voice-story':snapshot}
            old_images={'voice-story/story':'image'}
        original_get=kube.get;operator_reads=[]
        def get(kind,name):
            if kind=='deployment':return copy.deepcopy(snapshot)
            row=original_get(kind,name)
            if kind=='secret' and name.startswith('voice-nats-operator-'):
                operator_reads.append(True)
                if len(operator_reads)>1:row['metadata']['resourceVersion']='drift'
            return row
        kube.get=get
        with patch.object(actor_root,'story_schema',return_value={'version':4}), \
             patch.object(actor_root,'story_content',side_effect=lambda image,automatic:automatic('a','b')), \
             patch.object(story_witness,'ensure_content',side_effect=lambda child,config,prove,unchanged:unchanged()):
            with self.assertRaises(Blocked):actor_root.preflight(kube,None,None,None,Stage(),['story'],None,installation=domain)
        self.assertEqual(len(operator_reads),2)

    def test_renderer_checks_actual_account_on_revalidation(self):
        domain,_,kube=fixture()
        class Stage:
            expected={'generation':domain.generation,'source_claim':'pvc','source_claim_uid':'pvc-uid','source_pv_uid':'pv-uid'}
            def verify_selected_storage_identity(self):pass
        stage=Stage()
        operator=kube.get('secret','voice-nats-operator-'+domain.generation)
        authority={'input_binding':{'operator':'bound'},'descriptor':{'storage_binding':renderer_root.storage(stage)}}
        with patch.object(renderer_root,'inputs',return_value=({'operator':operator},authority['input_binding'])):
            renderer_root.revalidate(kube,stage,authority,installation=domain)
            other=InstallationAuthority('owned-isolated-v1',str(uuid.uuid4()),domain.generation,'d'*64,domain.bootstrap_actor_sha256,domain.genesis_sha256)
            with self.assertRaisesRegex(Blocked,'renderer_input_account_changed'):
                renderer_root.revalidate(kube,stage,authority,installation=other)

if __name__=='__main__':unittest.main()
