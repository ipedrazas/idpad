#!/usr/bin/env bash
#
# Create the role and database that idpad connects to.
#
# `task up` does not need this: the postgres container's entrypoint creates both
# from the POSTGRES_* variables the first time its volume is initialised. Use
# this when pointing idpad at a server you already run — a managed instance, a
# Homebrew install, or a shared development box.
#
# The script is idempotent; running it a second time reports what already
# exists and changes nothing.

set -euo pipefail

usage() {
    cat <<'USAGE'
Usage: scripts/create-db.sh [--drop] [--force] [--help]

Creates the idpad role and database, then makes the role the owner of the
database's public schema. Applying the migrations afterwards is a separate
step: `task migrate`.

Options:
  --drop     Drop the database and the role instead of creating them.
  --force    Skip the confirmation prompt that --drop asks for.
  --help     Show this message.

Superuser connection, used to run the DDL:
  PGHOST            server host                   (default: localhost)
  PGPORT            server port                   (default: $POSTGRES_PORT, else 5432)
  PGSUPERUSER       role with CREATEDB/CREATEROLE (default: postgres)
  PGSUPERPASSWORD   its password                  (default: unset)

What gets created, read from .env when that file exists:
  POSTGRES_USER     role to create                (default: idpad)
  POSTGRES_PASSWORD password for that role        (default: idpad)
  POSTGRES_DB       database to create            (default: idpad)

psql is used when it is on PATH; otherwise the same statements run inside a
throwaway postgres:16-alpine container, so Docker on its own is enough.

Examples:
  scripts/create-db.sh
  PGHOST=db.internal PGSUPERUSER=admin PGSUPERPASSWORD=secret scripts/create-db.sh
  scripts/create-db.sh --drop --force

USAGE
}

say() { printf '  %s\n' "$*"; }
die() { printf 'error: %s\n' "$*" >&2; exit 1; }

drop=false
force=false

while [[ $# -gt 0 ]]; do
    case "$1" in
        --drop)  drop=true ;;
        --force) force=true ;;
        --help|-h) usage; exit 0 ;;
        *) die "unknown option $1 (try --help)" ;;
    esac
    shift
done

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# .env is the same file compose and the Taskfile read, so the script targets
# whatever the rest of the stack is configured for.
if [[ -f "$root_dir/.env" ]]; then
    set -a
    # shellcheck disable=SC1091  # path is resolved at runtime
    . "$root_dir/.env"
    set +a
fi

PGHOST="${PGHOST:-localhost}"
PGPORT="${PGPORT:-${POSTGRES_PORT:-5432}}"
PGSUPERUSER="${PGSUPERUSER:-postgres}"
PGSUPERPASSWORD="${PGSUPERPASSWORD:-}"

db_user="${POSTGRES_USER:-idpad}"
db_password="${POSTGRES_PASSWORD:-idpad}"
db_name="${POSTGRES_DB:-idpad}"

psql_image="postgres:16-alpine"

# Prefer a local psql; fall back to the postgres image, which every other part
# of this repo already pulls.
psql_bin="$(command -v psql || true)"
if [[ -z "$psql_bin" ]] && ! command -v docker >/dev/null 2>&1; then
    die "neither psql nor docker is available; install either one and retry"
fi

# A container cannot reach the host through "localhost", so the Docker path
# rewrites it. --add-host covers Linux, where the name is not built in.
docker_host="$PGHOST"
case "$PGHOST" in
    localhost|127.0.0.1|::1) docker_host="host.docker.internal" ;;
esac

# Only export the password when there is one: an empty PGPASSWORD would stop
# psql consulting ~/.pgpass or using peer authentication.
if [[ -n "$PGSUPERPASSWORD" ]]; then
    export PGPASSWORD="$PGSUPERPASSWORD"
fi

# run_psql <database> [psql arguments...]
run_psql() {
    local database="$1"
    shift

    if [[ -n "$psql_bin" ]]; then
        # -w keeps a missing password an immediate error rather than a prompt
        # that would hang a script.
        "$psql_bin" -X -q -w -v ON_ERROR_STOP=1 \
            -h "$PGHOST" -p "$PGPORT" -U "$PGSUPERUSER" -d "$database" "$@"
    else
        docker run --rm -i \
            --add-host=host.docker.internal:host-gateway \
            -e PGPASSWORD="$PGSUPERPASSWORD" \
            "$psql_image" \
            psql -X -q -w -v ON_ERROR_STOP=1 \
            -h "$docker_host" -p "$PGPORT" -U "$PGSUPERUSER" -d "$database" "$@"
    fi
}

