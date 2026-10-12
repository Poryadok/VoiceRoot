import unittest

from nats_contract_integration import fixture_bootstrap_grants
from nats_contract_plan import DECLARATIONS


class Tests(unittest.TestCase):
    def test_synthetic_renewal_actor_has_baseline_and_only_exact_v3_consumer_api_rights(self):
        baseline=[['social_events','rt_realtime1_friend_removed'],['chat_events','existing_chat'],['role_events','existing_role']]
        grants=fixture_bootstrap_grants({'streams':['chat_events','social_events','role_events'],'consumer_pairs':baseline})
        expected_pairs={tuple(pair) for pair in baseline}|{(stream,config['durable_name']) for stream,config in DECLARATIONS}
        expected={prefix+stream+'.'+durable for stream,durable in expected_pairs
            for prefix in ('$JS.API.CONSUMER.INFO.','$JS.API.CONSUMER.CREATE.')}
        actual={grant for grant in grants if grant.startswith('$JS.API.CONSUMER.')}
        self.assertEqual(actual,expected)
        self.assertEqual({grant for grant in grants if grant.startswith('$JS.API.STREAM.UPDATE.')},
            {'$JS.API.STREAM.UPDATE.chat_events','$JS.API.STREAM.UPDATE.social_events','$JS.API.STREAM.UPDATE.role_events'})
        self.assertEqual(len(grants),len(set(grants)))
        self.assertFalse(any('*' in grant or '>' in grant for grant in grants))

    def test_fixture_grants_reject_duplicate_streams(self):
        with self.assertRaises(ValueError):
            fixture_bootstrap_grants({'streams':['chat_events','chat_events'],'consumer_pairs':[]})


if __name__=='__main__':
    unittest.main()
