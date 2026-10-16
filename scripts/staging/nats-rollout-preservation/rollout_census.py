"""Private generic census of a complete pinned NATS 2.12.12 JSZ tree.

Authority: nats-server v2.12.12 server/monitor.go AccountDetail/JSInfo and
server/consumer.go ConsumerInfo. Runtime owns network isolation and JSON bounds.
"""
import copy
from datetime import datetime
import hashlib
import json
import re

SCHEMA = 'voice-nats-census-v1'
MAX_BYTES = 64 * 1024 * 1024
MAX_STREAMS = 100000
MAX_CONSUMERS = 1000000

class CensusError(ValueError):
    pass


def _fail():
    raise CensusError('nats_census_incomplete_or_invalid')


def _uint(value):
    if type(value) is not int or not 0 <= value <= 2 ** 64 - 1: _fail()
    return value


def _time(value):
    if not isinstance(value, str) or not value: _fail()
    if datetime.fromisoformat(value.replace('Z', '+00:00')).tzinfo is None: _fail()
    return value


def _digest(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(',', ':'), allow_nan=False).encode()).hexdigest()


def _state(value):
    # StreamState contains unsigned counters, message identity timestamps,
    # deleted sequence lists and optional subjects/lost maps. Keep all fields.
    if isinstance(value, dict):
        for item in value.values(): _state(item)
    elif isinstance(value, list):
        for item in value: _state(item)
    elif isinstance(value, str):
        _time(value)
    else:
        _uint(value)


def _sequence(value):
    if not isinstance(value, dict) or set(value) - {'consumer_seq', 'stream_seq', 'last_active'}: _fail()
    seq = {key: _uint(value[key]) for key in ('consumer_seq', 'stream_seq')}
    observed = _time(value['last_active']) if 'last_active' in value else None
    return seq, observed


def _consumer(row, stream):
    if set(row) - {'stream_name', 'name', 'created', 'config', 'delivered', 'ack_floor', 'num_ack_pending',
        'num_redelivered', 'num_waiting', 'num_pending', 'cluster', 'push_bound', 'paused', 'pause_remaining', 'ts', 'priority_groups'}: _fail()
    name = row['name']
    if not isinstance(name, str) or not name or row['stream_name'] != stream: _fail()
    config = row['config']
    if not isinstance(config, dict) or not config: _fail()
    durable = config.get('durable_name', '')
    if durable not in ('', name) or config.get('name', name) not in ('', name): _fail()
    if config.get('mem_storage', False) is not False: _fail()
    threshold = _uint(config.get('inactive_threshold', 0))
    if not durable and threshold == 0: _fail()
    delivered, delivered_active = _sequence(row['delivered'])
    ack, ack_active = _sequence(row['ack_floor'])
    if any(ack[key] > delivered[key] for key in ack): _fail()
    result = {'stream': stream, 'name': name, 'durable': durable, 'created': _time(row['created']),
        'config_sha256': _digest(config), 'inactive_threshold': threshold,
        'delivered': delivered, 'ack_floor': ack,
        **{key: _uint(row[key]) for key in ('num_ack_pending', 'num_redelivered', 'num_pending')}}
    observations = {key: copy.deepcopy(row[key]) for key in ('ts', 'cluster', 'push_bound', 'paused',
        'pause_remaining', 'priority_groups') if key in row}
    observations['num_waiting'] = _uint(row['num_waiting'])
    if 'ts' in row: _time(row['ts'])
    if delivered_active: observations['delivered_last_active'] = delivered_active
    if ack_active: observations['ack_floor_last_active'] = ack_active
    result['observations'] = observations
    return result


