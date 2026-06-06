-- Complaints, staff escalations, platform settings, freelance moderation

CREATE TABLE IF NOT EXISTS user_complaints (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reporter_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    target_type VARCHAR(32) NOT NULL,
    target_id UUID NOT NULL,
    reason TEXT NOT NULL,
    details TEXT DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT 'open',
    reviewed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    resolution TEXT DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_complaints_status ON user_complaints(status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_complaints_target ON user_complaints(target_type, target_id);

CREATE TABLE IF NOT EXISTS staff_tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    assigned_to UUID REFERENCES users(id) ON DELETE SET NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    entity_type VARCHAR(64),
    entity_id UUID,
    status VARCHAR(32) NOT NULL DEFAULT 'open',
    resolution TEXT DEFAULT '',
    resolved_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_staff_tasks_status ON staff_tasks(status, created_at DESC);

CREATE TABLE IF NOT EXISTS platform_settings (
    key VARCHAR(128) PRIMARY KEY,
    value JSONB NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by UUID REFERENCES users(id) ON DELETE SET NULL
);

INSERT INTO platform_settings (key, value) VALUES
    ('platform', '{"name":"Steppy","support_email":"support@steppy.kz","freelance_commission_percent":10,"vacancy_auto_publish":false}'::jsonb)
ON CONFLICT (key) DO NOTHING;

ALTER TABLE freelance_tasks
    ADD COLUMN IF NOT EXISTS moderation_submitted_at TIMESTAMPTZ;

ALTER TABLE freelance_disputes
    ADD COLUMN IF NOT EXISTS escalated_at TIMESTAMPTZ;
