"""Immutable prior repair can cross only the admitted helper-code transition."""
import copy
import hashlib
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import expired_recovery as recovery


class RepairLineageTests(unittest.TestCase):
    def test_only_helper_binding_changes_and_original_receipt_is_retained(self):
        with tempfile.TemporaryDirectory() as td:
            base=Path(td)/'rollout-3340764a7d24';base.mkdir()
            (base/'expired-recovery-input-repair.json').write_bytes(b'presence')
            prior={'schema':'voice-expired-native-admission-v1','operation':'3340764a7d24',
                'checkpoint_sha256':'1'*64,'helper_binding_sha256':'d81ddbb34b13da1a94fea99d675091109984c6cbf708031d3c9c2483660350d4',
                'target':{'exact':'target'},'input_hashes':{'exact':'inputs'},'database_sha256':'2'*64}
            raw=json.dumps(prior,sort_keys=True,separators=(',',':')).encode()
            receipt={'admission_sha256':recovery.upload.digest(prior)}
            fresh={**prior,'helper_binding_sha256':'3'*64}
            with patch.object(recovery,'verify_failed_decoder_slot',autospec=True,return_value=prior) as slot,\
                patch.object(recovery.upload,'private_json',return_value=receipt):
                self.assertEqual(recovery.repair_admission_digest(base,fresh),recovery.upload.digest(prior))
                slot.assert_called_with(base)
                for field in ('schema','operation','checkpoint_sha256','target','input_hashes','database_sha256'):
                    changed=copy.deepcopy(fresh);changed[field]='changed'
                    with self.subTest(field=field),self.assertRaises(recovery.RecoveryError):
                        recovery.repair_admission_digest(base,changed)
                receipt['admission_sha256']='4'*64
                with self.assertRaises(recovery.RecoveryError):recovery.repair_admission_digest(base,fresh)
            self.assertEqual((base/'expired-recovery-input-repair.json').read_bytes(),b'presence')

    def test_failed_slot_and_preserved_code_are_both_authenticated(self):
        base=Path('/fixed/rollout-3340764a7d24')
        binding=json.loads((Path(__file__).parent/'testdata/expired-recovery-v13-manifest.json').read_text())
        files={'admission.json':json.dumps({'helper_binding_sha256':recovery.upload.digest(binding)}).encode(),
            'repair.json':b'{}','source.json':b'{}','rollout-before.tar':b'fixed archive'}
        hashes={name:hashlib.sha256(raw).hexdigest() for name,raw in files.items()}
        def read(path,limit=0):return files[path.name]
        with patch('root_cli.code_binding',return_value=binding),\
            patch.object(recovery,'FAILED_DECODER_FILES',hashes),\
            patch.object(recovery.upload,'private_read',side_effect=read):
            self.assertEqual(recovery.verify_failed_decoder_slot(base)['helper_binding_sha256'],recovery.upload.digest(binding))
            for name in files:
                saved=files[name];files[name]=b'changed'
                with self.subTest(file=name),self.assertRaises(recovery.RecoveryError):recovery.verify_failed_decoder_slot(base)
                files[name]=saved
            with patch('root_cli.code_binding',return_value={'wrong':'1'*64}),self.assertRaises(recovery.RecoveryError):
                recovery.verify_failed_decoder_slot(base)
