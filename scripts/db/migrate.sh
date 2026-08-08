#!/usr/bin/env sh
set -eu

: "${GARFEX_ADMIN_DSN:?GARFEX_ADMIN_DSN must be set}"

project_name=${INTEGRATION_PROJECT_NAME:-garfex-integration}
network_name=${project_name}_default
script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
repository_root=${GARFEX_REPOSITORY_ROOT:-$(CDPATH='' cd -- "$script_dir/../.." && pwd)}
migrations_dir=${GARFEX_MIGRATIONS_DIR:-$repository_root/migrations}

if [ ! -d "$migrations_dir" ]; then
  printf '%s\n' "migration directory does not exist" >&2
  exit 64
fi
migrations_dir=$(CDPATH='' cd -- "$migrations_dir" && pwd)

case $(uname -s) in
  MINGW*|MSYS*) host_migrations_dir=$(cygpath -m "$migrations_dir") ;;
  *) host_migrations_dir=$migrations_dir ;;
esac

run_migrate() {
  MSYS_NO_PATHCONV=1 docker run --rm --network "$network_name" \
    -v "$host_migrations_dir:/migrations:ro" \
    migrate/migrate:v4.18.2 -path=/migrations -database "$GARFEX_ADMIN_DSN" "$@"
}

usage() {
  printf '%s\n' "usage: $0 {up|down|version|force VERSION}" >&2
  exit 64
}

case ${1-} in
  up) [ "$#" -eq 1 ] || usage; run_migrate up ;;
  down) [ "$#" -eq 1 ] || usage; run_migrate down 1 ;;
  version) [ "$#" -eq 1 ] || usage; run_migrate version ;;
  force)
    [ "$#" -eq 2 ] || usage
    case $2 in *[!0-9]*|'') usage ;; esac
    run_migrate force "$2"
    ;;
  *) usage ;;
esac
