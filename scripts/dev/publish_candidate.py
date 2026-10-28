#!/usr/bin/env python3
"""Publish the selected DEV candidate only; never contacts a cluster or deployer."""
import argparse
import base64
from contextlib import contextmanager, nullcontext
import hashlib
import io
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import urllib.error
import urllib.parse
import urllib.request
import zipfile

PRODUCT = '31a04a471c604488bedd5b442039dfdf0434f9c3'
AUTH_USER_PRODUCT = 'f1946355a084bed4c62a602ec1dc04e2d57e52ee'
FROZEN_FILE_PRODUCT = '0d75a990dae5d9672f1b13e2ed346f8850e3c036'
FROZEN_WEB_PRODUCT = 'df00a393e03feb176439a50bfe5605f32c16b322'
MEDIA_PRODUCT = 'ec214b28747a4552eb89f4e431fc018aa82f023a'
WEB_WORKFLOW = '.github/workflows/source-web-publish.yml'
WEB_JOB_NAME = 'Publish exact source Web image'
# Each reviewed source has its own bounded consumer; never admit arbitrary SHAs.
PRODUCT_COMPONENTS = {
    PRODUCT: ('web', 'admin', 'developer-portal', 'auth', 'user',
              'notification', 'realtime', 'space-native-migrate'),
    AUTH_USER_PRODUCT: ('auth', 'user'),
    FROZEN_FILE_PRODUCT: ('file',),
    MEDIA_PRODUCT: ('file', 'story', 'messaging'),
}
MEDIA_SERVICES = ('file', 'story', 'messaging')
REPOSITORY = 'Poryadok/VoiceRoot'
REGISTRY = 'ghcr.io/poryadok/voiceroot'
WORKFLOW = '.github/workflows/dev-candidate-publish.yml'
JOB_NAME = 'Publish exact DEV candidate'
SHA = re.compile(r'[0-9a-f]{40}')
DIGEST = re.compile(r'sha256:[0-9a-f]{64}')


class Refused(ValueError):
    pass


def require(condition, code):
    if not condition:
        raise Refused(code)


def build_plan(product, publisher, run_id, attempt, *, route='dev'):
    require(route in ('dev', 'web') and isinstance(product, str) and
            (product in PRODUCT_COMPONENTS if route == 'dev' else product == FROZEN_WEB_PRODUCT), 'unselected_product')
    require(isinstance(publisher, str) and SHA.fullmatch(publisher), 'publisher_identity')
    require(all(isinstance(v, str) and re.fullmatch(r'[1-9][0-9]{0,19}', v)
                for v in (run_id, attempt)), 'run_identity')
    components = {}
    if route == 'web':
        components['web'] = {'context': 'src/frontend', 'dockerfile': 'src/frontend/Dockerfile',
            'build_args': {'VOICE_RUNTIME_CONFIG_REQUIRED': 'true', 'SOURCE_REVISION': product},
            'platform': 'linux/amd64',
            'tag': REGISTRY + '/web:source-' + product + '-' + run_id + '-' + attempt}
        return {'product_sha': product, 'publisher_sha': publisher, 'run_id': int(run_id),
                'run_attempt': int(attempt), 'components': components}
    for name in PRODUCT_COMPONENTS[product]:
        args = {}
        if name in MEDIA_SERVICES:
            context, dockerfile = 'src/backend', 'src/backend/' + name + '/Dockerfile'
        elif name == 'space-native-migrate':
            context, dockerfile = '.', 'src/backend/space/Dockerfile.native-migrate'
            args = {'SOURCE_REVISION': product}
        elif name == 'auth':
            context, dockerfile = '.', 'src/backend/auth/Dockerfile'
            args = {'VOICE_SOURCE_SHA': product}
        elif name in ('user', 'notification', 'realtime'):
            context, dockerfile = 'src/backend', 'src/backend/' + name + '/Dockerfile'
            args = {'VOICE_SOURCE_SHA': product}
        else:
            context = 'src/frontend' if name == 'web' else 'src/' + name
            dockerfile = context + '/Dockerfile'
            args = ({'VOICE_API_BASE_URL': 'https://api-dev.comrade.click',
                     'VOICE_LIVEKIT_URL': 'wss://livekit-dev.comrade.click'} if name == 'web' else
                    {'VITE_VOICE_API_BASE': 'https://api-dev.comrade.click',
                     'VITE_OAUTH_CLIENT_ID': 'voice-' + name, 'VITE_OAUTH_DISABLED': 'false'})
        component = name if name in MEDIA_SERVICES else 'dev-' + name
        components[component] = {'context': context, 'dockerfile': dockerfile,
            'build_args': args, 'platform': 'linux/amd64',
            'tag': REGISTRY + '/' + component + ':dev-' + product + '-' + run_id + '-' + attempt}
    return {'product_sha': product, 'publisher_sha': publisher, 'run_id': int(run_id),
            'run_attempt': int(attempt), 'components': components}


