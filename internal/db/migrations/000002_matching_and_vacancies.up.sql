-- Student profile: skill-based fields for matching (plain text for search)
ALTER TABLE student_profiles
  ADD COLUMN IF NOT EXISTS skills TEXT DEFAULT '',
  ADD COLUMN IF NOT EXISTS education TEXT DEFAULT '',
  ADD COLUMN IF NOT EXISTS experience_years INT DEFAULT 0,
  ADD COLUMN IF NOT EXISTS location TEXT DEFAULT '',
  ADD COLUMN IF NOT EXISTS availability TEXT DEFAULT '';

-- Vacancy: requirements for matching (plain text for filter/match)
ALTER TABLE vacancies
  ADD COLUMN IF NOT EXISTS required_skills TEXT DEFAULT '',
  ADD COLUMN IF NOT EXISTS location TEXT DEFAULT '',
  ADD COLUMN IF NOT EXISTS employment_type TEXT DEFAULT '',
  ADD COLUMN IF NOT EXISTS min_experience_years INT DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_vacancies_location ON vacancies(location);
CREATE INDEX IF NOT EXISTS idx_vacancies_employment_type ON vacancies(employment_type);
