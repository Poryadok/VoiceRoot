import base64
import copy
import hashlib
import unittest
import space_authority as authority

def metadata(number):return {'uid':f'{number:08x}-0000-4000-8000-000000000000','resourceVersion':'17'}

class Kube:
    def __init__(self):
        secret={'POSTGRES_PASSWORD':'fixture-password','SPACE_DATABASE_URL':'postgres://fixture:fixture-password@voice-postgres:5432/space_db?sslmode=disable'}
        self.rows={
            ('namespace','voice-staging'):{'metadata':metadata(1)},
            ('statefulset','voice-postgres'):{'metadata':metadata(2),'spec':{'replicas':1}},
            ('pod','voice-postgres-0'):{'metadata':dict(metadata(3),ownerReferences=[{'kind':'StatefulSet','name':'voice-postgres','uid':metadata(2)['uid'],'controller':True}]),
                'spec':{'volumes':[{'persistentVolumeClaim':{'claimName':'pgdata-voice-postgres-0'}}]},
                'status':{'phase':'Running','containerStatuses':[{'name':'postgres','ready':True,'imageID':'docker.io/library/postgres@sha256:'+'a'*64}]}},
            ('persistentvolumeclaim','pgdata-voice-postgres-0'):{'metadata':metadata(4),'status':{'phase':'Bound'},'spec':{'volumeName':'pvc-source'}},
            ('persistentvolume','pvc-source'):{'metadata':metadata(5),'spec':{'claimRef':{'uid':metadata(4)['uid'],'name':'pgdata-voice-postgres-0','namespace':'voice-staging'}}},
            ('secret','voice-app-secrets'):{'metadata':metadata(6),'data':{k:base64.b64encode(v.encode()).decode() for k,v in secret.items()}}}
    def get(self,kind,name):return copy.deepcopy(self.rows[kind,name])
    def secret_meta(self,name):return copy.deepcopy(self.rows['secret',name]['metadata'])
    def run(self,args):
        self.last_args=args
        return {'password_sha256':hashlib.sha256(b'fixture-password').hexdigest()}

class AuthorityTests(unittest.TestCase):
    def test_binds_physical_database_and_secret_without_private_values(self):
        kube=Kube();binding=authority.capture(kube)
        self.assertEqual(binding['pod'],metadata(3));self.assertEqual(binding['claim'],metadata(4))
        self.assertNotIn('fixture-password',repr(binding));self.assertNotIn('fixture-password',repr(kube.last_args))
        authority.revalidate(kube,binding)

    def test_wrong_database_or_host_refused(self):
        for url in ('postgres://fixture:password@voice-postgres:5432/user_db?sslmode=disable',
                    'postgres://fixture:password@other:5432/space_db?sslmode=disable'):
            kube=Kube();kube.rows['secret','voice-app-secrets']['data']['SPACE_DATABASE_URL']=base64.b64encode(url.encode()).decode()
            with self.subTest(url=url),self.assertRaises(authority.AuthorityError):authority.capture(kube)

    def test_wrong_owner_claim_or_running_image_refused(self):
        for kind in ('owner','claim','image'):
            kube=Kube()
            if kind=='owner':kube.rows['pod','voice-postgres-0']['metadata']['ownerReferences'][0]['uid']=metadata(7)['uid']
            elif kind=='claim':kube.rows['persistentvolume','pvc-source']['spec']['claimRef']['uid']=metadata(7)['uid']
            else:kube.rows['pod','voice-postgres-0']['status']['containerStatuses'][0]['imageID']='postgres:16-alpine'
            with self.subTest(kind=kind),self.assertRaises(authority.AuthorityError):authority.capture(kube)

    def test_drift_of_any_recorded_object_prevents_backup(self):
        for key in Kube().rows:
            kube=Kube();binding=authority.capture(kube);kube.rows[key]['metadata']['resourceVersion']='18'
            with self.subTest(key=key),self.assertRaises(authority.AuthorityError):authority.revalidate(kube,binding)

    def test_pod_root_password_authority_mismatch_refused(self):
        kube=Kube();kube.run=lambda args:{'password_sha256':'0'*64}
        with self.assertRaises(authority.AuthorityError):authority.capture(kube)
