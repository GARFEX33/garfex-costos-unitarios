#!/bin/sh
set -eu

: "${POSTGRES_USER:?POSTGRES_USER must be set}"
: "${POSTGRES_DB:?POSTGRES_DB must be set}"
: "${GARFEX_ADMIN_PASSWORD:?GARFEX_ADMIN_PASSWORD must be set}"
: "${GARFEX_APP_PASSWORD:?GARFEX_APP_PASSWORD must be set}"

psql --set=ON_ERROR_STOP=1 \
  --set=db_name="$POSTGRES_DB" \
  --set=admin_role=garfex_admin \
  --set=admin_password="$GARFEX_ADMIN_PASSWORD" \
  --set=app_role=garfex_app \
  --set=app_password="$GARFEX_APP_PASSWORD" \
  --username "$POSTGRES_USER" \
  --dbname "$POSTGRES_DB" <<'SQL'
SELECT format(
  'CREATE ROLE %I LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE PASSWORD %L',
  :'admin_role',
  :'admin_password'
)
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = :'admin_role')
\gexec

SELECT format(
  'CREATE ROLE %I LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE PASSWORD %L',
  :'app_role',
  :'app_password'
)
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = :'app_role')
\gexec

REVOKE ALL ON DATABASE :"db_name" FROM PUBLIC;
REVOKE ALL ON SCHEMA public FROM PUBLIC;
ALTER SCHEMA public OWNER TO :"admin_role";
GRANT CONNECT, CREATE ON DATABASE :"db_name" TO :"admin_role";
GRANT CONNECT ON DATABASE :"db_name" TO :"app_role";
GRANT USAGE ON SCHEMA public TO :"app_role";
SQL