def validate_run(run, jobs, publisher, run_id, attempt, *, route='dev'):
    require(route in ('dev', 'web'), 'publisher_route')
    workflow, job_name = (WORKFLOW, JOB_NAME) if route == 'dev' else (WEB_WORKFLOW, WEB_JOB_NAME)
    require(run.get('id') == int(run_id) and run.get('run_attempt') == int(attempt)
            and run.get('head_sha') == publisher and run.get('event') == 'workflow_dispatch'
            and run.get('path') == workflow
            and run.get('repository', {}).get('full_name') == REPOSITORY, 'authenticated_run_binding')
    matches = [j for j in jobs.get('jobs', []) if j.get('name') == job_name]
    require(len(matches) == 1, 'unique_publisher_job')
    job = matches[0]
    require(job.get('run_id') == int(run_id) and job.get('head_sha') == publisher
            and type(job.get('id')) is int, 'authenticated_job_binding')
    return {'repository': REPOSITORY, 'workflow': workflow, 'publisher_sha': publisher,
            'run_id': int(run_id), 'run_attempt': int(attempt), 'job_id': job['id']}


def decode_manifest(raw, expected):
    require(isinstance(expected, str) and DIGEST.fullmatch(expected), 'manifest_digest_shape')
    require('sha256:' + hashlib.sha256(raw).hexdigest() == expected, 'registry_manifest_bytes')
    return json.loads(raw)


def platform_manifest(index):
    require(isinstance(index, dict) and isinstance(index.get('manifests'), list), 'OCI_index_required')
    matches = [m for m in index['manifests'] if m.get('platform', {}).get('os') == 'linux'
               and m.get('platform', {}).get('architecture') == 'amd64'
               and not m.get('platform', {}).get('variant')]
    require(len(matches) == 1 and DIGEST.fullmatch(matches[0].get('digest', '')),
            'unique_linux_amd64_manifest')
    return matches[0]['digest']


def require_absent(value):
    require(value is None, 'publication_tag_conflict')


def validate_config_identity(expected, actual):
    require(DIGEST.fullmatch(expected) and expected == actual, 'pulled_config_identity')


def verify_config_source(config, product):
    require((product in PRODUCT_COMPONENTS or product == FROZEN_WEB_PRODUCT) and config.get('config', {}).get('Labels', {}).get(
        'org.opencontainers.image.revision') == product, 'pulled_config_product_source')


