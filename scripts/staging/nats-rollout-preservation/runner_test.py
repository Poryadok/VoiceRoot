import hashlib
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch
import runner
from controller import Blocked


class BootstrapAuthorityTests(unittest.TestCase):
    def test_authenticated_target_bootstrap_can_differ_from_deployed_contract(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);parts=('realtime','notification','search','analytics-chat')
            contract={'scripts':[{'part':p,'sha256':'0'*64} for p in parts]};hashes={}
            for part in parts:
                relative='deploy/templates/nats-'+part+'-bootstrap.yaml'
                path=root/relative;path.parent.mkdir(parents=True,exist_ok=True)
                path.write_bytes(part.encode());hashes[relative]=hashlib.sha256(path.read_bytes()).hexdigest()
            def decoder(raw):
                part=raw.decode();return {'kind':'ConfigMap','metadata':{'name':'voice-nats-'+part+'-bootstrap'},'data':{'bootstrap.sh':'target '+part}}
            with patch.object(runner,'decode_yaml',side_effect=decoder):
                pins=runner.authenticated_bootstrap(root,contract,{'source_hashes':hashes})
            self.assertEqual(pins,{p:hashlib.sha256(('target '+p).encode()).hexdigest() for p in parts})

    def test_unenrolled_or_modified_bootstrap_is_not_authority(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);relative='deploy/templates/nats-realtime-bootstrap.yaml'
            path=root/relative;path.parent.mkdir(parents=True);path.write_bytes(b'changed')
            contract={'scripts':[{'part':'realtime','sha256':'0'*64}]}
            for hashes in ({},{relative:'0'*64}):
                with self.subTest(hashes=bool(hashes)),patch.object(runner,'decode_yaml') as decoder:
                    with self.assertRaises(Blocked):runner.authenticated_bootstrap(root,contract,{'source_hashes':hashes})
                    decoder.assert_not_called()


if __name__=='__main__':unittest.main()
