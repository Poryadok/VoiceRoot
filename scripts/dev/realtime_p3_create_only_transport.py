"""Fixed kubectl requests with bounded, private stdin and stdout."""
import json
import os
import selectors
import subprocess
import time

import frontend_runtime_executor
import realtime_p3_create_only_packet as packet

LIMIT = 2 * 1024 * 1024
PUBLIC = {('voice-principal-ca','ca.crt'),('voice-media-ca','ca.crt'),('voice-space-media-server-tls','tls.crt')}


class TransportError(RuntimeError): pass


def _run(arguments, payload=None):
    process = None; selector = selectors.DefaultSelector()
    try:
        if payload is not None and (not isinstance(payload,bytes) or len(payload)>LIMIT): raise ValueError()
        process = subprocess.Popen(frontend_runtime_executor.KubectlTransport.PREFIX+arguments,
            stdin=subprocess.PIPE if payload is not None else subprocess.DEVNULL,
            stdout=subprocess.PIPE,stderr=subprocess.DEVNULL)
        os.set_blocking(process.stdout.fileno(),False); selector.register(process.stdout,selectors.EVENT_READ)
        remaining = memoryview(payload) if payload is not None else None
        if remaining is not None:
            os.set_blocking(process.stdin.fileno(),False); selector.register(process.stdin,selectors.EVENT_WRITE)
        output = bytearray(); deadline = time.monotonic()+30
        while selector.get_map():
            wait = deadline-time.monotonic()
            if wait <= 0: raise TimeoutError()
            events = selector.select(wait)
            if not events: raise TimeoutError()
            for event,_ in events:
                if event.fileobj is process.stdout:
                    chunk = os.read(process.stdout.fileno(),min(65536,LIMIT+1-len(output)))
                    if not chunk: selector.unregister(process.stdout); process.stdout.close()
                    else:
                        output.extend(chunk)
                        if len(output)>LIMIT: raise ValueError()
                else:
                    count = os.write(process.stdin.fileno(),remaining[:65536]); remaining = remaining[count:]
                    if not remaining: selector.unregister(process.stdin); process.stdin.close()
        if process.wait(timeout=max(0.01,deadline-time.monotonic())) != 0: raise ValueError()
        return bytes(output)
    except Exception:
        raise TransportError('P3 Kubernetes request unavailable') from None
    finally:
        selector.close()
        if process is not None:
            if process.poll() is None: process.kill(); process.wait()
            for stream in (process.stdin,process.stdout):
                if stream is not None and not stream.closed: stream.close()


def public_certificate(name,key):
    if (name,key) not in PUBLIC: raise TransportError('P3 public source invalid')
    template = '{{.metadata.uid}}{{"\\n"}}{{.metadata.resourceVersion}}{{"\\n"}}{{index .data "'+key+'"}}'
    return _run(['get','secret',name,'-o','go-template='+template])


def get(kind,name):
    if (kind,name) not in packet.MEMBERS: raise TransportError('P3 object outside scope')
    raw = _run(['get',kind.lower(),name,'--ignore-not-found=true','-o','json'])
    return None if not raw else json.loads(raw)


def create(obj,dry_run):
    kind = obj.get('kind'); name = obj.get('metadata',{}).get('name')
    if (kind,name) not in packet.MEMBERS or type(dry_run) is not bool: raise TransportError('P3 object outside scope')
    payload = json.dumps(obj,separators=(',', ':'),ensure_ascii=True).encode('ascii')
    args = ['create','-f','-','-o','json']
    if dry_run: args.insert(1,'--dry-run=server')
    return json.loads(_run(args,payload))
