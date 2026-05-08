package migrations

import (
	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

// V2_2_15 stands up the RAG (Retrieval-Augmented Generation) infrastructure
// for T3a — the foundation commit for the v1.0.3 AI-powered response system.
// Runs in two stages so the AI settings rows land regardless of whether the
// host's Postgres has the pgvector extension installed:
//
//  1. Insert default AI settings rows. The setting manager's update statement
//     is a plain UPDATE (not UPSERT), so every key the admin form saves must
//     pre-exist or the toggle silently no-ops. ON CONFLICT DO NOTHING covers
//     re-runs and avoids stomping on values changed via Admin → AI between
//     migrations. Existing T3v keys (ai.transcription_*) are already in the
//     settings table from v2.2.14.
//
//  2. Try `CREATE EXTENSION IF NOT EXISTS vector`. If that fails (e.g. host
//     is on stock postgres:17 without pgvector compiled in) we early-return
//     successfully — RAG features simply remain unavailable until the
//     extension lands. The cmd-layer route registration is unconditional so
//     a 500 from /api/v1/rag/search makes the missing-extension state
//     visible; admin form for knowledge sources will surface the failure
//     when the first sync runs.
//
//  3. Create rag_sources + rag_documents tables idempotently. The
//     embedding column is sized 1536 — the dimension of OpenAI's
//     text-embedding-3-small (the default we ship; T3a doesn't expose
//     embedding-model selection in the UI yet). The hnsw index uses
//     vector_cosine_ops because the search path orders by `embedding <=> $1`
//     (cosine distance). Three btree indexes for the non-vector lookup
//     paths (source FK, source_ref existence checks during sync, content
//     hash dedup), plus a partial unique index on (source_id, source_ref)
//     for the upsert-on-sync path which keys on source_ref but allows
//     NULLs (for bare documents not tied to an external ref).
//
// schema.sql carries matching baseline rows + table DDL so fresh installs
// converge with upgraded ones.
func V2_2_15(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf) error {
	// 1. Default AI/RAG settings rows. T3v's transcription keys are already
	//    present from v2.2.14; this round adds the RAG-specific keys.
	if _, err := db.Exec(`
		INSERT INTO settings (key, value) VALUES
			('ai.enabled', 'false'::jsonb),
			('ai.embedding_model', '"text-embedding-3-small"'::jsonb),
			('ai.system_prompt', '""'::jsonb),
			('ai.max_context_chunks', '5'::jsonb),
			('ai.similarity_threshold', '0.25'::jsonb)
		ON CONFLICT (key) DO NOTHING
	`); err != nil {
		return err
	}

	// 2. pgvector extension. Treat failure as "feature unavailable on this
	//    host" — install proceeds, but RAG tables aren't created.
	if _, err := db.Exec(`CREATE EXTENSION IF NOT EXISTS vector`); err != nil {
		// pgvector not available on this host; skip RAG tables. App boots
		// fine, the cmd-layer routes return 5xx until the extension is
		// installed and the migration is re-run.
		return nil
	}

	// 3. RAG schema.
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS rag_sources (
			id SERIAL PRIMARY KEY,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW(),
			name TEXT NOT NULL,
			source_type TEXT NOT NULL,
			config JSONB DEFAULT '{}'::jsonb NOT NULL,
			enabled BOOL DEFAULT TRUE NOT NULL,
			last_synced_at TIMESTAMPTZ NULL,
			CONSTRAINT constraint_rag_sources_on_name CHECK (length(name) <= 255),
			CONSTRAINT constraint_rag_sources_on_source_type CHECK (source_type IN ('macro', 'webpage', 'file', 'custom'))
		)
	`); err != nil {
		return err
	}

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS rag_documents (
			id SERIAL PRIMARY KEY,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW(),
			source_id INT REFERENCES rag_sources(id) ON DELETE CASCADE NOT NULL,
			source_ref TEXT NULL,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			content_hash TEXT NOT NULL,
			embedding vector(1536),
			metadata JSONB DEFAULT '{}'::jsonb NOT NULL,
			CONSTRAINT constraint_rag_documents_on_title CHECK (length(title) <= 500)
		)
	`); err != nil {
		return err
	}

	if _, err := db.Exec(`
		CREATE INDEX IF NOT EXISTS index_rag_documents_on_embedding
		ON rag_documents USING hnsw (embedding vector_cosine_ops)
	`); err != nil {
		return err
	}

	if _, err := db.Exec(`
		CREATE INDEX IF NOT EXISTS index_rag_documents_on_source_id
		ON rag_documents(source_id)
	`); err != nil {
		return err
	}

	if _, err := db.Exec(`
		CREATE INDEX IF NOT EXISTS index_rag_documents_on_content_hash
		ON rag_documents(content_hash)
	`); err != nil {
		return err
	}

	if _, err := db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS index_unique_rag_documents_on_source_id_source_ref
		ON rag_documents(source_id, source_ref) WHERE source_ref IS NOT NULL
	`); err != nil {
		return err
	}

	return nil
}
