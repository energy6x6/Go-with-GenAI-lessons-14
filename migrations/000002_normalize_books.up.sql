BEGIN;
CREATE TABLE authors (
    id BIGSERIAL PRIMARY KEY,
    full_name TEXT NOT NULL CHECK (btrim(full_name) <> ''),
    bio TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE categories (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE CHECK (btrim(name) <> ''),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE books ADD COLUMN category_id BIGINT;
INSERT INTO categories(name) SELECT DISTINCT category FROM books ORDER BY category;
INSERT INTO authors(full_name) SELECT DISTINCT author FROM books ORDER BY author;
UPDATE books b SET category_id = c.id FROM categories c WHERE c.name = b.category;
ALTER TABLE books ALTER COLUMN category_id SET NOT NULL;
ALTER TABLE books ADD CONSTRAINT fk_books_category
    FOREIGN KEY(category_id) REFERENCES categories(id) ON DELETE RESTRICT;
CREATE INDEX idx_books_category_id ON books(category_id);
CREATE TABLE book_authors (
    book_id BIGINT NOT NULL,
    author_id BIGINT NOT NULL,
    PRIMARY KEY(book_id, author_id),
    CONSTRAINT fk_book_authors_book FOREIGN KEY(book_id) REFERENCES books(id) ON DELETE CASCADE,
    CONSTRAINT fk_book_authors_author FOREIGN KEY(author_id) REFERENCES authors(id) ON DELETE RESTRICT
);
CREATE INDEX idx_book_authors_author_id ON book_authors(author_id);
INSERT INTO book_authors(book_id, author_id)
    SELECT b.id, a.id FROM books b JOIN authors a ON a.full_name = b.author;
ALTER TABLE books DROP COLUMN author, DROP COLUMN category;
CREATE TABLE copies (
    id BIGSERIAL PRIMARY KEY,
    book_id BIGINT NOT NULL,
    inventory_no TEXT NOT NULL UNIQUE CHECK (btrim(inventory_no) <> ''),
    status TEXT NOT NULL DEFAULT 'available' CHECK (status IN ('available', 'borrowed', 'lost')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_copies_book FOREIGN KEY(book_id) REFERENCES books(id) ON DELETE CASCADE
);
CREATE INDEX idx_copies_book_id ON copies(book_id);
CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    email TEXT NOT NULL UNIQUE CHECK (btrim(email) <> ''),
    name TEXT,
    role TEXT NOT NULL DEFAULT 'reader' CHECK (role IN ('admin', 'librarian', 'reader')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
COMMIT;
