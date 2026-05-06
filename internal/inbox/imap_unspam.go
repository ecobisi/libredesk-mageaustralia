package inbox

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"strings"

	imodels "github.com/abhinavxd/libredesk/internal/inbox/models"
	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

// UnspamIMAPMessage searches the inbox's spam/junk folder for a message by
// Message-ID and moves it to INBOX. Gmail (and most major providers)
// interprets a user-driven MOVE out of Spam as a "not spam" training signal,
// so future deliveries from this sender are more likely to land in INBOX.
//
// The IMAP MOVE command (RFC 6851) is used directly; the underlying client
// auto-falls-back to COPY+STORE+EXPUNGE when the server lacks MOVE support.
//
// Best-effort by design: callers (the conversation auto-rescue path and the
// manual handleMarkAsNotSpam handler) treat any error as non-fatal. The
// not-spam classification is already persisted in the DB before this runs;
// failure here just means Gmail's filter doesn't get the training signal
// this round.
//
// OAuth note: when an inbox is configured for OAuth2 we currently fall back
// to whatever password is stored alongside (Gmail app passwords still work
// for some setups). True XOAUTH2 here would require relocating the refresh
// helpers from internal/inbox/channel/email into the inbox package, which
// hasn't been done yet — when it is, swap out the password branch below.
func (m *Manager) UnspamIMAPMessage(inboxID int, messageID string) error {
	dbInbox, err := m.GetDBRecord(inboxID)
	if err != nil {
		return fmt.Errorf("failed to get inbox config: %w", err)
	}

	var cfg imodels.Config
	if err := json.Unmarshal(dbInbox.Config, &cfg); err != nil {
		return fmt.Errorf("failed to parse inbox config: %w", err)
	}

	if len(cfg.IMAP) == 0 {
		return fmt.Errorf("no IMAP config for inbox %d", inboxID)
	}

	imapCfg := cfg.IMAP[0]

	spamMailbox := pickSpamMailbox(imapCfg.Mailbox)
	if spamMailbox == "" {
		// Inbox doesn't poll a spam folder; nothing to move. Not an error.
		return nil
	}

	address := fmt.Sprintf("%s:%d", imapCfg.Host, imapCfg.Port)
	imapOptions := &imapclient.Options{
		TLSConfig: &tls.Config{
			InsecureSkipVerify: imapCfg.TLSSkipVerify,
		},
	}

	var client *imapclient.Client
	switch imapCfg.TLSType {
	case "none":
		client, err = imapclient.DialInsecure(address, imapOptions)
	case "starttls":
		client, err = imapclient.DialStartTLS(address, imapOptions)
	case "tls":
		client, err = imapclient.DialTLS(address, imapOptions)
	default:
		return fmt.Errorf("unknown IMAP TLS type: %q", imapCfg.TLSType)
	}
	if err != nil {
		return fmt.Errorf("failed to connect to IMAP: %w", err)
	}
	defer client.Logout()

	if cfg.AuthType == imodels.AuthTypeOAuth2 && cfg.OAuth != nil {
		if imapCfg.Password != "" {
			if err := client.Login(imapCfg.Username, imapCfg.Password).Wait(); err != nil {
				return fmt.Errorf("IMAP login failed: %w", err)
			}
		} else {
			return fmt.Errorf("OAuth IMAP unspam not yet supported")
		}
	} else {
		if err := client.Login(imapCfg.Username, imapCfg.Password).Wait(); err != nil {
			return fmt.Errorf("IMAP login failed: %w", err)
		}
	}

	// Select the spam mailbox read/write so MOVE/STORE work.
	if _, err := client.Select(spamMailbox, &imap.SelectOptions{ReadOnly: false}).Wait(); err != nil {
		return fmt.Errorf("failed to select spam mailbox %q: %w", spamMailbox, err)
	}

	criteria := &imap.SearchCriteria{
		Header: []imap.SearchCriteriaHeaderField{
			{Key: "Message-ID", Value: messageID},
		},
	}
	searchResult, err := client.Search(criteria, nil).Wait()
	if err != nil {
		return fmt.Errorf("IMAP search failed: %w", err)
	}

	seqNums := searchResult.AllSeqNums()
	if len(seqNums) == 0 {
		// Already moved, expunged, or never landed in spam to begin with.
		// Not a failure mode worth alerting on.
		m.lo.Info("IMAP message not found in spam folder, may have already been moved", "message_id", messageID, "inbox_id", inboxID, "mailbox", spamMailbox)
		return nil
	}

	seqSet := imap.SeqSet{}
	seqSet.AddNum(seqNums...)

	if _, err := client.Move(seqSet, "INBOX").Wait(); err != nil {
		return fmt.Errorf("IMAP MOVE to INBOX failed: %w", err)
	}

	m.lo.Info("moved message from spam to INBOX", "message_id", messageID, "inbox_id", inboxID, "from", spamMailbox)
	return nil
}

// pickSpamMailbox returns the first spam/junk-named folder from the
// comma-separated IMAP mailbox list configured on an inbox. Returns the
// empty string when none is configured (the inbox doesn't poll spam) so
// callers can treat that as a no-op.
func pickSpamMailbox(mailboxList string) string {
	for _, mb := range strings.Split(mailboxList, ",") {
		mb = strings.TrimSpace(mb)
		lower := strings.ToLower(mb)
		if strings.Contains(lower, "spam") || strings.Contains(lower, "junk") {
			return mb
		}
	}
	return ""
}
