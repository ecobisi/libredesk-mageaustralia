// Package rag implements Retrieval-Augmented Generation: a knowledge-source
// store, an embedding-backed vector search, and the AddDocument upsert path
// that the sync subpackage uses to keep rag_documents in step with the
// configured macro / webpage / file sources.
//
// The package is intentionally thin — it does not own the LLM call (that
// stays in cmd/rag.go via the AI manager) or the periodic sync loop (the
// sub-package rag/sync owns scheduling). It owns the SQL surface for
// rag_sources / rag_documents and the embedding pipeline.
package rag

import (
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/abhinavxd/libredesk/internal/dbutil"
	"github.com/abhinavxd/libredesk/internal/envelope"
	"github.com/abhinavxd/libredesk/internal/rag/models"
	"github.com/jmoiron/sqlx"
	"github.com/knadh/go-i18n"
	"github.com/zerodha/logf"
)

var (
	//go:embed queries.sql
	efs embed.FS
)

// EmbeddingFunc generates an OpenAI-compatible embedding for arbitrary
// text. Wired in cmd/init.go to ai.Manager.GenerateEmbedding so the
// per-call lookup picks up provider-config changes without a restart.
type EmbeddingFunc func(text string) ([]float32, error)

// Manager handles RAG storage + retrieval. The embedding callback is set
// at init time but can be swapped via SetEmbeddingFunc (used by tests).
// Every search and AddDocument call is gated on a non-nil embeddingFunc;
// callers without an OpenAI key configured surface a clean error instead
// of a nil-deref.
type Manager struct {
	q             queries
	db            *sqlx.DB
	lo            *logf.Logger
	i18n          *i18n.I18n
	embeddingFunc EmbeddingFunc
}

type queries struct {
	GetSources              *sqlx.Stmt `query:"get-sources"`
	GetSource               *sqlx.Stmt `query:"get-source"`
	GetEnabledSources       *sqlx.Stmt `query:"get-enabled-sources"`
	CreateSource            *sqlx.Stmt `query:"create-source"`
	UpdateSource            *sqlx.Stmt `query:"update-source"`
	DeleteSource            *sqlx.Stmt `query:"delete-source"`
	UpdateSourceSynced      *sqlx.Stmt `query:"update-source-synced"`
	GetDocumentsBySource    *sqlx.Stmt `query:"get-documents-by-source"`
	GetDocumentBySourceRef  *sqlx.Stmt `query:"get-document-by-source-ref"`
	DeleteDocument          *sqlx.Stmt `query:"delete-document"`
	DeleteDocumentsBySource *sqlx.Stmt `query:"delete-documents-by-source"`
}

// Opts contains options for initializing the Manager.
type Opts struct {
	DB            *sqlx.DB
	Lo            *logf.Logger
	I18n          *i18n.I18n
	EmbeddingFunc EmbeddingFunc
}

// New creates a new RAG manager. ScanSQLFile may fail at boot if the
// host's Postgres lacks pgvector and the rag_* tables therefore don't
// exist; that's surfaced as a fatal in cmd/init.go so the operator can
// fix the missing extension rather than booting into a half-broken
// state.
func New(opts Opts) (*Manager, error) {
	var q queries
	if err := dbutil.ScanSQLFile("queries.sql", &q, opts.DB, efs); err != nil {
		return nil, err
	}
	return &Manager{
		q:             q,
		db:            opts.DB,
		lo:            opts.Lo,
		i18n:          opts.I18n,
		embeddingFunc: opts.EmbeddingFunc,
	}, nil
}

// SetEmbeddingFunc swaps the embedding callback. Used by tests; production
// wires the callback once via Opts.
func (m *Manager) SetEmbeddingFunc(fn EmbeddingFunc) {
	m.embeddingFunc = fn
}

// GetDB returns the underlying sqlx handle so the sync subpackage can
// run a couple of bespoke queries (existing-document scans) that don't
// warrant their own prepared statements.
func (m *Manager) GetDB() *sqlx.DB {
	return m.db
}

