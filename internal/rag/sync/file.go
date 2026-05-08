package sync

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/abhinavxd/libredesk/internal/rag"
	"github.com/abhinavxd/libredesk/internal/rag/models"
	"github.com/zerodha/logf"
)

// FileSyncer indexes uploaded files. The full file body is captured at
// upload time (FileConfig.Content), so re-syncs are local: parse +
// re-embed, no network. Three formats: txt (chunked), csv (one
// document per row), json (one document per object).
type FileSyncer struct {
	rag *rag.Manager
	lo  *logf.Logger
}

// NewFileSyncer creates a new file syncer.
func NewFileSyncer(ragMgr *rag.Manager, lo *logf.Logger) *FileSyncer {
	return &FileSyncer{rag: ragMgr, lo: lo}
}

// Sync parses the file content + indexes each parsed document.
func (s *FileSyncer) Sync(sourceID int, config models.FileConfig) error {
	s.lo.Info("starting file sync", "source_id", sourceID, "filename", config.Filename, "type", config.FileType)

	var documents []struct {
		Title   string
		Content string
	}
	switch config.FileType {
	case "txt":
		documents = s.parseTXT(config.Content)
	case "csv":
		documents = s.parseCSV(config.Content)
	case "json":
		documents = s.parseJSON(config.Content)
	default:
		return fmt.Errorf("unsupported file type: %s", config.FileType)
	}

	syncedRefs := make(map[string]bool)
	for i, doc := range documents {
		if strings.TrimSpace(doc.Content) == "" {
			continue
		}
		sourceRef := fmt.Sprintf("file_%d_%d", sourceID, i)
		syncedRefs[sourceRef] = true

		title := doc.Title
		if title == "" {
			title = fmt.Sprintf("%s (part %d)", config.Filename, i+1)
		}
		metadata, _ := json.Marshal(map[string]interface{}{
			"filename": config.Filename,
			"type":     config.FileType,
			"part":     i + 1,
		})
		if err := s.rag.AddDocument(sourceID, sourceRef, title, doc.Content, metadata); err != nil {
			s.lo.Error("error syncing file document", "ref", sourceRef, "error", err)
			continue
		}
	}

	s.lo.Info("file sync complete", "source_id", sourceID, "documents", len(syncedRefs))
	return nil
}

// parseTXT chunks plain text on line boundaries with a soft 2KB cap.
func (s *FileSyncer) parseTXT(content string) []struct{ Title, Content string } {
	var docs []struct{ Title, Content string }
	lines := strings.Split(content, "\n")
	var chunk strings.Builder
	for _, line := range lines {
		if chunk.Len()+len(line) > 2000 && chunk.Len() > 0 {
			docs = append(docs, struct{ Title, Content string }{Content: strings.TrimSpace(chunk.String())})
			chunk.Reset()
		}
		chunk.WriteString(line)
		chunk.WriteString("\n")
	}
	if chunk.Len() > 0 {
		docs = append(docs, struct{ Title, Content string }{Content: strings.TrimSpace(chunk.String())})
	}
	return docs
}

// parseCSV makes one document per row, formatted as `header: value;
// header: value`. The first row is treated as headers.
func (s *FileSyncer) parseCSV(content string) []struct{ Title, Content string } {
	var docs []struct{ Title, Content string }
	reader := csv.NewReader(strings.NewReader(content))
	records, err := reader.ReadAll()
	if err != nil {
		s.lo.Error("error parsing CSV", "error", err)
		return docs
	}
	if len(records) == 0 {
		return docs
	}
	headers := records[0]
	for i, row := range records[1:] {
		var parts []string
		for j, val := range row {
			if j < len(headers) && val != "" {
				parts = append(parts, fmt.Sprintf("%s: %s", headers[j], val))
			}
		}
		if len(parts) > 0 {
			docs = append(docs, struct{ Title, Content string }{
				Title:   fmt.Sprintf("Row %d", i+1),
				Content: strings.Join(parts, "; "),
			})
		}
	}
	return docs
}

// parseJSON treats the input as either an array (one doc per element)
// or a single object (one doc total). Each parsed item is re-marshalled
// to canonical JSON for consistent embedding.
func (s *FileSyncer) parseJSON(content string) []struct{ Title, Content string } {
	var docs []struct{ Title, Content string }

	var arr []map[string]interface{}
	if err := json.Unmarshal([]byte(content), &arr); err == nil {
		for i, item := range arr {
			if text, err := json.Marshal(item); err == nil {
				docs = append(docs, struct{ Title, Content string }{
					Title:   fmt.Sprintf("Item %d", i+1),
					Content: string(text),
				})
			}
		}
		return docs
	}

	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(content), &obj); err == nil {
		if text, err := json.Marshal(obj); err == nil {
			docs = append(docs, struct{ Title, Content string }{Content: string(text)})
		}
	}
	return docs
}
