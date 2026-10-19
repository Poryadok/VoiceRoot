#!/bin/sh
# Run inside a disposable Linux PostgreSQL fixture with PGUSER/PGPASSWORD set.
# Every database is created by this process; no application database is touched.
set -eu
root=${1:?repository root required}
mode=${2:-all}
case "$mode" in rr|race|empty|live|all) ;; *) exit 2;; esac
tmp=$(mktemp -d /tmp/automod-down.XXXXXX)
db= child= writer=
cleanup() {
  [ -z "$child" ] || kill "$child" 2>/dev/null || true
  [ -z "$writer" ] || kill "$writer" 2>/dev/null || true
  if [ -n "$db" ]; then dropdb --maintenance-db=postgres --force "$db" >/dev/null 2>&1 || true; fi
  case "$tmp" in /tmp/automod-down.*) rm -rf "$tmp";; esac
}
trap cleanup EXIT INT TERM
failures=0
sql() { psql -X -v ON_ERROR_STOP=1 -Atq -d "$db" "$@"; }
wait_file() {
  attempt=0
  while [ ! -f "$1" ]; do
    attempt=$((attempt+1));[ "$attempt" -lt 500 ] || { echo "fixture latch timeout: $1";return 1; }
    sleep 0.02
  done
}
prefix() {
  for file in "$root/src/backend/migrations/$family"/*.up.sql; do
    version=${file##*/};version=${version%%_*}
    case "$family" in
      auth_db) [ "$version" -le "$target" ] && [ "$version" != 000029 ] || continue;;
      messaging_db) [ "$version" -le 31 ] || continue;;
      moderation_db) case "$version" in 000001|000002|000004|000006) ;; 000009) [ "$target" -ge 9 ] || continue;; 000013) [ "$target" -ge 13 ] || continue;; *) continue;; esac;;
    esac
    sql -f "$file" >>"$tmp/up.log" 2>&1
    [ "$version" != "$number" ] || up=$file
  done
}
background() {
  label=$1;input=$2
  (if sql -f "$input" >"$tmp/$label.log" 2>&1;then echo 0 >"$tmp/$label.exit";else echo $? >"$tmp/$label.exit";fi) &
  child=$!
}
for testmode in rr race empty live; do
  [ "$mode" = all ] || [ "$mode" = "$testmode" ] || continue
  for selected in mod6 mod9 mod13 msg31 auth28 auth30 auth31; do
    [ "$testmode" != live ] || [ "$selected" = auth28 ] || continue
    case "$selected" in
      mod6) family=moderation_db;target=6;number=000006;table=automod_terminal_subjects;insert="INSERT INTO $table(subject_id) VALUES(gen_random_uuid())";guard='automod history prevents destructive downgrade';;
      mod9) family=moderation_db;target=9;number=000009;table=automod_subjects;insert="INSERT INTO $table(subject_id,account_id,lifecycle_revision,lifecycle_state,purge_at,deletion_cycle_id) VALUES(gen_random_uuid(),gen_random_uuid(),1,'DELETED',clock_timestamp()+interval '30 days',gen_random_uuid())";guard='cannot discard current deletion cycle authority';;
      mod13) family=moderation_db;target=13;number=000013;table=automod_lifecycle_authority;insert="INSERT INTO $table(authority_generation) VALUES(gen_random_uuid())";guard='cannot discard installed lifecycle authority or current facts';;
      msg31) family=messaging_db;target=31;number=000031;table=automod_hmac_keys;insert="INSERT INTO $table(key_id,fingerprint) VALUES('native-fixture',decode(repeat('00',32),'hex'))";guard='cannot discard automod source, key or terminal history';;
      auth28) family=auth_db;target=28;number=000028;table=moderation_terminal_subjects;insert="INSERT INTO $table(subject_id) VALUES(gen_random_uuid())";guard='moderation authority requires an explicit preserved-state downgrade';;
      auth30) family=auth_db;target=30;number=000030;table=moderation_terminal_subjects;insert="INSERT INTO $table(subject_id) VALUES(gen_random_uuid())";guard='cannot discard terminal authority generation or progress';;
      auth31) family=auth_db;target=31;number=000031;table=moderation_lifecycle_current;insert="INSERT INTO accounts(id,password_hash,type,status) VALUES(gen_random_uuid(),'native-fixture','regular','active')";guard='moderation lifecycle authority cannot be discarded';;
    esac
    candidate="down_guard_$$_${selected}_${testmode}"
    createdb --maintenance-db=postgres "$candidate"
    # A failed/unknown create never grants cleanup custody over this name.
    db=$candidate
    prefix
    down=${up%.up.sql}.down.sql
    ready="$tmp/${selected}_${testmode}.ready";go="$tmp/${selected}_${testmode}.go"
    if [ "$testmode" = rr ]; then
      cat >"$tmp/input.sql" <<SQL