def verify_output(component, files, product, expected_sql=None):
    if product == FROZEN_WEB_PRODUCT:
        require(component == 'web' and files.get('index.html') and files.get('main.dart.js'), 'output_source_or_inventory')
        scripts = b'\n'.join(v for k, v in files.items() if k.endswith('.js'))
        require(b'voice-runtime-config.json' in scripts and all(url not in scripts for url in
                (b'https://api-dev.comrade.click', b'https://api-staging.comrade.click',
                 b'wss://livekit-dev.comrade.click', b'wss://livekit-staging.comrade.click')),
                'web_runtime_config_output')
        manifest = json.loads(files.get('web-asset-manifest.json', b'{}'))
        require(manifest.get('schema') == 'voice-web-assets-v1', 'web_asset_manifest')
        for kind, prefix, suffix in (('main', 'main.', '.dart.js'), ('bootstrap', 'flutter_bootstrap.', '.js')):
            row = manifest.get(kind, {})
            sha = row.get('sha256', '')
            require(isinstance(sha, str) and re.fullmatch(r'[0-9a-f]{64}', sha), 'web_asset_digest')
            name = prefix + sha + suffix
            data = files.get(name)
            require(row.get('path') == name and isinstance(data, bytes) and row.get('bytes') == len(data)
                    and hashlib.sha256(data).hexdigest() == sha, 'web_asset_identity')
        require(manifest['bootstrap']['path'].encode() in files['index.html'], 'web_index_binding')
        require(manifest['main']['path'].encode() in files[manifest['bootstrap']['path']], 'web_bootstrap_binding')
        return {'verified': True, 'files': {k: {'sha256': hashlib.sha256(v).hexdigest(), 'bytes': len(v)}
                                          for k, v in sorted(files.items())}}
    require(product in PRODUCT_COMPONENTS, 'output_source_or_inventory')
    selected = {name if name in MEDIA_SERVICES else 'dev-' + name
                for name in PRODUCT_COMPONENTS[product]}
    require(component in selected and files, 'output_source_or_inventory')
    if component in MEDIA_SERVICES:
        require(set(files) == {'service'} and files['service'].startswith(b'\x7fELF'),
                component + '_binary_missing')
    elif component == 'dev-auth':
        with zipfile.ZipFile(io.BytesIO(files['auth.jar'])) as archive:
            require(archive.read('BOOT-INF/classes/voice-build-source.sha').decode().strip() == product,
                    'auth_embedded_source')
    elif component in ('dev-user', 'dev-notification', 'dev-realtime'):
        binary = files.get('service', b'')
        require(binary.startswith(b'\x7fELF') and product.encode() in binary,
                'compiled_owner_source_missing')
    elif component == 'dev-space-native-migrate':
        actual = {Path(k).name: v for k, v in files.items() if k.startswith('migrations/') and k.endswith('.sql')}
        binary = files.get('native-migrate', b'')
        require(expected_sql and actual == expected_sql and binary.startswith(b'\x7fELF')
                and product.encode() in binary,
                'native_SQL_or_binary_mismatch')
    else:
        require(component in ('dev-web', 'dev-admin', 'dev-developer-portal'), 'unknown_output')
        scripts = b'\n'.join(v for k, v in files.items() if k.endswith('.js'))
        require(b'https://api-dev.comrade.click' in scripts, 'frontend_DEV_API')
        if component == 'dev-web':
            require(b'wss://livekit-dev.comrade.click' in scripts, 'frontend_DEV_LiveKit')
    return {'verified': True, 'files': {k: {'sha256': hashlib.sha256(v).hexdigest(), 'bytes': len(v)}
                                      for k, v in sorted(files.items())}}


class SafeRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        require(urllib.parse.urlparse(newurl).scheme == 'https', 'registry_insecure_redirect')
        redirected = super().redirect_request(req, fp, code, msg, headers, newurl)
        if urllib.parse.urlparse(req.full_url).netloc != urllib.parse.urlparse(newurl).netloc:
            redirected.remove_header('Authorization')
        return redirected


