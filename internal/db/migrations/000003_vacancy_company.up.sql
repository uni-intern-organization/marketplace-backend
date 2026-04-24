-- Add company name to vacancies (for "Компания" field when creating a vacancy)
ALTER TABLE vacancies ADD COLUMN IF NOT EXISTS company_name TEXT DEFAULT '';