BEGIN ISOLATION LEVEL REPEATABLE READ;
SELECT count(*) FROM $table;
\! touch $ready
\! while [ ! -f $go ]; do sleep 0.02; done
\i $down
COMMIT;
SQL
      background down "$tmp/input.sql";wait_file "$ready"
      sql -c "$insert"
      touch "$go";wait "$child";child=
      if [ "$(cat "$tmp/down.exit")" = 0 ] || ! grep -q 'requires READ COMMITTED' "$tmp/down.log";then
        echo "FAIL $selected pinned RR accepted/incorrect refusal";cat "$tmp/down.log";failures=$((failures+1))
      elif [ "$(sql -c "SELECT count(*) FROM $table")" != 1 ];then echo "FAIL $selected RR lost history";failures=$((failures+1));else echo "PASS $selected pinned RR refuses before DDL; history retained";fi
    elif [ "$testmode" = race ];then
      cat >"$tmp/writer.sql" <<SQL
BEGIN;
$insert;
\! touch $ready
\! while [ ! -f $go ]; do sleep 0.02; done
COMMIT;
SQL
      sql -f "$tmp/writer.sql" >"$tmp/writer.log" 2>&1 & writer=$!
      wait_file "$ready"
      printf "SET default_transaction_isolation='read committed';\n\\i %s\n" "$down" >"$tmp/input.sql"
      PGAPPNAME="${db}_down";export PGAPPNAME
      background down "$tmp/input.sql"
      attempt=0
      while [ "$(sql -c "SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE application_name='${db}_down' AND wait_event_type='Lock')")" != t ];do
        attempt=$((attempt+1));[ "$attempt" -lt 500 ] || { echo 'fixture SQL lock wait missing';exit 1; };sleep 0.02
      done
      unset PGAPPNAME
      touch "$go";wait "$writer";writer=;wait "$child";child=
      if [ "$(cat "$tmp/down.exit")" = 0 ] || ! grep -q "$guard" "$tmp/down.log";then
        echo "FAIL $selected fresh RC missed first writer";cat "$tmp/down.log";failures=$((failures+1))
      elif [ "$(sql -c "SELECT count(*) FROM $table")" != 1 ];then echo "FAIL $selected RC lost history";failures=$((failures+1));else echo "PASS $selected fresh RC sees committed first writer";fi
    elif [ "$testmode" = live ];then
      sql -c "INSERT INTO accounts(id,password_hash,type,status) VALUES(gen_random_uuid(),'native-fixture','regular','active')"
      printf "\\i %s\n" "$down" >"$tmp/input.sql"
      background down "$tmp/input.sql";wait "$child";child=
      if [ "$(cat "$tmp/down.exit")" = 0 ] || ! grep -q "$guard" "$tmp/down.log";then
        echo 'FAIL auth28 discarded current LIVE subject at moderation revision0';cat "$tmp/down.log";failures=$((failures+1))
      elif [ "$(sql -c 'SELECT count(*) FROM accounts WHERE moderation_subject_id IS NOT NULL')" != 1 ];then echo 'FAIL auth28 lost LIVE binding';failures=$((failures+1));else echo 'PASS auth28 current LIVE binding retained';fi
    else
      sql -f "$down" >"$tmp/down.log" 2>&1
      sql -f "$up" >>"$tmp/down.log" 2>&1
      echo "PASS $selected empty RC DOWN/UP"
    fi
    dropdb --maintenance-db=postgres --force "$db";db=
  done
done
echo "failures=$failures"
[ "$failures" = 0 ]
