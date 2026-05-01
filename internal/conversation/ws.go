package conversation

import (
	"encoding/json"
	"regexp"
	"fmt"
	"time"

	cmodels "github.com/abhinavxd/libredesk/internal/conversation/models"
	notifier "github.com/abhinavxd/libredesk/internal/notification"
	nmodels "github.com/abhinavxd/libredesk/internal/notification/models"
	"github.com/abhinavxd/libredesk/internal/template"
	wsmodels "github.com/abhinavxd/libredesk/internal/ws/models"
	pciscrub "github.com/mageaustralia/go-pci-scrub"
	"github.com/volatiletech/null/v9"
)

// BroadcastNewMessage broadcasts a new message to all users.
func (m *Manager) BroadcastNewMessage(message *cmodels.Message) {
	m.broadcastToUsers([]int{}, wsmodels.Message{
		Type: wsmodels.MessageTypeNewMessage,
		Data: map[string]interface{}{
			"conversation_uuid": message.ConversationUUID,
			"content":           "",
			"created_at":        message.CreatedAt.Format(time.RFC3339),
			"uuid":              message.UUID,
			"private":           message.Private,
			"type":              message.Type,
			"sender_type":       message.SenderType,
		},
	})
}

// BroadcastMessageUpdate broadcasts a message update to all users.
func (m *Manager) BroadcastMessageUpdate(conversationUUID, messageUUID, prop string, value any) {
	message := wsmodels.Message{
		Type: wsmodels.MessageTypeMessagePropUpdate,
		Data: map[string]interface{}{
			"conversation_uuid": conversationUUID,
			"uuid":              messageUUID,
			"prop":              prop,
			"value":             value,
		},
	}
	m.broadcastToUsers([]int{}, message)
}

// BroadcastConversationUpdate broadcasts a conversation update to all users.
func (m *Manager) BroadcastConversationUpdate(conversationUUID, prop string, value any) {
	message := wsmodels.Message{
		Type: wsmodels.MessageTypeConversationPropertyUpdate,
		Data: map[string]interface{}{
			"uuid":  conversationUUID,
			"prop":  prop,
			"value": value,
		},
	}
	m.broadcastToUsers([]int{}, message)
}

// broadcastToUsers broadcasts a message to a list of users, if the list is empty it broadcasts to all users.
func (m *Manager) broadcastToUsers(userIDs []int, message wsmodels.Message) {
	messageBytes, err := json.Marshal(message)
	if err != nil {
		m.lo.Error("error marshalling WS message", "error", err)
		return
	}
	m.wsHub.BroadcastMessage(wsmodels.BroadcastMessage{
		Data:  messageBytes,
		Users: userIDs,
	})
}

