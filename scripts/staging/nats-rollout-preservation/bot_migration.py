"""Fixed Bot004 forward constraint prerequisite; no row changes or repair."""
class PrerequisiteError(ValueError):pass

# Bot004 includes empty strings, so pre-Bot007 schemas must veto those collisions.
# Bot007 excludes legacy empty message tokens; nonempty interaction identities
# remain unique under every supported schema.
SQL="""BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY;
SET LOCAL statement_timeout = '30s';
SELECT json_build_object('database',current_database(),
 'version',(SELECT version FROM schema_migrations),
 'dirty',(SELECT dirty FROM schema_migrations),
 'duplicate_token_groups',(SELECT count(*) FROM
   (SELECT bot_id,interaction_token FROM bot_event_log
    WHERE interaction_token IS NOT NULL
      AND ((SELECT version FROM schema_migrations) < 7 OR interaction_token <> '')
    GROUP BY bot_id,interaction_token
    HAVING count(*)>1) AS conflicts));
ROLLBACK;
"""

def required(plan):
    return any(row['database']=='bot' and
        '000004_slash_interaction_outbox.up.sql' in row['files'] for row in plan)

def verify(row):
    keys={'database','version','dirty','duplicate_token_groups'}
    if (not isinstance(row,dict) or set(row)!=keys or row['database']!='bot_db'
        or type(row['version']) is not int or not 1<=row['version']<=7
        or row['dirty'] is not False or type(row['duplicate_token_groups']) is not int
        or row['duplicate_token_groups']!=0):
        raise PrerequisiteError('bot_forward_constraint_prerequisite_failed')
    return dict(row)

def capture(kube):
    import space_authority
    authority=space_authority.capture(kube)
    # Existing PostgreSQL Pod authority is validated privately; no password or
    # DSN is passed through argv or emitted by this count-only query.
    argv=['exec','voice-postgres-0','--','sh','-ceu',
        'export PGPASSWORD="$POSTGRES_PASSWORD"; exec psql -X -A -t -q -v ON_ERROR_STOP=1 '
        '-h 127.0.0.1 -U "$POSTGRES_USER" -d bot_db -c "$1"','--',SQL]
    observation=verify(kube.run(argv,timeout=40))
    if space_authority.capture(kube)!=authority:
        raise PrerequisiteError('bot_forward_database_authority_changed')
    return {'authority':authority,'observation':observation}
