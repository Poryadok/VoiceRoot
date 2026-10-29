"""OCI consumer byte verification; registry is an in-memory external transport."""
import hashlib
import json
import unittest

import auth_live_migration_phase_eligibility as auth
import space_creator_runtime_profile_target_plan as gateway


def encoded(value):
    raw=json.dumps(value,sort_keys=True,separators=(',',':')).encode()
    return raw,'sha256:'+hashlib.sha256(raw).hexdigest()


class Registry:
    def __init__(self, objects):self.objects=objects
    def get(self, repository, path):return self.objects[repository,path]


class ConsumerManifestTest(unittest.TestCase):
    def fixture(self,service,arch='amd64',size_delta=0):
        repository='poryadok/voiceroot/'+service
        runtime=({'Entrypoint':['java','-jar','/auth.jar'],'Cmd':None,'WorkingDir':'/','Env':['PATH=/bin']}
                 if service=='auth' else {'Entrypoint':['/gateway'],'Cmd':None,'WorkingDir':'/','Env':['PATH=/bin']})
        config,config_sha=encoded({'os':'linux','architecture':arch,'config':runtime})
        child,child_sha=encoded({'schemaVersion':2,'mediaType':'application/vnd.oci.image.manifest.v1+json',
            'layers':[],'config':{'mediaType':'application/vnd.oci.image.config.v1+json',
                                  'digest':config_sha,'size':len(config)+size_delta}})
        index,index_sha=encoded({'schemaVersion':2,'mediaType':'application/vnd.oci.image.index.v1+json',
            'manifests':[{'digest':child_sha,'size':len(child),'mediaType':'application/vnd.oci.image.manifest.v1+json',
                          'platform':{'os':'linux','architecture':'amd64'}}]})
        objects={(repository,'manifests/'+index_sha):index,(repository,'manifests/'+child_sha):child,
                 (repository,'blobs/'+config_sha):config}
        return Registry(objects),index_sha,child_sha,config_sha

    def consume(self,service,registry,requested,platform):
        if service=='auth':return auth.read_image_config(registry,requested,platform)
        return gateway._gateway_image(registry,'ghcr.io/poryadok/voiceroot/gateway@'+requested,platform)

    def test_index_and_direct_child_use_identical_verified_config(self):
        for service in ('auth','gateway'):
            with self.subTest(service=service):
                registry,index,child,config=self.fixture(service)
                expected={'env':{'PATH':'/bin'},'config_digest':config}
                self.assertEqual(self.consume(service,registry,index,child),expected)
                self.assertEqual(self.consume(service,registry,child,child),expected)

    def test_wrong_platform_config_size_digest_or_repository_is_rejected(self):
        for service in ('auth','gateway'):
            for mutation in ('architecture','size','digest','repository','foreign_platform'):
                for kind in ('index','child'):
                    with self.subTest(service=service,mutation=mutation,kind=kind):
                        registry,index,child,_=self.fixture(service,arch='arm64' if mutation=='architecture' else 'amd64',
                                                            size_delta=1 if mutation=='size' else 0)
                        if mutation=='digest':
                            key=next(k for k in registry.objects if k[1].startswith('blobs/'))
                            registry.objects[key]=registry.objects[key]+b' '
                        if mutation=='repository':registry.objects={(('wrong/'+k[0],k[1])):v for k,v in registry.objects.items()}
                        requested=index if kind=='index' else child
                        platform='sha256:'+'f'*64 if mutation=='foreign_platform' else child
                        with self.assertRaises(Exception):self.consume(service,registry,requested,platform)

    def test_index_bytes_cannot_be_used_as_direct_manifest(self):
        for service in ('auth','gateway'):
            registry,index,_,_=self.fixture(service)
            with self.subTest(service=service),self.assertRaises(Exception):
                self.consume(service,registry,index,index)

    def test_direct_child_rejects_invalid_descriptor_types(self):
        for service in ('auth','gateway'):
            for field in ('config','layer'):
                registry,_,child,_=self.fixture(service)
                key=next(k for k in registry.objects if k[1]=='manifests/'+child)
                obj=json.loads(registry.objects[key])
                if field=='config':obj['config']['mediaType']='application/not-an-image'
                else:obj['layers']=[{'mediaType':'application/not-a-layer','digest':'sha256:'+'a'*64,'size':1}]
                raw,digest=encoded(obj);registry.objects[key[0],'manifests/'+digest]=raw
                with self.subTest(service=service,field=field),self.assertRaises(Exception):
                    self.consume(service,registry,digest,digest)


if __name__=='__main__':unittest.main()
