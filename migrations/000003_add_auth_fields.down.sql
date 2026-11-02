BEGIN;
DROP INDEX users_email_lower_key;
ALTER TABLE users DROP COLUMN password_hash;
-- Do not drop role: it belongs to the inherited Lesson 8 migration.
COMMIT;