printf 'idpad database bootstrap\n'
say "server    $PGHOST:$PGPORT"
say "superuser $PGSUPERUSER"
say "client    ${psql_bin:-docker run $psql_image psql}"
printf '\n'

# One round trip doubles as the connection check and the existence probe.
# Passing the names as psql variables keeps this script out of the business of
# escaping SQL literals.
if ! state="$(run_psql postgres -tA -v dbuser="$db_user" -v dbname="$db_name" <<'SQL' 2>&1
select (exists (select 1 from pg_roles    where rolname = :'dbuser'))::int
       || ':' ||
       (exists (select 1 from pg_database where datname = :'dbname'))::int
SQL
)"; then
    die "cannot connect to PostgreSQL at $PGHOST:$PGPORT as $PGSUPERUSER.
  $state
  Check that the server is running and reachable, and that PGSUPERUSER and
  PGSUPERPASSWORD name a role allowed to create roles and databases.
  If you meant to use the containerised database instead, run: task up"
fi

role_exists="${state%%:*}"
database_exists="${state##*:}"

if [[ "$drop" == true ]]; then
    if [[ "$force" != true ]]; then
        [[ -t 0 ]] || die "--drop needs a terminal to confirm; pass --force to skip the prompt"
        printf 'This permanently deletes the database "%s" and the role "%s" on %s:%s.\n' \
            "$db_name" "$db_user" "$PGHOST" "$PGPORT"
        read -r -p "Type the database name to confirm: " reply
        [[ "$reply" == "$db_name" ]] || die "aborted"
    fi

    # `with (force)` disconnects anything still attached (PostgreSQL 13+).
    run_psql postgres -v dbname="$db_name" >/dev/null <<'SQL'
drop database if exists :"dbname" with (force);
SQL
    say "dropped database $db_name"

    run_psql postgres -v dbuser="$db_user" >/dev/null <<'SQL'
drop role if exists :"dbuser";
SQL
    say "dropped role $db_user"

    printf '\nDone.\n'
    exit 0
fi

if [[ "$role_exists" == "1" ]]; then
    say "role $db_user already exists, leaving it untouched"
    say "  (to reset its password: alter role \"$db_user\" password '…';)"
else
    # format()'s %I and %L quote the identifier and the literal correctly, so a
    # password containing quotes cannot break out of the statement.
    run_psql postgres -v dbuser="$db_user" -v dbpass="$db_password" >/dev/null <<'SQL'
select format('create role %I login password %L', :'dbuser', :'dbpass')
where not exists (select 1 from pg_roles where rolname = :'dbuser')
\gexec
SQL
    say "created role $db_user"
fi

if [[ "$database_exists" == "1" ]]; then
    say "database $db_name already exists, leaving it untouched"
else
    # CREATE DATABASE cannot run inside a transaction, so this statement is sent
    # on its own rather than batched with the grants below.
    run_psql postgres -v dbname="$db_name" -v dbuser="$db_user" >/dev/null <<'SQL'
select format('create database %I owner %I', :'dbname', :'dbuser')
where not exists (select 1 from pg_database where datname = :'dbname')
\gexec
SQL
    say "created database $db_name owned by $db_user"
fi

run_psql "$db_name" -v dbname="$db_name" -v dbuser="$db_user" >/dev/null <<'SQL'
grant all privileges on database :"dbname" to :"dbuser";

-- PostgreSQL 15 stopped granting CREATE on the public schema to PUBLIC, so the
-- role needs it explicitly whenever it does not already own the schema.
alter schema public owner to :"dbuser";
grant all on schema public to :"dbuser";
SQL
say "granted $db_user ownership of the public schema"

printf '\nDone. Connection string:\n\n'
printf '  postgres://%s:%s@%s:%s/%s?sslmode=disable\n\n' \
    "$db_user" "$db_password" "$PGHOST" "$PGPORT" "$db_name"
printf 'Next: task migrate\n'
