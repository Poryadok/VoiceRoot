"""Compile parsed canonical target data; target scripts are never executed."""
import copy
import hashlib
import os
import json
from pathlib import Path
import re
import stat
import sys
import guard
import migrations
from apply import digest
from controller import Blocked
from commands import capture

SOURCE_MANIFESTS=('deploy/staging/services.yaml','deploy/staging/gateway-deployment.yaml',
 'deploy/templates/network-policy-social-privacy-principal.yaml',
 'deploy/templates/network-policy-file-user-principal.yaml',
 'deploy/templates/network-policy-search-user-projection.yaml')

def compatible_bootstrap(source,contract,decoder):
    paths=[]
    scripts=contract.get('scripts',[])
    if len(scripts)!=4 or {r.get('part') for r in scripts}!={'realtime','notification','search','analytics-chat'}:
        raise Blocked('rollout_bootstrap_contract_invalid')
    for expected in scripts:
        relative='deploy/templates/nats-'+expected['part']+'-bootstrap.yaml';path=Path(source)/relative
        if path.is_symlink() or not path.is_file() or not 1<=path.stat().st_size<=131072:
            raise Blocked('rollout_bootstrap_source_invalid')
        text=path.read_bytes().decode('utf-8').replace('__NAMESPACE__',guard.NS).replace('__K_NAMESPACE__',guard.NS)
        first=re.split(r'^---\s*$',text,flags=re.MULTILINE)[0]
        row=decoder(first.encode())
        if row.get('kind')!='ConfigMap' or row.get('metadata',{}).get('name')!='voice-nats-'+expected['part']+'-bootstrap':
            raise Blocked('rollout_bootstrap_source_invalid')
        script=row.get('data',{}).get('bootstrap.sh')
        if not isinstance(script,str) or hashlib.sha256(script.encode()).hexdigest()!=expected['sha256']:
            raise Blocked('rollout_bootstrap_contract_unsupported')
        paths.append(relative)
    return paths

def rendered_source(source,parameters,decoder):
    generation=parameters['generation'];claim=parameters['dataPVC']
    if not re.fullmatch(r'r[0-9]{8}[a-z0-9]{1,8}',generation) or not re.fullmatch(r'voice-nats-jsdata-d[0-9]{8}[a-z0-9]{0,8}',claim):
        raise Blocked('rollout_compile_identity_invalid')
    endpoint=parameters['s3_signing_endpoint']
    if not re.fullmatch(r'https?://[a-zA-Z0-9./:_-]{1,253}',endpoint):raise Blocked('rollout_compile_endpoint_invalid')
    replacements={'__IMAGE_REGISTRY__':parameters['registry'],'__IMAGE_TAG__':parameters['tag'],
        'IMAGE_PLACEHOLDER':parameters['registry']+'/gateway:'+parameters['tag'],
        '__S3_SIGNING_ENDPOINT__':endpoint,'__K_NAMESPACE__':guard.NS,
        'voice-nats-operator':'voice-nats-operator-'+generation,
        'voice-nats-hub-tls':'voice-nats-hub-tls-'+generation,
        'voice-nats-bootstrap-credentials':'voice-nats-bootstrap-credentials-'+generation,
        'voice-nats-service-credentials':'voice-nats-service-credentials-'+generation,
        'voice-nats-jsdata':claim}
    result=[]
    for relative in SOURCE_MANIFESTS:
        path=Path(source)/relative
        if path.is_symlink() or not path.is_file() or path.stat().st_size>2<<20:raise Blocked('rollout_compile_manifest_invalid')
        text=path.read_bytes().decode('utf-8')
        for key,value in replacements.items():text=text.replace(key,value)
        if re.search(r'__[A-Z0-9_]+__|IMAGE_PLACEHOLDER',text):raise Blocked('rollout_compile_placeholder_unresolved')
        # One object per invocation: kubectl printers do not guarantee one
        # JSON document for a multi-document YAML input.
        for piece in re.split(r'^---\s*$',text,flags=re.MULTILINE):
            if not any(line.strip() and not line.lstrip().startswith('#') for line in piece.splitlines()):continue
            row=decoder(piece.encode())
            if not isinstance(row,dict):raise Blocked('rollout_compile_decode_invalid')
            result.append(row)
    return result

def decode_yaml(raw):
    return json.loads(capture(['kubectl','create','--dry-run=client','--validate=false',
                              '-f','/dev/stdin','-o','json'],body=raw,limit=131072,timeout=30))

class ProducerKube:
    def run(self,args,body=None,timeout=30):
        return json.loads(capture(['kubectl','--namespace',guard.NS,*args],
            body=b'' if body is None else json.dumps(body).encode(),timeout=timeout))
    def get(self,kind,name):return self.run(['get',kind,name,'-o','json'])
    def secret_meta(self,name):
        values=self.run(['get','secret',name,'-o',"jsonpath-as-json={.metadata['uid','resourceVersion']}"])
        return dict(zip(('uid','resourceVersion'),values))

