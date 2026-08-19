# Connections

Alurnya:

1. docker-compose.yml → set env var DATABASE_URL
2. config.go  → baca + validasi env var
3. main.go → pgxpool.New(ctx, cfg.DatabaseURL) buat connection pool, ping, lalu pass dbPool ke repository layer

Kalau mau ganti DB credentials / host, edit di 
docker-compose.yml

 environment section, atau .env file kalau pakai env_file:.

# Credential

admin
user:postgres
pass:1818

user app
user:app_user
pass:1818

# Access
psql -h localhost -p 5432 -U postgres -d cms

psql -h localhost -p 5432 -U app_user -d cms

# Create DB

psql -U postgres -d postgres

//Akan muncul promt untuk input

CREATE DATABASE crown_db;
\c crown_db

# Create Table

psql -h localhost -p 5432 -U postgres -d cms -v ON_ERROR_STOP=1 -f "backend\migrations\001_create_roles_vendors_users.sql"

# Check Table

psql -U postgres -d cms
\dt

# Drop Table
 psql -U postgres -d cms -c "DROP TABLE IF EXISTS atms, currency, denoms;"

# Inject seed

psql -h localhost -p 5432 -U postgres -d crown_db -v ON_ERROR_STOP=1 -f "C:\Users\RB Yudha Rangga\OneDrive\Documents\Development\CMS2\backend\migrations\002_seed_sample_data.sql"

# Create DB User & Grant Previlegde

CREATE USER app_user WITH PASSWORD '1818';
GRANT CONNECT ON DATABASE crown_db TO app_user;

\c crown_db

GRANT USAGE ON SCHEMA public TO app_user;

GRANT SELECT, INSERT, UPDATE, DELETE
ON TABLE roles, vendors, users
TO app_user;

GRANT USAGE, SELECT
ON ALL SEQUENCES IN SCHEMA public
TO app_user;

ALTER DEFAULT PRIVILEGES IN SCHEMA public
GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO app_user;

ALTER DEFAULT PRIVILEGES IN SCHEMA public
GRANT USAGE, SELECT ON SEQUENCES TO app_user;

* create db
psql -h localhost -p 5432 -U postgres -v ON_ERROR_STOP=1 -f "C:\Users\RB Yudha Rangga\OneDrive\Documents\Development\CMS2\backend\migrations\create_db_cms.sql"       

* create table
psql -h localhost -p 5432 -U postgres -d cms -v ON_ERROR_STOP=1 -f "C:\Users\RB Yudha Rangga\OneDrive\Documents\Development\CMS2\backend\migrations\002_cms_tables.sql"