// GetSources returns all knowledge sources, ordered by created_at desc.
func (m *Manager) GetSources() ([]models.Source, error) {
	sources := make([]models.Source, 0)
	if err := m.q.GetSources.Select(&sources); err != nil {
		m.lo.Error("error fetching sources", "error", err)
		return nil, envelope.NewError(envelope.GeneralError, m.i18n.Ts("globals.messages.errorFetching", "name", "knowledge sources"), nil)
	}
	return sources, nil
}

// GetSource returns a source by ID.
func (m *Manager) GetSource(id int) (models.Source, error) {
	var source models.Source
	if err := m.q.GetSource.Get(&source, id); err != nil {
		if err == sql.ErrNoRows {
			return source, envelope.NewError(envelope.NotFoundError, m.i18n.Ts("globals.messages.notFound", "name", "knowledge source"), nil)
		}
		m.lo.Error("error fetching source", "error", err)
		return source, envelope.NewError(envelope.GeneralError, m.i18n.Ts("globals.messages.errorFetching", "name", "knowledge source"), nil)
	}
	return source, nil
}

// CreateSource creates a new knowledge source. The cmd-layer handler
// validates source_type before getting here.
func (m *Manager) CreateSource(name, sourceType string, config json.RawMessage, enabled bool) (models.Source, error) {
	var source models.Source
	if err := m.q.CreateSource.Get(&source, name, sourceType, config, enabled); err != nil {
		m.lo.Error("error creating source", "error", err)
		return source, envelope.NewError(envelope.GeneralError, m.i18n.Ts("globals.messages.errorCreating", "name", "knowledge source"), nil)
	}
	return source, nil
}

// UpdateSource updates a knowledge source. SourceType is immutable post-
// creation (different types use incompatible config shapes); only name,
// config, and enabled flip.
func (m *Manager) UpdateSource(id int, name string, config json.RawMessage, enabled bool) (models.Source, error) {
	var source models.Source
	if err := m.q.UpdateSource.Get(&source, id, name, config, enabled); err != nil {
		m.lo.Error("error updating source", "error", err)
		return source, envelope.NewError(envelope.GeneralError, m.i18n.Ts("globals.messages.errorUpdating", "name", "knowledge source"), nil)
	}
	return source, nil
}

// DeleteSource deletes a knowledge source. ON DELETE CASCADE on
// rag_documents.source_id reaps the indexed chunks atomically.
func (m *Manager) DeleteSource(id int) error {
	result, err := m.q.DeleteSource.Exec(id)
	if err != nil {
		m.lo.Error("error deleting source", "error", err)
		return envelope.NewError(envelope.GeneralError, m.i18n.Ts("globals.messages.errorDeleting", "name", "knowledge source"), nil)
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return envelope.NewError(envelope.NotFoundError, m.i18n.Ts("globals.messages.notFound", "name", "knowledge source"), nil)
	}
	return nil
}

// UpdateSourceSynced sets last_synced_at = NOW() on the row. Called by
// the Coordinator after a successful sync; failures don't surface up
// because a missed bookkeeping update doesn't break correctness.
func (m *Manager) UpdateSourceSynced(id int) error {
	if _, err := m.q.UpdateSourceSynced.Exec(id); err != nil {
		m.lo.Error("error updating source synced time", "error", err)
		return err
	}
	return nil
}

