#!/usr/bin/env python3
"""Own one private Compose smoke catalog fixture; never print its identity."""
import os
from pathlib import Path
import re
import subprocess
import sys
import uuid


class FixtureError(Exception):
    pass


def fixture_sql(action, game_id):
    try:
        parsed = uuid.UUID(game_id)
    except (ValueError, TypeError, AttributeError):
        raise FixtureError('identity') from None
    if str(parsed) != game_id or parsed.version != 4:
        raise FixtureError('identity')
    name = 'Compose MM accept fixture ' + game_id
    marker = 'voice-compose-mm-accept:' + game_id
    if action == 'create':
        return f"""DO $fixture$
DECLARE source_config jsonb;
BEGIN
  SELECT config INTO STRICT source_config FROM games
    WHERE lower(name)=lower('MM Duo Live') AND status='active' FOR SHARE;
  INSERT INTO games(id,name,external_id,config,status)
    VALUES('{game_id}','{name}','{marker}',source_config,'active');
END $fixture$;
"""
    if action == 'archive':
        return f"""DO $fixture$
BEGIN
  -- A missing row may mean create is still delayed after a lost transport.
  -- Reserve this exact UUID as terminal. The unique key also waits for an
  -- already-inserting create; a later INSERT cannot reactivate this row.
  INSERT INTO games(id,name,external_id,config,status)
    VALUES('{game_id}','{name}','{marker}',
      COALESCE((SELECT config FROM games WHERE lower(name)=lower('MM Duo Live')
        AND status='active'), '{{}}'::jsonb), 'archived')
    ON CONFLICT(id) DO NOTHING;
  IF EXISTS(SELECT 1 FROM games WHERE id='{game_id}'
    AND (name IS DISTINCT FROM '{name}' OR external_id IS DISTINCT FROM '{marker}')) THEN
    RAISE EXCEPTION 'fixture ownership mismatch';
  END IF;
  UPDATE games SET status='archived',updated_at=clock_timestamp()
    WHERE id='{game_id}' AND name='{name}' AND external_id='{marker}';
END $fixture$;
"""
    raise FixtureError('action')


def apply(action, game_id):
    project = os.environ.get('COMPOSE_PROJECT_NAME', '')
    if not re.fullmatch(r'voice-compose-e2e-[0-9]+-[0-9]+', project):
        raise FixtureError('project')
    sql = fixture_sql(action, game_id)
    root = Path(__file__).resolve().parents[2]
    command = ['docker', 'compose', '-f', str(root / 'docker-compose.yml'),
               'exec', '-T', 'postgres', 'psql', '-X', '-qAt',
               '-v', 'ON_ERROR_STOP=1', '-U', os.environ.get('POSTGRES_USER', 'voice'),
               '-d', 'matchmaking_db']
    try:
        result = subprocess.run(command, cwd=root, input=sql, text=True,
                                capture_output=True, timeout=20, check=False)
    except (OSError, subprocess.TimeoutExpired):
        raise FixtureError('transport') from None
    if result.returncode != 0 or result.stdout.strip():
        raise FixtureError('database')


def main(argv=None):
    args = sys.argv[1:] if argv is None else argv
    try:
        if len(args) != 2:
            raise FixtureError('arguments')
        apply(*args)
    except FixtureError as error:
        print('compose_mm_fixture failed=' + str(error), file=sys.stderr)
        return 1
    return 0


if __name__ == '__main__':
    sys.exit(main())