class Registry:
    def __init__(self, actor, token):
        require(actor and token, 'registry_identity_missing')
        self.actor, self.token, self.bearers = actor, token, {}
        self.opener = urllib.request.build_opener(SafeRedirect())

    def _read(self, request, absent=False):
        try:
            with self.opener.open(request, timeout=60) as response:
                raw = response.read((2 << 20) + 1)
        except urllib.error.HTTPError as error:
            if absent and error.code == 404:
                return None
            raise Refused('registry_HTTP_' + str(error.code)) from None
        require(len(raw) <= 2 << 20, 'registry_metadata_size')
        return raw

    def get(self, repository, path, absent=False):
        for attempt in range(2):
            bearer = self._bearer(repository)
            headers = {'Authorization': 'Bearer ' + bearer,
                       'Accept': 'application/vnd.oci.image.index.v1+json, application/vnd.oci.image.manifest.v1+json, application/vnd.docker.distribution.manifest.v2+json'}
            try:
                return self._read(urllib.request.Request('https://ghcr.io/v2/' + repository + '/' + path,
                                                        headers=headers), absent=absent)
            except Refused as error:
                # No TTL assumption and no mutation retry: only one fresh
                # authorization for the same read after an actual401.
                if attempt == 0 and str(error) == 'registry_HTTP_401':
                    self.bearers.pop(repository, None)
                else:
                    raise

    def _bearer(self, repository):
        if repository not in self.bearers:
            query = urllib.parse.urlencode({'service': 'ghcr.io', 'scope': 'repository:' + repository + ':pull,push'})
            authorization = base64.b64encode((self.actor + ':' + self.token).encode()).decode()
            raw = self._read(urllib.request.Request('https://ghcr.io/token?' + query,
                                                  headers={'Authorization': 'Basic ' + authorization}))
            value = json.loads(raw)
            bearer = value.get('token') or value.get('access_token')
            require(isinstance(bearer, str) and bearer, 'registry_bearer_missing')
            self.bearers[repository] = bearer
        return self.bearers[repository]


def command(args, *, cwd=None, log=None):
    if log:
        with log.open('wb') as stream:
            result = subprocess.run(args, cwd=cwd, stdout=stream, stderr=subprocess.STDOUT, timeout=2400)
        require(result.returncode == 0, 'command_failed_' + Path(args[0]).name)
        return b''
    result = subprocess.run(args, cwd=cwd, capture_output=True, timeout=180)
    require(result.returncode == 0, 'command_failed_' + Path(args[0]).name)
    return result.stdout


def source_snapshot(root, expected):
    require(command(['git', 'rev-parse', 'HEAD'], cwd=root).decode().strip() == expected, 'checkout_identity')
    require(not command(['git', 'status', '--porcelain', '--untracked-files=all'], cwd=root).strip(), 'checkout_dirty')
    tree = command(['git', 'ls-tree', '-rz', expected], cwd=root)
    files = {}
    for row in tree.split(b'\0'):
        if not row:
            continue
        metadata, name = row.split(b'\t', 1)
        mode, kind, oid = metadata.decode().split()
        path = root / name.decode()
        require(kind == 'blob' and mode in ('100644', '100755') and path.is_file()
                and not path.is_symlink(), 'source_file_type')
        data = path.read_bytes()
        require(hashlib.sha1(b'blob ' + str(len(data)).encode() + b'\0' + data).hexdigest() == oid, 'source_blob_changed')
        files[name.decode()] = hashlib.sha256(data).hexdigest()
    return {'source_sha': expected, 'git_tree_sha256': hashlib.sha256(tree).hexdigest(), 'files': files}


def story_file_migrations(snapshot, *, include_messaging=False):
    """Record selected source SQL, without claiming live schema compatibility."""
    files = snapshot.get('files')
    require(isinstance(files, dict), 'migration_snapshot_missing')
    selected = {}
    databases = ('file_db', 'story_db', 'messaging_db') if include_messaging else ('file_db', 'story_db')
    for database in databases:
        prefix = 'src/backend/migrations/' + database + '/'
        rows = {name: value for name, value in files.items()
                if name.startswith(prefix) and name.endswith('.sql')}
        require(rows and all(re.fullmatch(r'[0-9a-f]{64}', value) for value in rows.values()),
                'migration_inventory_missing')
        for direction in ('.up.sql', '.down.sql'):
            require(any(name.endswith(direction) for name in rows), 'migration_direction_missing')
        selected.update(rows)
    return {'source_sha': snapshot['source_sha'], 'files': dict(sorted(selected.items())),
            'live_schema_qualified': False}