def validate_snapshot(row):
    """Validate and detach a previously canonical complete live census."""
    try:
        encoded=json.dumps(row,sort_keys=True,separators=(',',':'),allow_nan=False).encode()
        if len(encoded)>MAX_BYTES:_fail()
        if not isinstance(row, dict) or set(row) != {'schema', 'account', 'streams', 'consumers'}:
            _fail()
        account = row['account']
        if row['schema'] != SCHEMA or not isinstance(account, str) or not re.fullmatch(r'A[A-Z2-7]{55}', account):
            _fail()
        streams, consumers = row['streams'], row['consumers']
        if (not isinstance(streams, list) or not isinstance(consumers, list)
            or len(streams) > MAX_STREAMS or len(consumers) > MAX_CONSUMERS):
            _fail()
        clean_streams=[];stream_names=set();stream_counts={}
        for stream in streams:
            if not isinstance(stream, dict) or set(stream) != {'name','created','config_sha256','state','observations'}:
                _fail()
            name=stream['name']
            if (not isinstance(name,str) or not name or name in stream_names
                or not re.fullmatch(r'[a-f0-9]{64}',stream['config_sha256'])):
                _fail()
            _time(stream['created'])
            state=stream['state']
            if not isinstance(state,dict) or not {'messages','bytes','first_seq','last_seq','consumer_count'} <= set(state):
                _fail()
            for field in ('messages','bytes','first_seq','last_seq','consumer_count'):_uint(state[field])
            _state(state)
            observations=stream['observations']
            if not isinstance(observations,dict) or set(observations)-{'cluster','mirror','sources','stream_raft_group','consumer_raft_groups'}:
                _fail()
            stream_names.add(name);stream_counts[name]=state['consumer_count']
            clean_streams.append(stream)
        if [row['name'] for row in clean_streams] != sorted(stream_names): _fail()
        clean_consumers=[];consumer_keys=set();actual_counts={name:0 for name in stream_names}
        consumer_keys_allowed={'stream','name','durable','created','config_sha256','inactive_threshold',
            'delivered','ack_floor','num_ack_pending','num_redelivered','num_pending','observations'}
        observation_keys={'cluster','push_bound','paused','pause_remaining','priority_groups','num_waiting',
            'delivered_last_active','ack_floor_last_active','ts'}
        for consumer in consumers:
            if not isinstance(consumer,dict) or set(consumer)!=consumer_keys_allowed: _fail()
            stream,name,durable=consumer['stream'],consumer['name'],consumer['durable']
            if (stream not in stream_names or not isinstance(name,str) or not name
                or not isinstance(durable,str) or durable not in ('',name)
                or not re.fullmatch(r'[a-f0-9]{64}',consumer['config_sha256'])): _fail()
            _time(consumer['created']);threshold=_uint(consumer['inactive_threshold'])
            if not durable and threshold==0: _fail()
            sequences=[]
            for field in ('delivered','ack_floor'):
                value=consumer[field]
                if not isinstance(value,dict) or set(value)!={'consumer_seq','stream_seq'}: _fail()
                sequences.append({key:_uint(value[key]) for key in ('consumer_seq','stream_seq')})
            delivered,ack=sequences
            if any(ack[key]>delivered[key] for key in ack): _fail()
            for field in ('num_ack_pending','num_redelivered','num_pending'):_uint(consumer[field])
            observations=consumer['observations']
            if not isinstance(observations,dict) or set(observations)-observation_keys or 'num_waiting' not in observations: _fail()
            _uint(observations['num_waiting'])
            for field in ('delivered_last_active','ack_floor_last_active','ts'):
                if field in observations:_time(observations[field])
            key=(stream,name)
            if key in consumer_keys: _fail()
            consumer_keys.add(key);actual_counts[stream]+=1;clean_consumers.append(consumer)
        if [ (row['stream'],row['name']) for row in clean_consumers ] != sorted(consumer_keys): _fail()
        if actual_counts!=stream_counts: _fail()
        return copy.deepcopy({'schema':SCHEMA,'account':account,'streams':clean_streams,'consumers':clean_consumers})
    except Exception:
        raise CensusError('nats_census_incomplete_or_invalid') from None


def semantic(row):
    """Remove only explicit runtime observations from a private canonical cut.

    Delivery/ACK sequences, pending/redelivery counts, complete config hashes,
    creation identity and every stream-state field remain in equality. Runtime
    activity timestamps, waiting/connection/cluster observations do not.
    Ephemerals are retained; caller must establish a proof interval supported by
    the recorded inactive_threshold. Missing or expired consumers never waive
    count reconciliation or cut equality.
    """
    result = copy.deepcopy(row)
    for key in ('streams', 'consumers'):
        for item in result.get(key, []): item.pop('observations', None)
    return result


def census(runtime, broker, account_id):
    """Require the complete global account tree, never an acc-filtered page."""
    try:
        if not isinstance(account_id, str) or not re.fullmatch(r'A[A-Z2-7]{55}', account_id): _fail()
        tree = runtime.monitor_jsz(broker)
        if not isinstance(tree, dict) or tree.get('disabled') or tree.get('error'): _fail()
        if len(json.dumps(tree, allow_nan=False).encode()) > MAX_BYTES: _fail()
        accounts = tree['account_details']
        if not isinstance(accounts, list) or _uint(tree['total']) != len(accounts) or not 0 < len(accounts) <= 2048: _fail()
        ids = [account['id'] for account in accounts]
        if len(set(ids)) != len(ids) or ids.count(account_id) != 1: _fail()
        streams, consumers, stream_names, consumer_keys = [], [], set(), set()
        for account in accounts:
            details = account.get('stream_detail', [])
            if not isinstance(details, list): _fail()
            if account['id'] != account_id:
                if details or any(_uint(account.get(key, 0)) for key in ('memory', 'storage')): _fail()
                continue
            for row in details:
                name, config, state = row['name'], row['config'], row['state']
                if (not isinstance(name, str) or not name or name in stream_names or not isinstance(config, dict)
                        or config.get('name') != name or config.get('storage') != 'file'
                        or not isinstance(state, dict) or row.get('direct_consumer_detail')): _fail()
                stream_names.add(name)
                _state(state)
                for key in ('messages', 'bytes', 'first_seq', 'last_seq', 'consumer_count'): _uint(state[key])
                consumer_rows = row.get('consumer_detail', [])
                if not isinstance(consumer_rows, list) or state['consumer_count'] != len(consumer_rows): _fail()
                observations = {key: copy.deepcopy(row[key]) for key in ('cluster', 'mirror', 'sources', 'stream_raft_group',
                    'consumer_raft_groups') if key in row}
                streams.append({'name': name, 'created': _time(row['created']), 'config_sha256': _digest(config),
                    'state': copy.deepcopy(state), 'observations': observations})
                for consumer in consumer_rows:
                    captured = _consumer(consumer, name)
                    key = name, captured['name']
                    if key in consumer_keys: _fail()
                    consumer_keys.add(key)
                    consumers.append(captured)
                if len(streams) > MAX_STREAMS or len(consumers) > MAX_CONSUMERS: _fail()
        if (_uint(tree['streams']) != len(streams) or _uint(tree['consumers']) != len(consumers)
                or _uint(tree['messages']) != sum(row['state']['messages'] for row in streams)
                or _uint(tree['bytes']) != sum(row['state']['bytes'] for row in streams)): _fail()
        return {'schema': SCHEMA, 'account': account_id,
            'streams': sorted(streams, key=lambda row: row['name']),
            'consumers': sorted(consumers, key=lambda row: (row['stream'], row['name']))}
    except Exception:
        raise CensusError('nats_census_incomplete_or_invalid') from None
