"""Full retained-data accounting for the distinct restored8 writer contract."""
import copy
import unittest
import story_fixture as fixture
import staging_cohort_preservation_runtime as runtime
from controller import Blocked

ID='10000000-0000-4000-8000-000000000001'
NEW='20000000-0000-4000-8000-000000000001'

class PreservedWriterTests(unittest.TestCase):
    def test_exact_clone_command_rejects_foreign_network_image_mount_and_exec(self):
        pg='a'*64;image='sha256:'+'b'*64;name='voice-media-restore-'+'c'*32
        dsn='postgres://actual_runtime:'+'e'*48+'@127.0.0.1:5432/story_db?sslmode=disable'
        targets={pg:{'kind':'postgres','status':'ACTIVE'}}
        tools={'images':{'story0':'immutable-image'},'ids':{'story0':image}}
        for kind in ('story','story-probe'):
            args=runtime.story_create(name,pg,image,kind,dsn)
            runtime._story_create_check(args,{'kind':kind},targets,tools)
            for mutation in ('foreign-network','host-network','image','mount','privileged','entrypoint'):
                changed=list(args)
                if mutation=='foreign-network':changed[4]='container:'+'f'*64
                elif mutation=='host-network':changed[4]='host'
                elif mutation=='image':changed[-1]='sha256:'+'f'*64
                elif mutation=='mount':changed[7:7]=['--mount','type=bind,src=/,dst=/host']
                elif mutation=='privileged':changed[7:7]=['--privileged']
                else:changed[changed.index('--entrypoint')+1]='/bin/sh'
                with self.subTest(kind=kind,mutation=mutation),self.assertRaises(Blocked):
                    runtime._story_create_check(changed,{'kind':kind},targets,tools)

    def test_retained_owner_acl_rls_policy_sequence_and_function_are_immutable(self):
        before={'database':{'owner':'source'},'schema':{'acl':'source'},'defaults':[],
            'policies':[],'sequences':[],'functions':[],'relations':{'stories':{'owner':'source','rls':False}}}
        after=copy.deepcopy(before);after['relations']['new_table']={'owner':'migration'}
        fixture.retained_authority(before,after)
        reference=copy.deepcopy(after);reference['functions']=[{'name':'expected_SQL5_trigger','owner':'migration'}]
        new=copy.deepcopy(after);new['functions']=copy.deepcopy(reference['functions'])
        fixture.retained_authority(before,new,reference)
        new['functions'][0]['owner']='foreign'
        with self.assertRaises(Blocked):fixture.retained_authority(before,new,reference)
        for field in before:
            changed=copy.deepcopy(after)
            if field=='relations':changed[field]['stories']['rls']=True
            elif isinstance(changed[field],list):changed[field].append({'foreign':'authority'})
            else:changed[field]['owner']='foreign'
            with self.subTest(field=field),self.assertRaises(Blocked):fixture.retained_authority(before,changed)

    def before(self):
        return {'stories':[{'id':ID,'text_content':'retained'}],
                'story_lfp_consent_actors':[{'sidecar':'original-human-evidence'}],
                'story_media_reference_intents':[{'story_id':ID,'media_file_id':NEW}],
                'story_reference_inventory_gate':[{'singleton':True,'owner_incarnation_id':ID,'revision':0,'capture_started':False}],
                'story_reference_inventory_pages':[{'page':'original'}],
                'schema_migrations':[{'version':8,'dirty':False}]}

    def positive(self):
        return {'status':'PASS','contract':'restored-data8-v1','create_grpc_code':'OK',
          'response_identity_type_verified':True,'sql_fields_verified':True,
          'baseline_story_count':1,'story_count':2,'new_story_id':NEW,
          'default_ttl_24h_verified':True,'archive_plus30d_verified':True,
          'missing_profile_code':'Unauthenticated','negative_count_unchanged':True,
          'unrelated_row_count':0,'row_retained':True}

    def test_only_returned_story_can_be_added_and_every_retained_table_is_exact(self):
        before=self.before();after=copy.deepcopy(before)
        after['stories'].append({'id':NEW,'text_content':'new'})
        fixture.validate_preserved_writer(self.positive(),0,{'status':'FAIL','phase':'wrong_expected_row_count'},17,before,after,0)
        for table in before:
            changed=copy.deepcopy(after);changed[table].append({'unexpected':'mutation'})
            with self.subTest(table=table),self.assertRaises(Blocked):
                fixture.validate_preserved_writer(self.positive(),0,{'status':'FAIL','phase':'wrong_expected_row_count'},17,before,changed,0)

    def test_missing_original_duplicate_id_or_extra_probe_field_rejects(self):
        before=self.before();after=copy.deepcopy(before);after['stories'].append({'id':NEW})
        for mutation in ('missing','duplicate','field','old-id','count','contract'):
            result=self.positive();changed=copy.deepcopy(after)
            if mutation=='missing':changed['stories']=changed['stories'][1:]
            elif mutation=='duplicate':changed['stories'].append({'id':NEW})
            elif mutation=='field':result['trusted_restore']=True
            elif mutation=='old-id':result['new_story_id']=ID
            elif mutation=='count':result['baseline_story_count']=0
            else:result['contract']='current4-v1'
            with self.subTest(mutation=mutation),self.assertRaises(Blocked):
                fixture.validate_preserved_writer(result,0,{'status':'FAIL','phase':'wrong_expected_row_count'},17,before,changed,0)

    def test_migration_retains_all_baseline_rows_and_only_creates_closed_gate(self):
        before={'stories':[{'id':ID}],'schema_migrations':[{'version':4,'dirty':False}]}
        after=copy.deepcopy(before);after['schema_migrations']=[{'version':8,'dirty':False}]
        after['story_reference_inventory_gate']=[{'singleton':True,'owner_incarnation_id':NEW,'revision':0,'capture_started':False}]
        after['story_lfp_admission_head']=[]
        self.assertEqual(fixture.validate_preserved_migration(before,after),NEW)
        for mutation in ('row','gate','activation','extra','dirty'):
            changed=copy.deepcopy(after)
            if mutation=='row':changed['stories'][0]['id']=NEW
            elif mutation=='gate':changed['story_reference_inventory_gate'][0]['revision']=1
            elif mutation=='activation':changed['story_reference_inventory_gate'][0]['capture_started']=True
            elif mutation=='extra':changed['story_lfp_admission_head']=[{'state':'open'}]
            else:changed['schema_migrations'][0]['dirty']=True
            with self.subTest(mutation=mutation),self.assertRaises(Blocked):
                fixture.validate_preserved_migration(before,changed)

if __name__=='__main__':unittest.main()
