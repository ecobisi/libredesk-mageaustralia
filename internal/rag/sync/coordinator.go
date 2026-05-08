// Package sync runs periodic re-syncs of every enabled rag_source row,
// pumping fresh content into rag_documents via rag.Manager.AddDocument.
//
// Topology: one Coordinator per app instance, owns a goroutine that
// ticks on the configured interval (1h default) and dispatches per-
// source-type to a dedicated syncer (macros / webpages / files). File
// sources skip the periodic loop because their content is captured at
// upload time and only changes on explicit re-sync.
package sync

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/abhinavxd/libredesk/internal/macro"
	"github.com/abhinavxd/libredesk/internal/rag"
	"github.com/abhinavxd/libredesk/internal/rag/models"
	"github.com/zerodha/logf"
)

// Coordinator orchestrates periodic re-syncs of every enabled
// knowledge source.
type Coordinator struct {
	rag      *rag.Manager
	macro    *macro.Manager
	lo       *logf.Logger
	interval time.Duration

	macroSyncer   *MacroSyncer
	webpageSyncer *WebpageSyncer
	fileSyncer    *FileSyncer

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// CoordinatorOpts contains options for creating a Coordinator.
type CoordinatorOpts struct {
	RAG          *rag.Manager
	Macro        *macro.Manager
	Lo           *logf.Logger
	SyncInterval time.Duration
}

// NewCoordinator creates a new sync coordinator. Zero SyncInterval falls
// back to 1h — slow enough not to thrash OpenAI's embeddings rate limit,
// fast enough for fresh macro edits to surface within the work-day.
func NewCoordinator(opts CoordinatorOpts) *Coordinator {
	if opts.SyncInterval == 0 {
		opts.SyncInterval = 1 * time.Hour
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Coordinator{
		rag:           opts.RAG,
		macro:         opts.Macro,
		lo:            opts.Lo,
		interval:      opts.SyncInterval,
		macroSyncer:   NewMacroSyncer(opts.Macro, opts.RAG, opts.Lo),
		webpageSyncer: NewWebpageSyncer(opts.RAG, opts.Lo),
		fileSyncer:    NewFileSyncer(opts.RAG, opts.Lo),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Start begins background syncing. The initial SyncAll runs immediately
// so a fresh boot doesn't have to wait an hour for the first index.
func (c *Coordinator) Start() {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.lo.Info("starting RAG sync coordinator", "interval", c.interval)
		c.SyncAll()
		ticker := time.NewTicker(c.interval)
		defer ticker.Stop()
		for {
			select {
			case <-c.ctx.Done():
				c.lo.Info("stopping RAG sync coordinator")
				return
			case <-ticker.C:
				c.SyncAll()
			}
		}
	}()
}

// Stop cancels the periodic loop and waits for the goroutine to exit.
// Called from cmd/main.go's shutdown sequence.
func (c *Coordinator) Stop() {
	c.cancel()
	c.wg.Wait()
}

// SyncAll syncs all enabled sources. File sources skip the periodic
// pass — their content is captured at upload time, so re-syncing them
// hourly would waste embed-calls on identical bytes (the AddDocument
// content-hash dedup would no-op anyway, but skipping is cleaner).
func (c *Coordinator) SyncAll() {
	sources, err := c.rag.GetSources()
	if err != nil {
		c.lo.Error("error fetching sources for sync", "error", err)
		return
	}
	for _, source := range sources {
		if !source.Enabled {
			continue
		}
		if source.SourceType == "file" {
			continue
		}
		if err := c.SyncSource(source); err != nil {
			c.lo.Error("error syncing source", "source_id", source.ID, "name", source.Name, "error", err)
		}
	}
}

// SyncSource syncs a single source. Used both by the periodic loop and
// the manual "Sync now" button on the admin UI.
func (c *Coordinator) SyncSource(source models.Source) error {
	c.lo.Info("syncing source", "source_id", source.ID, "name", source.Name, "type", source.SourceType)

	var err error
	switch source.SourceType {
	case "macro":
		err = c.macroSyncer.Sync(source.ID)
	case "webpage":
		var config models.WebpageConfig
		if err := json.Unmarshal(source.Config, &config); err != nil {
			return err
		}
		err = c.webpageSyncer.Sync(source.ID, config)
	case "file":
		var config models.FileConfig
		if err := json.Unmarshal(source.Config, &config); err != nil {
			return err
		}
		err = c.fileSyncer.Sync(source.ID, config)
	default:
		c.lo.Warn("unknown source type", "type", source.SourceType)
		return nil
	}
	if err != nil {
		return err
	}
	// Best-effort bookkeeping; missed update doesn't break correctness.
	_ = c.rag.UpdateSourceSynced(source.ID)
	return nil
}

// SyncSourceByID looks up a source by ID and syncs it. Used by the
// admin "Sync now" button + the file-upload path which kicks an
// immediate sync of the freshly created source.
func (c *Coordinator) SyncSourceByID(sourceID int) error {
	source, err := c.rag.GetSource(sourceID)
	if err != nil {
		return err
	}
	return c.SyncSource(source)
}
