"""Bounded authenticated Space JWKS observation through a Kubernetes tunnel."""
import base64
import hashlib
import json
import os
import re
import selectors
import socket
import ssl
import subprocess
import time

import full_cohort_schema_metadata as common
import realtime_p3_create_only_crypto as crypto
import realtime_p3_public_inputs as public


class SpaceObservationError(RuntimeError): pass

TUNNEL_DEADLINE = 20


def _tunnel(pod, ca, certificate, guard):
    """Return public JWKS bytes over a verified TLS connection to the named Pod."""
    if not re.fullmatch(r'[a-z0-9]([-a-z0-9]*[a-z0-9])?',pod) or len(pod)>253: raise ValueError()
    guard()
    deadline=time.monotonic()+TUNNEL_DEADLINE
    process = subprocess.Popen(public.PREFIX+['port-forward','--address=127.0.0.1','pod/'+pod,':8443'],
        stdin=subprocess.DEVNULL,stdout=subprocess.PIPE,stderr=subprocess.DEVNULL)
    try:
        sel=selectors.DefaultSelector(); sel.register(process.stdout,selectors.EVENT_READ)
        raw=bytearray(); port=None
        try:
            while time.monotonic()<deadline and len(raw)<4096:
                guard()
                events=sel.select(min(1,max(0,deadline-time.monotonic())))
                if not events:
                    if process.poll() is not None: raise ValueError()
                    continue
                chunk=os.read(process.stdout.fileno(),4096-len(raw))
                if not chunk: raise ValueError()
                raw.extend(chunk)
                match=re.search(rb'(?m)^Forwarding from 127\.0\.0\.1:([0-9]{1,5}) -> 8443\r?$',raw)
                if match:
                    port=int(match.group(1)); break
        finally: sel.close()
        if port is None or not 1<=port<=65535: raise ValueError()
        ctx=ssl.create_default_context(cadata=ca.decode('ascii')); ctx.minimum_version=ssl.TLSVersion.TLSv1_2
        with socket.create_connection(('127.0.0.1',port),timeout=min(2,max(0.01,deadline-time.monotonic()))) as tcp:
            with ctx.wrap_socket(tcp,server_hostname='voice-space.voice-dev.svc') as tls:
                tls.settimeout(1)
                if tls.getpeercert(binary_form=True) != crypto.der(certificate): raise ValueError()
                tls.sendall(b'GET /.well-known/jwks.json HTTP/1.1\r\nHost: voice-space.voice-dev.svc\r\nConnection: close\r\n\r\n')
                response=bytearray(); body_start=None; length=None
                while time.monotonic()<deadline:
                    guard()
                    try: chunk=tls.recv(min(4096,73728-len(response)))
                    except socket.timeout: continue
                    if not chunk: break
                    response.extend(chunk)
                    if body_start is None and b'\r\n\r\n' in response:
                        body_start=response.index(b'\r\n\r\n')+4
                        if body_start>8192: raise ValueError()
                        lines=bytes(response[:body_start-4]).split(b'\r\n')
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
                    if body_start is None and len(response)>8192: raise ValueError()
                    if body_start is not None and len(response)-body_start>=length: break
                    if len(response)>=73728: raise ValueError()
                if time.monotonic()>=deadline or body_start is None or len(response)-body_start != length: raise ValueError()
                value=bytes(response[body_start:])
        guard()
        return value
    finally:
        process.terminate()
        try: process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            process.kill(); process.wait(timeout=5)
        process.stdout.close()


