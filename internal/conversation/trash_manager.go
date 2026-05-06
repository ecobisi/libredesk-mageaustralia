package conversation

import (
	"context"
	"fmt"
	"time"
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
	if autoTrashResolvedDays > 0 {
		res, err := c.q.AutoTrashResolved.ExecContext(ctx, autoTrashResolvedDays)
		if err != nil {
			c.lo.Error("error auto-trashing resolved conversations", "error", err)
		} else if rows, _ := res.RowsAffected(); rows > 0 {
			c.lo.Info(fmt.Sprintf("auto-trashed %d resolved/closed conversations", rows))
		}
	}

	if autoTrashSpamDays > 0 {
		res, err := c.q.AutoTrashSpam.ExecContext(ctx, autoTrashSpamDays)
		if err != nil {
			c.lo.Error("error auto-trashing spam conversations", "error", err)
		} else if rows, _ := res.RowsAffected(); rows > 0 {
			c.lo.Info(fmt.Sprintf("auto-trashed %d spam conversations", rows))
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
