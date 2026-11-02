BEGIN;
CREATE TABLE loans (
 id BIGSERIAL PRIMARY KEY,
 copy_id BIGINT NOT NULL REFERENCES copies(id) ON DELETE RESTRICT,
 user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
 borrowed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 due_at TIMESTAMPTZ NOT NULL,
 returned_at TIMESTAMPTZ,
 CHECK (due_at > borrowed_at),
 CHECK (returned_at IS NULL OR returned_at >= borrowed_at)
);
CREATE INDEX loans_user_id_idx ON loans(user_id);
CREATE INDEX loans_copy_id_idx ON loans(copy_id);
CREATE UNIQUE INDEX loans_one_active_per_copy ON loans(copy_id) WHERE returned_at IS NULL;
COMMIT;
