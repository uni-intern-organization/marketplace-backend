-- Default platform administrator (created once; safe to re-run).
-- Login:  admin@steppy.local
-- Password: AdminChangeMe1!  (change immediately after first sign-in)
INSERT INTO users (email, password_hash, role, is_blocked)
VALUES (
    'admin@steppy.local',
    '$2a$12$XOTrcRyjZhDobacD6Zjrtu0HlNIw7q8GZQUHqBuAgW2ua.LsIVcgq',
    'admin',
    false
)
ON CONFLICT (email) DO NOTHING;
