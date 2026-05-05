package conversation

import (
	"context"
	"fmt"
	"time"

	"github.com/abhinavxd/libredesk/internal/conversation/models"
)

// TrashSettingsFunc is a function that returns trash cleanup settings.
// Returns (autoTrashResolvedDays, autoTrashSpamDays, purgeTrashDays, activityPurgeDays).
type TrashSettingsFunc func() (int, int, int, int)

// RunTrashManager runs the trash management routine every hour.
// It reads settings each cycle via the provided function so changes take effect without restart.
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
	// Auto-trash old resolved/closed conversations.
	// The query RETURNING uuid lets us record a per-conversation activity message
	// so the audit trail still shows who/what trashed each conversation. Activity
	// is attributed to the system user since this is automated retention.
	if autoTrashResolvedDays > 0 {
		var uuids []string
		if err := c.q.AutoTrashResolved.SelectContext(ctx, &uuids, autoTrashResolvedDays); err != nil {
			c.lo.Error("error auto-trashing resolved conversations", "error", err)
		} else if len(uuids) > 0 {
			c.lo.Info(fmt.Sprintf("auto-trashed %d resolved/closed conversations", len(uuids)))
			c.recordAutoTrashActivity(uuids, "Resolved/Closed retention policy")
		}
	}

	// Auto-trash old spam conversations
	if autoTrashSpamDays > 0 {
		var uuids []string
		if err := c.q.AutoTrashSpam.SelectContext(ctx, &uuids, autoTrashSpamDays); err != nil {
			c.lo.Error("error auto-trashing spam conversations", "error", err)
		} else if len(uuids) > 0 {
			c.lo.Info(fmt.Sprintf("auto-trashed %d spam conversations", len(uuids)))
			c.recordAutoTrashActivity(uuids, "Spam retention policy")
		}
	}

	// Purge old trashed conversations (permanent delete)
	if purgeTrashDays > 0 {
		// Clean up media first (before cascade deletes messages)
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

	// Purge old activity messages
	if activityPurgeDays > 0 {
		res, err := c.q.PurgeOldActivities.ExecContext(ctx, activityPurgeDays)
		if err != nil {
			c.lo.Error("error purging old activities", "error", err)
		} else if rows, _ := res.RowsAffected(); rows > 0 {
			c.lo.Info(fmt.Sprintf("purged %d old activity messages", rows))
		}
	}
}

// recordAutoTrashActivity inserts a status-change activity row for each affected
// conversation so the audit trail stays complete for system-driven retention trashing.
// Best-effort: errors are logged but don't roll back the SQL trash that already happened.
func (c *Manager) recordAutoTrashActivity(uuids []string, reason string) {
	systemUser, err := c.userStore.GetSystemUser()
	if err != nil {
		c.lo.Error("could not fetch system user for auto-trash activity log", "error", err)
		return
	}
	for _, uuid := range uuids {
		if err := c.InsertConversationActivity(models.ActivityStatusChange, uuid,
			fmt.Sprintf("%s — %s", models.StatusTrashed, reason), systemUser); err != nil {
			c.lo.Warn("could not record auto-trash activity", "uuid", uuid, "error", err)
		}
	}
}
