package main

import (
	"github.com/abhinavxd/libredesk/internal/envelope"
	"github.com/zerodha/fastglue"
)

// recentActivitiesMaxPageSize caps page size to keep the COUNT(*) OVER()
// window bounded under load. The UI defaults to 20 per page anyway.
const recentActivitiesMaxPageSize = 50

// handleGetRecentActivities returns paginated activities across all
// conversations for the Reports > Recent Activities timeline view.
func handleGetRecentActivities(r *fastglue.Request) error {
	app := r.Context.(*App)

	page, pageSize := getPagination(r)
	if pageSize > recentActivitiesMaxPageSize {
		pageSize = recentActivitiesMaxPageSize
	}

	activities, total, err := app.conversation.GetRecentActivities(page, pageSize)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	return r.SendEnvelope(envelope.PageResults{
		Results:    activities,
		Total:      total,
		PerPage:    pageSize,
		TotalPages: (total + pageSize - 1) / pageSize,
		Page:       page,
	})
}
