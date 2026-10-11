"""Root-only fixed migration authority from approved passive source + live INFO.

No arbitrary caller plan, seed, publication or live API mutation is accepted.
"""
import hashlib,re
from pathlib import Path
from bootstrap_selection import select
from bootstrap_root import secret_bytes
from controller import Blocked
from docker_runtime import NATS_IMAGE
from nats_contract_plan import compile_plan,digest
from rollout_census import census
from stage_runtime import HUB

TARGET_FILES={'realtime':'e466b67d6fdf66e816fe0ffbd7e91ad59be96f79b735977d6d5a77f819fbf23d','analytics-chat':'2c3befbb0817f55c61f781215625b21e887ab52c5abbb758dda4073c7f0d0d06'}
def fail():raise Blocked('nats_migration_root_preflight_refused')
def monitor(kube):
    hub=kube.get('deployment',HUB)
    pods=kube.run(['get','pods','-l','app='+HUB,'-o','json'])['items']
    if len(pods)!=1:fail()
    pod=pods[0];name=pod['metadata']['name']
    if not re.fullmatch(HUB+'-[a-z0-9-]+',name) or pod['status'].get('phase')!='Running' or not any(c.get('type')=='Ready' and c.get('status')=='True' for c in pod['status'].get('conditions',[])):fail()
    owners=pod['metadata'].get('ownerReferences',[])
    if len(owners)!=1 or owners[0].get('kind')!='ReplicaSet':fail()
    rs=kube.get('replicaset',owners[0]['name'])
    if rs['metadata']['uid']!=owners[0]['uid'] or not any(o.get('uid')==hub['metadata']['uid'] and o.get('kind')=='Deployment' for o in rs['metadata'].get('ownerReferences',[])):fail()
    url='/api/v1/namespaces/voice-staging/pods/'+name+':8222/proxy/jsz?accounts=true&streams=true&consumers=true&config=true&limit=2048'
    tree=kube.run(['get','--raw',url])
    if kube.get('deployment',HUB)!=hub:fail()
    return tree
def preflight(kube,source,enrollment,contract,decoder):
    return _from_tree(kube,source,enrollment,contract,decoder,monitor(kube))

def closed_copy_preflight(kube,source,enrollment,contract,decoder,runtime,broker,tree):
    """Root recovery callback: actual owned restored broker snapshot only."""
    owned=runtime.inspect(broker)
    if (owned['Config']['Image']!=NATS_IMAGE or owned['HostConfig']['NetworkMode']!='none'
        or runtime.owned[broker]['id']!=owned['Id']):fail()
    generation=enrollment['binding']['generation']
    account=secret_bytes(kube.get('secret','voice-nats-operator-'+generation),'account.public').decode().strip()
    if runtime.account_id()!=account:fail()
    result=_from_tree(kube,source,enrollment,contract,decoder,tree)
    if runtime.inspect(broker)['Id']!=owned['Id']:fail()
    return result

def _from_tree(kube,source,enrollment,contract,decoder,tree):
    generation=enrollment['binding']['generation'];select(kube,enrollment,generation)
    for role,wanted in TARGET_FILES.items():
        path=Path(source)/('deploy/templates/nats-'+role+'-bootstrap.yaml')
        if path.is_symlink() or hashlib.sha256(path.read_bytes()).hexdigest()!=wanted:fail()
    account=secret_bytes(kube.get('secret','voice-nats-operator-'+generation),'account.public').decode().strip()
    class Runtime:
        def monitor_jsz(self,broker):return tree
    row=census(Runtime(),'root-live-monitor',account)
    detail=next(a for a in tree['account_details'] if a['id']==account)['stream_detail']
    streams={s['name']:{'config':s['config'],'state':s['state'],'consumer_detail':s.get('consumer_detail',[])} for s in detail}
    current={}
    for stream in detail:
        for consumer in stream.get('consumer_detail',[]):
            key=(stream['name'],consumer.get('durable') or consumer['name'])
            if key in current:fail()
            current[key]=consumer
    from bootstrap_auth import prove
    def unchanged():select(kube,enrollment,generation)
    live_auth=prove(secret_bytes(kube.get('secret',enrollment['secret']['name']),'bootstrap.creds'),kube.get('secret','voice-nats-operator-'+generation),unchanged)
    if live_auth!=enrollment['auth_proof']:fail()
    normalized={(row['stream'],row['durable']):row['config'] for row in live_auth['normalized_consumers']}
    plan=compile_plan(source,streams,current,normalized)
    scripts={}
    for old in contract['scripts']:
        part=old['part']
        if part not in TARGET_FILES:continue
        text=(Path(source)/('deploy/templates/nats-'+part+'-bootstrap.yaml')).read_text(encoding='utf8').replace('__NAMESPACE__','voice-staging').replace('__K_NAMESPACE__','voice-staging')
        cm=decoder(re.split(r'^---\s*$',text,flags=re.MULTILINE)[0].encode())
        if cm['kind']!='ConfigMap' or cm['metadata']['name']!='voice-nats-'+part+'-bootstrap':fail()
        script=cm['data']['bootstrap.sh'];scripts[part]={'sha256':hashlib.sha256(script.encode()).hexdigest(),'script':script}
    return {'plan':plan,'binding':{'plan_sha256':digest(plan),'server_image':NATS_IMAGE,'enrollment_sha256':digest(enrollment),'sources':plan['sources']},'scripts':scripts,'live_census_sha256':digest(row)}
