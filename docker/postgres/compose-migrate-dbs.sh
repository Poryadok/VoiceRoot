#!/bin/sh
# Apply golang-migrate for Go-owned Postgres DBs (canonical: src/backend/migrations/*).
# auth_db is migrated by Auth Flyway on boot (Path A) unless VOICE_MIGRATE_AUTH_DB=1.
set -eu

MIGRATIONS_DIR="${MIGRATIONS_DIR:-/migrations}"
PGHOST="${PGHOST:-postgres}"
PGUSER="${PGUSER:-voice}"
PGPASSWORD="${PGPASSWORD:-voice}"

export PGPASSWORD

dsn() {
  echo "postgres://${PGUSER}:${PGPASSWORD}@${PGHOST}:5432/$1?sslmode=disable"
}

psql_db() {
  psql -v ON_ERROR_STOP=1 --dbname "$1" -tAc "$2"
}

latest_version() {
  db="$1"
  v="$(ls -1 "${MIGRATIONS_DIR}/${db}"/*.up.sql 2>/dev/null | sed 's/.*\///;s/_.*//' | sort -n | tail -1)"
  echo "$v" | sed 's/^0*//'
}

baseline_version() {
  db="$1"
  case "$db" in
    space_db)
      if [ "$(psql_db space_db "SELECT to_regclass('public.space_subscriptions') IS NOT NULL")" = "t" ]; then
        echo 5
      elif [ "$(psql_db space_db "SELECT to_regclass('public.space_bans') IS NOT NULL")" = "t" ]; then
        echo 4
      elif [ "$(psql_db space_db "SELECT to_regclass('public.invites') IS NOT NULL")" = "t" ]; then
        echo 3
      elif [ "$(psql_db space_db "SELECT to_regclass('public.categories') IS NOT NULL")" = "t" ]; then
        echo 2
      elif [ "$(psql_db space_db "SELECT to_regclass('public.spaces') IS NOT NULL")" = "t" ]; then
        echo 1
      else
        echo 0
      fi
      ;;
    story_db)
      if [ "$(psql_db story_db "SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'stories' AND column_name = 'visibility_audience')")" = "t" ]; then
        echo 2
      elif [ "$(psql_db story_db "SELECT to_regclass('public.stories') IS NOT NULL")" = "t" ]; then
        echo 1
      else
        echo 0
      fi
      ;;
    moderation_db)
      if [ "$(psql_db moderation_db "SELECT EXISTS (SELECT 1 FROM pg_constraint c JOIN pg_class t ON c.conrelid = t.oid WHERE t.relname = 'reports' AND c.conname = 'reports_target_type_check' AND pg_get_constraintdef(c.oid) LIKE '%story%')")" = "t" ]; then
        echo 3
      elif [ "$(psql_db moderation_db "SELECT to_regclass('public.sanctions') IS NOT NULL")" = "t" ]; then
        echo 2
      elif [ "$(psql_db moderation_db "SELECT to_regclass('public.reports') IS NOT NULL")" = "t" ]; then
        echo 1
      else
        echo 0
      fi
      ;;
    bot_db)
      if [ "$(psql_db bot_db "SELECT to_regclass('public.bot_presence') IS NOT NULL")" = "t" ]; then
        echo 2
      elif [ "$(psql_db bot_db "SELECT to_regclass('public.bots') IS NOT NULL")" = "t" ]; then
        echo 1
      else
        echo 0
      fi
      ;;
    matchmaking_db)
      if [ "$(psql_db matchmaking_db "SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'search_sessions' AND column_name = 'nudged_at')")" = "t" ]; then
        echo 6
      elif [ "$(psql_db matchmaking_db "SELECT to_regclass('public.match_ratings') IS NOT NULL")" = "t" ]; then
        echo 5
      elif [ "$(psql_db matchmaking_db "SELECT to_regclass('public.matches') IS NOT NULL")" = "t" ]; then
        echo 4
      elif [ "$(psql_db matchmaking_db "SELECT to_regclass('public.search_sessions') IS NOT NULL")" = "t" ]; then
        echo 3
      elif [ "$(psql_db matchmaking_db "SELECT to_regclass('public.profile_game_entries') IS NOT NULL")" = "t" ]; then
        echo 2
      elif [ "$(psql_db matchmaking_db "SELECT to_regclass('public.games') IS NOT NULL")" = "t" ]; then
        echo 1
      else
        echo 0
      fi
      ;;
    role_db)
      if [ "$(psql_db role_db "SELECT to_regclass('public.roles') IS NOT NULL")" != "t" ]; then
        echo 0
      elif [ "$(psql_db role_db "SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'roles' AND column_name = 'created_by_profile_id')")" = "t" ]; then
        echo 7
      elif [ "$(psql_db role_db "SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'roles' AND column_name = 'is_default_join')")" = "t" ]; then
        echo 6
      else
        echo 1
      fi
      ;;
    *)
      if [ "$(psql_db "$db" "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='public' AND table_type='BASE TABLE'")" -gt 0 ]; then
        echo 1
      else
        echo 0
      fi
      ;;
  esac
}

