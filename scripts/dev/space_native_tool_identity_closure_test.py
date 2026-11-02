import hashlib
import json
from pathlib import Path
import tempfile
import unittest

import space_native_tool_identity as tool
import space_native_tool_identity_custody as custody


class ClosureTests(unittest.TestCase):
    def setUp(self):
        self.tmp=tempfile.TemporaryDirectory(); self.addCleanup(self.tmp.cleanup)
        self.root=Path(self.tmp.name); base=self.root/'src/backend/space/cmd/native-migrate'; base.mkdir(parents=True)
        (base/'a.go').write_text('package pkg\n'); (base/'data.sql').write_text('select 1\n')
        (self.root/'src/backend/space/go.mod').write_text('module example\n')
        self.snapshot={'files':{p.relative_to(self.root).as_posix():hashlib.sha256(p.read_bytes()).hexdigest()
                                for p in (base/'a.go',base/'data.sql',self.root/'src/backend/space/go.mod')}}
        self.package={'Dir':'/src/space/cmd/native-migrate','Module':{'Dir':'/src/space'},'GoFiles':['a.go'],
                      'EmbedPatterns':['data.sql'],'EmbedFiles':['data.sql']}
        self.raw=json.dumps(self.package).encode()

    def test_actual_members_and_embed_are_bound(self):
        result=tool.normalize_closure(self.raw,self.root,self.snapshot)
        self.assertEqual(result['packages']['src/backend/space/cmd/native-migrate'],['src/backend/space/cmd/native-migrate/a.go'])
        self.assertIn('src/backend/space/cmd/native-migrate/data.sql',result['files'])
        (self.root/'src/backend/space/cmd/native-migrate/added.go').write_text('package pkg\n')
        with self.assertRaises(tool.ToolIdentityError): tool.normalize_closure(self.raw,self.root,self.snapshot)

    def test_wildcard_embed_and_wrong_member_fail(self):
        self.package['EmbedPatterns']=['*.sql']
        with self.assertRaises(tool.ToolIdentityError): tool.normalize_closure(json.dumps(self.package).encode(),self.root,self.snapshot)
        self.package['EmbedPatterns']=['data.sql']; self.package['GoFiles']=['wrong.go']
        with self.assertRaises(tool.ToolIdentityError): tool.normalize_closure(json.dumps(self.package).encode(),self.root,self.snapshot)

    def test_candidate_equivalence_tracks_relevant_files_not_unrelated_source(self):
        closure=tool.normalize_closure(self.raw,self.root,self.snapshot)
        changed={'files':dict(self.snapshot['files'], **{'src/backend/user/unrelated.go':'f'*64})}
        custody._equivalent(closure,changed)
        for name in ('src/backend/space/cmd/native-migrate/added.go','src/backend/space/go.sum'):
            bad={'files':dict(changed['files'], **{name:'f'*64})}
            with self.assertRaises(ValueError): custody._equivalent(closure,bad)
        bad={'files':dict(changed['files'])}; bad['files']['src/backend/space/cmd/native-migrate/data.sql']='0'*64
        with self.assertRaises(ValueError): custody._equivalent(closure,bad)

if __name__=='__main__': unittest.main()
