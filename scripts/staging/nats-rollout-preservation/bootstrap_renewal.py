"""Private inherited-pipe transport for the captured existing-actor reissuer.

Root caller verifies executable custody and fresh signed input under its lock.
This helper never opens a seed file, selects an issuer, or enrolls credentials.
"""
import json
import os
import selectors
import subprocess
import threading
import time

class RenewalError(ValueError):pass
def fail():raise RenewalError('existing_bootstrap_private_renewal_refused')
def invoke(executable,request,*,timeout=15):
    raw=json.dumps(request,separators=(',',':')).encode()
    if not 0<len(raw)<=1048576 or not 0<timeout<=30:fail()
    read_fd,write_fd=os.pipe();deadline=time.monotonic()+timeout
    output=bytearray();errors=[]
    def read_private():
        try:
            os.set_blocking(read_fd,False)
            with selectors.DefaultSelector() as selector:
                selector.register(read_fd,selectors.EVENT_READ)
                while True:
                    remaining=deadline-time.monotonic()
                    if remaining<=0:raise TimeoutError()
                    if not selector.select(min(remaining,0.2)):continue
                    chunk=os.read(read_fd,65536)
                    if not chunk:return
                    output.extend(chunk)
                    if len(output)>262144:raise ValueError()
        except Exception:errors.append(True)
        finally:os.close(read_fd)
    reader=threading.Thread(target=read_private,daemon=True);reader.start()
    process=None
    try:
        process=subprocess.Popen([str(executable),'--renew-existing-bootstrap',str(write_fd)],
            stdin=subprocess.PIPE,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,
            pass_fds=(write_fd,),env={'PATH':'/usr/bin:/bin','HOME':'/nonexistent'})
        os.close(write_fd);write_fd=None
        process.communicate(input=raw,timeout=max(0.001,deadline-time.monotonic()))
        reader.join(max(0.001,deadline-time.monotonic())+0.3)
        if process.returncode!=0 or reader.is_alive() or errors or not output:fail()
        return bytes(output)
    except Exception:
        if process is not None and process.poll() is None:
            process.kill();process.wait(timeout=2)
        fail()
    finally:
        if write_fd is not None:os.close(write_fd)
        reader.join(max(0,deadline-time.monotonic())+0.3)
        for i in range(len(output)):output[i]=0
