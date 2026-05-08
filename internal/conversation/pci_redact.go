package conversation

import (
	"context"
	"fmt"
	"time"

	"github.com/abhinavxd/libredesk/internal/conversation/models"
	"github.com/abhinavxd/libredesk/internal/envelope"
	notifier "github.com/abhinavxd/libredesk/internal/notification"
	nmodels "github.com/abhinavxd/libredesk/internal/notification/models"
	pciscrub "github.com/mageaustralia/go-pci-scrub"
	"github.com/volatiletech/null/v9"
)

// T3y PCI redaction pipeline. Two entry points feed the same scrub +
// activity-note + IMAP-cleanup path:
//
//   - RedactMessagePCI: synchronous, called from the manual "Redact Now"
//     handler. Caller (cmd/messages.go) wraps with permission checks and
//     drives IMAP delete + activity note from the returned metadata.
//
//   - RunPCIAutoRedact: hourly ticker that scrubs anything still flagged
//     after 7 days as a safety net for messages an agent never reviewed.
//
// The DB-side scrub is authoritative — once content + text_content are
// rewritten and has_pci_data cleared, the message is safe to display. The
// IMAP delete is best-effort: failure is logged + notified to a configured
// admin agent (PCISettings) but never blocks the redaction.

// PCIRedactMessage is the projection returned by both `get-message-for-redact`
// and `get-pci-messages-for-auto-redact`. The shared shape lets the manual
// and auto paths feed the same activity-note + IMAP-delete logic.
type PCIRedactMessage struct {
	ID               int         `db:"id"`
	UUID             string      `db:"uuid"`
	Content          string      `db:"content"`
	TextContent      string      `db:"text_content"`
	SourceID         null.String `db:"source_id"`
	ConversationID   int         `db:"conversation_id"`
	ConversationUUID string      `db:"conversation_uuid"`
	InboxID          int         `db:"inbox_id"`
}

// RedactMessagePCI scrubs PCI data from a message's content and text_content
// in the DB. Returns the pre-redaction message metadata so the caller can
// drive IMAP cleanup + activity note insertion. The caller is responsible
// for permission checks (cmd/messages.go enforces conversation access first).
func (m *Manager) RedactMessagePCI(msgUUID string) (PCIRedactMessage, error) {
	var msg PCIRedactMessage
	if err := m.q.GetMessageForRedact.Get(&msg, msgUUID); err != nil {
		return msg, envelope.NewError(envelope.GeneralError, "Message not found", nil)
	}

	scrubbedContent := pciscrub.Scrub(msg.Content)
	scrubbedText := pciscrub.Scrub(msg.TextContent)

	// RETURNING shape mirrors v1.0.3 (id, conversation_id, source_id, type)
	// even though we don't read the result here — keeps the SQL stable for
	// any future caller that does want it.
	var result struct {
		ID             int         `db:"id"`
		ConversationID int         `db:"conversation_id"`
		SourceID       null.String `db:"source_id"`
		Type           string      `db:"type"`
	}
	if err := m.q.RedactMessagePCI.Get(&result, msgUUID, scrubbedContent, scrubbedText); err != nil {
		m.lo.Error("error redacting PCI data from message", "error", err, "message_uuid", msgUUID)
		return msg, envelope.NewError(envelope.GeneralError, "Failed to redact message", nil)
	}

	m.lo.Info("PCI data redacted from message", "message_uuid", msgUUID, "conversation_uuid", msg.ConversationUUID)
	return msg, nil
}

// InsertPCIRedactActivityNote logs the redaction to the conversation activity
// stream so the audit trail shows who did what. The note is private (visible
// only to agents) and authored by the system user — actor name is in the
// content string itself rather than the message author so the auto-redact
// path can attribute to "System (auto-redact)" without juggling user IDs.
func (m *Manager) InsertPCIRedactActivityNote(conversationUUID string, actorName string, success bool, detail string) {
	noteContent := fmt.Sprintf("PCI data redacted by %s", actorName)
	if !success {
		noteContent = fmt.Sprintf("PCI data redacted by %s. %s", actorName, detail)
	}

	systemUser, err := m.userStore.GetSystemUser()
	if err != nil {
		m.lo.Error("error fetching system user for PCI activity note", "error", err)
		return
	}

	msg := &models.Message{
		Type:             models.MessageActivity,
		Status:           models.MessageStatusSent,
		ConversationUUID: conversationUUID,
		Content:          noteContent,
		TextContent:      noteContent,
		ContentType:      models.ContentTypeText,
		Private:          true,
		SenderID:         systemUser.ID,
		SenderType:       models.SenderTypeAgent,
	}
	if err := m.InsertMessage(msg); err != nil {
		m.lo.Error("error inserting PCI redact activity note", "error", err, "conversation_uuid", conversationUUID)
	}
}

