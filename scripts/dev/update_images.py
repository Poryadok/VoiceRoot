#!/usr/bin/env python3
"""Manually update only app images in the owned voice-dev namespace.

Preview (read-only, includes server dry-run):
  python3 scripts/dev/update_images.py --image user=registry/user@sha256:<64hex>
Apply and wait:
  python3 scripts/dev/update_images.py --image user=registry/user@sha256:<64hex> --apply

Build/push or import images first and resolve their registry manifest digest.
Tags, including full commit tags, are mutable and are never admitted.
Frontends must be built for dev API/WS URLs.
Schema-changing builds require explicit dev migration preparation before this
command. This tool runs no migrations and changes no data/configuration.
An interrupted or failed mutation retains its lock for operator inspection;
there is deliberately no force/unlock flag or automatic rollback.
"""
import argparse
import copy
import json
import re
import secrets
import subprocess
import sys

NS = 'voice-dev'
LABELS = {'voice.environment':'dev', 'app.kubernetes.io/managed-by':'voice-dev'}
SERVICES = {name:name for name in 'auth user social role space chat file messaging voice matchmaking search notification realtime bot subscription moderation story analytics gateway gameintegration web admin'.split()}
SERVICES['developer-portal'] = 'portal'
LOCK = 'voice-dev-image-update-lock'


def validate_namespace(namespace, requested=NS):
    if requested != NS or namespace.get('metadata',{}).get('name') != NS:
        raise ValueError('only voice-dev is admitted')
    if any(namespace['metadata'].get('labels',{}).get(k) != v for k,v in LABELS.items()):
        raise ValueError('dev namespace ownership marker missing')


def parse_images(values):
    result = {}
    for value in values:
        service, sep, image = value.partition('=')
        if not sep or service not in SERVICES or service in result:
            raise ValueError('unknown or duplicate app service')
        repository = r'[a-z0-9]+(?:[._-][a-z0-9]+)*(?::[0-9]+)?(?:/[a-z0-9]+(?:[._-][a-z0-9]+)*)*'
        if not re.fullmatch(repository + r'@sha256:[0-9a-f]{64}', image):
            raise ValueError('image must have an immutable sha256 manifest digest')
        result[service] = image
    if not result:
        raise ValueError('at least one app image required')
    return result


def plan_patch(service, image, deployment):
    if service not in SERVICES:
        raise ValueError('unknown app service')
    parse_images([service + '=' + image])
    meta = deployment.get('metadata',{})
    if (deployment.get('kind') != 'Deployment' or meta.get('namespace') != NS or
            meta.get('name') != 'voice-' + service or
            any(meta.get('labels',{}).get(k) != v for k,v in LABELS.items())):
        raise ValueError('deployment target ownership mismatch')
    containers = deployment['spec']['template']['spec']['containers']
    matches = [i for i,c in enumerate(containers) if c['name'] == SERVICES[service]]
    if len(matches) != 1:
        raise ValueError('exact main app container required')
    index = matches[0]
    prefix = '/spec/template/spec/containers/' + str(index)
    patch = [{'op':'test','path':'/metadata/uid','value':meta['uid']},
             {'op':'test','path':'/metadata/resourceVersion','value':meta['resourceVersion']},
             {'op':'test','path':prefix+'/name','value':SERVICES[service]},
             {'op':'test','path':prefix+'/image','value':containers[index]['image']},
             {'op':'replace','path':prefix+'/image','value':image}]
    expected = copy.deepcopy(deployment)
    expected['spec']['template']['spec']['containers'][index]['image'] = image
    return patch, expected


class Client:
    def call(self, *args, body=None):
        payload = None if body is None else json.dumps(body).encode()
        if args and args[0] == 'patch' and '--patch-file=-' in args:
            if not isinstance(body,list):
                raise ValueError('JSON image patch required')
            # kubectl patch does not support stdin as --patch-file=-. These
            # patches contain only image and CAS metadata, never Secret values.
            args = tuple('--patch='+json.dumps(body) if arg == '--patch-file=-' else arg for arg in args)
            payload = None
        command = ['kubectl','--namespace',NS,*args]
        try:
            result = subprocess.run(command,input=payload,capture_output=True,timeout=660)
        except (OSError,subprocess.TimeoutExpired) as exc:
            raise RuntimeError('kubectl transport unavailable or timed out') from exc
        if result.returncode:
            # Kubectl errors can echo complete patches/specs. Keep output finite.
            raise RuntimeError('kubectl operation failed: ' + args[0])
        return json.loads(result.stdout) if result.stdout.strip().startswith(b'{') else {}


