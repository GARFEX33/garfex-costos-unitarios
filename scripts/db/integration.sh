#!/usr/bin/env sh
set -eu

PROJECT_NAME=garfex-integration
MAIN_PROJECT_NAME=garfex-costos-unitarios
MAIN_VOLUME_NAME=garfex-costos-unitarios_garfex_pgdata
script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
repository_root=$(CDPATH='' cd -- "$script_dir/../.." && pwd)
COMPOSE_FILE=$repository_root/compose.integration.yaml

compose() {
  docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" "$@"
}

guard_project_name() {
  candidate=${1-}

  case "$candidate" in
    "$PROJECT_NAME") return 0 ;;
    ""|"$MAIN_PROJECT_NAME"|garfex_pgdata|"$MAIN_VOLUME_NAME")
      printf '%s\n' "refusing unsafe integration project or volume name" >&2
      return 1
      ;;
    *)
      printf '%s\n' "refusing unapproved integration project name" >&2
      return 1
      ;;
  esac
}

validate_config() {
  guard_project_name "$PROJECT_NAME"
  compose config --quiet
}

start() {
  validate_config
  compose up -d --wait db
}

status() {
  guard_project_name "$PROJECT_NAME"
  container_id=$(compose ps -q db)
  [ -n "$container_id" ]
  [ "$(docker inspect --format '{{.State.Health.Status}}' "$container_id")" = healthy ]
}

cleanup() {
  guard_project_name "$PROJECT_NAME"
  compose down --volumes --remove-orphans
}

expect_version() {
  expected=$1
  output=$(sh "$script_dir/migrate.sh" version 2>&1) || return 1
  printf '%s\n' "$output" | grep -q "$expected"
}

expect_no_version() {
  if sh "$script_dir/migrate.sh" version >/dev/null 2>&1; then
    printf '%s\n' "expected no applied migration" >&2
    return 1
  fi
}

assert_roles_retained() {
  compose exec -T db psql --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" \
    --tuples-only --no-align \
    --command "SELECT count(*) FROM pg_roles WHERE rolname IN ('garfex_admin', 'garfex_app');" |
    grep -q '^2$'
}

migrations() {
  temporary_migrations=$(mktemp -d "${TMPDIR:-/tmp}/garfex-migrations.XXXXXX")
  trap 'rm -rf "$temporary_migrations"; cleanup || true' EXIT HUP INT TERM
  cleanup || true
  cp "$repository_root"/migrations/000001_baseline.*.sql "$temporary_migrations"
  start
  migrate() { GARFEX_MIGRATIONS_DIR=$temporary_migrations sh "$script_dir/migrate.sh" "$@"; }
  migrate up
  expect_version '^1$'
  printf '%s\n' 'THIS IS INVALID SQL;' >"$temporary_migrations/000002_invalid.up.sql"
  printf '%s\n' 'SELECT 1;' >"$temporary_migrations/000002_invalid.down.sql"
  if migrate up; then
    printf '%s\n' "invalid migration unexpectedly succeeded" >&2
    return 1
  fi
  expect_version '^2 (dirty)$'
  rm -f "$temporary_migrations/000002_invalid.up.sql" "$temporary_migrations/000002_invalid.down.sql"
  migrate force 1
  expect_version '^1$'
  migrate down
  expect_no_version
  migrate up
  expect_version '^1$'
  migrate down
  expect_no_version
  assert_roles_retained
}

usage() {
  printf '%s\n' "usage: $0 {config|up|status|migrations|cleanup|guard PROJECT_NAME}" >&2
  exit 64
}

command=${1-}
case "$command" in
  config) validate_config ;;
  up) start ;;
  status) status ;;
  migrations) migrations ;;
  cleanup) cleanup ;;
  guard) guard_project_name "${2-}" ;;
  *) usage ;;
esac