def pulled_output(component, runtime_ref, config_digest, product_root, registry, repository, output, *, product):
    config = registry.get(repository, 'blobs/' + config_digest)
    verify_config_source(decode_manifest(config, config_digest), product)
    command(['docker', 'pull', '--platform', 'linux/amd64', runtime_ref], log=output / (component + '-pull.log'))
    actual = command(['docker', 'image', 'inspect', runtime_ref, '--format', '{{.Id}}']).decode().strip()
    validate_config_identity(config_digest, actual)
    container = command(['docker', 'create', '--platform', 'linux/amd64', '--network', 'none',
                         '--entrypoint', '/bin/true', runtime_ref]).decode().strip()
    require(re.fullmatch(r'[a-f0-9]{64}', container), 'owned_inspection_container')
    try:
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            if component == 'dev-auth':
                paths = {'auth.jar': '/auth.jar', 'auth-roots.pem': '/etc/ssl/certs/ca-certificates.crt',
                         'auth-cacerts': '/opt/java/openjdk/lib/security/cacerts'}
            elif component in (*MEDIA_SERVICES, 'dev-user', 'dev-notification', 'dev-realtime'):
                paths = {'service': '/service'}
            elif component == 'dev-space-native-migrate':
                paths = {'native-migrate': '/usr/local/bin/native-migrate', 'migrations': '/migrations'}
            else:
                paths = {'web': '/usr/share/nginx/html'}
                if component == 'web':
                    paths['nginx.conf'] = '/etc/nginx/conf.d/default.conf'
            for target, source in paths.items():
                command(['docker', 'cp', '-L', container + ':' + source, str(root / target)])
            files = {p.relative_to(root).as_posix().removeprefix('web/'): p.read_bytes()
                     for p in root.rglob('*') if p.is_file()}
            if component == 'web':
                nginx = files.pop('nginx.conf')
                require(nginx == (product_root / 'src/frontend/docker/nginx-default.conf').read_bytes(),
                        'web_nginx_source_mismatch')
            sql = ({p.name: p.read_bytes() for p in (product_root / 'src/backend/migrations/space_db').glob('*.sql')}
                   if component == 'dev-space-native-migrate' else None)
            proof = verify_output(component, files, product, sql)
            if component == 'web':
                proof['nginx_config_sha256'] = hashlib.sha256(nginx).hexdigest()
            if component == 'dev-auth':
                # Public trust inputs only; never export a private key or credential.
                for name in ('auth-roots.pem', 'auth-cacerts'):
                    (output / name).write_bytes(files[name])
    finally:
        command(['docker', 'rm', container])
    if component == 'dev-space-native-migrate':
        command(['docker', 'run', '--rm', '--network', 'none', '--read-only', '--cap-drop', 'ALL',
                 '--security-opt', 'no-new-privileges', '--memory', '128m', runtime_ref, '--help'],
                log=output / 'native-help.log')
        proof['native_help_exit_code'] = 0
    return proof


@contextmanager
def file_builder(run_id, attempt):
    # The dedicated BuildKit container is the only executor for this one image.
    builder = 'voice-file-' + str(run_id) + '-' + str(attempt)
    command(['docker', 'buildx', 'create', '--driver', 'docker-container', '--name', builder])
    try:
        command(['docker', 'buildx', 'inspect', '--builder', builder, '--bootstrap'])
        command(['docker', 'update', '--cpus', '2', '--memory', '4g', '--memory-swap', '4g',
                 'buildx_buildkit_' + builder + '0'])
        yield builder
    finally:
        command(['docker', 'buildx', 'rm', builder])


