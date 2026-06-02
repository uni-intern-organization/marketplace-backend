-- Extended billing: publication quotas, wallets, escrow, organizer type

DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'organizer_type') THEN
    CREATE TYPE organizer_type AS ENUM ('company', 'university');
  END IF;
END $$;

ALTER TABLE recruiter_profiles
  ADD COLUMN IF NOT EXISTS organizer_type organizer_type NOT NULL DEFAULT 'company',
  ADD COLUMN IF NOT EXISTS publications_quota INT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS publications_used INT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS quota_reset_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS wallets (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    balance_kzt NUMERIC(12,2) NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS escrow_holds (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    recruiter_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reference_type VARCHAR(32) NOT NULL,
    reference_id UUID NOT NULL,
    amount_kzt NUMERIC(12,2) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'held',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    released_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_escrow_reference ON escrow_holds(reference_type, reference_id);

CREATE TABLE IF NOT EXISTS notification_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    channel VARCHAR(32) NOT NULL,
    subject VARCHAR(255),
    body TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_notification_log_user ON notification_log(user_id);
