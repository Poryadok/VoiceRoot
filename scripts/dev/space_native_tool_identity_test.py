import copy
import hashlib
from pathlib import Path
import unittest

import space_native_tool_identity as tool


class SourceInventoryTests(unittest.TestCase):
    def test_actual_source_inventory_and_untrusted_drift(self):
        root=Path(__file__).resolve().parents[2]
        files={p.relative_to(root).as_posix():hashlib.sha256(p.read_bytes()).hexdigest()
               for p in (root/'src/backend').rglob('*.go') if not p.name.endswith('_test.go')}
        for path in (root/'src/backend/migrations/space_db').glob('*.sql'):
            files[path.relative_to(root).as_posix()]=hashlib.sha256(path.read_bytes()).hexdigest()
        dockerfile=root/'src/backend/space/Dockerfile.native-migrate'
        files[dockerfile.relative_to(root).as_posix()]=hashlib.sha256(dockerfile.read_bytes()).hexdigest()
        snapshot={'source_sha':'a'*40,'files':files}
        proof=tool.source_inventory(root,snapshot)
        self.assertEqual(proof['source_sha'],'a'*40)
        self.assertEqual(len(proof['build_source_sha256']),64)
        self.assertEqual(len(proof['sql_inventory_sha256']),64)
        for fault in ('source','sql','dockerfile','extra_sql'):
            wrong=copy.deepcopy(snapshot)
            if fault=='source': wrong['files']['src/backend/space/main.go']='0'*64
            if fault=='sql': wrong['files']['src/backend/migrations/space_db/000001_init.up.sql']='0'*64
            if fault=='dockerfile': wrong['files']['src/backend/space/Dockerfile.native-migrate']='0'*64
            if fault=='extra_sql': wrong['files']['src/backend/migrations/space_db/999999_wrong.up.sql']='0'*64
            with self.subTest(fault=fault),self.assertRaises(tool.ToolIdentityError): tool.source_inventory(root,wrong)


if __name__=='__main__': unittest.main()
