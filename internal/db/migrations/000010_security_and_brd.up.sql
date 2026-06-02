-- Security: refresh tokens, login attempts, password reset, 2FA
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(128) NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user ON refresh_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_hash ON refresh_tokens(token_hash);

CREATE TABLE IF NOT EXISTS login_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL,
    ip_address VARCHAR(64) DEFAULT '',
    success BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_login_attempts_email ON login_attempts(email, created_at DESC);

CREATE TABLE IF NOT EXISTS password_reset_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(128) NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS user_totp (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    secret_enc BYTEA NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT false,
    verified BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Push subscriptions (BRD §10)
CREATE TABLE IF NOT EXISTS push_subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    endpoint TEXT NOT NULL,
    p256dh TEXT NOT NULL,
    auth_key TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, endpoint)
);

-- Vacancy favorites (BRD §4)
CREATE TABLE IF NOT EXISTS vacancy_favorites (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    vacancy_id UUID NOT NULL REFERENCES vacancies(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, vacancy_id)
);

-- Payment sessions (BRD §5)
CREATE TABLE IF NOT EXISTS payment_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    recruiter_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider VARCHAR(32) NOT NULL DEFAULT 'demo',
    external_id VARCHAR(256),
    amount_kzt INT NOT NULL,
    currency VARCHAR(8) NOT NULL DEFAULT 'KZT',
    purpose VARCHAR(64) NOT NULL,
    metadata JSONB DEFAULT '{}',
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_payment_sessions_recruiter ON payment_sessions(recruiter_id);

-- Saved payment methods (tokenized, no PAN)
CREATE TABLE IF NOT EXISTS payment_methods (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    recruiter_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider VARCHAR(32) NOT NULL,
    token_ref VARCHAR(256) NOT NULL,
    last4 VARCHAR(4) DEFAULT '',
    brand VARCHAR(32) DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Student profile BRD fields
ALTER TABLE student_profiles ADD COLUMN IF NOT EXISTS availability_status VARCHAR(32) DEFAULT 'open_both';
ALTER TABLE student_profiles ADD COLUMN IF NOT EXISTS github_url VARCHAR(512) DEFAULT '';
ALTER TABLE student_profiles ADD COLUMN IF NOT EXISTS linkedin_url VARCHAR(512) DEFAULT '';
ALTER TABLE student_profiles ADD COLUMN IF NOT EXISTS behance_url VARCHAR(512) DEFAULT '';
ALTER TABLE student_profiles ADD COLUMN IF NOT EXISTS course_year INT DEFAULT 0;
ALTER TABLE student_profiles ADD COLUMN IF NOT EXISTS university VARCHAR(256) DEFAULT '';

-- Vacancy BRD fields (000009 extension)
ALTER TABLE vacancies ADD COLUMN IF NOT EXISTS vacancy_type VARCHAR(32) DEFAULT 'vacancy';
ALTER TABLE vacancies ADD COLUMN IF NOT EXISTS responsibilities_enc BYTEA;
ALTER TABLE vacancies ADD COLUMN IF NOT EXISTS requirements_enc BYTEA;
ALTER TABLE vacancies ADD COLUMN IF NOT EXISTS offers_enc BYTEA;
ALTER TABLE vacancies ADD COLUMN IF NOT EXISTS salary_type VARCHAR(32) DEFAULT 'negotiable';
ALTER TABLE vacancies ADD COLUMN IF NOT EXISTS salary_min INT;
ALTER TABLE vacancies ADD COLUMN IF NOT EXISTS salary_max INT;
ALTER TABLE vacancies ADD COLUMN IF NOT EXISTS duration_months INT;
ALTER TABLE vacancies ADD COLUMN IF NOT EXISTS application_deadline TIMESTAMPTZ;
ALTER TABLE vacancies ADD COLUMN IF NOT EXISTS contact_name_enc BYTEA;
ALTER TABLE vacancies ADD COLUMN IF NOT EXISTS contact_email VARCHAR(256) DEFAULT '';
ALTER TABLE vacancies ADD COLUMN IF NOT EXISTS specialty VARCHAR(128) DEFAULT '';
ALTER TABLE vacancies ADD COLUMN IF NOT EXISTS desired_start_date DATE;

-- Public stats cache
CREATE TABLE IF NOT EXISTS platform_stats (
    key VARCHAR(64) PRIMARY KEY,
    value BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
