"""Audit a rendered Compose model before an isolated local CI run.

This only rejects known shared-resource hazards; it neither starts a stack nor
establishes resource ownership or guarantees that callers use this audit.
"""
import argparse
import json
from pathlib import Path
import re
import sys


def _inside(path, root):
    try:
        Path(path).resolve().relative_to(root)
        return True
    except (OSError, RuntimeError, ValueError):
        return False


def audit(config, project, checkout, scratch):
    if not isinstance(config, dict) or not re.fullmatch(r'voiceci-[0-9a-f]{32}', project):
        raise ValueError('invalid Compose model or run project')
    checkout, scratch = Path(checkout).resolve(), Path(scratch).resolve()
    if (not checkout.is_dir() or not scratch.is_dir()
            or scratch != checkout / 'tmp' / 'ci-local' / project):
        raise ValueError('scratch must be the run directory under checkout/tmp/ci-local')
    services = config.get('services')
    volumes = config.get('volumes', {})
    networks = config.get('networks', {})
    if (not isinstance(services, dict) or not services or not isinstance(volumes, dict)
            or not isinstance(networks, dict)):
        raise ValueError('invalid rendered Compose resources')
    findings = []
    if config.get('name') != project:
        findings.append('project name differs from the run')
    for kind, resources in (('volume', volumes), ('network', networks)):
        for key, resource in resources.items():
            if not isinstance(key, str) or not re.fullmatch(r'[a-zA-Z0-9_.-]+', key) or not isinstance(resource, dict):
                raise ValueError('invalid rendered resource')
            allowed_driver = 'local' if kind == 'volume' else 'bridge'
            if (resource.get('external') or not isinstance(resource.get('name'), str)
                    or not resource['name'].startswith(project + '_') or resource.get('driver_opts')
                    or resource.get('driver') not in (None, allowed_driver)):
                findings.append(kind + ' ' + key + ' is not project-owned')
    for kind in ('secrets', 'configs'):
        resources = config.get(kind, {})
        if not isinstance(resources, dict):
            raise ValueError('invalid rendered ' + kind)
        for key, resource in resources.items():
            if not isinstance(key, str) or not re.fullmatch(r'[a-zA-Z0-9_.-]+', key) or not isinstance(resource, dict):
                raise ValueError('invalid rendered ' + kind + ' entry')
            source = resource.get('file')
            if (resource.get('external') or not isinstance(source, str) or not Path(source).is_absolute()
                    or not (_inside(source, checkout) or _inside(source, scratch))):
                findings.append(kind + ' ' + key + ' is not a scoped file')
    for service, value in services.items():
        if not isinstance(service, str) or not re.fullmatch(r'[a-zA-Z0-9_.-]+', service) or not isinstance(value, dict):
            raise ValueError('invalid rendered service')
        def reject(reason):
            findings.append('service ' + service + ': ' + reason)
        if value.get('container_name') or value.get('privileged') or value.get('use_api_socket'):
            reject('explicit container name or privileged Docker access')
        if value.get('devices') or value.get('cap_add') or value.get('pid') or value.get('ipc'):
            reject('host device, capability or namespace access')
        if any(value.get(field) for field in ('userns_mode', 'uts', 'cgroup', 'cgroup_parent', 'runtime',
                                               'security_opt', 'device_cgroup_rules',
                                               'gpus', 'deploy', 'provider', 'pre_start',
                                               'post_start', 'pre_stop')):
            reject('unverified runtime, security or lifecycle configuration')
        if value.get('external_links') or value.get('volumes_from'):
            reject('external links or inherited volumes')
        mode = value.get('network_mode')
        if mode and mode != 'none' and not (isinstance(mode, str) and mode.startswith('service:')
                                           and mode.removeprefix('service:') in services):
            reject('network mode is outside this project')
        image = value.get('image', '')
        if not isinstance(image, str):
            raise ValueError('invalid image name')
        local_image = image.rsplit('/', 1)[-1].startswith('voice-')
        if image and (value.get('build') or local_image) and not image.endswith(':' + project):
            reject('build or local image tag is shared')
        build = value.get('build')
        if build:
            if (not isinstance(build, dict) or not isinstance(build.get('context'), str)
                    or not Path(build['context']).is_absolute() or not _inside(build['context'], checkout)
                    or build.get('additional_contexts') or build.get('cache_to')
                    or build.get('network') not in (None, 'default', 'none')
                    or build.get('privileged') or build.get('entitlements') or build.get('ssh')
                    or build.get('secrets')
                    or any(not isinstance(tag, str) or not tag.endswith(':' + project)
                           for tag in build.get('tags', []))):
                reject('build context or output is not scoped to this run')
            elif build.get('dockerfile') is not None:
                dockerfile = build['dockerfile']
                context = Path(build['context']).resolve()
                if (not isinstance(dockerfile, str) or not dockerfile or
                        not _inside(context / dockerfile, context)):
                    reject('Dockerfile is outside the build context')
        ports = value.get('ports', [])
        mounts = value.get('volumes', [])
        if not isinstance(ports, list) or not isinstance(mounts, list):
            raise ValueError('invalid rendered ports or mounts')
        for port in ports:
            if (not isinstance(port, dict) or port.get('host_ip') != '127.0.0.1'
                    or type(port.get('target')) is not int or not 1 <= port['target'] <= 65535
                    or port.get('protocol', 'tcp') not in ('tcp', 'udp')
                    or str(port.get('published', '')) not in ('', '0')):
                reject('host port is fixed or not loopback-only')
        for mount in mounts:
            if not isinstance(mount, dict):
                raise ValueError('invalid rendered mount')
            kind, source, target = mount.get('type'), mount.get('source'), mount.get('target')
            if kind == 'volume':
                if not isinstance(source, str) or source not in volumes:
                    reject('volume is not declared by this project')
            elif kind == 'bind':
                if (not isinstance(source, str) or not isinstance(target, str)
                        or not Path(source).is_absolute() or not target.startswith('/')
                        or 'docker.sock' in source.lower() or 'docker.sock' in target.lower()
                        or not ((_inside(source, checkout) and mount.get('read_only') is True)
                                or _inside(source, scratch))):
                    reject('bind mount is outside the checkout or owned scratch')
            elif kind != 'tmpfs':
                reject('unsupported mount type')
    return sorted(set(findings))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--project', required=True)
    parser.add_argument('--checkout', required=True)
    parser.add_argument('--scratch', required=True)
    args = parser.parse_args()
    try:
        findings = audit(json.load(sys.stdin), args.project, args.checkout, args.scratch)
        for item in findings:
            print(item, file=sys.stderr)
        if findings:
            return 1
    except (OSError, ValueError, TypeError, AttributeError) as error:
        print('Compose isolation audit failed: ' + str(error), file=sys.stderr)
        return 1
    print('Compose model passed scoped isolation checks')
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
