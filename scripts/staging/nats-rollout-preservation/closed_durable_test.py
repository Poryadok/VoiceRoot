"""Accepted CLOSED-domain rules; not a live callback or ACK trace proof."""
import copy
import hashlib
import json
import unittest
import closed_durable as subject

class DurableSuccessorTests(unittest.TestCase):
    def fixture(self):
        state={'schema':'nats-closed-durable-consumer-v1','version':2,
            'ack_floor':{'consumer':1,'stream':1},'delivered':{'consumer':3,'stream':2},
            'pending':{'2':{'consumer_sequence':2,'timestamp_ns':1700000000000000000}},
            'redelivered':{'2':1}}
        config={'ack_policy':'explicit','ack_wait':1000000000,'max_deliver':1,'backoff':[]}
        interval={'start_ns':1700000000000000000,'stop_ns':1700000003000000000,
            'closed_inputs_verified':True,'writer':'nats-server-v2.12.12'}
        return state,config,interval

    def test_exact_and_source_typed_maxdeliver_not_application_ack(self):
        before,config,interval=self.fixture()
        self.assertEqual(subject.successor(before,before,config,interval,{2})['branch'],'EXACT')
        after=copy.deepcopy(before);after['pending']={}
        row=subject.successor(before,after,config,interval,{2})
        self.assertEqual(row['branch'],'PINNED_PENDING_MAXDELIVER')
        self.assertEqual(row['removed'],[{'stream_sequence':2,'consumer_sequence':2,'prior_redelivery_count':1}])
        self.assertFalse(row['application_ack_observed'])

    def test_unaccounted_floors_maps_record_time_and_isolation_reject(self):
        before,config,interval=self.fixture();after=copy.deepcopy(before);after['pending']={}
        cases=[]
        for path,value in [('ack_floor',{'consumer':3,'stream':2}),('delivered',{'consumer':4,'stream':2}),('redelivered',{}),('version',1)]:
            changed=copy.deepcopy(after);changed[path]=value;cases.append((changed,config,interval,{2}))
        for patch in ({'max_deliver':-1},{'max_deliver':2},{'ack_wait':10000000000},{'ack_policy':'none'},{'backoff':[10000000000]}):
            cases.append((after,{**config,**patch},interval,{2}))
        cases.extend([(after,config,{**interval,'closed_inputs_verified':False},{2}),
            (after,config,{**interval,'writer':'unknown'},{2}),
            (after,config,interval,set())])
        for args in cases:
            with self.subTest(args=args),self.assertRaises(subject.DurableError):subject.successor(before,*args)

    def test_normal_expired_pending_may_keep_durable_bytes(self):
        before,config,interval=self.fixture();config['max_deliver']=-1
        self.assertEqual(subject.successor(before,before,config,interval,{2})['branch'],'EXACT')
        after=copy.deepcopy(before);after['pending']['2']['timestamp_ns']+=1000000000
        with self.assertRaises(subject.DurableError):subject.successor(before,after,config,interval,{2})

    def test_inventory_verifies_unchanged_full_states_and_rejects_record_bytes(self):
        before,config,interval=self.fixture()
        path='jetstream/account/streams/events/obs/worker/o.dat'
        blob='jetstream/account/streams/events/msgs/1.blk'
        data={path:json.dumps(before,sort_keys=True).encode(),blob:b'protected record bytes'}
        def manifest(values):
            rows=[{'path':name,'size':len(raw),'sha256':hashlib.sha256(raw).hexdigest()} for name,raw in sorted(values.items())]
            return {'mechanism':'closed-jetstream-store-tar-v2','dirs':['jetstream'],'files':rows,'file_count':len(rows),'bytes':sum(row['size'] for row in rows)}
        decoded=[]
        def decode(raw):decoded.append(raw);return json.loads(raw)
        original=manifest(data)
        row=subject.verify_inventory(original,original,lambda side,name:data[name],decode,
            {path:config},{path:{2}},interval,lambda *args:False)
        self.assertEqual(len(decoded),2)
        self.assertTrue(row['all_other_native_bytes_identical'])
        changed={**data,blob:b'tampered record bytes'}
        with self.assertRaises(subject.DurableError):subject.verify_inventory(original,manifest(changed),
            lambda side,name:(data if side=='before' else changed)[name],decode,{path:config},{path:{2}},interval,lambda *args:False)
        with self.assertRaises(subject.DurableError):subject.verify_inventory(original,original,
            lambda side,name:b'wrong raw',decode,{path:config},{path:{2}},interval,lambda *args:False)
        with self.assertRaises(subject.DurableError):subject.verify_inventory(original,original,
            lambda side,name:data[name],decode,{}, {path:{2}},interval,lambda *args:False)

if __name__=='__main__':unittest.main()
