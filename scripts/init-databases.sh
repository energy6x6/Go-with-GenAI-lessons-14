#!/bin/sh
set -eu
# Only runs on first initialization of an empty volume.
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" \
  --set=app_user="$APP_DB_USER" --set=app_password="$APP_DB_PASSWORD" \
  --set=app_db="$POSTGRES_DB" --set=test_db="${POSTGRES_DB}_test" \
  --set=migr_db="${POSTGRES_DB}_migr" <<'SQL'
SELECT format('CREATE ROLE %I LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE PASSWORD %L', :'app_user', :'app_password') \gexec
SELECT format('ALTER DATABASE %I OWNER TO %I', :'app_db', :'app_user') \gexec
SELECT format('ALTER SCHEMA public OWNER TO %I', :'app_user') \gexec
SELECT format('CREATE DATABASE %I OWNER %I', :'test_db', :'app_user') \gexec
SELECT format('CREATE DATABASE %I OWNER %I', :'migr_db', :'app_user') \gexec
SQL
