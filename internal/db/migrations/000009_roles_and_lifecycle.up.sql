-- BRD v1.0: roles, lifecycles, messaging, notifications, audit, wallet extensions

-- Moderator role
DO $$ BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_enum e JOIN pg_type t ON e.enumtypid = t.oid
    WHERE t.typname = 'user_role' AND e.enumlabel = 'moderator'
  ) THEN
    ALTER TYPE user_role ADD VALUE 'moderator';
  END IF;
END $$;

-- Extended vacancy statuses
DO $$ BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_enum e JOIN pg_type t ON e.enumtypid = t.oid
    WHERE t.typname = 'vacancy_status' AND e.enumlabel = 'draft'
  ) THEN
    ALTER TYPE vacancy_status ADD VALUE 'draft';
  END IF;
END $$;
DO $$ BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_enum e JOIN pg_type t ON e.enumtypid = t.oid
    WHERE t.typname = 'vacancy_status' AND e.enumlabel = 'pending_review'
  ) THEN
    ALTER TYPE vacancy_status ADD VALUE 'pending_review';
  END IF;
END $$;
DO $$ BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_enum e JOIN pg_type t ON e.enumtypid = t.oid
    WHERE t.typname = 'vacancy_status' AND e.enumlabel = 'needs_revision'
  ) THEN
    ALTER TYPE vacancy_status ADD VALUE 'needs_revision';
  END IF;
END $$;
DO $$ BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_enum e JOIN pg_type t ON e.enumtypid = t.oid
    WHERE t.typname = 'vacancy_status' AND e.enumlabel = 'rejected'
  ) THEN
    ALTER TYPE vacancy_status ADD VALUE 'rejected';
  END IF;
END $$;

-- BRD subscription plans
DO $$ BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_enum e JOIN pg_type t ON e.enumtypid = t.oid
    WHERE t.typname = 'recruiter_plan' AND e.enumlabel = 'starter'
  ) THEN
    ALTER TYPE recruiter_plan ADD VALUE 'starter';
  END IF;
END $$;
DO $$ BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_enum e JOIN pg_type t ON e.enumtypid = t.oid
    WHERE t.typname = 'recruiter_plan' AND e.enumlabel = 'business'
  ) THEN
    ALTER TYPE recruiter_plan ADD VALUE 'business';
  END IF;
END $$;
DO $$ BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_enum e JOIN pg_type t ON e.enumtypid = t.oid
    WHERE t.typname = 'recruiter_plan' AND e.enumlabel = 'corporate'
  ) THEN
    ALTER TYPE recruiter_plan ADD VALUE 'corporate';
  END IF;
END $$;

-- User blocks
CREATE TABLE IF NOT EXISTS user_blocks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    blocked_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reason TEXT NOT NULL DEFAULT '',
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_user_blocks_user ON user_blocks(user_id);

