-- Chunks of vacancy text + OpenAI (or compatible) embedding vectors for RAG search
CREATE TABLE IF NOT EXISTS vacancy_rag_chunks (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  vacancy_id UUID NOT NULL REFERENCES vacancies(id) ON DELETE CASCADE,
  chunk_index INT NOT NULL,
  content TEXT NOT NULL,
  embedding double precision[] NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT uq_vacancy_rag_chunk UNIQUE (vacancy_id, chunk_index)
);
CREATE INDEX IF NOT EXISTS idx_vacancy_rag_chunks_vacancy_id ON vacancy_rag_chunks(vacancy_id);
