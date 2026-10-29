import copy
import os
import subprocess
from types import SimpleNamespace
import unittest
from unittest.mock import patch

import realtime_config_transition_redis as redis


class Client:
    PREFIX = ['kubectl','--namespace','voice-dev']
    def __init__(self):
        def meta(name): return {'name':name,'namespace':'voice-dev','uid':name+'-uid','resourceVersion':'1'}
        self.data = {
            ('service','voice-redis'):{'kind':'Service','metadata':meta('voice-redis'),'spec':{'type':'ClusterIP','clusterIP':'10.0.0.1','selector':{'app':'voice-redis'},'ports':[{'port':6379,'targetPort':6379}]}},
            ('endpoints','voice-redis'):{'kind':'Endpoints','metadata':meta('voice-redis'),'subsets':[{'addresses':[{'ip':'10.0.0.2','targetRef':{'kind':'Pod','name':'redis-0','uid':'redis-0-uid'}}],'ports':[{'port':6379}]}]},
            ('pod','redis-0'):{'kind':'Pod','metadata':meta('redis-0') | {'labels':{'app':'voice-redis'}},'status':{'podIP':'10.0.0.2','containerStatuses':[{'name':'redis','ready':True,'containerID':'containerd://fixture','restartCount':0}]}},
            ('secret','voice-p3-principal-replay'):{'kind':'Secret','metadata':meta('voice-p3-principal-replay'),'type':'Opaque','data':{'password':''}}}
    def call(self,verb,kind,name,*args): return copy.deepcopy(self.data[kind,name])


class RedisTest(unittest.TestCase):
    def setUp(self):
        self.client = Client(); self.fence = SimpleNamespace(ledger=SimpleNamespace(client=self.client))
        self.patches = [patch.object(redis.frontend_runtime_executor,'KubectlTransport',Client),
                        patch.object(redis.media_operation_guard,'verify')]
        for item in self.patches: item.start(); self.addCleanup(item.stop)

    def test_fixed_noauth_and_acl_observation(self):
        with patch.object(redis.metadata,'linux_private_bytes',side_effect=[b'PONG\n',b'default\n',b'OK\n',b'OK\n']) as run:
            result = redis.observe(self.fence,self.client,{},'ns')
        self.assertEqual(result['scope'],'redis-noauth-ping-and-sampled-acl-only')
        self.assertEqual(run.call_count,4)
        self.assertNotIn('password',str(result))
        command = run.call_args_list[0].args[0]
        suffix = command[command.index('--')+1:]
        self.assertEqual(suffix[:4],['env','-u','REDISCLI_AUTH','redis-cli'])
        result = subprocess.run(suffix[:3]+['python3','-c',
            'import os,sys;sys.exit(1 if "REDISCLI_AUTH" in os.environ else 0)'],
            env=dict(os.environ,REDISCLI_AUTH='private-fixture'),capture_output=True,check=False)
        self.assertEqual(result.returncode,0); self.assertNotIn(b'private-fixture',result.stdout+result.stderr)

    def test_credentials_endpoint_acl_and_drift_fail_closed(self):
        for fault in ('credential','endpoint','acl','drift'):
            with self.subTest(fault=fault):
                self.client = Client(); self.fence = SimpleNamespace(ledger=SimpleNamespace(client=self.client))
                if fault == 'credential': self.client.data['secret','voice-p3-principal-replay']['data']['password'] = 'c2VjcmV0'
                if fault == 'endpoint': self.client.data['endpoints','voice-redis']['subsets'][0]['addresses'][0]['targetRef']['uid'] = 'foreign'
                calls = 0
                def run(*args):
                    nonlocal calls
                    calls += 1
                    if fault == 'drift' and calls == 4: self.client.data['service','voice-redis']['metadata']['resourceVersion'] = '2'
                    return [b'PONG',b'default',b'ERR' if fault == 'acl' else b'OK',b'OK'][calls-1]
                with patch.object(redis.metadata,'linux_private_bytes',side_effect=run):
                    with self.assertRaises(redis.RedisObservationError) as error: redis.observe(self.fence,self.client,{},'ns')
                self.assertNotIn('secret',str(error.exception))


if __name__ == '__main__': unittest.main()
