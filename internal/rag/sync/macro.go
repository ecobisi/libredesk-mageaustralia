package sync

import (
	"encoding/json"
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/abhinavxd/libredesk/internal/macro"
	"github.com/abhinavxd/libredesk/internal/rag"
	"github.com/zerodha/logf"
)

// MacroSyncer indexes macros (saved replies) into rag_documents. Each
// macro becomes one document keyed on `macro_<id>`. Edits surface on
// the next periodic sync via the AddDocument content-hash dedup.
type MacroSyncer struct {
	macro *macro.Manager
	rag   *rag.Manager
	lo    *logf.Logger
}

// NewMacroSyncer creates a new macro syncer.
func NewMacroSyncer(macroMgr *macro.Manager, ragMgr *rag.Manager, lo *logf.Logger) *MacroSyncer {
	return &MacroSyncer{macro: macroMgr, rag: ragMgr, lo: lo}
}

// Sync indexes every macro into the given source.
func (s *MacroSyncer) Sync(sourceID int) error {
	s.lo.Info("starting macro sync", "source_id", sourceID)

	macros, err := s.macro.GetAll()
	if err != nil {
		s.lo.Error("error fetching macros", "error", err)
		return fmt.Errorf("fetching macros: %w", err)
	}

	syncedRefs := make(map[string]bool)
	for _, m := range macros {
		if strings.TrimSpace(m.MessageContent) == "" {
			continue
		}
		sourceRef := fmt.Sprintf("macro_%d", m.ID)
		syncedRefs[sourceRef] = true

		// Strip HTML for embeddings — the embedding model treats markup as
		// noise and the content is delivered to the LLM as plain text in
		// the prompt anyway.
		content := stripHTML(m.MessageContent)
		if strings.TrimSpace(content) == "" {
			continue
		}

		metadata, _ := json.Marshal(map[string]interface{}{
			"macro_id":   m.ID,
			"visibility": m.Visibility,
			"updated_at": m.UpdatedAt,
		})

		if err := s.rag.AddDocument(sourceID, sourceRef, m.Name, content, metadata); err != nil {
			s.lo.Error("error syncing macro", "macro_id", m.ID, "error", err)
			continue
		}
		s.lo.Debug("synced macro", "macro_id", m.ID, "name", m.Name)
	}

	// Detect deletions. We log them rather than delete because v1.0.3
	// shipped this as a deliberate no-op: the AddDocument upsert path
	// keeps the row alive on a stale ref, and a deleted-macro row in
	// the index is mostly harmless (it just stops getting refreshed).
	// T3a-followup units may flip this to actual deletes.
	if existing, err := s.getExistingDocuments(sourceID); err != nil {
		s.lo.Error("error fetching existing documents", "error", err)
	} else {
		for _, ref := range existing {
			if !syncedRefs[ref] {
				s.lo.Info("stale RAG document for deleted macro", "source_ref", ref)
			}
		}
	}

	s.lo.Info("macro sync complete", "source_id", sourceID, "synced", len(syncedRefs))
	return nil
}

// getExistingDocuments returns the source_refs for every document tied
// to a source, used for stale-ref detection above.
func (s *MacroSyncer) getExistingDocuments(sourceID int) ([]string, error) {
	var refs []string
	err := s.rag.GetDB().Select(&refs,
		`SELECT source_ref FROM rag_documents WHERE source_id = $1 AND source_ref IS NOT NULL`,
		sourceID)
	return refs, err
}

// stripHTML removes HTML tags + decodes entities + normalises whitespace.
// Lifted from v1.0.3 verbatim. T3l (prompt injection mitigation) replaces
// this with a streaming parser to fix an O(n^2) re-allocation issue on
// long content; not in scope here.
func stripHTML(s string) string {
	re := regexp.MustCompile(`<[^>]*>`)
	s = re.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	s = regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}