// HashContent generates a SHA256 hash of content for change detection so
// AddDocument can skip the embed-call when content is unchanged. The
// expensive part of a sync is the OpenAI round-trip per chunk; this
// dedup is a real win on hourly re-syncs of stable knowledge bases.
func HashContent(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

// GetDocumentBySourceRef gets a document by source ID and source_ref. Used
// by AddDocument for the unchanged-content shortcut and by the sync
// package's deletion-of-stale-rows pass.
func (m *Manager) GetDocumentBySourceRef(sourceID int, sourceRef string) (models.Document, error) {
	var doc models.Document
	err := m.q.GetDocumentBySourceRef.Get(&doc, sourceID, sourceRef)
	return doc, err
}

// AddDocument upserts a document with its embedding. If a row with the
// same (source_id, source_ref) exists and content_hash hasn't changed we
// skip the embed-call entirely — that's the hot path during a re-sync.
//
// pgvector embeddings are written via raw SQL rather than a prepared
// stmt because sqlx + the vector type don't compose cleanly without a
// custom driver dance. The cast `$6::vector` does the lift from the text
// representation we serialize to the pgvector column type.
func (m *Manager) AddDocument(sourceID int, sourceRef, title, content string, metadata json.RawMessage) error {
	if m.embeddingFunc == nil {
		return fmt.Errorf("embedding function not configured")
	}

	contentHash := HashContent(content)

	// Skip the OpenAI round-trip if nothing changed. err on the existence
	// lookup is intentionally swallowed: any failure means we'll fall
	// through to a re-embed, which is the safe direction.
	if existing, err := m.GetDocumentBySourceRef(sourceID, sourceRef); err == nil && existing.ContentHash == contentHash {
		return nil
	}

	embedding, err := m.embeddingFunc(content)
	if err != nil {
		m.lo.Error("error generating embedding", "error", err)
		return fmt.Errorf("generating embedding: %w", err)
	}

	embeddingStr := Float32SliceToVector(embedding)

	if _, err = m.db.Exec(`
		INSERT INTO rag_documents (source_id, source_ref, title, content, content_hash, embedding, metadata)
		VALUES ($1, $2, $3, $4, $5, $6::vector, $7)
		ON CONFLICT (source_id, source_ref) WHERE source_ref IS NOT NULL
		DO UPDATE SET
			title = EXCLUDED.title,
			content = EXCLUDED.content,
			content_hash = EXCLUDED.content_hash,
			embedding = EXCLUDED.embedding,
			metadata = EXCLUDED.metadata,
			updated_at = NOW()
	`, sourceID, sourceRef, title, content, contentHash, embeddingStr, metadata); err != nil {
		m.lo.Error("error inserting document", "error", err)
		return fmt.Errorf("inserting document: %w", err)
	}

	return nil
}

// Search finds documents similar to the query. Returns at most `limit`
// rows with cosine similarity >= `threshold`. limit/threshold validation
// (default 5 / 0.25) lives at the cmd handler — this method trusts its
// inputs.
func (m *Manager) Search(query string, limit int, threshold float64) ([]models.SearchResult, error) {
	if m.embeddingFunc == nil {
		m.lo.Error("embedding function not configured")
		return nil, fmt.Errorf("embedding function not configured")
	}

	m.lo.Info("RAG search started", "limit", limit, "threshold", threshold)

	embedding, err := m.embeddingFunc(query)
	if err != nil {
		m.lo.Error("error generating query embedding", "error", err)
		return nil, fmt.Errorf("generating query embedding: %w", err)
	}

	embeddingStr := Float32SliceToVector(embedding)

	results := make([]models.SearchResult, 0)
	if err := m.db.Select(&results, `
		SELECT
			id, created_at, updated_at, source_id, source_ref, title, content, content_hash, metadata,
			1 - (embedding <=> $1::vector) as similarity
		FROM rag_documents
		WHERE embedding IS NOT NULL
			AND 1 - (embedding <=> $1::vector) >= $3
		ORDER BY embedding <=> $1::vector
		LIMIT $2
	`, embeddingStr, limit, threshold); err != nil {
		m.lo.Error("error searching documents", "error", err)
		return nil, fmt.Errorf("searching documents: %w", err)
	}

	m.lo.Info("RAG search complete", "results_count", len(results))
	return results, nil
}

// Float32SliceToVector serializes an embedding to the textual form
// pgvector parses with `::vector`. Format: `[f0,f1,...,fN]`. Six decimal
// places is enough fidelity for HNSW search and keeps the SQL payload
// reasonable (4096 chars for a 1536-dim vector).
func Float32SliceToVector(v []float32) string {
	result := "["
	for i, f := range v {
		if i > 0 {
			result += ","
		}
		result += fmt.Sprintf("%f", f)
	}
	result += "]"
	return result
}
