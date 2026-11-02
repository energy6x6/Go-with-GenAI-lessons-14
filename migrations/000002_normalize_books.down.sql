BEGIN;
DROP TABLE users;
DROP TABLE copies;
ALTER TABLE books ADD COLUMN author TEXT;
ALTER TABLE books ADD COLUMN category TEXT;
UPDATE books b SET category = c.name FROM categories c WHERE c.id = b.category_id;
UPDATE books b SET author = names.full_names
    FROM (SELECT ba.book_id, string_agg(a.full_name, ', ' ORDER BY a.id) AS full_names
          FROM book_authors ba JOIN authors a ON a.id = ba.author_id GROUP BY ba.book_id) names
    WHERE names.book_id = b.id;
-- A book without links cannot originate from the application; keep rollback explicit.
UPDATE books SET author = 'Unknown' WHERE author IS NULL;
ALTER TABLE books ALTER COLUMN author SET NOT NULL;
ALTER TABLE books ALTER COLUMN category SET NOT NULL;
ALTER TABLE books ADD CONSTRAINT books_author_check CHECK (btrim(author) <> '');
ALTER TABLE books ADD CONSTRAINT books_category_check CHECK (btrim(category) <> '');
DROP TABLE book_authors;
ALTER TABLE books DROP CONSTRAINT fk_books_category;
DROP INDEX idx_books_category_id;
ALTER TABLE books DROP COLUMN category_id;
DROP TABLE categories;
DROP TABLE authors;
COMMIT;