-- Moderation reviews
CREATE TABLE IF NOT EXISTS moderation_reviews (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_type VARCHAR(32) NOT NULL,
    entity_id UUID NOT NULL,
    moderator_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    action VARCHAR(32) NOT NULL,
    reason VARCHAR(64),
    comment TEXT DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_moderation_entity ON moderation_reviews(entity_type, entity_id);

-- Audit log (append-only)
CREATE TABLE IF NOT EXISTS audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id UUID REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(128) NOT NULL,
    entity_type VARCHAR(64),
    entity_id UUID,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_audit_log_created ON audit_log(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_log_actor ON audit_log(actor_id);

-- Recruiter verification
CREATE TABLE IF NOT EXISTS recruiter_verifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    recruiter_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE UNIQUE,
    bin VARCHAR(12) NOT NULL DEFAULT '',
    document_key VARCHAR(512),
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    reviewed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    comment TEXT DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Student profile section visibility
CREATE TABLE IF NOT EXISTS student_profile_sections (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    skills_visibility VARCHAR(32) NOT NULL DEFAULT 'public',
    education_visibility VARCHAR(32) NOT NULL DEFAULT 'public',
    experience_visibility VARCHAR(32) NOT NULL DEFAULT 'public',
    portfolio_visibility VARCHAR(32) NOT NULL DEFAULT 'public',
    hackathons_visibility VARCHAR(32) NOT NULL DEFAULT 'public',
    reviews_visibility VARCHAR(32) NOT NULL DEFAULT 'public',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Messaging
CREATE TABLE IF NOT EXISTS conversations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    student_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    recruiter_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    context_type VARCHAR(32) NOT NULL,
    context_id UUID NOT NULL,
    context_title VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(student_id, recruiter_id, context_type, context_id)
);
CREATE INDEX IF NOT EXISTS idx_conversations_student ON conversations(student_id);
CREATE INDEX IF NOT EXISTS idx_conversations_recruiter ON conversations(recruiter_id);

CREATE TABLE IF NOT EXISTS messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    sender_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    body_enc BYTEA,
    attachment_key VARCHAR(512),
    read_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(conversation_id, created_at);

-- In-app notifications
CREATE TABLE IF NOT EXISTS notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type VARCHAR(64) NOT NULL,
    title VARCHAR(255) NOT NULL,
    body TEXT NOT NULL DEFAULT '',
    payload JSONB,
    read_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_notifications_user ON notifications(user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS notification_preferences (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    notification_type VARCHAR(64) NOT NULL,
    channel_in_app BOOLEAN NOT NULL DEFAULT true,
    channel_email BOOLEAN NOT NULL DEFAULT true,
    channel_push BOOLEAN NOT NULL DEFAULT false,
    PRIMARY KEY (user_id, notification_type)
);

-- Wallet transactions and withdrawals
CREATE TABLE IF NOT EXISTS wallet_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount_kzt NUMERIC(12,2) NOT NULL,
    type VARCHAR(32) NOT NULL,
    reference_type VARCHAR(32),
    reference_id UUID,
    status VARCHAR(32) NOT NULL DEFAULT 'completed',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_wallet_tx_user ON wallet_transactions(user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS withdrawal_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount_kzt NUMERIC(12,2) NOT NULL,
    card_last4 VARCHAR(4) NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    processed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_withdrawal_user ON withdrawal_requests(user_id);

-- Promo codes
CREATE TABLE IF NOT EXISTS promo_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code VARCHAR(32) NOT NULL UNIQUE,
    discount_percent INT NOT NULL DEFAULT 0,
    max_uses INT NOT NULL DEFAULT 1,
    uses_count INT NOT NULL DEFAULT 0,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Hackathon team join requests
CREATE TABLE IF NOT EXISTS hackathon_team_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id UUID NOT NULL REFERENCES hackathon_teams(id) ON DELETE CASCADE,
    student_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    message TEXT DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(team_id, student_id)
);

-- Hackathon scoring criteria
CREATE TABLE IF NOT EXISTS hackathon_criteria (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    hackathon_id UUID NOT NULL REFERENCES hackathons(id) ON DELETE CASCADE,
    name VARCHAR(128) NOT NULL,
    weight_percent INT NOT NULL DEFAULT 0,
    sort_order INT NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS hackathon_scores (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    hackathon_id UUID NOT NULL REFERENCES hackathons(id) ON DELETE CASCADE,
    submission_id UUID NOT NULL REFERENCES hackathon_submissions(id) ON DELETE CASCADE,
    criterion_id UUID NOT NULL REFERENCES hackathon_criteria(id) ON DELETE CASCADE,
    score NUMERIC(5,2) NOT NULL DEFAULT 0,
    UNIQUE(submission_id, criterion_id)
);

-- Vacancy extra fields for multi-step form
ALTER TABLE vacancies
  ADD COLUMN IF NOT EXISTS vacancy_type VARCHAR(32) DEFAULT 'internship',
  ADD COLUMN IF NOT EXISTS responsibilities_enc BYTEA,
  ADD COLUMN IF NOT EXISTS requirements_enc BYTEA,
  ADD COLUMN IF NOT EXISTS offers_enc BYTEA,
  ADD COLUMN IF NOT EXISTS salary_type VARCHAR(32) DEFAULT 'negotiable',
  ADD COLUMN IF NOT EXISTS salary_min NUMERIC(12,2),
  ADD COLUMN IF NOT EXISTS salary_max NUMERIC(12,2),
  ADD COLUMN IF NOT EXISTS application_deadline TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS contact_name VARCHAR(128) DEFAULT '',
  ADD COLUMN IF NOT EXISTS contact_email VARCHAR(255) DEFAULT '',
  ADD COLUMN IF NOT EXISTS moderation_submitted_at TIMESTAMPTZ;

-- Recruiter invitations quota
ALTER TABLE recruiter_profiles
  ADD COLUMN IF NOT EXISTS invitations_quota INT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS invitations_used INT NOT NULL DEFAULT 0;

-- Migrate application statuses to BRD
UPDATE applications SET status = 'new' WHERE status = 'submitted';
UPDATE applications SET status = 'viewed' WHERE status = 'reviewed';

-- Users blocked flag
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_blocked BOOLEAN NOT NULL DEFAULT false;