// notifyParticipants sends in-app + email notifications to the assigned agent
// and conversation followers about a new message, excluding the sender.
func (m *Manager) notifyParticipants(message *cmodels.Message) {
	if m.dispatcher == nil {
		return
	}

	// Get conversation for assigned agent + reference number.
	conv, err := m.GetConversation(message.ConversationID, message.ConversationUUID, "")
	if err != nil {
		m.lo.Warn("failed to get conversation for notification", "error", err)
		return
	}

	// Build recipient set: assigned agent + followers.
	recipientMap := make(map[int]bool)

	// Add assigned agent.
	if conv.AssignedUserID.Int > 0 {
		recipientMap[conv.AssignedUserID.Int] = true
	}

	// Add followers (conversation participants).
	participants, _ := m.GetConversationParticipants(message.ConversationUUID)
	for _, p := range participants {
		recipientMap[p.ID] = true
	}

	// Exclude the sender.
	delete(recipientMap, message.SenderID)

	if len(recipientMap) == 0 {
		return
	}

	// Determine sender name.
	senderFirstName := "Someone"
	senderFullName := "Someone"
	if message.SenderType == cmodels.SenderTypeContact {
		senderFirstName = conv.Contact.FirstName
		senderFullName = conv.Contact.FirstName + " " + conv.Contact.LastName
	} else if message.SenderID > 0 {
		if agent, err := m.userStore.GetAgent(message.SenderID, ""); err == nil {
			senderFirstName = agent.FirstName
			senderFullName = agent.FirstName + " " + agent.LastName
		}
	}

	title := fmt.Sprintf("%s replied in #%s", senderFullName, conv.ReferenceNumber)
	notifType := nmodels.NotificationTypeNewReply
	if message.Private {
		title = fmt.Sprintf("%s added a private note in #%s", senderFullName, conv.ReferenceNumber)
	}

	// Choose email template based on message type.
	tmplName := template.TmplNewReply
	if message.Private {
		tmplName = template.TmplNoteAdded
	}

	// Build per-recipient notification with personalized emails.
	var recipientIDs []int
	var emails []notifier.EmailNotification

	for userID := range recipientMap {
		recipientIDs = append(recipientIDs, userID)

		// Get agent details for email.
		agent, err := m.userStore.GetAgent(userID, "")
		if err != nil || agent.Email.String == "" {
			emails = append(emails, notifier.EmailNotification{})
			continue
		}

		// Render personalized email.
		emailContent, subject, err := m.template.RenderStoredEmailTemplate(tmplName,
			map[string]any{
				"Conversation": map[string]any{
					"ReferenceNumber": conv.ReferenceNumber,
					"Subject":         conv.Subject.String,
					"UUID":            conv.UUID,
				},
				"Recipient": map[string]any{
					"FirstName":  agent.FirstName,
					"LastName":   agent.LastName,
					"FullName":   agent.FirstName + " " + agent.LastName,
					"Email":      agent.Email.String,
					// IsAssignee lets the email template show a different subject/body
					// for the ticket owner vs a watcher. Mirrors Freshdesk’s
					// distinction between owner notifications and "you are watching"
					// activity emails.
					"IsAssignee": userID == conv.AssignedUserID.Int,
				},
				"Author": map[string]any{
					"FirstName": senderFirstName,
					"FullName":  senderFullName,
				},
				"Message": map[string]any{
					"UUID":    message.UUID,
					"Content": pciscrub.Scrub(m.makeAbsoluteURLs(message.Content)),
				},
			})
		if err != nil {
			m.lo.Error("error rendering notification email", "template", tmplName, "error", err)
			emails = append(emails, notifier.EmailNotification{})
			continue
		}

		emails = append(emails, notifier.EmailNotification{
			Recipients: []string{agent.Email.String},
			Subject:    subject,
			Content:    emailContent,
		})
	}

	m.dispatcher.SendWithEmails(notifier.Notification{
		Type:             notifType,
		RecipientIDs:     recipientIDs,
		Title:            title,
		Body:             null.StringFrom(conv.Subject.String),
		ConversationID:   null.IntFrom(message.ConversationID),
		MessageID:        null.IntFrom(message.ID),
		ActorID:          null.IntFrom(message.SenderID),
		ConversationUUID: message.ConversationUUID,
		ActorFirstName:   senderFirstName,
		ActorLastName:    "",
	}, emails)
}

// makeAbsoluteURLs rewrites relative /uploads/ URLs in HTML content to signed absolute URLs
// so that images display correctly in notification emails without requiring authentication.
func (m *Manager) makeAbsoluteURLs(content string) string {
	// Match both relative (/uploads/UUID) and absolute (https://domain/uploads/UUID) URLs
	// to handle quoted replies where the email client has already made URLs absolute.
	re := regexp.MustCompile(`(?:https?://[^/]+)?/uploads/([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})(?:\?[^"\s]*)?`)
	return re.ReplaceAllStringFunc(content, func(match string) string {
		uuid := re.FindStringSubmatch(match)[1]
		// GetEmailURL returns a signed URL with 30-day expiry for email clients.
		return m.mediaStore.GetEmailURL(uuid)
	})
}

// pushNotifyAssignedAgent sends an FCM push notification to the assigned agent
// when a customer replies to a conversation. This is called directly from the
// incoming message path to ensure push notifications are always sent.
func (m *Manager) pushNotifyAssignedAgent(message *cmodels.Message) {
	if m.dispatcher == nil {
		return
	}

	conv, err := m.GetConversation(message.ConversationID, message.ConversationUUID, "")
	if err != nil {
		m.lo.Warn("pushNotifyAssignedAgent: failed to get conversation", "error", err)
		return
	}

	if conv.AssignedUserID.Int == 0 {
		return
	}

	// Determine sender name.
	senderName := "Customer"
	if conv.Contact.FirstName != "" {
		senderName = conv.Contact.FirstName
		if conv.Contact.LastName != "" {
			senderName += " " + conv.Contact.LastName
		}
	}

	title := fmt.Sprintf("%s replied in #%s", senderName, conv.ReferenceNumber)
	body := conv.Subject.String

	// Create in-app notification + FCM push via dispatcher.
	m.dispatcher.Send(notifier.Notification{
		Type:             nmodels.NotificationTypeNewReply,
		RecipientIDs:     []int{conv.AssignedUserID.Int},
		Title:            title,
		Body:             null.StringFrom(body),
		ConversationID:   null.IntFrom(message.ConversationID),
		MessageID:        null.IntFrom(message.ID),
		ActorID:          null.IntFrom(message.SenderID),
		ConversationUUID: message.ConversationUUID,
		ActorFirstName:   senderName,
	})

	m.lo.Info("push notification sent for incoming reply",
		"conversation_id", message.ConversationID,
		"assigned_user_id", conv.AssignedUserID.Int,
		"sender", senderName)
}
