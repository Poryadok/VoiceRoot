"""Linux subprocess capture with enforced byte and wall-clock bounds."""
import os
import hashlib
import selectors
import signal
import subprocess
import sys
import time

from controller import Blocked
diagnostic_observer=None
authority_guard=None
TT_FENCE_VERSION=1
_current_action='command'
ACTIONS=frozenset(('get','patch','wait','scale','rollout','apply','create','delete','exec','run','inspect','stop','rm','image','info'))

class CommandFailure(Blocked):
    """Failure emitted by the bounded native subprocess transport."""

def action_id(argv):
    executable=os.path.basename(str(argv[0])) if argv else ''
    if executable not in ('kubectl','docker','nats'):return 'command'
    verb=next((value for value in argv[1:] if value in ACTIONS),'other')
    return executable+'.'+verb


def failure(code,*,exit_code=None,timeout=False,cause='unknown'):
    error=CommandFailure(code)
    error.diagnostic={'exit_code':exit_code,'timeout':timeout,'cause':cause,'action':_current_action}
    return error


def launch_snapshot(argv,body,timeout,limit,env,pass_fds):
    return (tuple(argv),bytes(body),timeout,limit,None if env is None else tuple(sorted(env.items())),tuple(pass_fds))


def capture(argv, *, body=b'', timeout=30, limit=2<<20, env=None, pass_fds=()):
    global _current_action
    argv=list(argv);body=bytes(body);env=None if env is None else dict(env);pass_fds=tuple(pass_fds)
    if len(body)>2<<20: raise Blocked('command_input_limit')
    action=action_id(argv)
    _current_action=action
    guard=authority_guard
    module=sys.modules.get('resource_claim');session=getattr(guard,'__self__',None)
    bound=(module is not None and type(session) is module.Session and getattr(guard,'__func__',None) is module.Session.check)
    privileged=bound and session._prerequisite_readonly_permit is not None
    if guard is not None:
        if bound:guard(argv,launch=launch_snapshot(argv,body,timeout,limit,env,pass_fds))
        else:guard(argv)
    if diagnostic_observer is not None:diagnostic_observer(action,'STARTED')
    if privileged:
        if authority_guard != guard:raise RuntimeError('staging_resource_guard_changed')
        session.readonly_current()
    try:
        child=subprocess.Popen(argv,stdin=subprocess.PIPE,stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,env=env,start_new_session=True,pass_fds=pass_fds)
    except PermissionError:raise failure('command_failed',cause='os_permission_denied') from None
    output=bytearray(); errors=bytearray(); pending=memoryview(body); deadline=time.monotonic()+timeout
    selector=selectors.DefaultSelector()
    try:
        os.set_blocking(child.stdout.fileno(),False)
        selector.register(child.stdout,selectors.EVENT_READ,'out')
        os.set_blocking(child.stderr.fileno(),False)
        selector.register(child.stderr,selectors.EVENT_READ,'err')
        if pending:
            os.set_blocking(child.stdin.fileno(),False)
            selector.register(child.stdin,selectors.EVENT_WRITE,'in')
        else: child.stdin.close()
        while selector.get_map():
            remaining=deadline-time.monotonic()
            if remaining<=0: raise failure('command_timeout',timeout=True,cause='timeout')
            for key,_ in selector.select(min(remaining,0.1)):
                if key.data in ('out','err'):
                    target=output if key.data=='out' else errors
                    bound=limit if key.data=='out' else 65536
                    raw=os.read(key.fd,65536 if key.data=='err' else min(65536,bound-len(target)+1))
                    if not raw:
                        selector.unregister(key.fileobj);key.fileobj.close()
                    else:
                        if key.data=='err':target.extend(raw[:max(0,bound-len(target))])
                        else:
                            target.extend(raw)
                            if len(target)>bound: raise Blocked('command_output_limit')
                else:
                    count=os.write(key.fd,pending[:65536]);pending=pending[count:]
                    if not pending:
                        selector.unregister(key.fileobj);child.stdin.close()
        exit_code=child.wait(timeout=max(0.01,deadline-time.monotonic()))
        if exit_code!=0:
            cause='application_authorization' if b'permissions violation' in bytes(errors).lower() or b'authorization violation' in bytes(errors).lower() else 'nonzero_exit'
            error=failure('command_failed',exit_code=exit_code,cause=cause)
            # Hash only the already bounded stderr buffer. Never expose its text,
            # which may contain credential material even for a read-only command.
            lower=bytes(errors).lower()
            reason=next((name for needle,name in (
                (b'connection refused','connection_refused'),
                (b'no such file or directory','executable_missing'),
                (b'forbidden','forbidden')) if needle in lower),'nonzero_exit')
            error.diagnostic.update(stderr_reason=reason,stderr_bytes=len(errors),
                stderr_sha256=hashlib.sha256(errors).hexdigest())
            raise error
        if diagnostic_observer is not None:diagnostic_observer(action,'COMPLETE')
        return bytes(output)
    except PermissionError:raise failure('command_failed',cause='os_permission_denied') from None
    except subprocess.TimeoutExpired:raise failure('command_timeout',timeout=True,cause='timeout') from None
    except OSError:raise failure('command_failed') from None
    finally:
        selector.close()
        if child.poll() is None:
            os.killpg(child.pid,signal.SIGKILL);child.wait(timeout=2)
        for stream in (child.stdin,child.stdout,child.stderr):
            if not stream.closed:stream.close()
