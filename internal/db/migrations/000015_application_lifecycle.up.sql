-- Full internship application lifecycle: interview, offer, hiring and completion.
ALTER TABLE applications
  ADD COLUMN IF NOT EXISTS interview_format VARCHAR(32),
  ADD COLUMN IF NOT EXISTS interview_message TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS proposed_slots TIMESTAMPTZ[] NOT NULL DEFAULT '{}',
  ADD COLUMN IF NOT EXISTS interview_scheduled_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS decision_reason TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS offer_start_date DATE,
  ADD COLUMN IF NOT EXISTS offer_terms TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS offer_duration TEXT NOT NULL DEFAULT '';

UPDATE applications SET status = 'new' WHERE status IN ('submitted', 'pending');

