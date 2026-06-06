-- Admin panel: enforcement, manual subscriptions, analytics sources

ALTER TABLE recruiter_profiles
    ADD COLUMN IF NOT EXISTS plan_started_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS activation_source TEXT NOT NULL DEFAULT 'self_serve';

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS registration_source TEXT NOT NULL DEFAULT 'direct',
    ADD COLUMN IF NOT EXISTS registration_metadata JSONB;

CREATE INDEX IF NOT EXISTS idx_login_attempts_ip ON login_attempts(ip_address, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_users_created_role ON users(role, created_at);
CREATE INDEX IF NOT EXISTS idx_recruiter_profiles_plan ON recruiter_profiles(plan, plan_expires_at);
