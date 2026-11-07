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
    def test_password_permit_denies_modified_launch_or_observer_before_spawn(self):
        import commands,resource_claim,resource_claim_test,sys
        from unittest.mock import Mock,patch
        context=resource_claim_test.context();card=resource_claim_test.card(context)
        session=resource_claim.Session(context,'a'*12,'b'*64,lambda:copy.deepcopy(card),resource_claim_test.MemoryJournal())
        parent={'protected':'fixture'};session.admit(start=True)
        session.journal.row.update(phase='prerequisites-captured',target_sha256=resource_claim.digest(parent),cohort_preservation_sha256='c'*64)
        session.expected_target=resource_claim.digest(parent);session._cohort_lineage='c'*64
        prefix=['/usr/local/bin/k3s','kubectl','--namespace','voice-staging'];module=Mock();module.load_capture.return_value=parent
        for fault in ('script','pod','body','timeout','env','fds','phase','target','lineage','observer_guard','observer_fence'):
            with self.subTest(fault=fault):
                session.journal.row.update(status='active',phase='prerequisites-captured',target_sha256=resource_claim.digest(parent),cohort_preservation_sha256='c'*64)
                def run(args):
                    argv=prefix+args;kwargs={'body':b'','timeout':30,'limit':2<<20,
                        'env':{'PATH':'/usr/local/bin:/usr/bin:/bin','HOME':'/root'},'pass_fds':()}
                    if fault=='script':argv[-1]+='; true'
                    elif fault=='pod':argv[5]='other-pod'
                    elif fault=='body':kwargs['body']=b'changed'
                    elif fault=='timeout':kwargs['timeout']=31
                    elif fault=='env':kwargs['env']={'PATH':'/bin'}
                    elif fault=='fds':kwargs['pass_fds']=(0,)
                    return commands.capture(argv,**kwargs)
                kube=Kube();kube.run=run
                def load(_):
                    if fault=='phase':session.journal.row['phase']='prerequisites'
                    elif fault=='target':session.journal.row['target_sha256']='e'*64
                    elif fault=='lineage':session.journal.row['cohort_preservation_sha256']='e'*64
                    return parent
                module.load_capture.side_effect=load
                def observer(action,status):
                    if fault=='observer_guard':commands.authority_guard=lambda _argv:None
                    elif fault=='observer_fence':session.journal.row['status']='uncertain'
                with patch.dict(sys.modules,{'staging_prerequisites':module}), \
                     patch.object(commands,'authority_guard',session.check), \
                     patch.object(commands,'diagnostic_observer',side_effect=observer), \
                     patch.object(commands.subprocess,'Popen') as spawn:
                    with self.assertRaises((RuntimeError,authority.AuthorityError)):authority._password_observation(kube)
                    spawn.assert_not_called()
                self.assertIsNone(session._prerequisite_readonly_permit)
                self.assertFalse(session._prerequisite_readonly_busy)

    def test_captured_password_read_requires_exact_one_shot_parent_bound_permit(self):
        import commands,resource_claim,resource_claim_test,sys
        from unittest.mock import Mock,patch
        context=resource_claim_test.context();card=resource_claim_test.card(context)
        session=resource_claim.Session(context,'a'*12,'b'*64,lambda:copy.deepcopy(card),resource_claim_test.MemoryJournal())
        parent={'protected':'fixture'};session.admit(start=True)
        session.journal.row.update(phase='prerequisites-captured',target_sha256=resource_claim.digest(parent))
        session.expected_target=resource_claim.digest(parent)
        prefix=['/usr/local/bin/k3s','kubectl','--namespace','voice-staging']
        env={'PATH':'/usr/local/bin:/usr/bin:/bin','HOME':'/root'};calls=[]
        def run(args):
            argv=prefix+args
            launch=commands.launch_snapshot(argv,b'',30,2<<20,env,())
            session.check(argv,launch=launch);session.readonly_current();calls.append(argv)
            return {'password_sha256':'a'*64}
        kube=Kube();kube.run=run;module=Mock();module.load_capture.return_value=parent
        with patch.dict(sys.modules,{'staging_prerequisites':module}),patch.object(commands,'authority_guard',session.check):
            self.assertEqual(authority._password_observation(kube),{'password_sha256':'a'*64})
            self.assertEqual(calls,[prefix+list(authority.PASSWORD_ARGS)])
            self.assertIsNone(session._prerequisite_readonly_permit)
            with self.assertRaisesRegex(RuntimeError,'prerequisite_command_forbidden'):session.check(calls[0])
            kube.run=lambda args:run(args+['unexpected'])
            with self.assertRaisesRegex(RuntimeError,'readonly_launch_changed'):authority._password_observation(kube)
            self.assertIsNone(session._prerequisite_readonly_permit)
            kube.run=lambda args:(_ for _ in ()).throw(RuntimeError('lost'))
            with self.assertRaisesRegex(RuntimeError,'lost'):authority._password_observation(kube)
            self.assertFalse(session._prerequisite_readonly_busy)

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
