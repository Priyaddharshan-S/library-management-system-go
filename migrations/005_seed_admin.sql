-- Creates one known admin account on first startup, so you never need to
-- manually promote a user via psql again.
--
-- Login:  admin@library.local / admin123
--
-- IMPORTANT: this is a convenience for local development only. In any real
-- deployment, change this password immediately (or remove this file and
-- create your admin a safer way) — this hash is public, in this repo.
--
-- ON CONFLICT DO NOTHING means this is safe to leave in place: it only
-- inserts if that email doesn't already exist.
INSERT INTO users (name, email, password_hash, role)
VALUES (
    'System Admin',
    'admin@library.local',
    '$2b$10$Jd1kat1MPnvdY1/0tcdQ.uFWqsFYRXZ4xDDLdizwEqaHNYTTdUkqK',
    'admin'
)
ON CONFLICT (email) DO NOTHING;
