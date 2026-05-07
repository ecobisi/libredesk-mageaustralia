package conversation

import (
	"context"
	"fmt"
	"time"

	"github.com/abhinavxd/libredesk/internal/conversation/models"
)

// TrashSettingsFunc returns the auto-trash, purge, and activity-purge retention
// windows in days. A zero return value disables that particular cleanup pass
// for the cycle.
type TrashSettingsFunc func() (autoTrashResolvedDays, autoTrashSpamDays, purgeTrashDays, activityPurgeDays int)

// RunTrashManager runs the trash management routine every hour.
// Settings are re-read each cycle via the provided function so admin changes take
// effect without a restart.
func (c *Manager) RunTrashManager(ctx context.Context, getSettings TrashSettingsFunc) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			autoTrashResolvedDays, autoTrashSpamDays, purgeTrashDays, activityPurgeDays := getSettings()
			c.runTrashCycle(ctx, autoTrashResolvedDays, autoTrashSpamDays, purgeTrashDays, activityPurgeDays)
		}
	}
}

func (c *Manager) runTrashCycle(ctx context.Context, autoTrashResolvedDays, autoTrashSpamDays, purgeTrashDays, activityPurgeDays int) {
	// Both auto-trash queries RETURNING uuid so we can record a per-conversation
	// activity message; the bulk-update remains a single SQL statement.
	if autoTrashResolvedDays > 0 {
		var uuids []string
		if err := c.q.AutoTrashResolved.SelectContext(ctx, &uuids, autoTrashResolvedDays); err != nil {
			c.lo.Error("error auto-trashing resolved conversations", "error", err)
		} else if len(uuids) > 0 {
			c.lo.Info(fmt.Sprintf("auto-trashed %d resolved/closed conversations", len(uuids)))
			c.recordAutoTrashActivity(uuids, "Resolved retention policy")
		}
	}

	if autoTrashSpamDays > 0 {
		var uuids []string
		if err := c.q.AutoTrashSpam.SelectContext(ctx, &uuids, autoTrashSpamDays); err != nil {
			c.lo.Error("error auto-trashing spam conversations", "error", err)
		} else if len(uuids) > 0 {
			c.lo.Info(fmt.Sprintf("auto-trashed %d spam conversations", len(uuids)))
			c.recordAutoTrashActivity(uuids, "Spam retention policy")
		}
	}

	if purgeTrashDays > 0 {
		// Drop media first so the cascade message delete doesn't orphan blobs.
		if _, err := c.q.PurgeOldTrashMedia.ExecContext(ctx, purgeTrashDays); err != nil {
			c.lo.Error("error purging media for old trash", "error", err)
		}

		res, err := c.q.PurgeOldTrash.ExecContext(ctx, purgeTrashDays)
		if err != nil {
			c.lo.Error("error purging old trash", "error", err)
		} else if rows, _ := res.RowsAffected(); rows > 0 {
			c.lo.Info(fmt.Sprintf("permanently deleted %d trashed conversations", rows))
		}
	}

	// Purge old activity messages (status changes, assignments) that have
	// outlived the configured retention window. Only `type = 'activity'` rows
	// are deleted; agent replies and customer messages are never touched.
	if activityPurgeDays > 0 {
		res, err := c.q.PurgeOldActivities.ExecContext(ctx, activityPurgeDays)
		if err != nil {
			c.lo.Error("error purging old activity messages", "error", err)
		} else if rows, _ := res.RowsAffected(); rows > 0 {
			c.lo.Info(fmt.Sprintf("purged %d old activity messages", rows))
		}
	}
}

// recordAutoTrashActivity inserts a status-change activity row for each
// conversation that the retention sweep just trashed. Activity is attributed to
// the system user since this is automated; errors are logged but don't roll
// back the SQL trash that has already happened.
func (c *Manager) recordAutoTrashActivity(uuids []string, reason string) {
	systemUser, err := c.userStore.GetSystemUser()
	if err != nil {
		c.lo.Error("could not fetch system user for auto-trash activity log", "error", err)
		return
	}
	content := fmt.Sprintf("%s — %s", models.StatusTrashed, reason)
	for _, uuid := range uuids {
		if err := c.InsertConversationActivity(models.ActivityStatusChange, uuid, content, systemUser); err != nil {
			c.lo.Warn("could not record auto-trash activity", "uuid", uuid, "error", err)
		}
	}
}
