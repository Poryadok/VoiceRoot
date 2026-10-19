#!/usr/bin/env python3
"""Fresh, owned Compose CI installation; never repairs an existing database.

prepare must precede the first PostgreSQL startup/migration. Its newly created
volume and nonce are provenance under the trusted CI owner, not an attestation of
arbitrary previous privileged DDL. install is one-shot and uses the separately
built maintenance executable, not the serving process.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import secrets
import subprocess
from urllib.parse import quote

ROOT = Path(__file__).resolve().parents[2]
ROLE = 'voice_space_ci_app'
LABEL = 'voice.native.installation_nonce'


def application_url(password):
    return 'postgres://'+ROLE+':'+password+'@postgres:5432/space_db?sslmode=disable'


def execute(command, **kwargs):
    result = subprocess.run(command, text=True, capture_output=True, **kwargs)
    if result.returncode:
        # Commands may contain private environment values. Do not print argv or
        # captured Compose config, credentials, or an exception containing argv.
        raise RuntimeError('bootstrap operation failed: ' + str(command[0]))
    return result.stdout.strip()


def config_for(compose):
    value = json.loads(execute(compose + ['config', '--format', 'json']))
    project = value['name']
    if not re.fullmatch(r'voice-(ci|a1-multi)-[a-z0-9-]+', project):
        raise RuntimeError('an isolated CI project name is required')
    mounts = [m for m in value['services']['postgres']['volumes']
              if m.get('type') == 'volume']
    if len(mounts) != 1:
        raise RuntimeError('one owned PostgreSQL data volume is required')
    key = mounts[0]['source']
    volume = value['volumes'][key]
    if volume.get('external') or not volume.get('name', '').startswith(project + '_'):
        raise RuntimeError('external or foreign PostgreSQL volume refused')
    return value, project, key, volume['name']


def migration_hashes():
    return {p.name: hashlib.sha256(p.read_bytes()).hexdigest()
            for p in sorted((ROOT/'src/backend/migrations/space_db').glob('*.sql'))}


def private_write(path, value, exclusive=False):
    flags = os.O_WRONLY | os.O_CREAT | (os.O_EXCL if exclusive else os.O_TRUNC)
    descriptor = os.open(path, flags, 0o600)
    with os.fdopen(descriptor, 'w', encoding='utf-8') as output:
        output.write(value)
    os.chmod(path, 0o600)


def inspect_volume(volume):
    return json.loads(execute(['docker', 'volume', 'inspect', volume]))[0]


def prepare(args):
    state_path, env_path = Path(args.state), Path(args.env_file)
    if state_path.exists():
        raise RuntimeError('bootstrap state already exists; use a fresh project')
    value, project, key, volume = config_for(args.compose)
    engine = execute(['docker', 'info', '--format', '{{.ID}}'])
    if not engine:
        raise RuntimeError('Docker engine identity unavailable')
    existing = execute(['docker', 'volume', 'ls', '--format', '{{.Name}}']).splitlines()
    containers = execute(['docker', 'ps', '-aq', '--filter', 'label=com.docker.compose.project=' + project])
    if volume in existing or containers:
        raise RuntimeError('existing project/database provenance is unknown; refused')
    if execute(['docker', 'network', 'ls', '-q', '--filter', 'label=com.docker.compose.project='+project]):
        raise RuntimeError('existing project network refused')
    if 'SPACE_DATABASE_URL' in os.environ or any(line.startswith('SPACE_DATABASE_URL=') for line in env_path.read_text().splitlines()):
        raise RuntimeError('caller-supplied Space database identity refused')
    nonce, password = secrets.token_hex(16), secrets.token_hex(32)
    execute(['docker', 'volume', 'create', '--label', LABEL+'='+nonce,
             '--label', 'com.docker.compose.project='+project,
             '--label', 'com.docker.compose.volume='+key, volume])
    actual = inspect_volume(volume)
    if actual.get('Labels', {}).get(LABEL) != nonce:
        raise RuntimeError('fresh volume custody mismatch; refused')
    admin = value['services']['compose-db-init']['environment']
    admin_url = ('postgres://' + quote(admin['PGUSER'], safe='') + ':' +
                 quote(admin['PGPASSWORD'], safe='') + '@postgres:5432/space_db?sslmode=disable')
    app_url = application_url(password)
    state = {'engine': engine, 'project': project, 'volume': volume,
             'volume_created_at': actual['CreatedAt'], 'nonce': nonce,
             'admin_url': admin_url, 'app_password': password,
             'migrations': migration_hashes(), 'install_attempted': False}
    private_write(state_path, json.dumps(state), exclusive=True)
    private_write(env_path, env_path.read_text()+'\nSPACE_DATABASE_URL='+app_url+'\n')
    print('Fresh native CI volume reserved for '+project)


def install(args):
    state_path = Path(args.state)
    state = json.loads(state_path.read_text())
    value, project, _, volume = config_for(args.compose)
    actual = inspect_volume(volume)
    if (state.get('install_attempted') or state['project'] != project or
            state['volume'] != volume or actual['CreatedAt'] != state['volume_created_at'] or
            actual.get('Labels', {}).get(LABEL) != state['nonce'] or
            execute(['docker', 'info', '--format', '{{.ID}}']) != state['engine'] or
            migration_hashes() != state['migrations'] or
            value['services']['space']['environment']['DATABASE_URL'] != application_url(state['app_password'])):
        raise RuntimeError('fresh installation custody/source mismatch or repeat refused')
    state['install_attempted'] = True
    private_write(state_path, json.dumps(state))
    # The canonical migrator owns the complete initial migration sequence.
    execute(args.compose + ['up', '-d', '--build', 'compose-db-init'])
    execute(args.compose + ['wait', 'compose-db-init'])
    container = execute(args.compose + ['ps', '-a', '-q', 'compose-db-init'])
    if not container:
        raise RuntimeError('canonical migration receipt unavailable')
    migrated = json.loads(execute(['docker', 'inspect', container]))[0]
    if migrated['State']['Status'] != 'exited' or migrated['State']['ExitCode'] != 0:
        raise RuntimeError('canonical migration failed')
    postgres = execute(args.compose + ['ps', '-q', 'postgres'])
    target = json.loads(execute(['docker', 'inspect', postgres]))[0]
    if not any(m.get('Type') == 'volume' and m.get('Name') == volume for m in target['Mounts']):
        raise RuntimeError('migrated target does not use the reserved fresh volume')
    execute(args.compose + ['build', 'space'])
    installer_image = execute(['docker', 'image', 'inspect', value['services']['space'].get('image', project+'-space'), '--format', '{{.Id}}'])
    env = os.environ.copy()
    env.update(DATABASE_URL=state['admin_url'], SPACE_NATIVE_APP_PASSWORD=state['app_password'],
               SPACE_NATIVE_INSTALLATION_NONCE=state['nonce'])
    output = execute(args.compose + ['run', '--rm', '--no-deps',
                     '-e', 'DATABASE_URL', '-e', 'SPACE_NATIVE_APP_PASSWORD',
                     '-e', 'SPACE_NATIVE_INSTALLATION_NONCE', '--entrypoint',
                     '/native-membership-installer', 'space', '--trusted-fresh-installation'], env=env)
    receipt = json.loads(output)
    if receipt.get('application_role') != ROLE or receipt.get('installation_nonce') != state['nonce']:
        raise RuntimeError('installer receipt mismatch')
    receipt['project'], receipt['engine'], receipt['volume_created_at'] = project, state['engine'], state['volume_created_at']
    receipt['canonical_migrator_image'] = migrated['Image']
    receipt['installer_image'] = installer_image
    receipt['migrations'] = state['migrations']
    private_write(str(state_path)+'.receipt.json', json.dumps(receipt, indent=2))
    print('Qualified native45 installation and separate nonowner LOGIN completed for '+project)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('phase', choices=['prepare', 'install'])
    parser.add_argument('--state', required=True)
    parser.add_argument('--env-file')
    parser.add_argument('--compose', nargs=argparse.REMAINDER, required=True)
    args = parser.parse_args()
    if args.phase == 'prepare' and not args.env_file:
        parser.error('prepare requires --env-file')
    try:
        (prepare if args.phase == 'prepare' else install)(args)
    except (RuntimeError, KeyError, ValueError, OSError) as error:
        parser.exit(1, 'Native CI bootstrap refused: '+str(error)+'\n')


if __name__ == '__main__':
    main()
