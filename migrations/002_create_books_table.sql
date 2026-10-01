CREATE TABLE IF NOT EXISTS books (
    id SERIAL PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    author VARCHAR(255) NOT NULL,
    isbn VARCHAR(20) UNIQUE NOT NULL,
    total_copies INT NOT NULL DEFAULT 1 CHECK (total_copies >= 0),
    available_copies INT NOT NULL DEFAULT 1 CHECK (available_copies >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Speeds up case-insensitive partial-match search on title/author/isbn
CREATE INDEX IF NOT EXISTS idx_books_title ON books (LOWER(title));
CREATE INDEX IF NOT EXISTS idx_books_author ON books (LOWER(author));
CREATE INDEX IF NOT EXISTS idx_books_isbn ON books (isbn);
