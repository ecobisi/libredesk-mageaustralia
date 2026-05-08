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

// DeleteIMAPMessage searches the inbox's IMAP source mailbox for a message
// by Message-ID header, flags it as \Deleted, and EXPUNGEs. T3y wires this
// into the conversation package via Manager.IMAPDeleteFunc so the auto-
// redact loop and the manual "Redact Now" handler can clean up the
// original email after card data has been scrubbed from the DB copy.
//
// Gmail moves IMAP-deleted messages to Trash and auto-purges after 30 days,
// which is the desired behaviour: the redaction is undone if the agent
// changes their mind within that window. For other providers behaviour
// varies (e.g., Dovecot expunges immediately) — that's the cost of using
// a standard IMAP path rather than provider-specific APIs.
//
// Best-effort by design: callers (RunPCIAutoRedact and handleRedactMessagePCI)
// treat any error as non-fatal, log + notify the configured admin, and
// leave the in-DB redaction intact. The shape mirrors UnspamIMAPMessage
// for consistency — same connection/auth pattern, OAuth fallback caveat
// included.
func (m *Manager) DeleteIMAPMessage(inboxID int, messageID string) error {
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

	// OAuth note: matches UnspamIMAPMessage — when an inbox is configured
	// for OAuth2 we currently fall back to whatever password is stored
	// alongside (Gmail app passwords still work for some setups). True
	// XOAUTH2 here would require relocating the refresh helpers from
	// internal/inbox/channel/email into the inbox package.
	if cfg.AuthType == imodels.AuthTypeOAuth2 && cfg.OAuth != nil {
		if imapCfg.Password != "" {
			if err := client.Login(imapCfg.Username, imapCfg.Password).Wait(); err != nil {
				return fmt.Errorf("IMAP login failed: %w", err)
			}
		} else {
			return fmt.Errorf("OAuth IMAP delete not yet supported")
		}
	} else {
		if err := client.Login(imapCfg.Username, imapCfg.Password).Wait(); err != nil {
			return fmt.Errorf("IMAP login failed: %w", err)
		}
	}

	// Mailbox config is a comma-separated list (the IMAP scanner polls all
	// of them); for redaction we target INBOX by default. If the operator
	// has narrowed the polled list to a single non-INBOX folder, use that.
	mailbox := pickPrimaryMailbox(imapCfg.Mailbox)
	if mailbox == "" {
		mailbox = "INBOX"
	}
	if _, err := client.Select(mailbox, &imap.SelectOptions{ReadOnly: false}).Wait(); err != nil {
		return fmt.Errorf("failed to select mailbox: %w", err)
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
		// Already deleted, expunged, or moved out of the polled folder.
		// Not a failure — the original email is no longer in the source
		// mailbox, which is exactly the desired end state.
		m.lo.Warn("IMAP message not found for deletion", "message_id", messageID, "inbox_id", inboxID)
		return nil
	}

	seqSet := imap.SeqSet{}
	seqSet.AddNum(seqNums...)

	storeFlags := imap.StoreFlags{
		Op:    imap.StoreFlagsAdd,
		Flags: []imap.Flag{imap.FlagDeleted},
	}
	if err := client.Store(seqSet, &storeFlags, nil).Close(); err != nil {
		return fmt.Errorf("failed to flag message as deleted: %w", err)
	}

	if err := client.Expunge().Wait(); err != nil {
		return fmt.Errorf("failed to expunge message: %w", err)
	}

	m.lo.Info("deleted PCI email from IMAP", "message_id", messageID, "inbox_id", inboxID)
	return nil
}

// pickPrimaryMailbox returns the first non-spam/junk folder from the
// comma-separated IMAP mailbox list configured on an inbox. Empty string
// falls through to the caller's "INBOX" default. Mirrors pickSpamMailbox
// in imap_unspam.go but with inverted logic: PCI redaction targets the
// folder where customer mail actually arrives, not the spam folder.
func pickPrimaryMailbox(mailboxList string) string {
	for _, mb := range strings.Split(mailboxList, ",") {
		mb = strings.TrimSpace(mb)
		if mb == "" {
			continue
		}
		lower := strings.ToLower(mb)
		if strings.Contains(lower, "spam") || strings.Contains(lower, "junk") {
			continue
		}
		return mb
	}
	return ""
}