def main(args):
    if len(args)!=3:raise Blocked('rollout_compile_arguments_invalid')
    source=Path(args[0]).resolve(strict=True);parameter_path=Path(args[1]);output=Path(args[2])
    if parameter_path.is_symlink() or not parameter_path.is_file() or not 1<=parameter_path.stat().st_size<=65536:
        raise Blocked('rollout_compile_parameters_invalid')
    p=json.loads(parameter_path.read_bytes(),object_pairs_hook=guard.object_pairs)
    required={'registry','tag','mode','changed_services','images','contract','enrolled','generation','dataPVC','s3_signing_endpoint'}
    optional={'gateway_host','storage_host','livekit_host','web_host','admin_host','developer_portal_host',
        'gateway_tls_secret','storage_tls_secret','image_pull_secret','apply_observability',
        'minio_image','minio_mc_image','minio_storage_class','minio_storage_size','web_origin'}
    if not required<=set(p) or set(p)-required-optional:
        raise Blocked('rollout_compile_parameters_invalid')
    if not re.fullmatch(r'[a-z0-9][a-z0-9./_-]{1,253}',p['registry']) or not re.fullmatch(r'[a-f0-9]{40}',p['tag']):
        raise Blocked('rollout_compile_version_invalid')
    scope={**p,'template_hashes':{'voice-'+name:None for name in p['changed_services']}}
    frontend_only=guard.frontend_image_only(scope)
    producer=ProducerKube()
    if frontend_only:
        bootstrap_paths=[]
        rows=[producer.get('Deployment','voice-'+name) for name in sorted(set(p['changed_services']))]
        source_paths=tuple('deploy/staging/'+({'web':'flutter-web'}.get(name,name))+'.yaml' for name in sorted(set(p['changed_services'])))
    else:
        bootstrap_paths=compatible_bootstrap(source,p['contract'],decode_yaml)
        rows=rendered_source(source,p,decode_yaml)
        source_paths=SOURCE_MANIFESTS
    import source_plan
    plan=source_plan.compile_plan(source,p,decode_yaml,producer)
    if frontend_only and plan['actions']:raise Blocked('rollout_frontend_auxiliary_action_forbidden')
    for action in plan['actions']:
        wanted=action['manifest'];key=(wanted['kind'],wanted['metadata']['name'])
        rows=[r for r in rows if (r['kind'],r['metadata']['name'])!=key]+[wanted]
    changed=set(p['changed_services'])
    if p['mode']=='images-only':rows=[r for r in rows if r.get('kind')=='Deployment' and r.get('metadata',{}).get('name','').removeprefix('voice-') in changed]
    selected_images=p['images']
    if p['mode']!='images-only':
        keys={r['metadata']['name']+'/'+c['name'] for r in rows if r.get('kind')=='Deployment' for c in r['spec']['template']['spec'].get('initContainers',[])+r['spec']['template']['spec']['containers']}
        preserved={}
        for check in plan['checks']:
            for descriptor in check['objects']:
                if descriptor['kind']!='Deployment' or descriptor['name'] not in ('voice-web','voice-admin','voice-developer-portal') or descriptor.get('disposition',check['disposition'])!='preserve':continue
                spec=descriptor['desired']['spec']['template']['spec']
                for container in spec.get('initContainers',[])+spec['containers']:preserved[descriptor['name']+'/'+container['name']]=container['image']
        if any(preserved.get(key)!=image for key,image in p['images'].items() if key not in keys):raise Blocked('rollout_compile_image_inventory_changed')
        selected_images={key:image for key,image in p['images'].items() if key in keys}
    pack=compile_target(source,rows,p['registry'],p['tag'],p['mode'],p['changed_services'],selected_images,p['contract'],set(p['enrolled']),
                        (*source_paths,*bootstrap_paths,'deploy/nats/acl-intent.yaml'))
    pack['nonnats']=plan;pack['target']['nonnats_sha256']=digest(plan)
    for row in plan['checks']:
        for binding in row.get('sources',[]):
            relative=binding['path'];path=source/relative
            if not str(relative).startswith(('deploy/','scripts/','src/backend/migrations/','src/backend/auth/src/main/resources/db/migration/','docker/clickhouse/init/')) or Path(relative).is_absolute() or '..' in Path(relative).parts or path.is_symlink() or not path.is_file() or path.stat().st_size>2<<20 or hashlib.sha256(path.read_bytes()).hexdigest()!=binding['sha256']:
                raise Blocked('rollout_compile_plan_source_changed')
            prior=pack['target']['source_hashes'].get(relative)
            if prior is not None and prior!=binding['sha256']:raise Blocked('rollout_compile_plan_source_changed')
            pack['target']['source_hashes'][relative]=binding['sha256']
    raw=json.dumps(pack,sort_keys=True,separators=(',',':')).encode()
    if len(raw)>2<<20 or output.name!='target-'+p['tag']+'.json':raise Blocked('rollout_compile_output_invalid')
    fd=os.open(output,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o644)
    try:
        with os.fdopen(fd,'wb') as stream:stream.write(raw)
    except Exception:
        raise Blocked('rollout_compile_output_failed') from None
    print('NATS_ROLLOUT_TARGET_SHA256='+hashlib.sha256(raw).hexdigest())

