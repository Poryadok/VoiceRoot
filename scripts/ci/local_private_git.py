"""Build a private Git index for a captured source snapshot."""
import os
from pathlib import Path
import re
import subprocess

import local_source_snapshot


def prepare(root, source, snapshot, directory, *, runner=None):
    root, snapshot, directory = (Path(value).resolve() for value in (root, snapshot, directory))
    if (directory.exists() or directory == root or root in directory.parents or
            directory == snapshot or snapshot in directory.parents or root == snapshot or root in snapshot.parents):
        raise ValueError('private Git directory and snapshot must be new and outside the checkout')
    if not isinstance(source, dict) or not re.fullmatch(r'[0-9a-f]{40}', source.get('tree', '')):
        raise ValueError('captured source tree identity is required')
    local_source_snapshot.verify_materialized(root, source, snapshot)
    env = {key: value for key, value in os.environ.items() if not key.upper().startswith('GIT_')}
    def execute(command, *, input=None):
        if runner is not None:
            return runner(command, env=env, input=input, timeout=30)
        return subprocess.run(command, env=env, input=input, timeout=30, capture_output=True, check=False)
    config = execute(['git', '-C', str(root), 'config', '--get', 'core.autocrlf'])
    value = config.stdout.decode('utf-8').strip().lower()
    if config.returncode not in (0, 1) or value not in ('', 'true', 'false', 'input'):
        raise ValueError('source Git line ending configuration is invalid')
    autocrlf = value or 'false'
    env.update(GIT_CONFIG_NOSYSTEM='1', GIT_CONFIG_GLOBAL=os.devnull,
               GIT_AUTHOR_NAME='Local CI fixture', GIT_AUTHOR_EMAIL='fixture@example.invalid',
               GIT_COMMITTER_NAME='Local CI fixture', GIT_COMMITTER_EMAIL='fixture@example.invalid')
    common_result = execute(['git', '-C', str(root), 'rev-parse', '--git-common-dir'])
    common_result.check_returncode()
    common = common_result.stdout.decode('utf-8').strip()
    objects = (root / common / 'objects').resolve(strict=True)
    if not objects.is_dir() or any(character in str(objects) for character in '\r\n'):
        raise ValueError('source Git object directory is invalid')
    execute(['git', 'init', '--bare', str(directory)]).check_returncode()
    execute(['git', '--git-dir', str(directory), 'config', 'core.autocrlf', autocrlf]).check_returncode()
    (directory / 'objects/info/alternates').write_bytes((objects.as_posix() + '\n').encode('utf-8'))
    env.update(GIT_DIR=str(directory), GIT_WORK_TREE=str(snapshot))
    commit_result = execute(['git', 'commit-tree', source['tree']], input=b'private snapshot baseline\n')
    commit_result.check_returncode()
    commit = commit_result.stdout.decode('ascii').strip()
    execute(['git', 'update-ref', 'refs/heads/master', commit]).check_returncode()
    execute(['git', 'symbolic-ref', 'HEAD', 'refs/heads/master']).check_returncode()
    execute(['git', 'read-tree', commit]).check_returncode()
    actual_result = execute(['git', 'rev-parse', 'HEAD^{tree}'])
    actual_result.check_returncode()
    actual = actual_result.stdout.decode('ascii').strip()
    if actual != source['tree']:
        raise ValueError('private Git baseline differs from captured tree')
    return env