// NotifyPCIIMAPDeleteFailed pings the configured PCI admin when card data was
// scrubbed from the DB but the corresponding IMAP email could not be removed
// — typical causes are expired Gmail OAuth tokens or the message having been
// moved/deleted out-of-band. The configured agent is expected to log into
// Gmail and delete the original manually.
//
// Notification method is one of "in_app", "email", or "both" (default).
// Falls back to a no-op when no agent is configured (notify_agent_id == 0)
// or when the dispatcher isn't wired in (test contexts).
func (m *Manager) NotifyPCIIMAPDeleteFailed(conversationUUID string, msgUUID string) {
	if m.dispatcher == nil {
		return
	}

	settings, err := m.settingsStore.GetPCISettings()
	if err != nil || settings.NotifyAgentID == 0 {
		return
	}

	agent, err := m.userStore.GetAgent(settings.NotifyAgentID, "")
	if err != nil {
		m.lo.Error("error fetching PCI notify agent", "error", err, "agent_id", settings.NotifyAgentID)
		return
	}

	title := "PCI: Failed to delete email from Gmail for conversation"
	body := fmt.Sprintf("Card data was redacted from a message but the original email could not be deleted from Gmail. Please delete it manually. Conversation: %s", conversationUUID)

	n := notifier.Notification{
		// Custom NotificationType — not registered in nmodels constants since
		// it's a one-off ops alert, not a user-facing routed-notification kind.
		Type:             nmodels.NotificationType("pci_imap_delete_failed"),
		RecipientIDs:     []int{settings.NotifyAgentID},
		Title:            title,
		Body:             null.StringFrom(body),
		ConversationUUID: conversationUUID,
	}

	method := settings.NotifyMethod
	if method == "" {
		method = "both"
	}

	switch method {
	case "in_app":
		m.dispatcher.Send(n)
	case "email":
		if agent.Email.Valid && agent.Email.String != "" {
			n.Email = &notifier.EmailNotification{
				Recipients: []string{agent.Email.String},
				Subject:    title,
				Content:    fmt.Sprintf("<p>%s</p>", body),
			}
			m.dispatcher.Send(n)
		}
	case "both":
		if agent.Email.Valid && agent.Email.String != "" {
			n.Email = &notifier.EmailNotification{
				Recipients: []string{agent.Email.String},
				Subject:    title,
				Content:    fmt.Sprintf("<p>%s</p>", body),
			}
		}
		m.dispatcher.Send(n)
	}
}

// RunPCIAutoRedact is the hourly safety net that scrubs anything still
// flagged after 7 days. Designed to be invoked as a goroutine from cmd/main.go
// alongside the other long-running workers (RunTrashManager, etc.). The
// deleteIMAP callback is the inbox manager's IMAP delete; nil-safe so the
// loop still runs when no IMAP cleanup is configured.
func (m *Manager) RunPCIAutoRedact(ctx context.Context, deleteIMAPFunc func(inboxID int, messageID string) error) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.runPCIAutoRedactCycle(ctx, deleteIMAPFunc)
		}
	}
}

func (m *Manager) runPCIAutoRedactCycle(ctx context.Context, deleteIMAPFunc func(inboxID int, messageID string) error) {
	var messages []PCIRedactMessage
	if err := m.q.GetPCIMessagesForAutoRedact.SelectContext(ctx, &messages); err != nil {
		m.lo.Error("error fetching PCI messages for auto-redact", "error", err)
		return
	}

	if len(messages) == 0 {
		return
	}

	m.lo.Info(fmt.Sprintf("auto-redacting PCI data from %d messages", len(messages)))

	for _, msg := range messages {
		scrubbedContent := pciscrub.Scrub(msg.Content)
		scrubbedText := pciscrub.Scrub(msg.TextContent)

		var result struct {
			ID             int         `db:"id"`
			ConversationID int         `db:"conversation_id"`
			SourceID       null.String `db:"source_id"`
			Type           string      `db:"type"`
		}
		if err := m.q.RedactMessagePCI.Get(&result, msg.UUID, scrubbedContent, scrubbedText); err != nil {
			m.lo.Error("error auto-redacting PCI message", "error", err, "message_uuid", msg.UUID)
			continue
		}

		m.lo.Info("auto-redacted PCI data from message", "message_uuid", msg.UUID)

		// Try to delete from IMAP. Failure here doesn't roll back the DB
		// scrub — once card data is out of the DB, the in-app exposure is
		// closed; the IMAP copy is a separate (lower-impact) leak that
		// requires the configured admin to clean up manually.
		if msg.SourceID.Valid && msg.SourceID.String != "" && deleteIMAPFunc != nil {
			if err := deleteIMAPFunc(msg.InboxID, msg.SourceID.String); err != nil {
				m.lo.Error("failed to delete PCI email from IMAP", "error", err, "source_id", msg.SourceID.String)
				m.InsertPCIRedactActivityNote(msg.ConversationUUID, "System (auto-redact)", false,
					"Card data was redacted but the original email could not be deleted from Gmail. Please delete manually.")
				m.NotifyPCIIMAPDeleteFailed(msg.ConversationUUID, msg.UUID)
			} else {
				m.InsertPCIRedactActivityNote(msg.ConversationUUID, "System (auto-redact)", true, "")
			}
		} else {
			m.InsertPCIRedactActivityNote(msg.ConversationUUID, "System (auto-redact)", true, "")
		}
	}
}
