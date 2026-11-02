BEGIN;
DELETE FROM users WHERE email = 'demo-reader@example.com';
DELETE FROM books WHERE isbn IN ('9780000000001','9780000000002','9780000000003','9780000000004','9780000000005',
                                '9780000000006','9780000000007','9780000000008','9780000000009','9780000000010');
DELETE FROM authors a WHERE full_name IN ('Alan A. A. Donovan', 'Brian W. Kernighan', 'Jon Bodner', 'Ursula K. Le Guin', 'Carl Sagan')
    AND NOT EXISTS (SELECT 1 FROM book_authors ba WHERE ba.author_id = a.id);
DELETE FROM categories c WHERE name IN ('Programming', 'Fiction', 'Science')
    AND NOT EXISTS (SELECT 1 FROM books b WHERE b.category_id = c.id);
COMMIT;
