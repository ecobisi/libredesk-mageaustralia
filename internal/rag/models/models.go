// Package models holds the database-row + config types for the RAG
// (Retrieval-Augmented Generation) pipeline.
package models

import (
	"encoding/json"
	"time"
)

// Source is one configured knowledge source — a macro feed, a list of web
// pages, or an uploaded file. The Coordinator (sync package) iterates these
// rows on a schedule and feeds chunks into rag_documents.
type Source struct {
	ID           int             `db:"id" json:"id"`
	CreatedAt    time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time       `db:"updated_at" json:"updated_at"`
	Name         string          `db:"name" json:"name"`
	SourceType   string          `db:"source_type" json:"source_type"`
	Config       json.RawMessage `db:"config" json:"config"`
	Enabled      bool            `db:"enabled" json:"enabled"`
	LastSyncedAt *time.Time      `db:"last_synced_at" json:"last_synced_at"`
}

// Document is a single indexed chunk: title + content + the OpenAI embedding
// (sized vector(1536) at the schema layer for text-embedding-3-small). The
// embedding column itself is not surfaced through the model — sqlx can't
// scan pgvector — so we leave it off and let SQL handle it inline.
type Document struct {
	ID          int             `db:"id" json:"id"`
	CreatedAt   time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time       `db:"updated_at" json:"updated_at"`
	SourceID    int             `db:"source_id" json:"source_id"`
	SourceRef   string          `db:"source_ref" json:"source_ref"`
	Title       string          `db:"title" json:"title"`
	Content     string          `db:"content" json:"content"`
	ContentHash string          `db:"content_hash" json:"content_hash"`
	Metadata    json.RawMessage `db:"metadata" json:"metadata"`
}

// SearchResult is a Document plus the cosine-similarity score from the
// nearest-neighbour search. Higher = closer to the query embedding.
type SearchResult struct {
	Document
	Similarity float64 `db:"similarity" json:"similarity"`
}

// WebpageConfig is the JSON shape stored in rag_sources.config when
// SourceType == "webpage". URLs are fetched + chunked on each sync.
type WebpageConfig struct {
	URLs []string `json:"urls"`
}

// MacroConfig is the JSON shape for SourceType == "macro". Currently the
// macro syncer indexes every macro in the macros table; IncludeAll is the
// default. MacroIDs is reserved for a future "only specific macros" mode.
type MacroConfig struct {
	IncludeAll bool  `json:"include_all"`
	MacroIDs   []int `json:"macro_ids,omitempty"`
}

// FileConfig is the JSON shape for SourceType == "file". The full file
// content is captured at upload time so re-syncs are cheap (no need to keep
// the original file on disk). FileType is one of "txt" | "csv" | "json".
type FileConfig struct {
	Filename string `json:"filename"`
	FileType string `json:"file_type"`
	Content  string `json:"content"`
}
