#!/bin/sh
# Linux-only command stubs: never connects to a database or starts a listener.
set -eu
tests=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
tmp=$(mktemp -d /tmp/automod-ownership.XXXXXX)
trap 'rm -rf "$tmp"' EXIT INT TERM
mkdir -p "$tmp/bin" "$tmp/root/src/backend/migrations/moderation_db"
: > "$tmp/root/src/backend/migrations/moderation_db/000006_fixture.up.sql"
cat > "$tmp/bin/createdb" <<'STUB'
#!/bin/sh
printf '%s\n' "$*" >> "$CALLS/createdb"
exit "$CREATE_EXIT"
STUB
cat > "$tmp/bin/dropdb" <<'STUB'
#!/bin/sh
printf '%s\n' "$*" >> "$CALLS/dropdb"
STUB
cat > "$tmp/bin/psql" <<'STUB'
#!/bin/sh
exit 74
STUB
chmod +x "$tmp/bin/createdb" "$tmp/bin/dropdb" "$tmp/bin/psql"
for scenario in collision unknown-create owned-create; do
  CALLS="$tmp/$scenario";export CALLS
  mkdir "$CALLS"
  case "$scenario" in collision) CREATE_EXIT=1;; unknown-create) CREATE_EXIT=73;; owned-create) CREATE_EXIT=0;; esac
  export CREATE_EXIT
  if PATH="$tmp/bin:$PATH" sh "$tests/automod-down-isolation.sh" "$tmp/root" empty > "$CALLS/output" 2>&1;then
    echo "FAIL $scenario unexpectedly succeeded";exit 1
  else status=$?;fi
  [ "$(wc -l < "$CALLS/createdb")" -eq 1 ]
  if [ "$scenario" = owned-create ];then
    [ "$status" -eq 74 ]
    [ "$(wc -l < "$CALLS/dropdb")" -eq 1 ]
    created=$(sed 's/^--maintenance-db=postgres //' "$CALLS/createdb")
    dropped=$(sed 's/^--maintenance-db=postgres --force //' "$CALLS/dropdb")
    [ "$created" = "$dropped" ]
    echo 'PASS successful create owns exact cleanup after later failure'
  else
    [ "$status" -eq "$CREATE_EXIT" ]
    if [ -s "$CALLS/dropdb" ];then
      echo "FAIL $scenario attempted cleanup without successful create";exit 1
    fi
    echo "PASS $scenario leaves unowned database untouched"
  fi
done
