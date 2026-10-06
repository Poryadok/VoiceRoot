"""Fixed additive contract plan; performs no API calls or shell execution.

Root caller owns approved source custody and server-normalized consumer config.
This plan cannot authorize mutation: isolated actor authentication, pre-change
closed cut/off-node verification, exact journal and post-INFO remain required.
"""
import copy
import hashlib
import json
from pathlib import Path
import re

SCHEMA='voice-nats-additive-contract-v1'
DECLARATION={'name':'rt_realtime1_friend_removed','durable_name':'rt_realtime1_friend_removed','filter_subject':'social.friend_removed','deliver_subject':'_INBOX.voice.realtime1.friend_removed','deliver_policy':'new','ack_policy':'explicit'}
class ContractError(ValueError): pass
def fail(): raise ContractError('nats_additive_contract_unsupported')
def digest(value): return hashlib.sha256(json.dumps(value,sort_keys=True,separators=(',',':'),allow_nan=False).encode()).hexdigest()
def subject_overlap(left,right):
    def tokens(subject):
        if not isinstance(subject,str) or not subject or len(subject)>1024:fail()
        row=subject.split('.')
        if any(not token or any(c.isspace() for c in token) or ('*' in token and token!='*') or ('>' in token and (token!='>' or i!=len(row)-1)) for i,token in enumerate(row)):fail()
        return row
    a,b=tokens(left),tokens(right)
    for x,y in zip(a,b):
        if x=='>' or y=='>':return True
        if x!='*' and y!='*' and x!=y:return False
    return len(a)==len(b)
def compile_plan(source,streams,consumer,normalized_consumer):
    sources={}; texts={}
    for role in ('realtime','analytics-chat'):
        path='deploy/templates/nats-'+role+'-bootstrap.yaml'
        raw=(Path(source)/path).read_bytes()
        if len(raw)>65536:fail()
        texts[role]=raw.decode('utf8');sources[path]=hashlib.sha256(raw).hexdigest()
    chat=[];social=[]
    for role,text in texts.items():
        rows=re.findall(r'^    stream chat_events (.+)$',text,re.M)
        if len(rows)!=1:fail()
        subjects=rows[0].split()
        if len(set(subjects))!=len(subjects):fail()
        if chat and set(chat)!=set(subjects):fail()
        chat=subjects
    rows=re.findall(r'^    stream_with_duplicate_window social_events 86400000000000 (.+)$',texts['realtime'],re.M)
    if len(rows)!=1:fail()
    social=rows[0].split()
    additions={'space.deletion_scheduled','space.restored'}
    if not additions<=set(chat) or len(set(social))!=len(social):fail()
    # streams is the complete account discovery, including dynamic resources.
    # Known conflicts must veto while clients still run, never at UPDATE.
    for name,info in streams.items():
        config=info.get('config') if isinstance(info,dict) else None
        if not isinstance(config,dict) or config.get('name')!=name or not isinstance(config.get('subjects'),list):fail()
        if name!='chat_events' and any(subject_overlap(added,subject) for added in additions for subject in config['subjects']):fail()
    line='    consumer social_events rt_realtime1_friend_removed social.friend_removed _INBOX.voice.realtime1.friend_removed'
    if texts['realtime'].splitlines().count(line)!=1:fail()
    if not isinstance(normalized_consumer,dict) or any(normalized_consumer.get(k)!=v for k,v in DECLARATION.items()):fail()
    # Defaults must come from the isolated pinned server, not caller-selected
    # arbitrary desired state. Root binds their full hash in its authority.
    actions=[];observed={}
    for name,expected in [('chat_events',chat),('social_events',social)]:
        info=streams.get(name); config=info.get('config') if isinstance(info,dict) else None
        if not isinstance(config,dict) or config.get('name')!=name:fail()
        if (config.get('storage')!='file' or config.get('retention')!='limits' or config.get('max_age')!=604800000000000):fail()
        actual=config.get('subjects'); require_sets=[set(expected)]
        if name=='chat_events':require_sets.append(set(expected)-additions)
        if not isinstance(actual,list) or len(actual)!=len(set(actual)) or set(actual) not in require_sets:fail()
        if name=='social_events' and config.get('duplicate_window') not in (120000000000,86400000000000):fail()
        before=copy.deepcopy(config);after=copy.deepcopy(config)
        if name=='chat_events' and set(actual)!=set(expected):after['subjects']=list(actual)+[x for x in expected if x not in actual]
        if name=='social_events':after['duplicate_window']=86400000000000
        # Complete mutable delivery/record state belongs to the subsequent
        # authoritative cold cut, rather than the pre-fence config intent.
        observed[name]={'config_sha256':digest(before)}
        if before!=after:actions.append({'api':'$JS.API.STREAM.UPDATE.'+name,'object':name,'before':before,'after':after})
    if consumer is not None:
        if consumer.get('config')!=normalized_consumer:fail()
    else:
        actions.append({'api':'$JS.API.CONSUMER.CREATE.social_events.rt_realtime1_friend_removed','object':'social_events/rt_realtime1_friend_removed','before':None,'after':{'stream_name':'social_events','action':'create','config':copy.deepcopy(DECLARATION)},'normalized_config_sha256':digest(normalized_consumer)})
    return {'schema':SCHEMA,'migration_id':'space-chat-social-24h-friend-removed-v1','sources':sources,'observed':observed,'normalized_consumer_sha256':digest(normalized_consumer),'actions':actions}


