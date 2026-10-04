import tempfile
import unittest
from pathlib import Path
from unittest.mock import Mock, patch
from controller import Blocked
import scenario


class RecoveryScenarioTests(unittest.TestCase):
    def zero_census(self):
        return {'schema':'known-nats-census-v1',
                'streams':[{'name':str(i),'config_sha256':'s'+str(i),'messages':0,'last_seq':0} for i in range(15)],
                'consumers':[dict(stream=str(i),name=str(i),config_sha256='c'+str(i),
                   **dict.fromkeys(('ack_consumer','ack_stream','delivered_consumer','delivered_stream','ack_pending','pending'),0)) for i in range(42)]}

    def test_existing_baseline_is_not_bootstrapped_and_reference_is_hash_bound(self):
        with tempfile.TemporaryDirectory() as d:
            base=Path(d);store=base/'existing';store.mkdir();(store/'block').write_bytes(b'preserve')
            runtime=Mock(base=base);guard=Mock();row=self.zero_census()
            with patch('scenario.ready'),patch('scenario.census',return_value=row),patch('scenario.closed_backup',return_value=('restored',{'messages':0})):
                receipt=scenario.recover_staging_baseline(runtime,store,{'archive_sha256':'a'*64},guard)
            runtime.bootstrap.assert_not_called();runtime.allow_new_store.assert_not_called()
            runtime.restore.assert_called_once()
            self.assertEqual(runtime.restore.call_args.args[0],base/'fixture.tar')
            guard.assert_called_once();self.assertEqual(receipt['closed_census'],row)
            self.assertEqual((store/'block').read_bytes(),b'preserve')

    def test_changed_configuration_or_nonzero_state_never_archived(self):
        for change in ('config','messages'):
            with self.subTest(change=change),tempfile.TemporaryDirectory() as d:
                runtime=Mock(base=Path(d));store=Path(d)/'store';store.mkdir();before=self.zero_census();after=self.zero_census()
                if change=='config':after['consumers'][0]['config_sha256']='changed'
                else:after['streams'][0]['messages']=1
                with patch('scenario.ready'),patch('scenario.census',side_effect=[before,after]),patch('scenario.closed_backup',return_value=('restored',{})) as backup:
                    with self.assertRaises(Blocked):scenario.recover_staging_baseline(runtime,store,{},Mock())
                    backup.assert_not_called()


if __name__=='__main__':unittest.main()
