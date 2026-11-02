BEGIN;
CREATE TABLE books (
    id BIGSERIAL PRIMARY KEY,
    title TEXT NOT NULL CHECK (btrim(title) <> ''),
    isbn TEXT NOT NULL UNIQUE CHECK (btrim(isbn) <> ''),
    author TEXT NOT NULL CHECK (btrim(author) <> ''),
    category TEXT NOT NULL CHECK (btrim(category) <> ''),
    published_year INTEGER,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
COMMIT;
