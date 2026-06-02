-- Freelance marketplace tables

CREATE TABLE IF NOT EXISTS freelance_tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    recruiter_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title_enc BYTEA NOT NULL,
    description_enc BYTEA,
    category VARCHAR(128) NOT NULL DEFAULT 'general',
    budget_kzt NUMERIC(12,2) NOT NULL,
    deadline TIMESTAMPTZ NOT NULL,
    required_skills TEXT DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT 'open',
    escrow_status VARCHAR(32) NOT NULL DEFAULT 'held',
    accepted_student_id UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_freelance_tasks_status ON freelance_tasks(status);
CREATE INDEX IF NOT EXISTS idx_freelance_tasks_recruiter ON freelance_tasks(recruiter_id);

CREATE TABLE IF NOT EXISTS freelance_proposals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES freelance_tasks(id) ON DELETE CASCADE,
    student_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    message_enc BYTEA,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(task_id, student_id)
);

CREATE TABLE IF NOT EXISTS freelance_submissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES freelance_tasks(id) ON DELETE CASCADE,
    student_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    deliverable_key VARCHAR(512),
    student_note TEXT DEFAULT '',
    revision_count INT NOT NULL DEFAULT 0,
    status VARCHAR(32) NOT NULL DEFAULT 'submitted',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS freelance_reviews (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES freelance_tasks(id) ON DELETE CASCADE,
    reviewer_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rating INT NOT NULL CHECK (rating >= 1 AND rating <= 5),
    comment TEXT DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(task_id, reviewer_id)
);

CREATE TABLE IF NOT EXISTS freelance_disputes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES freelance_tasks(id) ON DELETE CASCADE,
    opened_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reason TEXT NOT NULL,
    resolution TEXT,
    status VARCHAR(32) NOT NULL DEFAULT 'open',
    resolved_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ
);
