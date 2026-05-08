package notifier

import (
	"context"
	"fmt"
	"time"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"github.com/zerodha/logf"
	"google.golang.org/api/option"
)

// PushToken is a single registered device for an agent: opaque FCM token
// plus the platform string we use to set the platform-specific FCM payload.
// Mirrors user.PushToken — we re-declare it here so this package doesn't
// have to import internal/user (matches the WSHub-interface pattern v2
// already uses elsewhere in this file to keep dispatch decoupled from the
// concrete sub-systems).
type PushToken struct {
	Token    string
	Platform string
}

// PushTokenStore is the slice of the user manager that the FCM dispatcher
// needs. Implementations: *user.Manager (via the adapter wired in init.go)
// or a stub in tests.
type PushTokenStore interface {
	GetPushTokens(userID int) ([]PushToken, error)
	DeletePushToken(userID int, token string) error
}

// FCMSender sends push notifications via Firebase Cloud Messaging.
//
// Construction is gated on a service account JSON file existing on disk —
// if absent, the dispatcher is wired with a nil FCMSender and every
// SendToUser call short-circuits. This is the v1.0.3 graceful-degradation
// contract: the feature is opt-in via mounting firebase-service-account.json,
// not via a config flag.
type FCMSender struct {
	client *messaging.Client
	store  PushTokenStore
	lo     *logf.Logger
}

// NewFCMSender initializes the FCM sender with a service account key file.
// Returns an error if Firebase init fails — callers should log the error
// and continue with a nil sender (push disabled), matching v1.0.3's startup
// behaviour.
func NewFCMSender(serviceAccountPath string, store PushTokenStore, lo *logf.Logger) (*FCMSender, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	app, err := firebase.NewApp(ctx, nil, option.WithCredentialsFile(serviceAccountPath))
	if err != nil {
		return nil, fmt.Errorf("initializing firebase app: %w", err)
	}

	client, err := app.Messaging(ctx)
	if err != nil {
		return nil, fmt.Errorf("initializing firebase messaging: %w", err)
	}

	lo.Info("FCM sender initialized")
	return &FCMSender{client: client, store: store, lo: lo}, nil
}

// SendToUser sends a push notification to all registered devices for the
// given user. Errors that indicate the token is no longer valid trigger
// automatic deletion of the offending row from user_push_tokens — this is
// the standard FCM lifecycle: a token rotates or the app is uninstalled,
// Firebase tells us on the next send, we forget it.
//
// Safe to call with f == nil — caller paths use `if d.fcm != nil` to gate.
// Intended to be called from a goroutine; runs all per-token sends inline.
func (f *FCMSender) SendToUser(userID int, title, body, conversationUUID string) {
	tokens, err := f.store.GetPushTokens(userID)
	if err != nil {
		// Manager already logged; nothing more to do.
		return
	}
	if len(tokens) == 0 {
		return
	}

	for _, t := range tokens {
		msg := &messaging.Message{
			Token: t.Token,
			Notification: &messaging.Notification{
				Title: title,
				Body:  body,
			},
			Data: map[string]string{
				"conversation_uuid": conversationUUID,
				"click_action":      "FLUTTER_NOTIFICATION_CLICK",
			},
		}

		// Platform-specific config. The Flutter side expects a
		// "libredesk_tickets" Android channel and the iOS APNS flags
		// below; keep both in sync if either changes.
		switch t.Platform {
		case "android":
			msg.Android = &messaging.AndroidConfig{
				Priority: "high",
				Notification: &messaging.AndroidNotification{
					ChannelID: "libredesk_tickets",
					Sound:     "default",
				},
			}
		case "ios":
			msg.APNS = &messaging.APNSConfig{
				Payload: &messaging.APNSPayload{
					Aps: &messaging.Aps{
						Sound:            "default",
						MutableContent:   true,
						ContentAvailable: true,
					},
				},
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, err := f.client.Send(ctx, msg)
		cancel()

		if err != nil {
			// Auto-remove invalid tokens. Firebase exposes typed predicates
			// for the two error codes that mean "this token is dead":
			//   - registration-token-not-registered (app uninstalled / token rotated)
			//   - invalid-argument (malformed token, e.g. truncated by client)
			// Anything else is logged but kept — transient send failures are
			// not a reason to forget a valid token.
			if messaging.IsUnregistered(err) || messaging.IsInvalidArgument(err) {
				f.lo.Warn("removing invalid FCM token", "user_id", userID, "platform", t.Platform)
				_ = f.store.DeletePushToken(userID, t.Token)
			} else {
				f.lo.Error("error sending FCM push", "user_id", userID, "platform", t.Platform, "error", err)
			}
		} else {
			f.lo.Debug("FCM push sent", "user_id", userID, "platform", t.Platform)
		}
	}
}
