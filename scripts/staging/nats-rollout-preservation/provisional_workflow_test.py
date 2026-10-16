import ast
import os
from pathlib import Path
import unittest
from unittest.mock import patch

class Tests(unittest.TestCase):
    def select(self,changes=None,action='--bridge-prepare'):
        root=Path(os.environ.get('VOICE_TEST_REPO_ROOT',Path(__file__).resolve().parents[3]))
        text=(root/'.github/workflows/staging-deploy.yml').read_text()
        body=text.split("          python3 -I -S - <<'PY'\n",1)[1].split('          PY\n',1)[0]
        tree=ast.parse('\n'.join(line[10:] for line in body.splitlines()))
        node=next(n for n in tree.body if isinstance(n,ast.If) and ast.unparse(n.test)=="action == '--bridge-prepare'")
        env={'DEPLOY_MODE':'images-only','CHANGED_SERVICES':'voice','VOICE_IMAGE_TAG':'a'*40,
             'GITHUB_SHA':'a'*40,'GITHUB_RUN_ID':'123','GITHUB_EVENT_NAME':'workflow_dispatch',
             'ROLLOUT_PROVISIONAL_STAGING':'true','ROLLOUT_NEEDS_FULL_ROLLOUT':'false',
             'ROLLOUT_NEEDS_USER_SPACE_ROLLOUT':'false',**(changes or {})}
        with patch.dict(os.environ,env,clear=True):
            scope={'os':os,'action':action,'op':'1'*12}
            exec(compile(ast.Module(body=[node],type_ignores=[]),'<actual-workflow>','exec'),scope)
            return scope['arguments']

    def test_provisional_exact_dispatch_and_repair_arguments(self):
        self.assertEqual(self.select(),['prepare','images-only','voice','a'*40,'123','true'])
        self.assertEqual(self.select({'ROLLOUT_REPAIR_PARENT':'1'*12}),
            ['prepare-repair','1'*12,'images-only','voice','a'*40,'123','true'])

    def test_automatic_full_and_recovery_combinations_refused(self):
        for key,value in [('GITHUB_EVENT_NAME','push'),('DEPLOY_MODE','app-only'),
                          ('ROLLOUT_NEEDS_FULL_ROLLOUT','true'),('ROLLOUT_NEEDS_USER_SPACE_ROLLOUT','true'),
                          ('ROLLOUT_ROLLBACK_OPERATION','2'*12),('ROLLOUT_COLD_BACKUP_OPERATION','2'*12)]:
            with self.subTest(key=key),self.assertRaises(SystemExit):self.select({key:value})

    def test_default_ordinary_arguments_unchanged(self):
        self.assertEqual(self.select({'ROLLOUT_PROVISIONAL_STAGING':'false'}),
            ['prepare','images-only','voice','a'*40,'123'])

if __name__=='__main__':unittest.main()
