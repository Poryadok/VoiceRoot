"""Exact workflow argument-selection AST and real bridge request identity."""
import ast
import copy
import os
from pathlib import Path
import unittest
from unittest.mock import patch
import bridge_client
import paused_recovery

class Tests(unittest.TestCase):
    def test_extracted_workflow_selects_only_exact_recovery_inputs(self):
        root=Path(os.environ.get('VOICE_TEST_REPO_ROOT',Path(__file__).resolve().parents[3]))
        text=(root/'.github/workflows/staging-deploy.yml').read_text()
        body=text.split("          python3 -I -S - <<'PY'\n",1)[1].split('          PY\n',1)[0]
        tree=ast.parse('\n'.join(line[10:] for line in body.splitlines()))
        node=next(node for node in tree.body if isinstance(node,ast.If) and ast.unparse(node.test)=="action == '--bridge-prepare'")
        code=compile(ast.Module(body=[node],type_ignores=[]),'<actual-workflow-selector>','exec')
        env={'ROLLOUT_COLD_BACKUP_OPERATION':paused_recovery.OPERATION,'DEPLOY_MODE':'images-only',
             'CHANGED_SERVICES':'story','VOICE_IMAGE_TAG':'a'*40,'GITHUB_SHA':'a'*40,'GITHUB_RUN_ID':'123'}
        with patch.dict(os.environ,env,clear=True):
            scope={'os':os,'action':'--bridge-prepare'};exec(code,scope)
            self.assertEqual(scope['arguments'],['resume-cold-backup',paused_recovery.OPERATION,'123'])
        for key,value in (('ROLLOUT_ROLLBACK_OPERATION','b'*12),('ROLLOUT_COLD_BACKUP_OPERATION','b'*12),
                ('DEPLOY_MODE','full'),('CHANGED_SERVICES','story,bot'),('VOICE_IMAGE_TAG','b'*40)):
            bad={**env,key:value}
            with self.subTest(key=key),patch.dict(os.environ,bad,clear=True),self.assertRaises(SystemExit):
                exec(code,{'os':os,'action':'--bridge-prepare'})

    def test_actual_client_full_nonce_and_exact_run_identity(self):
        env={'GITHUB_RUN_ID':'123','GITHUB_RUN_ATTEMPT':'1','GITHUB_JOB':'deploy','GITHUB_TOKEN':'synthetic-token'}
        args=['resume-cold-backup',paused_recovery.OPERATION,'123']
        first=bridge_client.request(args,env)
        self.assertEqual(first,bridge_client.request(args,env))
        self.assertEqual(set(first),{'action','operation','run_id','nonce','token'})
        self.assertEqual(len(first['nonce']),64)
        self.assertNotEqual(first['nonce'][:12],paused_recovery.OPERATION)
        second=bridge_client.request(args,{**env,'GITHUB_RUN_ATTEMPT':'2'})
        self.assertNotEqual(first['nonce'],second['nonce'])
        with self.assertRaises(bridge_client.bridge.BridgeError):bridge_client.request([*args[:2],'124'],env)

if __name__=='__main__':unittest.main()
