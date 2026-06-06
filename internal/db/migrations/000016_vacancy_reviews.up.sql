CREATE TABLE IF NOT EXISTS vacancy_reviews (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vacancy_id UUID NOT NULL REFERENCES vacancies(id) ON DELETE CASCADE,
    author_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    text TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(vacancy_id, author_user_id)
);

CREATE INDEX IF NOT EXISTS idx_vacancy_reviews_vacancy ON vacancy_reviews(vacancy_id, created_at DESC);