def capture(source_root,source_sha,source_authority,space_pod,namespace_uid,claim_guard,destination,
            run_bytes=common.linux_private_bytes):
    """Capture public inputs + actual Space HTTPS JWKS before the writer cut.

    Service/Endpoints metadata is checked, but the port-forward is not a
    dataplane-routing or application-to-application connectivity proof.
    """
    try:
        observed=public.capture(source_root,source_sha,source_authority,space_pod,namespace_uid,claim_guard,destination,run_bytes)
        def guard():
            common._fence(claim_guard)
            namespace=common._external(run_bytes,['kubectl','--kubeconfig','/home/pmd/.kube/config','--context','default',
                '--request-timeout=15s','get','namespace','voice-dev','-o','json'])
            if json.loads(namespace)['metadata']['uid'] != namespace_uid: raise common.GuardLost('namespace_guard_failed')
            common._fence(claim_guard)
        workload=common.kube_workload_snapshot('space',space_pod,run_bytes,guard)
        def snapshot():
            before=workload(); pod=public._get(run_bytes,guard,'pod',space_pod)
            if any(before[k] != observed['space_identity'][k] for k in observed['space_identity']): raise ValueError()
            public._mounted(pod); service=public._get(run_bytes,guard,'service','voice-space')
            if public._service(service,pod) != observed['space_service_spec']: raise ValueError()
            endpoints=public._get(run_bytes,guard,'endpoints','voice-space'); meta=endpoints['metadata']
            if (meta.get('name')!='voice-space' or meta.get('namespace')!='voice-dev' or meta.get('deletionTimestamp')
                    or any(not isinstance(meta.get(k),str) or not meta[k] for k in ('uid','resourceVersion'))): raise ValueError()
            subsets=endpoints.get('subsets',[])
            if len(subsets)!=1 or subsets[0].get('notReadyAddresses') or len(subsets[0].get('addresses',[]))!=1: raise ValueError()
            address=subsets[0]['addresses'][0]; ref=address.get('targetRef',{})
            if (ref.get('kind')!='Pod' or ref.get('name')!=space_pod or ref.get('uid')!=before['pod_uid']
                    or address.get('ip')!=pod.get('status',{}).get('podIP')
                    or sum(p.get('port')==8443 and p.get('protocol','TCP')=='TCP' for p in subsets[0].get('ports',[]))!=1): raise ValueError()
            for name in public.PUBLIC:
                if public._meta(run_bytes,guard,'secret',name)!=observed['objects'][name]: raise ValueError()
            return {'workload':before,'service':observed['space_service_spec'],
                    'endpoints':{'uid':meta['uid'],'resourceVersion':meta['resourceVersion']}}
        first=snapshot()
        ca=public._value(run_bytes,guard,'secret','voice-media-ca','ca.crt')
        cert=public._value(run_bytes,guard,'secret','voice-space-media-server-tls','tls.crt')
        value=_tunnel(space_pod,ca,cert,guard)
        def unique(pairs):
            result={}
            for key,item in pairs:
                if key in result: raise ValueError()
                result[key]=item
            return result
        document=json.loads(value,object_pairs_hook=unique)
        if (not isinstance(document,dict) or set(document)!={'keys'} or not isinstance(document['keys'],list)
                or len(document['keys'])!=2): raise ValueError()
        kids=set(); public_keys=set()
        for key in document['keys']:
            if (not isinstance(key,dict) or set(key)!={'kty','kid','use','alg','n','e'}
                    or (key['kty'],key['use'],key['alg'])!=('RSA','sig','RS256')
                    or not isinstance(key['kid'],str) or not re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9._-]{0,127}',key['kid'])
                    or any(not isinstance(key[x],str) or not re.fullmatch(r'[A-Za-z0-9_-]{1,2048}',key[x]) for x in ('n','e'))
                    or key['kid'] in kids): raise ValueError()
            kids.add(key['kid'])
            numbers=[]
            for field in ('n','e'):
                encoded=key[field]
                decoded=base64.b64decode(encoded+'='*((-len(encoded))%4),altchars=b'-_',validate=True)
                if not decoded or base64.urlsafe_b64encode(decoded).decode().rstrip('=')!=encoded: raise ValueError()
                numbers.append(int.from_bytes(decoded,'big'))
            if numbers[0].bit_length()<2048 or numbers[1]<3 or numbers[1]%2==0 or tuple(numbers) in public_keys: raise ValueError()
            public_keys.add(tuple(numbers))
        if snapshot()!=first: raise ValueError()
        guard()
        return {'source_sha':source_sha,'namespace_uid':namespace_uid,'identity':first,
                'public_objects':{name:observed['objects'][name] for name in public.PUBLIC},
                'jwks_sha256':hashlib.sha256(value).hexdigest(),'jwks_bytes':value,
                'scope':'space-pod-https-through-kubernetes-tunnel'}
    except common.GuardLost: raise
    except Exception: raise SpaceObservationError('Space HTTPS observation unavailable') from None
