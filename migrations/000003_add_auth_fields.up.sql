BEGIN;
-- Role and its CHECK constraint already exist in the Lesson 8 schema.
-- Existing demo accounts remain unable to log in until registered separately.
ALTER TABLE users ADD COLUMN password_hash TEXT NOT NULL DEFAULT '';
CREATE UNIQUE INDEX users_email_lower_key ON users (lower(email));
COMMIT;
