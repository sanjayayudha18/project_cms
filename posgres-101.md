# Credential

admin
user:postgres
pass:1818

user app
user:app_user
pass:1818

# Access
psql -h localhost -p 5432 -U postgres -d crown_db

psql -h localhost -p 5432 -U app_user -d crown_db

# Create DB

psql -U postgres -d postgres

//Akan muncul promt untuk input

CREATE DATABASE crown_db;
\c crown_db

# Create Table

psql -h localhost -p 5432 -U postgres -d crown_db -v ON_ERROR_STOP=1 -f "backend\migrations\001_create_roles_vendors_users.sql"

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