"""Capture Space's public signer set before the cut without exporting keys."""
import base64
import hashlib
import json
import os
import re
import selectors
import socket
import subprocess
import time

import full_cohort_schema_metadata as common
import realtime_p3_public_inputs as public


class PlanError(RuntimeError): pass


def _parse(value):
    def unique(pairs):
        result={}
        for key,item in pairs:
            if key in result: raise ValueError()
            result[key]=item
        return result
    document=json.loads(value,object_pairs_hook=unique)
    if not isinstance(document,dict) or set(document)!={'keys'} or not isinstance(document['keys'],list) or len(document['keys'])!=2: raise ValueError()
    kids=set(); keys=set()
    for key in document['keys']:
        if (not isinstance(key,dict) or set(key)!={'kty','kid','use','alg','n','e'}
                or (key['kty'],key['use'],key['alg'])!=('RSA','sig','RS256')
                or not isinstance(key['kid'],str) or not re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9._-]{0,127}',key['kid'])
                or any(not isinstance(key[x],str) or not re.fullmatch(r'[A-Za-z0-9_-]{1,2048}',key[x]) for x in ('n','e'))
                or key['kid'] in kids): raise ValueError()
        kids.add(key['kid']); numbers=[]
        for field in ('n','e'):
            encoded=key[field]; raw=base64.b64decode(encoded+'='*((-len(encoded))%4),altchars=b'-_',validate=True)
            if not raw or base64.urlsafe_b64encode(raw).decode().rstrip('=')!=encoded: raise ValueError()
            numbers.append(int.from_bytes(raw,'big'))
        if numbers[0].bit_length()<2048 or numbers[1]<3 or numbers[1]%2==0 or tuple(numbers) in keys: raise ValueError()
        keys.add(tuple(numbers))


def _http(pod,guard):
    if not isinstance(pod,str) or not re.fullmatch(r'voice-space-[a-z0-9-]+',pod) or len(pod)>253: raise ValueError()
    deadline=time.monotonic()+20; guard()
    process=subprocess.Popen(public.PREFIX+['port-forward','--address=127.0.0.1','pod/'+pod,':8080'],
        stdin=subprocess.DEVNULL,stdout=subprocess.PIPE,stderr=subprocess.DEVNULL)
    try:
        sel=selectors.DefaultSelector(); sel.register(process.stdout,selectors.EVENT_READ); buffer=bytearray(); port=None
        try:
            while time.monotonic()<deadline and len(buffer)<4096:
                guard(); events=sel.select(min(1,max(0,deadline-time.monotonic())))
                if not events:
                    if process.poll() is not None: raise ValueError()
                    continue
                chunk=os.read(process.stdout.fileno(),4096-len(buffer))
                if not chunk: raise ValueError()
                buffer.extend(chunk)
                match=re.search(rb'(?m)^Forwarding from 127\.0\.0\.1:([0-9]{1,5}) -> 8080\r?$',buffer)
                if match: port=int(match.group(1)); break
        finally: sel.close()
        if port is None or not 1<=port<=65535: raise ValueError()
        with socket.create_connection(('127.0.0.1',port),timeout=min(2,max(.01,deadline-time.monotonic()))) as conn:
            conn.settimeout(1)
            conn.sendall(b'GET /.well-known/jwks.json HTTP/1.1\r\nHost: voice-space.voice-dev.svc\r\nConnection: close\r\n\r\n')
            response=bytearray(); start=None; length=None
            while time.monotonic()<deadline:
                guard()
                try: chunk=conn.recv(min(4096,73728-len(response)))
                except socket.timeout: continue
                if not chunk: break
                response.extend(chunk)
                if start is None and b'\r\n\r\n' in response:
                    start=response.index(b'\r\n\r\n')+4
                    if start>8192: raise ValueError()
                    lines=bytes(response[:start-4]).split(b'\r\n')
                    if not re.fullmatch(rb'HTTP/1\.[01] 200(?: [^\r\n]*)?',lines[0]): raise ValueError()
                    headers={}
                    for line in lines[1:]:
                        if b':' not in line: raise ValueError()
                        name,value=line.split(b':',1); name=name.strip().lower()
                        if not name or name in headers: raise ValueError()
                        headers[name]=value.strip()
                    if b'transfer-encoding' in headers or not re.fullmatch(rb'[0-9]{1,5}',headers.get(b'content-length',b'')): raise ValueError()
                    length=int(headers[b'content-length'])
                    if not 0<length<=65536: raise ValueError()
                if start is None and len(response)>8192: raise ValueError()
                if start is not None and len(response)-start>=length: break
                if len(response)>=73728: raise ValueError()
            if time.monotonic()>=deadline or start is None or len(response)-start!=length: raise ValueError()
            value=bytes(response[start:])
        _parse(value); guard(); return value
    finally:
        process.terminate()
        try: process.wait(timeout=5)
        except subprocess.TimeoutExpired: process.kill(); process.wait(timeout=5)
        process.stdout.close()


def capture(source_root,source_sha,source_authority,space_pod,namespace_uid,claim_guard,destination,
            run_bytes=common.linux_private_bytes):
    """Return a bounded public plan observation; HTTPS serving remains unverified."""
    try:
        observed=public.capture(source_root,source_sha,source_authority,space_pod,namespace_uid,claim_guard,
                                destination,run_bytes,mode='issuer-plan')
        signer=observed['current_signer']; expected={'signing_dir':'exact','active_kid_ref':'exact','signing_mount':'exact'}
        if (signer['configuration']!=expected or signer['secret']['state']!='present'
                or set(signer['secret']['keys'])!={'current.pem','next.pem','active-kid'}): raise ValueError()
        def guard():
            common._fence(claim_guard)
            raw=common._external(run_bytes,['kubectl','--kubeconfig','/home/pmd/.kube/config','--context','default',
                '--request-timeout=15s','get','namespace','voice-dev','-o','json'])
            if json.loads(raw)['metadata']['uid']!=namespace_uid: raise common.GuardLost('namespace_guard_failed')
            common._fence(claim_guard)
        workload=common.kube_workload_snapshot('space',space_pod,run_bytes,guard)
        first=workload()
        if any(first[k]!=observed['space_identity'][k] for k in observed['space_identity']): raise ValueError()
        if public._signer_identity(run_bytes,guard)!=signer['secret']: raise ValueError()
        value=_http(space_pod,guard)
        if workload()!=first or public._signer_identity(run_bytes,guard)!=signer['secret']: raise ValueError()
        for name in public.PUBLIC:
            if public._meta(run_bytes,guard,'secret',name)!=observed['objects'][name]: raise ValueError()
        if public._meta(run_bytes,guard,'configmap','voice-app-config')!=observed['objects']['voice-app-config']: raise ValueError()
        common._fence(guard)
        return {'source_sha':source_sha,'namespace_uid':namespace_uid,'space_identity':observed['space_identity'],
            'signer':signer,'public_sources':observed['objects'],'public_der_sha256':observed['public_der_sha256'],
            'jwks_sha256':hashlib.sha256(value).hexdigest(),'jwks_bytes':value,
            'scope':'space-current-http-pod-public-keys-not-https-readiness'}
    except common.GuardLost: raise
    except Exception: raise PlanError('Space public signer plan unavailable') from None
