"""Root-captured workflow entry, independent of the downloaded target code."""
import os
from pathlib import Path
import re
import subprocess
import sys
sys.path.insert(0,str(Path(__file__).resolve().parent))
import runner
from root_cli import code_binding
from controller import Blocked

def main(args):
    code=Path(__file__).resolve().parents[1]
    code_binding(code)
    operation=os.environ.get('VOICE_NATS_ROLLOUT_OPERATION','')
    if not re.fullmatch(r'[a-f0-9]{12}',operation) or code.parent.name!='rollout-'+operation:
        raise Blocked('rollout_workflow_operation_invalid')
    os.environ['VOICE_NATS_PRESERVATION_RECEIPT']=str(code.parent/'apply-authorization.json')
    os.environ['VOICE_NATS_TARGET_SOURCE']=os.environ['GITHUB_WORKSPACE']
    if args==['--configure']:
        helper=code/'configure-kubectl-ci.sh'
        if 'configure-kubectl-ci.sh' not in code_binding(code):raise Blocked('rollout_configure_code_missing')
        subprocess.run(['/bin/bash',str(helper)],check=True,timeout=120)
        return
    if args not in (['--preflight'],['--apply']):raise Blocked('rollout_workflow_arguments_invalid')
    runner.main(['--preflight'] if args==['--preflight'] else [])

def cli(args):
    try:main(args);return 0
    except Exception as error:
        from bridge_client import emit_failure
        emit_failure(error,'workflow',args[0][2:] if args and args[0] in ('--preflight','--apply','--configure') else 'unknown',os.environ)
        print('NATS_ROLLOUT_WORKFLOW=BLOCKED',file=sys.stderr)
        return 1

if __name__=='__main__':sys.exit(cli(sys.argv[1:]))
