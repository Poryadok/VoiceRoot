import ast
import itertools
import json
import os
from pathlib import Path
import unittest
from unittest.mock import patch
from types import SimpleNamespace

class Tests(unittest.TestCase):
    def test_actual_deploy_guard_keeps_ordinary_calls_and_excludes_acceptance_and_diagnostics(self):
        root=Path(os.environ.get('VOICE_TEST_REPO_ROOT',Path(__file__).resolve().parents[3]))
        text=(root/'.github/workflows/staging-deploy.yml').read_text()
        raw=text.split('  deploy:\n',1)[1].splitlines()[0].strip().removeprefix('if: ')
        expression=json.loads(raw).replace('&&',' and ').replace('||',' or ').replace('!inputs.','not inputs.')
        for event in ('workflow_dispatch','workflow_call'):
            for accept,probe,secret,mail in itertools.product((False,True),repeat=4):
                with self.subTest(event=event,accept=accept,probe=probe,secret=secret,mail=mail):
                    inputs=SimpleNamespace(accept_provisional_operation='1'*12 if accept else '',
                        nats_probe_only=probe,validate_app_secret_only=secret,mail_only=mail)
                    actual=eval(expression,{'__builtins__':{},'true':True,'inputs':inputs,
                        'github':SimpleNamespace(event_name=event)})
                    self.assertEqual(actual,not accept and (event!='workflow_dispatch' or not any((probe,secret,mail))))

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
        self.assertEqual(self.select(),['prepare','images-only','voice','a'*40,'123','true','false'])
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
