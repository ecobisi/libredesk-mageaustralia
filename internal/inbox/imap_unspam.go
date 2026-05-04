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

// UnspamIMAPMessage searches the inbox's spam/junk folder for a message by Message-ID
// and moves it to INBOX. Gmail interprets this as a "not spam" signal and improves
// filtering for the sender on future deliveries. The MOVE command falls back to
// COPY+STORE+EXPUNGE automatically when the server lacks RFC 6851 support.
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
		// Inbox doesn't poll a spam folder; nothing to move.
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

// pickSpamMailbox returns the first spam/junk-named folder from the comma-separated
// IMAP mailbox list configured on an inbox. Returns empty if none is configured.
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
