-- Billing: recruiter plans and vacancy promotion

DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'recruiter_plan') THEN
    CREATE TYPE recruiter_plan AS ENUM ('free', 'pro');
  END IF;
END $$;

ALTER TABLE recruiter_profiles
  ADD COLUMN IF NOT EXISTS plan recruiter_plan NOT NULL DEFAULT 'free',
  ADD COLUMN IF NOT EXISTS plan_expires_at TIMESTAMPTZ;

ALTER TABLE vacancies
  ADD COLUMN IF NOT EXISTS is_featured BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS featured_until TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS billing_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    recruiter_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event_type VARCHAR(64) NOT NULL,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_billing_events_recruiter ON billing_events(recruiter_id);
CREATE INDEX IF NOT EXISTS idx_vacancies_featured ON vacancies(is_featured, featured_until);