migrate_db() {
  db="$1"
  dir="${MIGRATIONS_DIR}/${db}"
  if [ ! -d "$dir" ]; then
    echo "==> skip ${db}: no migrations directory"
    return 0
  fi

  local has_sm tables ver latest dirty current_ver
  has_sm="$(psql_db "$db" "SELECT to_regclass('public.schema_migrations') IS NOT NULL")"
  if [ "$has_sm" = "t" ]; then
    dirty="$(psql_db "$db" "SELECT dirty FROM schema_migrations LIMIT 1")"
    if [ "$dirty" = "t" ]; then
      current_ver="$(psql_db "$db" "SELECT version FROM schema_migrations LIMIT 1")"
      echo "==> fix dirty ${db} at v${current_ver}"
      migrate -path "$dir" -database "$(dsn "$db")" force "$current_ver"
    fi
    echo "==> migrate up: ${db}"
    migrate -path "$dir" -database "$(dsn "$db")" up
    return 0
  fi

  tables="$(psql_db "$db" "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='public' AND table_type='BASE TABLE'")"
  if [ "$tables" -eq 0 ]; then
    echo "==> migrate up (fresh): ${db}"
    migrate -path "$dir" -database "$(dsn "$db")" up
    return 0
  fi

  ver="$(baseline_version "$db")"
  latest="$(latest_version "$db")"
  if [ -z "$latest" ]; then
    echo "==> skip ${db}: no migration files"
    return 0
  fi

  if [ "$ver" -ge "$latest" ]; then
    echo "==> baseline legacy ${db} at v${latest}"
    migrate -path "$dir" -database "$(dsn "$db")" force "$latest"
    return 0
  fi

  if [ "$ver" -eq 0 ]; then
    echo "==> migrate up: ${db}"
    migrate -path "$dir" -database "$(dsn "$db")" up
    return 0
  fi

  echo "==> baseline legacy ${db} at v${ver}, then migrate up"
  migrate -path "$dir" -database "$(dsn "$db")" force "$ver"
  migrate -path "$dir" -database "$(dsn "$db")" up
}

GO_OWNED_DBS="
  chat_db messaging_db bot_db story_db
  user_db social_db file_db space_db role_db notification_db
  matchmaking_db search_db moderation_db gateway_db subscription_db
"

for db in $GO_OWNED_DBS; do
  migrate_db "$db"
done

if [ "${VOICE_MIGRATE_AUTH_DB:-}" = "1" ]; then
  echo "==> auth_db Path B (golang-migrate); set AUTH_FLYWAY_ENABLED=false for Auth"
  migrate_db auth_db
else
  echo "==> auth_db Path A: Flyway on Auth boot — skipped"
fi

echo "compose migrate OK"
