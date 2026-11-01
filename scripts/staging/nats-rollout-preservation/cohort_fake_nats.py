"""Synthetic NATS transport for composition, not broker or P2 acceptance proof."""
import copy


class ScratchBroker:
    def __init__(self, base, operation, account):
        self.base=base;self.operation=operation;self.account=account
        self.owned={};self.streams={};self.consumers={}

    def start_broker(self, name, store):
        self.owned[name]={'id':'f'*64}
        return name

    def inspect(self, name):
        return {'Id':self.owned[name]['id']}

    def run(self, args, **kwargs):
        if args[0]=='rm':
            self.owned.clear()
        return b''

    def monitor_jsz(self, name):
        return {'messages':0,'streams':len(self.streams),'consumers':len(self.consumers),
            'account_details':[{'id':self.account,'stream_detail':[{'name':key} for key in self.streams]}]}

    def stop(self, name):
        pass

    def no_operation_containers(self):
        if self.owned:raise AssertionError('synthetic broker not removed')


class ScratchActor:
    def __init__(self, runtime, broker, broker_id, store, unchanged, **kwargs):
        self.runtime=runtime;self.unchanged=unchanged

    def _request(self, api, payload):
        self.unchanged()
        if not api.startswith('$JS.API.STREAM.CREATE.'):
            raise AssertionError('unexpected synthetic NATS request')
        self.runtime.streams[payload['name']]=copy.deepcopy(payload)

    def mutate(self, api, payload):
        self.unchanged()
        if api.startswith('$JS.API.STREAM.UPDATE.'):
            self.runtime.streams[payload['name']]=copy.deepcopy(payload)
        elif api.startswith('$JS.API.CONSUMER.CREATE.'):
            config=payload['config']
            self.runtime.consumers[(payload['stream_name'],config['durable_name'])]=copy.deepcopy(config)
        else:
            raise AssertionError('unexpected synthetic NATS mutation')

    def info_config(self, name):
        self.unchanged()
        if '/' in name:
            return copy.deepcopy(self.runtime.consumers[tuple(name.split('/'))])
        return copy.deepcopy(self.runtime.streams[name])
