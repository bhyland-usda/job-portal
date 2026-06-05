CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS semantic_embeddings (
    entity_type TEXT NOT NULL,
    entity_id UUID NOT NULL,
    source_text TEXT NOT NULL,
    embedding vector(256) NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (entity_type, entity_id)
);

CREATE INDEX IF NOT EXISTS idx_semantic_embeddings_updated_at
    ON semantic_embeddings(updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_semantic_embeddings_vector
    ON semantic_embeddings USING hnsw (embedding vector_cosine_ops);
