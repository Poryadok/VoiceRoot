"""Strict full DURABLE successor under the admitted no-input HUB interval.

Source contract nats-server v2.12.12 consumer.checkPending/hasMaxDeliveries and
consumerFileStore.UpdateDelivered. Caller must bind actual kernel decode,
writer/image, original records/config, CLOSED bytes and enforced input isolation.
This result alone is not a seal, capability, live queue export or ACK evidence.
"""
import copy
import hashlib
import re

class DurableError(ValueError):pass

def veto():raise DurableError('closed_durable_successor_invalid')

def integer(value,minimum=0):
    if type(value) is not int or not minimum<=value<=2**63-1:veto()
    return value

def state_shape(row):
    if (type(row) is not dict or set(row)!={'schema','version','ack_floor','delivered','pending','redelivered'}
        or row['schema']!='nats-closed-durable-consumer-v1' or row['version']!=2):veto()
    for field in ('ack_floor','delivered'):
        value=row[field]
        if type(value) is not dict or set(value)!={'consumer','stream'}:veto()
        for number in value.values():integer(number)
    for role in ('consumer','stream'):
        if row['ack_floor'][role]>row['delivered'][role]:veto()
    for field in ('pending','redelivered'):
        if type(row[field]) is not dict or len(row[field])>16384:veto()
        for key,value in row[field].items():
            if type(key) is not str or len(key)>19 or not key.isascii() or not key.isdecimal() or str(int(key))!=key:veto()
            seq=integer(int(key),1)
            if not row['ack_floor']['stream']<seq<=row['delivered']['stream']:veto()
            if field=='redelivered':integer(value,1)
            else:
                if type(value) is not dict or set(value)!={'consumer_sequence','timestamp_ns'}:veto()
                cseq=integer(value['consumer_sequence'],1);integer(value['timestamp_ns'])
                if not row['ack_floor']['consumer']<cseq<=row['delivered']['consumer'] or value['timestamp_ns']%1000000000:veto()

def successor(before,after,config,interval,record_sequences):
    state_shape(before);state_shape(after)
    if (type(interval) is not dict or set(interval)!={'start_ns','stop_ns','closed_inputs_verified','writer'}
        or interval['closed_inputs_verified'] is not True or interval['writer']!='nats-server-v2.12.12'):veto()
    start=integer(interval['start_ns'],1);stop=integer(interval['stop_ns'],1)
    if start>stop or stop-start>600000000000:veto()
    if type(record_sequences) is not set or any(type(n) is not int or n<=0 for n in record_sequences):veto()
    if any(int(key) not in record_sequences for key in before['pending']):veto()
    # Config is a bounded projection from unchanged admitted full config.
    if (type(config) is not dict or set(config)!={'ack_policy','ack_wait','max_deliver','backoff'}
        or config['ack_policy'] not in ('explicit','all','none')
        or type(config['max_deliver']) is not int or not -1<=config['max_deliver']<=2**31-1
        or type(config['backoff']) is not list or len(config['backoff'])>256):veto()
    integer(config['ack_wait'],1)
    for delay in config['backoff']:integer(delay,1)
    if before==after:return {'schema':'voice-closed-durable-successor-v1','branch':'EXACT','removed':[], 'application_ack_observed':False}
    expected=copy.deepcopy(before);removed=[]
    # checkPending can expire due records, but does not persist the live rdq.
    # The only admitted removal is hasMaxDeliveries -> UpdateDelivered. That
    # file-store call keeps durable floors/delivered and all remaining maps.
    for key,pending in before['pending'].items():
        if key in after['pending']:continue
        dc=before['redelivered'].get(key,1)
        deadline=config['ack_wait']
        if config['backoff']:deadline=config['backoff'][min(before['redelivered'].get(key,0),len(config['backoff'])-1)]
        if (config['ack_policy']=='none' or config['max_deliver']<=0 or dc<config['max_deliver']
            or stop-pending['timestamp_ns']<deadline):veto()
        del expected['pending'][key]
        expected['redelivered'][key]=max(expected['redelivered'].get(key,0),dc)
        removed.append({'stream_sequence':int(key),'consumer_sequence':pending['consumer_sequence'],'prior_redelivery_count':dc})
    if not removed or expected!=after:veto()
    return {'schema':'voice-closed-durable-successor-v1','branch':'PINNED_PENDING_MAXDELIVER',
        'removed':sorted(removed,key=lambda row:row['stream_sequence']),'application_ack_observed':False}

def verify_inventory(before,after,read_bytes,decode,configs,record_sequences,interval,metadata_pair):
    """Full CLOSED inventory composition; supplied readers are root adapters.

    The caller first verifies both immutable archives and full records/config
    proofs. Every o.dat is decoded, not just files which changed. Metadata pair
    verification uses the pinned checksum classifier, never a filename waiver.
    No input in this function is a successful runtime seal or mutation right.
    """
    def inventory(manifest):
        if type(manifest) is not dict or not {'mechanism','dirs','files','file_count','bytes'}<=set(manifest):veto()
        rows={}
        if type(manifest['files']) is not list or len(manifest['files'])>100000:veto()
        total=0
        for row in manifest['files']:
            if (type(row) is not dict or set(row)!={'path','size','sha256'}
                or type(row['path']) is not str or not row['path'] or len(row['path'])>4096
                or row['path'].startswith('/') or any(part in ('','..','.') for part in row['path'].split('/'))
                or row['path'] in rows or type(row['sha256']) is not str
                or not re.fullmatch('[a-f0-9]{64}',row['sha256'])):veto()
            integer(row['size']);total+=row['size'];rows[row['path']]=row
        if total!=manifest['bytes'] or len(rows)!=manifest['file_count']:veto()
        return rows
    old,new=inventory(before),inventory(after)
    if (before['mechanism']!=after['mechanism'] or before['dirs']!=after['dirs'] or set(old)!=set(new)
        or type(configs) is not dict or type(record_sequences) is not dict):veto()
    state_paths={path for path in old if path.endswith('/o.dat')}
    if set(configs)!=state_paths or set(record_sequences)!=state_paths:veto()
    def raw(side,path):
        row=(old if side=='before' else new)[path]
        data=read_bytes(side,path)
        if type(data) is not bytes or len(data)!=row['size'] or hashlib.sha256(data).hexdigest()!=row['sha256']:veto()
        return data
    dispositions={};changed_metadata=set()
    for path in sorted(old):
        if path in state_paths:
            dispositions[path]=successor(decode(raw('before',path)),decode(raw('after',path)),
                configs[path],interval,record_sequences[path])
        elif old[path]!=new[path]:
            if not path.endswith(('/meta.inf','/meta.sum')):veto()
            parent=path.rsplit('/',1)[0]
            if '/obs/' not in parent:veto()
            changed_metadata.add(parent)
    for parent in sorted(changed_metadata):
        inf,sum_path=parent+'/meta.inf',parent+'/meta.sum'
        if inf not in old or sum_path not in old:veto()
        # The verifier receives actual byte-bound old/new identity+checksums.
        if metadata_pair(parent,raw('before',inf),raw('before',sum_path),
            raw('after',inf),raw('after',sum_path),interval) is not True:veto()
    return {'schema':'voice-closed-native-verification-v1','consumer_states':dispositions,
        'metadata_rewrites':sorted(changed_metadata),'all_other_native_bytes_identical':True,
        'application_ack_observed':False}
