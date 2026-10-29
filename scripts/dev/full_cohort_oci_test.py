import hashlib
import json
import unittest

import canonical_application_package as package
import full_cohort_oci as oci


def encoded(value):
    raw = json.dumps(value, separators=(',', ':'), sort_keys=True).encode()
    return raw, 'sha256:' + hashlib.sha256(raw).hexdigest()


class Registry:
    def __init__(self, objects): self.objects = objects
    def get(self, repository, path): return self.objects[repository, path]


class OCITest(unittest.TestCase):
    def setUp(self):
        config, config_sha = encoded({'architecture':'amd64', 'os':'linux'})
        child, child_sha = encoded({'schemaVersion':2, 'layers':[], 'config':{'digest':config_sha,
            'size':len(config),'mediaType':'application/vnd.oci.image.config.v1+json'}})
        index, index_sha = encoded({'schemaVersion':2, 'mediaType':'application/vnd.oci.image.index.v1+json', 'manifests':[{'digest':child_sha,
            'size':len(child),'mediaType':'application/vnd.oci.image.manifest.v1+json',
            'platform':{'os':'linux','architecture':'amd64'}}]})
        self.images = {name:package.REGISTRY+'/'+name+'@'+index_sha for name in package.IMAGE_NAMES}
        self.objects = {}
        for name in package.APPLICATIONS:
            repo = 'poryadok/voiceroot/'+name
            self.objects[repo,'manifests/'+index_sha] = index
            self.objects[repo,'manifests/'+child_sha] = child
            self.objects[repo,'blobs/'+config_sha] = config
        self.child = child_sha

    def test_exact_package_derived_map(self):
        result = oci.resolve(self.images, Registry(self.objects))
        self.assertEqual(len(result), 23)
        self.assertEqual(set(result.values()), {self.child})
        self.assertIn('voice-developer-portal/portal', result)

    def test_missing_package_and_corrupt_bytes_hold(self):
        with self.assertRaises(oci.OCIError): oci.resolve({}, Registry(self.objects))
        key = next(iter(self.objects)); self.objects[key] += b'bad'
        with self.assertRaises(oci.OCIError): oci.resolve(self.images, Registry(self.objects))

    def test_ambiguous_platform_holds(self):
        index, index_sha = encoded({'schemaVersion':2, 'mediaType':'application/vnd.oci.image.index.v1+json', 'manifests':[
            {'digest':self.child,'platform':{'os':'linux','architecture':'amd64'}},
            {'digest':self.child,'platform':{'os':'linux','architecture':'amd64'}}]})
        name = package.APPLICATIONS[0]; self.images[name] = package.REGISTRY+'/'+name+'@'+index_sha
        self.objects['poryadok/voiceroot/'+name,'manifests/'+index_sha] = index
        with self.assertRaises(oci.OCIError): oci.resolve(self.images, Registry(self.objects))

    def test_wrong_config_architecture_holds(self):
        config, config_sha = encoded({'architecture':'arm64', 'os':'linux'})
        child, child_sha = encoded({'schemaVersion':2,'layers':[], 'config':{'digest':config_sha,
            'size':len(config),'mediaType':'application/vnd.oci.image.config.v1+json'}})
        index, index_sha = encoded({'schemaVersion':2,'mediaType':'application/vnd.oci.image.index.v1+json','manifests':[{'digest':child_sha,
            'size':len(child),'mediaType':'application/vnd.oci.image.manifest.v1+json',
            'platform':{'os':'linux','architecture':'amd64'}}]})
        name = package.APPLICATIONS[0]; repo = 'poryadok/voiceroot/'+name
        self.images[name] = package.REGISTRY+'/'+name+'@'+index_sha
        self.objects[repo,'manifests/'+index_sha] = index
        self.objects[repo,'manifests/'+child_sha] = child
        self.objects[repo,'blobs/'+config_sha] = config
        with self.assertRaises(oci.OCIError): oci.resolve(self.images, Registry(self.objects))

    def test_duplicate_index_key_holds_even_with_matching_digest(self):
        raw = b'{"schemaVersion":2,"schemaVersion":2,"manifests":[]}'
        digest = 'sha256:' + hashlib.sha256(raw).hexdigest()
        name = package.APPLICATIONS[0]; repo = 'poryadok/voiceroot/'+name
        self.images[name] = package.REGISTRY+'/'+name+'@'+digest
        self.objects[repo,'manifests/'+digest] = raw
        with self.assertRaises(oci.OCIError): oci.resolve(self.images, Registry(self.objects))

    def test_non_image_manifest_holds(self):
        child, child_sha = encoded({'schemaVersion':2, 'config':{'digest':'sha256:'+'a'*64}})
        index, index_sha = encoded({'schemaVersion':2,'mediaType':'application/vnd.oci.image.index.v1+json',
            'manifests':[{'digest':child_sha,'size':len(child),
                'mediaType':'application/vnd.oci.image.manifest.v1+json',
                'platform':{'os':'linux','architecture':'amd64'}}]})
        name = package.APPLICATIONS[0]; repo = 'poryadok/voiceroot/'+name
        self.images[name] = package.REGISTRY+'/'+name+'@'+index_sha
        self.objects[repo,'manifests/'+index_sha] = index
        self.objects[repo,'manifests/'+child_sha] = child
        with self.assertRaises(oci.OCIError): oci.resolve(self.images, Registry(self.objects))

    def test_docker_list_with_nonplatform_attestation(self):
        config, config_sha = encoded({'architecture':'amd64','os':'linux'})
        child, child_sha = encoded({'schemaVersion':2,'mediaType':'application/vnd.docker.distribution.manifest.v2+json',
            'layers':[{'mediaType':'application/vnd.docker.image.rootfs.diff.tar.gzip','size':12,'digest':'sha256:'+'a'*64}],
            'config':{'mediaType':'application/vnd.docker.container.image.v1+json','size':len(config),'digest':config_sha}})
        index, index_sha = encoded({'schemaVersion':2,'mediaType':'application/vnd.docker.distribution.manifest.list.v2+json',
            'manifests':[{'digest':'sha256:'+'b'*64,'size':1,'mediaType':'application/vnd.oci.image.manifest.v1+json',
                          'platform':{'os':'unknown','architecture':'unknown'}},
                         {'digest':child_sha,'size':len(child),'mediaType':'application/vnd.docker.distribution.manifest.v2+json',
                          'platform':{'os':'linux','architecture':'amd64'}}]})
        name = package.APPLICATIONS[0]; repo = 'poryadok/voiceroot/'+name
        self.images[name] = package.REGISTRY+'/'+name+'@'+index_sha
        self.objects[repo,'manifests/'+index_sha] = index
        self.objects[repo,'manifests/'+child_sha] = child
        self.objects[repo,'blobs/'+config_sha] = config
        self.assertEqual(oci.resolve(self.images, Registry(self.objects))['voice-'+name+'/'+oci.update_images.SERVICES[name]], child_sha)

    def test_index_optional_media_type(self):
        name = package.APPLICATIONS[0]; repo = 'poryadok/voiceroot/'+name
        previous = self.images[name].rsplit('@',1)[1]
        index = json.loads(self.objects[repo,'manifests/'+previous])
        del index['mediaType']
        raw, digest = encoded(index)
        self.images[name] = package.REGISTRY+'/'+name+'@'+digest
        self.objects[repo,'manifests/'+digest] = raw
        self.assertEqual(len(oci.resolve(self.images, Registry(self.objects))), 23)

    def test_direct_manifest_and_source_index_provenance(self):
        source='a'*40
        for name in package.APPLICATIONS:
            repo='poryadok/voiceroot/'+name; index=self.images[name].rsplit('@',1)[1]
            self.objects[repo,'manifests/'+source]=self.objects[repo,'manifests/'+index]
            self.images[name]=package.REGISTRY+'/'+name+'@'+self.child
        mapped,proof=oci.resolve_provenance(self.images,Registry(self.objects),source)
        self.assertEqual(set(mapped.values()),{self.child})
        for item in proof.values():
            self.assertEqual(item['requested_kind'],'manifest'); self.assertEqual(item['platform_digest'],self.child)
            self.assertNotEqual(item['index_digest'],self.child)
            self.assertEqual(item['source_tag_kind'],'index')

    def test_direct_manifest_wrong_architecture_and_foreign_source_fail(self):
        source='a'*40; name=package.APPLICATIONS[0]; repo='poryadok/voiceroot/'+name
        for item in package.APPLICATIONS:
            root='poryadok/voiceroot/'+item; digest=self.images[item].rsplit('@',1)[1]
            self.objects[root,'manifests/'+source]=self.objects[root,'manifests/'+digest]
            self.images[item]=package.REGISTRY+'/'+item+'@'+self.child
        config,config_sha=encoded({'architecture':'arm64','os':'linux'})
        child,child_sha=encoded({'schemaVersion':2,'layers':[], 'config':{'digest':config_sha,'size':len(config),
                                  'mediaType':'application/vnd.oci.image.config.v1+json'}})
        self.images[name]=package.REGISTRY+'/'+name+'@'+child_sha
        self.objects[repo,'manifests/'+child_sha]=child; self.objects[repo,'blobs/'+config_sha]=config
        with self.assertRaises(oci.OCIError): oci.resolve(self.images,Registry(self.objects))
        self.images[name]=package.REGISTRY+'/'+name+'@'+self.child
        bad,sha=encoded({'schemaVersion':2,'layers':[],'config':{'digest':config_sha,'size':len(config),
                         'mediaType':'application/vnd.oci.image.config.v1+json'}})
        self.objects[repo,'manifests/'+source]=bad
        with self.assertRaises(oci.OCIError): oci.resolve_provenance(self.images,Registry(self.objects),source)

    def test_direct_tag_has_no_invented_parent_index(self):
        source='a'*40
        for name in package.APPLICATIONS:
            repo='poryadok/voiceroot/'+name
            self.objects[repo,'manifests/'+source]=self.objects[repo,'manifests/'+self.child]
            self.images[name]=package.REGISTRY+'/'+name+'@'+self.child
        mapped,proof=oci.resolve_provenance(self.images,Registry(self.objects),source)
        images={'voice-'+name+'/'+oci.update_images.SERVICES[name]:self.images[name] for name in package.APPLICATIONS}
        self.assertTrue(oci.validate_provenance(images,mapped,proof))
        self.assertTrue(all(item['index_digest'] is None and item['index_source'] is None for item in proof.values()))
        first=next(iter(proof.values())); first['index_digest']=first['platform_digest']
        with self.assertRaises(oci.OCIError): oci.validate_provenance(images,mapped,proof)

    def test_wrong_source_revision_label_fails(self):
        source='a'*40; name=package.APPLICATIONS[0]; repo='poryadok/voiceroot/'+name
        for item in package.APPLICATIONS:
            root='poryadok/voiceroot/'+item; digest=self.images[item].rsplit('@',1)[1]
            self.objects[root,'manifests/'+source]=self.objects[root,'manifests/'+digest]
        config,config_sha=encoded({'architecture':'amd64','os':'linux','config':{'Labels':{'org.opencontainers.image.revision':'f'*40}}})
        child,child_sha=encoded({'schemaVersion':2,'layers':[], 'config':{'digest':config_sha,'size':len(config),
                                  'mediaType':'application/vnd.oci.image.config.v1+json'}})
        self.images[name]=package.REGISTRY+'/'+name+'@'+child_sha
        self.objects[repo,'manifests/'+child_sha]=child; self.objects[repo,'blobs/'+config_sha]=config
        with self.assertRaises(oci.OCIError): oci.resolve_provenance(self.images,Registry(self.objects),source)

    def test_source_tag_movement_fails_even_when_child_is_unchanged(self):
        source='a'*40
        for name in package.APPLICATIONS:
            repo='poryadok/voiceroot/'+name; digest=self.images[name].rsplit('@',1)[1]
            self.objects[repo,'manifests/'+source]=self.objects[repo,'manifests/'+digest]
        first='poryadok/voiceroot/'+package.APPLICATIONS[0]; objects=self.objects
        class Drifting(Registry):
            seen=False
            def get(inner,repository,path):
                raw=super().get(repository,path)
                if repository==first and path=='manifests/'+source:
                    if inner.seen: return raw+b' '
                    inner.seen=True
                return raw
        with self.assertRaises(oci.OCIError): oci.resolve_provenance(self.images,Drifting(objects),source)


if __name__ == '__main__': unittest.main()
