-- Hackathon BRD: extended fields, jury, materials, registration status, submission versions

ALTER TABLE hackathons ADD COLUMN IF NOT EXISTS rules_enc BYTEA;
ALTER TABLE hackathons ADD COLUMN IF NOT EXISTS registration_opens_at TIMESTAMPTZ;
ALTER TABLE hackathons ADD COLUMN IF NOT EXISTS results_announced_at TIMESTAMPTZ;
ALTER TABLE hackathons ADD COLUMN IF NOT EXISTS team_min_size INT NOT NULL DEFAULT 1;
ALTER TABLE hackathons ADD COLUMN IF NOT EXISTS team_max_size INT NOT NULL DEFAULT 5;
ALTER TABLE hackathons ADD COLUMN IF NOT EXISTS prize_type VARCHAR(32) NOT NULL DEFAULT 'none';
ALTER TABLE hackathons ADD COLUMN IF NOT EXISTS prize_breakdown JSONB NOT NULL DEFAULT '{}';
ALTER TABLE hackathons ADD COLUMN IF NOT EXISTS registration_mode VARCHAR(32) NOT NULL DEFAULT 'auto';
ALTER TABLE hackathons ADD COLUMN IF NOT EXISTS task_reveal VARCHAR(32) NOT NULL DEFAULT 'at_start';
ALTER TABLE hackathons ADD COLUMN IF NOT EXISTS task_body_enc BYTEA;
ALTER TABLE hackathons ADD COLUMN IF NOT EXISTS submission_schema JSONB NOT NULL DEFAULT '{"artifact":true,"description":true}';
ALTER TABLE hackathons ADD COLUMN IF NOT EXISTS blind_judging BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE hackathons ADD COLUMN IF NOT EXISTS winner_mode VARCHAR(32) NOT NULL DEFAULT 'auto';
ALTER TABLE hackathons ADD COLUMN IF NOT EXISTS catalog_priority INT NOT NULL DEFAULT 0;
ALTER TABLE hackathons ADD COLUMN IF NOT EXISTS public_submissions BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE hackathons ADD COLUMN IF NOT EXISTS prize_escrow_recorded BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE hackathons ADD COLUMN IF NOT EXISTS results_locked BOOLEAN NOT NULL DEFAULT false;

UPDATE hackathons SET registration_opens_at = created_at WHERE registration_opens_at IS NULL;
UPDATE hackathons SET results_announced_at = ends_at + interval '7 days' WHERE results_announced_at IS NULL;

ALTER TABLE hackathon_registrations ADD COLUMN IF NOT EXISTS status VARCHAR(32) NOT NULL DEFAULT 'approved';

ALTER TABLE hackathon_submissions ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '';
ALTER TABLE hackathon_submissions ADD COLUMN IF NOT EXISTS presentation_key VARCHAR(512) DEFAULT '';
ALTER TABLE hackathon_submissions ADD COLUMN IF NOT EXISTS repo_url VARCHAR(512) DEFAULT '';
ALTER TABLE hackathon_submissions ADD COLUMN IF NOT EXISTS video_url VARCHAR(512) DEFAULT '';
ALTER TABLE hackathon_submissions ADD COLUMN IF NOT EXISTS version_no INT NOT NULL DEFAULT 1;

CREATE TABLE IF NOT EXISTS hackathon_submission_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    submission_id UUID NOT NULL REFERENCES hackathon_submissions(id) ON DELETE CASCADE,
    version_no INT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    artifact_key VARCHAR(512) DEFAULT '',
    presentation_key VARCHAR(512) DEFAULT '',
    repo_url VARCHAR(512) DEFAULT '',
    video_url VARCHAR(512) DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(submission_id, version_no)
);

CREATE TABLE IF NOT EXISTS hackathon_jury_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    hackathon_id UUID NOT NULL REFERENCES hackathons(id) ON DELETE CASCADE,
    display_name VARCHAR(128) NOT NULL,
    title VARCHAR(256) NOT NULL DEFAULT '',
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_hackathon_jury_hackathon ON hackathon_jury_members(hackathon_id);

CREATE TABLE IF NOT EXISTS hackathon_materials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    hackathon_id UUID NOT NULL REFERENCES hackathons(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    storage_key VARCHAR(512) NOT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE hackathon_teams ADD COLUMN IF NOT EXISTS recruiting BOOLEAN NOT NULL DEFAULT true;

ALTER TABLE hackathon_scores ADD COLUMN IF NOT EXISTS jury_member_id UUID REFERENCES hackathon_jury_members(id) ON DELETE CASCADE;
ALTER TABLE hackathon_scores ADD COLUMN IF NOT EXISTS comment TEXT NOT NULL DEFAULT '';

ALTER TABLE hackathon_scores DROP CONSTRAINT IF EXISTS hackathon_scores_submission_id_criterion_id_key;
CREATE UNIQUE INDEX IF NOT EXISTS idx_hackathon_scores_jury_unique
    ON hackathon_scores (submission_id, criterion_id, COALESCE(jury_member_id, '00000000-0000-0000-0000-000000000000'::uuid));

ALTER TABLE hackathon_certificates ADD COLUMN IF NOT EXISTS cert_type VARCHAR(32) NOT NULL DEFAULT 'participant';
ALTER TABLE hackathon_certificates ADD COLUMN IF NOT EXISTS place INT NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_hackathons_catalog ON hackathons(catalog_priority DESC, starts_at ASC);
CREATE INDEX IF NOT EXISTS idx_hackathon_reg_status ON hackathon_registrations(hackathon_id, status);