def execute(plan,actor,journal):
    """Fixed adapter seam. journal must durably commit each event before return.

    Root owns immutable plan binding and authenticated isolated actor; this is
    not a callable CLI or generic privileged mutation interface.
    """
    if plan.get('schema')!=SCHEMA or plan.get('migration_id')!='space-chat-social-24h-friend-removed-v1':fail()
    fixed={'$JS.API.STREAM.UPDATE.chat_events':'chat_events','$JS.API.STREAM.UPDATE.social_events':'social_events','$JS.API.CONSUMER.CREATE.social_events.rt_realtime1_friend_removed':'social_events/rt_realtime1_friend_removed'}
    seen=set();receipt=[]
    for action in plan['actions']:
        api=action['api'];obj=action['object']
        if fixed.get(api)!=obj or api in seen:fail()
        seen.add(api);actor.assert_closed_isolation()
        current=actor.info_config(obj)
        expected_hash=action.get('normalized_config_sha256',digest(action['after']))
        if current is not None and digest(current)==expected_hash:
            journal({'kind':'nats_contract_applied','object':obj,'config_sha256':expected_hash,'recovered_exact_target':True})
            receipt.append({'object':obj,'config_sha256':expected_hash});continue
        if current!=action['before']:fail()
        journal({'kind':'nats_contract_intent','object':obj,'api':api,'before_sha256':digest(current),'target_sha256':expected_hash})
        actor.assert_closed_isolation()
        if actor.info_config(obj)!=current:fail()
        journal({'kind':'nats_contract_mutation_issued','object':obj,'api':api,'target_sha256':expected_hash})
        actor.mutate(api,copy.deepcopy(action['after']))
        actor.assert_closed_isolation()
        after=actor.info_config(obj)
        if after is None or digest(after)!=expected_hash:fail()
        journal({'kind':'nats_contract_applied','object':obj,'config_sha256':expected_hash,'recovered_exact_target':False})
        receipt.append({'object':obj,'config_sha256':expected_hash})
    return receipt

def verify_census(plan,before,after):
    """Allow the fixed config delta; preserve every previously captured state.

    Both inputs must be validated complete rollout_census output. Native record
    byte identity is a separate mandatory root proof, not established here.
    """
    from rollout_census import semantic
    before,after=semantic(before),semantic(after)
    if (plan.get('schema')!=SCHEMA or before.get('schema')!='voice-nats-census-v1'
        or after.get('schema')!=before['schema'] or after.get('account')!=before.get('account')):fail()
    def indexed(rows,key):
        result={key(row):row for row in rows}
        if len(result)!=len(rows):fail()
        return result
    old_streams=indexed(before['streams'],lambda row:row['name'])
    new_streams=indexed(after['streams'],lambda row:row['name'])
    if set(old_streams)!=set(new_streams):fail()
    old_consumers=indexed(before['consumers'],lambda row:(row['stream'],row['name']))
    new_consumers=indexed(after['consumers'],lambda row:(row['stream'],row['name']))
    created=('social_events',DECLARATION['name'])
    create_actions=[a for a in plan['actions'] if a['api']=='$JS.API.CONSUMER.CREATE.social_events.rt_realtime1_friend_removed']
    if len(create_actions)>1:fail()
    expected_keys=set(old_consumers)
    if create_actions:
        if created in expected_keys:fail()
        expected_keys.add(created)
    if set(new_consumers)!=expected_keys:fail()
    for key,row in old_consumers.items():
        if new_consumers[key]!=row:fail()
    if create_actions:
        row=new_consumers[created]
        initial={'consumer_seq':0,'stream_seq':old_streams['social_events']['state']['last_seq']}
        if (row.get('durable')!=DECLARATION['name'] or row.get('config_sha256')!=plan['normalized_consumer_sha256']
            or any(row.get(key)!=0 for key in ('num_ack_pending','num_redelivered','num_pending'))
            or row.get('delivered')!=initial or row.get('ack_floor')!={'consumer_seq':0,'stream_seq':0}):fail()
    changes={a['object']:a for a in plan['actions'] if a['api'] in ('$JS.API.STREAM.UPDATE.chat_events','$JS.API.STREAM.UPDATE.social_events')}
    for name,row in old_streams.items():
        expected=copy.deepcopy(row)
        if name in plan['observed'] and row['config_sha256']!=plan['observed'][name]['config_sha256']:fail()
        if name in changes:
            action=changes[name]
            if row['config_sha256']!=digest(action['before']):fail()
            expected['config_sha256']=digest(action['after'])
        if name=='social_events' and create_actions:expected['state']['consumer_count']+=1
        if new_streams[name]!=expected:fail()
    return {'schema':'voice-nats-additive-census-proof-v1','migration_id':plan['migration_id'],
        'before_sha256':digest(before),'after_sha256':digest(after),
        'messages':sum(row['state']['messages'] for row in new_streams.values()),
        'old_consumers_preserved':len(old_consumers),'created_consumers':len(create_actions)}
