-- Add vacancy_id to applications for linking to vacancies
ALTER TABLE applications
  ADD COLUMN IF NOT EXISTS vacancy_id UUID REFERENCES vacancies(id) ON DELETE SET NULL;