def publish(product_root, publisher_root, output, env, *, route='dev'):
    plan = build_plan(env['PRODUCT_SHA'], env['PUBLISHER_SHA'], env['GITHUB_RUN_ID'], env['GITHUB_RUN_ATTEMPT'], route=route)
    product = plan['product_sha']
    require(env.get('GITHUB_REPOSITORY') == REPOSITORY and env.get('GITHUB_REF') == 'refs/heads/master', 'publisher_repository_or_ref')
    publisher_source = source_snapshot(publisher_root, plan['publisher_sha'])
    product_source = source_snapshot(product_root, plan['product_sha'])
    run = json.loads(command(['gh', 'api', 'repos/' + REPOSITORY + '/actions/runs/' + str(plan['run_id'])]))
    jobs = json.loads(command(['gh', 'api', 'repos/' + REPOSITORY + '/actions/runs/' + str(plan['run_id'])
                               + '/attempts/' + str(plan['run_attempt']) + '/jobs?per_page=100']))
    authority = validate_run(run, jobs, plan['publisher_sha'], str(plan['run_id']), str(plan['run_attempt']), route=route)
    output.mkdir(parents=True, exist_ok=False)
    write(output / 'product-source.json', product_source)
    write(output / 'publisher-source.json', publisher_source)
    write(output / 'publication-plan.json', plan)
    media_components = set(PRODUCT_COMPONENTS.get(product, ()))
    if {'file', 'story'} <= media_components:
        write(output / 'story-file-migrations.json', story_file_migrations(
            product_source, include_messaging='messaging' in media_components))
    registry = Registry(env['GITHUB_ACTOR'], env['GH_TOKEN'])
    for row in plan['components'].values():
        repository, tag = row['tag'].removeprefix('ghcr.io/').rsplit(':', 1)
        require_absent(registry.get(repository, 'manifests/' + tag, absent=True))
    published = []
    for component, row in plan['components'].items():
        metadata = output / (component + '-build-metadata.json')
        args = ['docker', 'buildx', 'build', '--platform', row['platform'], '--push', '--provenance=mode=max',
                '--label', 'org.opencontainers.image.revision=' + product, '--metadata-file', str(metadata),
                '-f', str(product_root / row['dockerfile']), '-t', row['tag']]
        for key, value in row['build_args'].items():
            args += ['--build-arg', key + '=' + value]
        args.append(str(product_root / row['context']))
        with (file_builder(plan['run_id'], plan['run_attempt'])
              if component in MEDIA_SERVICES else nullcontext(None)) as builder:
            if builder:
                args[3:3] = ['--builder', builder]
            write(output / (component + '-invocation.json'), {'command': args, 'product_sha': product, **authority})
            command(args, log=output / (component + '-build.log'))
        digest = json.loads(metadata.read_bytes())['containerimage.digest']
        repository, tag = row['tag'].removeprefix('ghcr.io/').rsplit(':', 1)
        raw = registry.get(repository, 'manifests/' + digest)
        index = decode_manifest(raw, digest)
        require(registry.get(repository, 'manifests/' + tag) == raw, 'published_tag_changed')
        platform = platform_manifest(index)
        manifest = decode_manifest(registry.get(repository, 'manifests/' + platform), platform)
        config_digest = manifest['config']['digest']
        reference = 'ghcr.io/' + repository + '@' + platform
        proof = pulled_output(component, reference, config_digest, product_root, registry, repository, output, product=product)
        published.append({'component': component, 'product_sha': product, 'tag': row['tag'],
            'index_digest': digest, 'platform_manifest_digest': platform, 'config_digest': config_digest,
            'reference': reference, 'build_args': row['build_args'], 'output_verification': proof,
            **({'configuration_contract': 'voice-web-origin-bound-runtime-v1'} if route == 'web' else {}), **authority})
        write(output / 'published-images.json', {'publication_complete': False, 'images': published, **authority})
    require(source_snapshot(product_root, product) == product_source, 'product_changed_during_build')
    require(source_snapshot(publisher_root, plan['publisher_sha']) == publisher_source, 'publisher_changed_during_build')
    write(output / 'published-images.json', {'publication_complete': True, 'product_sha': product,
          'images': published, 'runtime_qualified': False, 'deployment_performed': False, **authority})


def write(path, value):
    path.write_text(json.dumps(value, indent=2, sort_keys=True) + '\n', encoding='utf-8')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--product-root', required=True, type=Path)
    parser.add_argument('--publisher-root', required=True, type=Path)
    parser.add_argument('--output', required=True, type=Path)
    parser.add_argument('--route', choices=('dev', 'web'), default='dev')
    args = parser.parse_args()
    try:
        publish(args.product_root.resolve(), args.publisher_root.resolve(), args.output.resolve(), os.environ, route=args.route)
    except Exception as error:
        # Never print a request, token, redirected signed URL or raw CLI error.
        print('DEV candidate publisher failed: ' + (str(error) if isinstance(error, Refused) else type(error).__name__))
        return 1
    print('Immutable candidate publication verified; node/runtime/deployment gates remain separate')
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
