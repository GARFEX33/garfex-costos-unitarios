#!/usr/bin/env sh
set -eu

REPOSITORY_ROOT=$(pwd)
export GARFEX_REPOSITORY_ROOT="$REPOSITORY_ROOT"
PROJECT_NAME=garfex-integration
MAIN_VOLUME_NAME=garfex-costos-unitarios_garfex_pgdata
SCRIPT=$REPOSITORY_ROOT/scripts/db/integration.sh
COMPOSE_FILE=compose.integration.yaml
SHELL_BIN=${SHELL_BIN:-/usr/bin/sh}

export POSTGRES_USER=garfex_bootstrap
export POSTGRES_PASSWORD=integration-bootstrap-password
export POSTGRES_DB=garfex_integration
export GARFEX_ADMIN_PASSWORD=integration-admin-password
export GARFEX_APP_PASSWORD=integration-app-password

cleanup() {
  "$SHELL_BIN" "$SCRIPT" cleanup >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

expect_failure() {
  if "$@" >/dev/null 2>&1; then
    printf '%s\n' "expected command to fail: $*" >&2
    exit 1
  fi
}

expect_output_without_secret() {
  output=$("$@" 2>&1) || {
    printf '%s\n' "$output" >&2
    return 1
  }
  case "$output" in
    *"$GARFEX_ADMIN_PASSWORD"*|*"$GARFEX_APP_PASSWORD"*|*"$POSTGRES_PASSWORD"*)
      printf '%s\n' "command output exposed a secret" >&2
      return 1
      ;;
  esac
}

main_volume_fingerprint() {
  docker volume inspect --format '{{.Name}}|{{.CreatedAt}}|{{.Mountpoint}}' "$MAIN_VOLUME_NAME" 2>/dev/null ||
    printf '%s\n' absent
}

future_migrations_fingerprint() {
  find "$REPOSITORY_ROOT/migrations" -maxdepth 1 -type f -name '000002_*' \
    -exec sha256sum {} \; | sort
}

main_volume_before=$(main_volume_fingerprint)
future_migrations_before=$(future_migrations_fingerprint)

expect_failure "$SHELL_BIN" "$SCRIPT" guard ""
expect_failure "$SHELL_BIN" "$SCRIPT" guard garfex-costos-unitarios
expect_failure "$SHELL_BIN" "$SCRIPT" guard garfex_pgdata
expect_failure "$SHELL_BIN" "$SCRIPT" guard unapproved-project

"$SHELL_BIN" "$SCRIPT" config
if docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" port db 5432 >/dev/null 2>&1; then
  printf '%s\n' "integration database must not publish a host port" >&2
  exit 1
fi

export GARFEX_ADMIN_DSN="postgres://garfex_admin:${GARFEX_ADMIN_PASSWORD}@db:5432/${POSTGRES_DB}?sslmode=disable"
expect_output_without_secret "$SHELL_BIN" "$SCRIPT" migrations

integration_volume=${PROJECT_NAME}_integ_pgdata
expect_failure docker volume inspect "$integration_volume"
main_volume_after=$(main_volume_fingerprint)
[ "$main_volume_before" = "$main_volume_after" ]
[ "$future_migrations_before" = "$(future_migrations_fingerprint)" ]

printf '%s\n' "integration harness checks passed"