def compile_target(source,documents,registry,tag,mode,changed,images,contract,enrolled,source_paths):
    source=Path(source).resolve(strict=True)
    if mode not in ('full','app-only','images-only') or not re.fullmatch(r'[a-f0-9]{40}',tag) or not isinstance(documents,list) or not 1<=len(documents)<=128:
        raise Blocked('rollout_compile_input_invalid')
    result=copy.deepcopy(documents);templates={};used_images={};inventory=set();hashes={}
    def read(relative):
        p=Path(relative)
        if p.is_absolute() or '..' in p.parts or not str(p).startswith(('deploy/','scripts/','src/backend/migrations/','src/backend/auth/src/main/resources/db/migration/','docker/clickhouse/init/')):
            raise Blocked('rollout_compile_source_invalid')
        path=source/p
        for parent in (path.parent,*path.parent.parents):
            if parent.is_symlink():raise Blocked('rollout_compile_source_symlink')
            if parent==source:break
        fd=os.open(path,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK)
        try:
            s=os.fstat(fd)
            if not stat.S_ISREG(s.st_mode) or not 1<=s.st_size<=2<<20:raise Blocked('rollout_compile_source_size')
            raw=os.read(fd,s.st_size+1)
            if len(raw)!=s.st_size:raise Blocked('rollout_compile_source_changed')
        finally:os.close(fd)
        hashes[str(p).replace('\\','/')]=hashlib.sha256(raw).hexdigest()
        return raw
    for path in source_paths:read(path)
    for row in result:
        kind=row.get('kind');metadata=row.get('metadata',{});name=metadata.get('name','')
        if metadata.get('namespace')!=guard.NS or kind not in ('Deployment','Service','NetworkPolicy','ConfigMap','Ingress') or not re.fullmatch(r'voice-[a-z0-9-]{1,100}',name):
            raise Blocked('rollout_compile_resource_invalid')
        if name.startswith('voice-nats') or (kind,name) in inventory:
            raise Blocked('rollout_compile_reserved_resource')
        inventory.add((kind,name))
        if mode=='images-only' and kind!='Deployment':raise Blocked('rollout_images_only_resource_invalid')
        if kind=='Deployment':
            if name not in enrolled:raise Blocked('rollout_compile_unenrolled_deployment')
            template=row['spec']['template']
            for container in template['spec'].get('initContainers',[])+template['spec']['containers']:
                key=name+'/'+container['name'];image=images.get(key,'')
                if not re.fullmatch(r'[a-z0-9][a-zA-Z0-9._/:~-]{1,255}@sha256:[a-f0-9]{64}',image):
                    raise Blocked('rollout_compile_image_unpinned')
                container['image']=image;used_images[key]=image
            templates[name]=digest(template)
    if not templates or used_images!=images:raise Blocked('rollout_compile_image_inventory_changed')
    plan=[]
    if mode!='images-only':
        for database in migrations.DATABASES:
            relative=Path('src/backend/migrations')/(database+'_db');directory=source/relative
            if not directory.is_dir() or directory.is_symlink():raise Blocked('rollout_compile_migrations_missing')
            files={}
            entries=list(directory.iterdir())
            if len(entries)>256:raise Blocked('rollout_compile_migration_bound')
            for path in sorted(entries):
                if path.suffix=='.sql':files[path.name]=read(relative/path.name).decode('utf-8')
            plan.append({'database':database,'image':migrations.IMAGE,'secret_key':database.upper()+'_DATABASE_URL',
                         'files':files,'hashes':{name:hashlib.sha256(text.encode()).hexdigest() for name,text in files.items()}})
        relative=Path('src/backend/auth/src/main/resources/db/migration');directory=source/relative
        if not directory.is_dir() or directory.is_symlink():raise Blocked('rollout_compile_auth_migrations_missing')
        entries=list(directory.iterdir())
        if len(entries)>256:raise Blocked('rollout_compile_migration_bound')
        files={p.name:read(relative/p.name).decode('utf-8') for p in sorted(entries) if p.suffix=='.sql'}
        plan.append({'database':'auth','image':migrations.FLYWAY_IMAGE,'secret_key':'POSTGRES_PASSWORD',
                     'files':files,'hashes':{name:hashlib.sha256(text.encode()).hexdigest() for name,text in files.items()}})
    migrations.validate_plan(plan,mode)
    return {'target':{'registry':registry,'tag':tag,'mode':mode,'changed_services':sorted(set(changed)),
                     'source_hashes':hashes,'template_hashes':templates,'images':used_images,
                     'manifest_sha256':digest(result),'migration_sha256':digest(plan)},
            'manifests':result,'migrations':plan,'contract':copy.deepcopy(contract)}

if __name__=='__main__':
    try:main(sys.argv[1:])
    except Exception:
        print('NATS_ROLLOUT_COMPILE=BLOCKED',file=sys.stderr);sys.exit(1)
