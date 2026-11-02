BEGIN;
INSERT INTO categories(name) VALUES ('Programming'), ('Fiction'), ('Science');
INSERT INTO authors(full_name, bio) VALUES
    ('Alan A. A. Donovan', 'Go author'),
    ('Brian W. Kernighan', 'Computer scientist'),
    ('Jon Bodner', 'Go author'),
    ('Ursula K. Le Guin', 'Fiction author'),
    ('Carl Sagan', 'Science author');
INSERT INTO books(title, isbn, category_id, published_year, description)
SELECT d.title, d.isbn, c.id, d.year, 'Lesson 8 demo data'
FROM (VALUES
    ('The Go Programming Language', '9780000000001', 'Programming', 2015),
    ('Learning Go', '9780000000002', 'Programming', 2024),
    ('Go Workshop', '9780000000003', 'Programming', 2026),
    ('The C Programming Language', '9780000000004', 'Programming', 1988),
    ('A Wizard of Earthsea', '9780000000005', 'Fiction', 1968),
    ('The Left Hand of Darkness', '9780000000006', 'Fiction', 1969),
    ('The Dispossessed', '9780000000007', 'Fiction', 1974),
    ('Cosmos', '9780000000008', 'Science', 1980),
    ('Pale Blue Dot', '9780000000009', 'Science', 1994),
    ('The Demon-Haunted World', '9780000000010', 'Science', 1995)
) AS d(title, isbn, category, year) JOIN categories c ON c.name = d.category;
INSERT INTO book_authors(book_id, author_id)
SELECT b.id, a.id FROM books b
JOIN (VALUES
    ('9780000000001', 'Alan A. A. Donovan'),
    ('9780000000001', 'Brian W. Kernighan'),
    ('9780000000002', 'Jon Bodner'),
    ('9780000000003', 'Jon Bodner'),
    ('9780000000004', 'Brian W. Kernighan'),
    ('9780000000005', 'Ursula K. Le Guin'),
    ('9780000000006', 'Ursula K. Le Guin'),
    ('9780000000007', 'Ursula K. Le Guin'),
    ('9780000000008', 'Carl Sagan'),
    ('9780000000009', 'Carl Sagan'),
    ('9780000000010', 'Carl Sagan')
) AS d(isbn, full_name) ON d.isbn = b.isbn
JOIN authors a ON a.full_name = d.full_name;
INSERT INTO copies(book_id, inventory_no, status)
SELECT b.id, 'DEMO-' || b.isbn || '-' || n,
    CASE WHEN n = 3 THEN 'borrowed' ELSE 'available' END
FROM books b CROSS JOIN generate_series(1, 3) AS n
WHERE b.isbn IN ('9780000000001','9780000000002','9780000000003','9780000000004','9780000000005',
                 '9780000000006','9780000000007','9780000000008','9780000000009','9780000000010');
INSERT INTO users(email, name, role) VALUES ('demo-reader@example.com', 'Demo Reader', 'reader');
COMMIT;
