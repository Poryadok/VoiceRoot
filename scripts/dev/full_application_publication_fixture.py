"""TEST ONLY: raw synthetic HTTP around real source/publication verifiers.

The supplied checkout and native tool evidence must have the same real Git
source. CI, registry and GitHub transport responses are modeled; this fixture
does not establish a real publication, approval, or runtime qualification.
"""
import base64
import copy
import hashlib
import io
import json
from pathlib import Path
import re
import subprocess
import tarfile
import time
import urllib.parse
import urllib.request
from unittest.mock import patch
import zipfile

import canonical_application_package as canonical
import publish_candidate as common
import source_authority as authority


class FixtureError(RuntimeError): pass


def _json(value): return json.dumps(value,sort_keys=True,separators=(',',':')).encode()
def _sha(raw): return 'sha256:'+hashlib.sha256(raw).hexdigest()


class _Response:
    status=200
    def __init__(self,raw,headers=None): self.stream=io.BytesIO(raw); self.headers=headers or {}
    def read(self,size=-1): return self.stream.read(size)
    def close(self): self.stream.close()
    def __enter__(self): return self
    def __exit__(self,*args): self.close()


class PublicationFixture:
    """Supply synthetic HTTP bytes for exactly one real clean candidate checkout.

    `native_dir` optionally contains *actual* same-source Tool evidence files
    identity.json, index.json, manifest.json and config.json; never relabel them.
    Its artifact is modeled as a CI ZIP, but its executable/content proof is
    caller-supplied and is rechecked by the unmodified production reader.
    """
    def __init__(self,source_root,candidate_sha,native_dir=None):
        self.source_root=Path(source_root).resolve(); self.candidate_sha=candidate_sha
        if not self.source_root.is_absolute() or not re.fullmatch(r'[a-f0-9]{40}',candidate_sha): raise FixtureError('real source required')
        self.snapshot=common.source_snapshot(self.source_root,candidate_sha)
        self.parent_sha=self._git('rev-parse',candidate_sha+'^').decode().strip()
        self.tree_sha=self._git('rev-parse',candidate_sha+'^{tree}').decode().strip()
        self.request=canonical.parse_request((self.source_root/canonical.REQUEST).read_bytes())
        self.run_id=17; self.attempt=1; self.token='test-only-token'; self.actor='test-only-actor'
        self.repo={'id':9,'full_name':authority.REPO,'private':False,'default_branch':'master'}
        self.run={'id':self.run_id,'workflow_id':authority.WORKFLOW,'run_attempt':self.attempt,
            'head_sha':candidate_sha,'head_branch':'master','event':'push','path':'.github/workflows/ci.yml',
            'status':'completed','conclusion':'success','repository':self.repo,'head_repository':self.repo}
        self.jobs=[]
        for number,(name,step) in enumerate(canonical.required_jobs().items(),1):
            self.jobs.append({'id':number,'name':name,'run_id':self.run_id,'run_attempt':self.attempt,
                'head_sha':candidate_sha,'status':'completed','conclusion':'success',
                'started_at':'2025-01-01T00:00:00Z','completed_at':'2025-01-01T00:01:00Z',
                'steps':[{'name':step,'conclusion':'success'}]})
        for number,name in enumerate(('staging-images-promote','deploy-staging'),200):
            self.jobs.append({'id':number,'name':name,'run_id':self.run_id,'run_attempt':self.attempt,
                'head_sha':candidate_sha,'status':'completed','conclusion':'skipped','steps':[]})
        self.registry={}; self.images={}
        for name in sorted(canonical.IMAGE_NAMES): self._image(name)
        lock=('version: 1\nregistry: '+canonical.REGISTRY+'\ntag: '+candidate_sha+'\nimages:\n'+
              ''.join('  '+name+': '+candidate_sha+'\n' for name in sorted(canonical.IMAGE_NAMES))).encode()
        self.archives={99:self._zip('stack.lock.yaml',lock)}
        self.artifacts=[self._artifact(99,'staging-stack-lock',self.archives[99])]
        if native_dir is not None: self._native(Path(native_dir))
        self.archive=self._archive()
        self.requests=[]; self._patches=[]

    def _git(self,*args):
        return subprocess.run(['git','-C',str(self.source_root),*args],check=True,stdout=subprocess.PIPE,
                              stderr=subprocess.DEVNULL,timeout=180).stdout

    def _archive(self):
        """Package only Git-tracked checkout bytes already checked by source_snapshot."""
        buffer=io.BytesIO(); rows=self._git('ls-tree','-r','-z',self.candidate_sha)
        with tarfile.open(fileobj=buffer,mode='w:gz') as archive:
            for entry in rows.split(b'\0'):
                if not entry: continue
                meta,name=entry.split(b'\t',1); mode,kind,oid=meta.decode().split(); path=name.decode()
                if kind!='blob' or mode not in ('100644','100755') or path not in self.snapshot['files']: raise FixtureError('source tree invalid')
                raw=(self.source_root/path).read_bytes()
                if hashlib.sha256(raw).hexdigest()!=self.snapshot['files'][path] or canonical.blob_oid(raw)!=oid: raise FixtureError('source changed')
                info=tarfile.TarInfo('Poryadok-VoiceRoot-'+self.candidate_sha[:7]+'/'+path)
                info.size=len(raw); info.mode=0o755 if mode=='100755' else 0o644
                archive.addfile(info,io.BytesIO(raw))
        return buffer.getvalue()

    def _image(self,name):
        config=_json({'os':'linux','architecture':'amd64','config':{'Labels':{'org.opencontainers.image.revision':self.candidate_sha}},
                      'fixture_component':name})
        manifest=_json({'schemaVersion':2,'mediaType':'application/vnd.oci.image.manifest.v1+json',
            'config':{'mediaType':'application/vnd.oci.image.config.v1+json','size':len(config),'digest':_sha(config)},'layers':[]})
        index=_json({'schemaVersion':2,'mediaType':'application/vnd.oci.image.index.v1+json',
            'manifests':[{'mediaType':'application/vnd.oci.image.manifest.v1+json','size':len(manifest),
                          'digest':_sha(manifest),'platform':{'os':'linux','architecture':'amd64'}}]})
        repo='poryadok/voiceroot/'+name
        for path,raw in (('manifests/'+self.candidate_sha,index),('manifests/'+_sha(index),index),
                         ('manifests/'+_sha(manifest),manifest),('blobs/'+_sha(config),config)):
            self.registry[repo+'/'+path]=raw
        self.images[name]=canonical.REGISTRY+'/'+name+'@'+_sha(manifest)

    @staticmethod
    def _zip(name,raw):
        value=io.BytesIO()
        with zipfile.ZipFile(value,'w',compression=zipfile.ZIP_DEFLATED) as archive: archive.writestr(name,raw)
        return value.getvalue()

    def _artifact(self,number,name,raw):
        return {'id':number,'name':name,'expired':False,'size_in_bytes':len(raw),'created_at':'2025-01-01T00:00:30Z',
                'expires_at':'2099-01-01T00:00:00Z','workflow_run':{'id':self.run_id,'head_sha':self.candidate_sha,
                'repository_id':self.repo['id'],'head_repository_id':self.repo['id']}}

    def _native(self,directory):
        data=json.loads((directory/'identity.json').read_bytes()); identity=data['identity']; inventory=data['inventory']; proof=data['proof']
        if inventory.get('source_sha')!=self.candidate_sha: raise FixtureError('native tool belongs to a different source')
        tag=canonical.REGISTRY+'/space-native-installer:source-'+self.candidate_sha+'-'+str(self.run_id)+'-'+str(self.attempt)
        publication={'schema':'voice-space-native-tool-publication-v1','source_sha':self.candidate_sha,
                     'run_id':self.run_id,'run_attempt':self.attempt,'tag':tag,**identity,'source_inventory':inventory,
                     'request':copy.deepcopy(self.request),'content':proof,'runtime_qualified':False,'deployment_performed':False}
        repo='poryadok/voiceroot/space-native-installer'
        for name,path in (('index','index_digest'),('manifest','platform_digest'),('config','config_digest')):
            raw=(directory/(name+'.json')).read_bytes()
            if _sha(raw)!=identity[path]: raise FixtureError('native OCI bytes mismatch')
            self.registry[repo+'/'+('blobs/' if name=='config' else 'manifests/')+identity[path]]=raw
        self.registry[repo+'/manifests/'+tag.rsplit(':',1)[1]]=self.registry[repo+'/manifests/'+identity['index_digest']]
        self.jobs.append({'id':300,'name':'Space native tool image','run_id':self.run_id,'run_attempt':self.attempt,
            'head_sha':self.candidate_sha,'status':'completed','conclusion':'success',
            'started_at':'2025-01-01T00:00:00Z','completed_at':'2025-01-01T00:01:00Z',
            'steps':[{'name':name,'conclusion':'success'} for name in ('Verify Space native tool source','Capture Space native tool identity')]})
        self.archives[101]=self._zip('space-native-tool.json',_json(publication))
        self.artifacts.append(self._artifact(101,'space-native-tool-'+str(self.run_id)+'-'+str(self.attempt),self.archives[101]))

    def _tree(self,tree,recursive):
        args=['ls-tree','-l','-z']+(['-r','-t'] if recursive else [])+[tree]
        rows=[]
        for entry in self._git(*args).split(b'\0'):
            if not entry: continue
            meta,path=entry.split(b'\t',1); fields=meta.decode().split(); mode,kind,oid=fields[:3]
            row={'path':path.decode(),'mode':mode,'type':kind,'sha':oid}
            if kind=='blob': row['size']=int(fields[3])
            rows.append(row)
        return {'sha':tree,'truncated':False,'tree':rows}

    def _api(self,url):
        parsed=urllib.parse.urlsplit(url); suffix=parsed.path.removeprefix('/repos/'+authority.REPO)
        if suffix=='': return _json(self.repo)
        if suffix=='/git/ref/heads/master': return _json({'ref':'refs/heads/master','object':{'type':'commit','sha':self.candidate_sha}})
        if suffix=='/actions/runs/'+str(self.run_id): return _json(self.run)
        if suffix.startswith('/actions/runs/'+str(self.run_id)+'/attempts/'+str(self.attempt)+'/jobs'):
            page=int(urllib.parse.parse_qs(parsed.query)['page'][0]); rows=self.jobs[(page-1)*20:page*20]
            return _json({'total_count':len(self.jobs),'jobs':rows})
        if suffix.startswith('/actions/runs/'+str(self.run_id)+'/artifacts'):
            page=int(urllib.parse.parse_qs(parsed.query)['page'][0]); rows=self.artifacts[(page-1)*20:page*20]
            return _json({'total_count':len(self.artifacts),'artifacts':rows})
        if suffix.startswith('/actions/artifacts/'):
            parts=suffix.split('/'); number=int(parts[3])
            if len(parts)==5 and parts[4]=='zip': return self.archives[number]
            return _json(next(row for row in self.artifacts if row['id']==number))
        if suffix.startswith('/git/commits/'):
            sha=suffix.rsplit('/',1)[1]; parents=self._git('rev-list','--parents','-n','1',sha).decode().split()
            return _json({'sha':sha,'tree':{'sha':self._git('rev-parse',sha+'^{tree}').decode().strip()},
                          'parents':[{'sha':value} for value in parents[1:]]})
        if suffix.startswith('/git/trees/'):
            return _json(self._tree(suffix.rsplit('/',1)[1],parsed.query=='recursive=1'))
        if suffix.startswith('/git/blobs/'):
            oid=suffix.rsplit('/',1)[1]; raw=self._git('cat-file','blob',oid)
            return _json({'sha':oid,'encoding':'base64','content':base64.b64encode(raw).decode()})
        if suffix=='/tarball/'+self.candidate_sha: return self.archive
        raise FixtureError('unexpected synthetic GitHub route: '+suffix)

    def _raw(self,url):
        parsed=urllib.parse.urlsplit(url)
        if parsed.hostname=='api.github.com': return self._api(url),{}
        if parsed.hostname=='ghcr.io' and parsed.path=='/token': return _json({'token':'test-only-bearer'}),{}
        prefix='/v2/'
        if parsed.hostname=='ghcr.io' and parsed.path.startswith(prefix):
            raw=self.registry[parsed.path[len(prefix):]]
            return raw,{'Docker-Content-Digest':_sha(raw)}
        raise FixtureError('unexpected synthetic HTTP host')

    def __enter__(self):
        def request(url,headers,limit,deadline):
            self.requests.append(url); raw,headers=self._raw(url); return _Response(raw,headers)
        def opener(_opener,request,timeout=None):
            self.requests.append(request.full_url); raw,headers=self._raw(request.full_url); return _Response(raw,headers)
        self._patches=[patch.object(authority.transport,'_request',side_effect=request),
                       patch.object(urllib.request.OpenerDirector,'open',new=opener)]
        for item in self._patches: item.start()
        return self

    def __exit__(self,*args):
        for item in reversed(self._patches): item.stop()
        self._patches=[]
