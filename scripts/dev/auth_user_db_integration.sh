#!/bin/sh
# Run only against the caller-owned disposable PostgreSQL fixture. No kubectl.
set -eu
: "${PGHOST:?}" "${PGPASSWORD:?}" "${AUTH_DB_PASSWORD:?}" "${USER_DB_PASSWORD:?}"
export PGUSER=voice PGDATABASE=postgres
admin() { psql -X -q -v ON_ERROR_STOP=1 "$@" >/dev/null 2>/tmp/dimas16-private-pg-error; }
admin <<'SQL'
CREATE DATABASE auth_db OWNER voice;
CREATE DATABASE user_db OWNER voice;
\connect auth_db
CREATE TABLE accounts(id serial PRIMARY KEY, marker text);
INSERT INTO accounts(marker) VALUES ('preserve-auth');
CREATE TABLE flyway_schema_history(installed_rank int);
\connect user_db
CREATE TABLE profiles(id serial PRIMARY KEY, marker text);
INSERT INTO profiles(marker) VALUES ('preserve-user');
CREATE TABLE schema_migrations(version bigint, dirty boolean);
SQL
provision=/repo/docker/postgres/provision-auth-user-runtime-roles.sql
if [ -f "$provision" ]; then admin -f "$provision"; fi
runtime() {
 role=$1; database=$2; password=$3; sql=$4
 PGUSER="$role" PGDATABASE="$database" PGPASSWORD="$password" psql -X -qAt -v ON_ERROR_STOP=1 -c "$sql" 2>/tmp/dimas16-private-pg-error
}
# This is the first baseline failure: no usable service-scoped login yet.
test "$(runtime auth_runtime auth_db "$AUTH_DB_PASSWORD" 'SELECT marker FROM accounts WHERE id=1')" = preserve-auth
test "$(runtime user_runtime user_db "$USER_DB_PASSWORD" 'SELECT marker FROM profiles WHERE id=1')" = preserve-user
# Correct credentials are essential to distinguish authorization from password failure.
if runtime auth_runtime user_db "$AUTH_DB_PASSWORD" 'SELECT 1' >/dev/null; then exit 1; fi
if runtime user_runtime auth_db "$USER_DB_PASSWORD" 'SELECT 1' >/dev/null; then exit 1; fi
runtime auth_runtime auth_db "$AUTH_DB_PASSWORD" "INSERT INTO accounts(marker) VALUES ('new'); UPDATE accounts SET marker='updated' WHERE id=2; DELETE FROM accounts WHERE id=2" >/dev/null
runtime user_runtime user_db "$USER_DB_PASSWORD" "INSERT INTO profiles(marker) VALUES ('new'); UPDATE profiles SET marker='updated' WHERE id=2; DELETE FROM profiles WHERE id=2" >/dev/null
runtime user_runtime user_db "$USER_DB_PASSWORD" 'BEGIN; CREATE TEMP TABLE user_identity_preparation(transaction_id bigint NOT NULL,profile_id uuid,account_id uuid,association_id uuid,kind text NOT NULL) ON COMMIT DROP; INSERT INTO pg_temp.user_identity_preparation VALUES(txid_current(),NULL,NULL,NULL,'"'"'ACTIVATE'"'"'); COMMIT; SELECT version FROM schema_migrations' >/dev/null
for sql in 'CREATE TABLE forbidden(id int)' 'CREATE TEMP TABLE forbidden(id int)' 'TRUNCATE accounts' 'DROP TABLE accounts' 'INSERT INTO flyway_schema_history VALUES (1)' 'SET ROLE voice'; do
 if runtime auth_runtime auth_db "$AUTH_DB_PASSWORD" "$sql" >/dev/null; then echo AUTH_PRIVILEGE_TOO_BROAD; exit 1; fi
done
if runtime user_runtime user_db "$USER_DB_PASSWORD" 'UPDATE schema_migrations SET dirty=true' >/dev/null; then echo USER_LEDGER_MUTATION_ALLOWED; exit 1; fi
admin <<'SQL'
\connect auth_db
CREATE TABLE future_auth(id serial PRIMARY KEY);
DO $$ BEGIN IF (SELECT pg_get_userbyid(relowner) FROM pg_class WHERE oid='accounts'::regclass) <> 'voice' THEN RAISE EXCEPTION 'owner changed'; END IF; END $$;
\connect user_db
CREATE TABLE future_user(id serial PRIMARY KEY);
DO $$ BEGIN IF (SELECT pg_get_userbyid(relowner) FROM pg_class WHERE oid='profiles'::regclass) <> 'voice' THEN RAISE EXCEPTION 'owner changed'; END IF; END $$;
SQL
runtime auth_runtime auth_db "$AUTH_DB_PASSWORD" 'INSERT INTO future_auth DEFAULT VALUES' >/dev/null
runtime user_runtime user_db "$USER_DB_PASSWORD" 'INSERT INTO future_user DEFAULT VALUES' >/dev/null
# Reapplying exact issuance must preserve data/owners and remain usable.
admin -f "$provision"
test "$(runtime auth_runtime auth_db "$AUTH_DB_PASSWORD" 'SELECT marker FROM accounts WHERE id=1')" = preserve-auth
if (AUTH_DB_PASSWORD="$PGPASSWORD"; export AUTH_DB_PASSWORD; admin -f "$provision"); then
 echo SHARED_ADMIN_PASSWORD_ACCEPTED; exit 1
fi
# Never silently normalize an existing privileged runtime role.
admin -c 'ALTER ROLE auth_runtime SUPERUSER'
if admin -f "$provision"; then echo PRIVILEGED_ROLE_ACCEPTED; exit 1; fi
echo AUTH_USER_DATABASE_ISOLATION_PASS
