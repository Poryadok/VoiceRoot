"""Linux subprocess capture with enforced byte and wall-clock bounds."""
import os
import selectors
import signal
import subprocess
import time

from controller import Blocked


def capture(argv, *, body=b'', timeout=30, limit=2<<20, env=None):
    if len(body)>2<<20: raise Blocked('command_input_limit')
    child=subprocess.Popen(argv,stdin=subprocess.PIPE,stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,env=env,start_new_session=True)
    output=bytearray(); pending=memoryview(body); deadline=time.monotonic()+timeout
    selector=selectors.DefaultSelector()
    try:
        os.set_blocking(child.stdout.fileno(),False)
        selector.register(child.stdout,selectors.EVENT_READ,'out')
        if pending:
            os.set_blocking(child.stdin.fileno(),False)
            selector.register(child.stdin,selectors.EVENT_WRITE,'in')
        else: child.stdin.close()
        while selector.get_map():
            remaining=deadline-time.monotonic()
            if remaining<=0: raise Blocked('command_timeout')
            for key,_ in selector.select(min(remaining,0.1)):
                if key.data=='out':
                    raw=os.read(key.fd,min(65536,limit-len(output)+1))
                    if not raw:
                        selector.unregister(key.fileobj);child.stdout.close()
                    else:
                        output.extend(raw)
                        if len(output)>limit: raise Blocked('command_output_limit')
                else:
                    count=os.write(key.fd,pending[:65536]);pending=pending[count:]
                    if not pending:
                        selector.unregister(key.fileobj);child.stdin.close()
        if child.wait(timeout=max(0.01,deadline-time.monotonic()))!=0:
            raise Blocked('command_failed')
        return bytes(output)
    except (OSError,subprocess.TimeoutExpired):
        raise Blocked('command_failed') from None
    finally:
        selector.close()
        if child.poll() is None:
            os.killpg(child.pid,signal.SIGKILL);child.wait(timeout=2)
        for stream in (child.stdin,child.stdout):
            if not stream.closed:stream.close()
