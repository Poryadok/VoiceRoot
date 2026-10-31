"""Build a private Git index for a captured source snapshot."""
import os
from pathlib import Path
import re
import subprocess

import local_source_snapshot


def prepare(root, source, snapshot, directory):
    root, snapshot, directory = (Path(value).resolve() for value in (root, snapshot, directory))
    if (directory.exists() or directory == root or root in directory.parents or
            directory == snapshot or snapshot in directory.parents or root == snapshot or root in snapshot.parents):
        raise ValueError('private Git directory and snapshot must be new and outside the checkout')
    if not isinstance(source, dict) or not re.fullmatch(r'[0-9a-f]{40}', source.get('tree', '')):
        raise ValueError('captured source tree identity is required')
    local_source_snapshot.verify_materialized(root, source, snapshot)
    env = {key: value for key, value in os.environ.items() if not key.upper().startswith('GIT_')}
    config = subprocess.run(['git', '-C', str(root), 'config', '--get', 'core.autocrlf'], env=env,
                            capture_output=True, text=True, timeout=30, check=False)
    if config.returncode not in (0, 1) or config.stdout.strip().lower() not in ('', 'true', 'false', 'input'):
        raise ValueError('source Git line ending configuration is invalid')
    autocrlf = config.stdout.strip().lower() or 'false'
    env.update(GIT_CONFIG_NOSYSTEM='1', GIT_CONFIG_GLOBAL=os.devnull,
               GIT_AUTHOR_NAME='Local CI fixture', GIT_AUTHOR_EMAIL='fixture@example.invalid',
               GIT_COMMITTER_NAME='Local CI fixture', GIT_COMMITTER_EMAIL='fixture@example.invalid')
    common = subprocess.check_output(['git', '-C', str(root), 'rev-parse', '--git-common-dir'],
                                     env=env, text=True, timeout=30).strip()
    objects = (root / common / 'objects').resolve(strict=True)
    if not objects.is_dir() or any(character in str(objects) for character in '\r\n'):
        raise ValueError('source Git object directory is invalid')
    subprocess.run(['git', 'init', '--bare', str(directory)], env=env, stdout=subprocess.PIPE,
                   stderr=subprocess.PIPE, timeout=30, check=True)
    subprocess.run(['git', '--git-dir', str(directory), 'config', 'core.autocrlf', autocrlf],
                   env=env, timeout=30, check=True)
    (directory / 'objects/info/alternates').write_bytes((objects.as_posix() + '\n').encode('utf-8'))
    env.update(GIT_DIR=str(directory), GIT_WORK_TREE=str(snapshot))
    commit = subprocess.check_output(['git', 'commit-tree', source['tree']], input=b'private snapshot baseline\n',
                                     env=env, timeout=30).decode('ascii').strip()
    subprocess.run(['git', 'update-ref', 'refs/heads/master', commit], env=env, timeout=30, check=True)
    subprocess.run(['git', 'symbolic-ref', 'HEAD', 'refs/heads/master'], env=env, timeout=30, check=True)
    subprocess.run(['git', 'read-tree', commit], env=env, timeout=30, check=True)
    actual = subprocess.check_output(['git', 'rev-parse', 'HEAD^{tree}'], env=env, text=True, timeout=30).strip()
    if actual != source['tree']:
        raise ValueError('private Git baseline differs from captured tree')
    return env