def acquire_lock(client):
    lock = {'apiVersion':'v1','kind':'ConfigMap',
            'metadata':{'name':LOCK,'namespace':NS,'labels':dict(LABELS)},
            'data':{'operation':secrets.token_hex(16)}}
    # Atomic CREATE, never apply/replace an existing lock.
    created = client.call('create','-f','-','-o','json',body=lock)
    if not created.get('metadata',{}).get('uid') or not created['metadata'].get('resourceVersion'):
        raise RuntimeError('lock creation outcome uncertain; inspect dev lock')
    return created


def release_lock(client, lock):
    meta = lock['metadata']
    options = {'apiVersion':'v1','kind':'DeleteOptions',
               'preconditions':{'uid':meta['uid'],'resourceVersion':meta['resourceVersion']}}
    client.call('delete','--raw=/api/v1/namespaces/'+NS+'/configmaps/'+LOCK,'-f','-',body=options)


def verify_ownership(client, lock=None):
    validate_namespace(client.call('get','namespace',NS,'-o','json'))
    if lock is not None:
        current = client.call('get','configmap',LOCK,'-o','json')
        if (current['metadata']['uid'] != lock['metadata']['uid'] or
                current.get('data') != lock.get('data')):
            raise RuntimeError('dev update lock ownership changed')


def update_images(client, images, apply=False):
    verify_ownership(client)
    lock = acquire_lock(client) if apply else None
    mutation_started = False
    success = False
    try:
        plans = []
        for service,image in images.items():
            deployment = client.call('get','deployment','voice-'+service,'-o','json')
            patch,expected = plan_patch(service,image,deployment)
            if expected['spec'] == deployment['spec']:
                print('DEV_IMAGE_UNCHANGED service='+service)
                continue
            client.call('patch','deployment','voice-'+service,'--type=json','--patch-file=-',
                        '--dry-run=server','-o','json',body=patch)
            plans.append((service,patch,expected))
            print('DEV_IMAGE_PLANNED service='+service+' image='+image)
        if not apply:
            print('DEV_IMAGE_PREVIEW_COMPLETE count='+str(len(plans)))
            return
        for service,patch,expected in plans:
            verify_ownership(client,lock)
            mutation_started = True
            result = client.call('patch','deployment','voice-'+service,'--type=json','--patch-file=-','-o','json',body=patch)
            if result.get('spec') != expected['spec']:
                raise RuntimeError('deployment specification changed beyond the selected image')
            client.call('rollout','status','deployment/voice-'+service,'--timeout=600s')
            actual = client.call('get','deployment','voice-'+service,'-o','json')
            if actual.get('metadata',{}).get('uid') != expected['metadata']['uid'] or actual.get('spec') != expected['spec']:
                raise RuntimeError('deployment changed during rollout')
            print('DEV_IMAGE_READY service='+service+' image='+images[service])
        success = True
    finally:
        if lock is not None:
            if success or not mutation_started:
                release_lock(client,lock)
            else:
                print('DEV_IMAGE_LOCK_RETAINED name='+LOCK, file=sys.stderr)


def main():
    parser = argparse.ArgumentParser(description=__doc__,formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument('--namespace',default=NS,choices=(NS,))
    parser.add_argument('--image',action='append',required=True,metavar='SERVICE=IMAGE')
    parser.add_argument('--apply',action='store_true')
    args = parser.parse_args()
    try:
        images = parse_images(args.image)
        update_images(Client(),images,apply=args.apply)
    except (ValueError,KeyError,TypeError,RuntimeError):
        print('Dev image update failed; inspect dev workload and retained lock metadata',file=sys.stderr)
        return 1
    return 0


if __name__=='__main__':
    sys.exit(main())
