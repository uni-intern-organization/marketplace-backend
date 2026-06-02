-- Vacancy listing tiers, lifecycle, views

DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'listing_tier') THEN
    CREATE TYPE listing_tier AS ENUM ('basic', 'standard', 'premium');
  END IF;
END $$;

DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'vacancy_status') THEN
    CREATE TYPE vacancy_status AS ENUM ('active', 'archived');
  END IF;
END $$;

ALTER TABLE vacancies
  ADD COLUMN IF NOT EXISTS listing_tier listing_tier NOT NULL DEFAULT 'basic',
  ADD COLUMN IF NOT EXISTS published_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS status vacancy_status NOT NULL DEFAULT 'active';

-- Migrate featured vacancies to premium tier
UPDATE vacancies
SET listing_tier = 'premium',
    published_at = COALESCE(published_at, created_at),
    expires_at = COALESCE(expires_at, COALESCE(featured_until, created_at + INTERVAL '30 days'))
WHERE is_featured = true AND (featured_until IS NULL OR featured_until > NOW());

UPDATE vacancies
SET published_at = COALESCE(published_at, created_at),
    expires_at = COALESCE(expires_at, created_at + INTERVAL '30 days')
WHERE published_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_vacancies_tier_status ON vacancies(listing_tier, status, expires_at);

CREATE TABLE IF NOT EXISTS vacancy_views (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vacancy_id UUID NOT NULL REFERENCES vacancies(id) ON DELETE CASCADE,
    viewer_id UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_vacancy_views_vacancy ON vacancy_views(vacancy_id);
