"""Known failed prebuild evidence is retained; no generic journal waiver."""
import json,tempfile,unittest
from pathlib import Path
from unittest.mock import patch
import installer
import prebuild_disposition as disposition

class Tests(unittest.TestCase):
    def fixture(self,root):
        installed=root/'installed';installed.mkdir()
        for name in ('journal','responses','sources'): (installed/name).mkdir()
        (installed/'sources'/disposition.OPERATION).mkdir()
        nonce=disposition.OPERATION+'a'*52
        request={'action':'prepare','nonce':nonce,'run_id':disposition.RUN,'source_sha':disposition.SOURCE,'mode':'images-only','changed_services':['user']}
        journal={'schema':'voice-nats-bridge-request-v1','phase':'STARTED','request':request,'request_sha256':disposition.digest(request)}
        path=installed/'journal'/(nonce+'.json');path.write_text(json.dumps(journal))
        (installed/'responses'/path.name).write_text(json.dumps({'status':'BLOCKED','error':'blocked_unclassified'}))
        marker={'metadata':dict(disposition.MARKER),'data':{'phase':'active','generation':'r20260930a4','dataPVC':'voice-nats-jsdata-d202610040049430b'}}
        return installed,path,marker

    def test_exact_failure_keeps_original_bytes_and_refuses_operation_or_marker_drift(self):
        with tempfile.TemporaryDirectory() as td:
            root=Path(td);installed,path,marker=self.fixture(root);before=path.read_bytes()
            with patch.object(disposition,'directory',side_effect=lambda p:Path(p)),patch.object(disposition,'private_json',side_effect=lambda p:json.loads(Path(p).read_text())),patch.object(disposition,'public_read',side_effect=lambda p:Path(p).read_bytes()):
                disposition.enroll(root,path,marker,disposition.V3_BINDING)
                disposition.enroll(root,path,marker,disposition.V3_BINDING)
                self.assertEqual(path.read_bytes(),before)
                (root/('rollout-'+disposition.OPERATION)).mkdir()
                with self.assertRaises(disposition.Blocked):disposition.enroll(root,path,marker,disposition.V3_BINDING)
                (root/('rollout-'+disposition.OPERATION)).rmdir()
                marker['metadata']['resourceVersion']='other'
                with self.assertRaises(disposition.Blocked):disposition.enroll(root,path,marker,disposition.V3_BINDING)

    def test_other_request_and_response_are_not_disposed(self):
        with tempfile.TemporaryDirectory() as td:
            root=Path(td);installed,path,marker=self.fixture(root)
            with patch.object(disposition,'directory',side_effect=lambda p:Path(p)),patch.object(disposition,'private_json',side_effect=lambda p:json.loads(Path(p).read_text())),patch.object(disposition,'public_read',side_effect=lambda p:Path(p).read_bytes()):
                (installed/'responses'/path.name).write_text('{}')
                with self.assertRaises(disposition.Blocked):disposition.enroll(root,path,marker,disposition.V3_BINDING)
                other=path.with_name('b'*64+'.json');other.write_bytes(path.read_bytes())
                with self.assertRaises(disposition.Blocked):disposition.enroll(root,other,marker,disposition.V3_BINDING)

    def test_upgrade_resume_requires_exact_preserved_predecessor_and_target_receipt(self):
        with tempfile.TemporaryDirectory() as td:
            root=Path(td);installed,path,marker=self.fixture(root);source=root/'source';source.mkdir()
            current=installed/'code';current.mkdir();old=installed/'code-v3-preserved';old.mkdir()
            wanted='b'*64
            def binding(p):return disposition.V3_BINDING if Path(p)==old else wanted
            with patch.object(installer,'binding_sha',side_effect=binding),patch.object(installer,'private_json',side_effect=lambda p:json.loads(Path(p).read_text())):
                with self.assertRaises(installer.Blocked):installer.disposition_predecessor(source,installed)
                record=installed/'upgrade-v4.json';record.write_text(json.dumps({'schema':'voice-nats-code-upgrade-v4','from':disposition.V3_BINDING,'to':wanted}))
                self.assertEqual(installer.disposition_predecessor(source,installed),disposition.V3_BINDING)
                current.rmdir() # Interrupted exactly between the two owned renames.
                self.assertEqual(installer.disposition_predecessor(source,installed),disposition.V3_BINDING)
                record.write_text(json.dumps({'schema':'voice-nats-code-upgrade-v4','from':disposition.V3_BINDING,'to':'c'*64}))
                with self.assertRaises(installer.Blocked):installer.disposition_predecessor(source,installed)

if __name__=='__main__':unittest.main()
