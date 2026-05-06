# Libredesk (mageaustralia Fork)

This is a maintained fork of [Libredesk](https://github.com/abhinavxd/libredesk), an open-source, self-hosted customer support desk.

We run Libredesk in production and love the project. This fork exists because we need to ship features at the pace our business demands, and the upstream PR review cycle doesn't always align with that timeline. Rather than pressure the maintainers, we maintain our own fork with the features we need.

We're not trying to replace or compete with upstream Libredesk — we actively track releases and rebase our changes onto new versions as they come out. If any of our additions are useful to the broader project, we're happy to contribute them back.

**Upstream**: [abhinavxd/libredesk](https://github.com/abhinavxd/libredesk) | [libredesk.io](https://libredesk.io) | [Live demo](https://demo.libredesk.io/)  
**Base version**: v2.1.1  
**Companion branch**: [`v1.0.3-plus-enhancements`](https://github.com/mageaustralia/libredesk/tree/v1.0.3-plus-enhancements) — the production fork. This branch is an in-progress port of those features onto v2.1.1 upstream.

---

## Port Status

This branch is being actively ported from `v1.0.3-plus-enhancements`. **Most non-AI / non-integration features have landed; Tier 3 (RAG, ecommerce, voicemail transcription, PCI redaction, mobile push) is still pending.** Live unit-by-unit status is in [`docs/superpowers/specs/2026-04-27-v103-port-design.md`](./docs/superpowers/specs/2026-04-27-v103-port-design.md) — search for `pending` to see what's outstanding.

## Fork Features

Everything from upstream Libredesk is included. The following are additions present in this branch.

**Latest** — Per-user macro MRU sort, macro file attachments, multi-folder IMAP polling, contact-upsert name protection, agent/team name resolution on assignment WS updates, duplicate-outgoing-email guard, HTTP timeout bumps.

### Spam & Trash

Full spam and trash lifecycle for conversations — manual actions, automatic cleanup, and Gmail spam folder integration.

- **Spam status**: Mark conversations as Spam manually, or automatically via IMAP spam/junk folder detection
- **Trash status**: Move conversations to Trash manually or via bulk action
- **Restore / Not Spam**: One-click actions to move conversations back to the inbox
- **Sidebar sections**: Dedicated Spam and Trash views in the sidebar
- **TrashManager background worker** (runs hourly):
  - Auto-trash resolved/closed conversations after configurable days (default 90)
  - Auto-trash spam conversations after configurable days (default 30)
  - Permanently purge trashed conversations after configurable days (default 30)
  - Media and attachments cleaned up on purge
- **Admin settings**: Configure all retention periods at Admin > Trash & Cleanup (set to 0 to disable)
- **Multi-folder IMAP polling**: Enter comma-separated mailbox names (e.g. `INBOX, [Gmail]/Spam`) to poll multiple folders — messages from spam/junk folders automatically get Spam status
- **Auto-rescue for known senders**: Incoming messages that land in a polled spam folder are checked against the contacts database — if the sender has any prior outgoing agent reply, the conversation stays Open instead of being auto-spammed
- **IMAP MOVE-back on un-spam**: When a message is auto-rescued, or when an agent manually clicks "Mark as not spam", the original message is MOVEd back to INBOX in IMAP. Gmail interprets this as a "not spam" signal and improves filtering for that sender on future deliveries. Falls back to COPY+STORE+EXPUNGE on servers without RFC 6851 support
- New incoming messages on Spam or Trashed conversations do not reopen them

### Submit & Set Status on New Conversations

The "New conversation" dialog gets the same Send & Set Status pattern the reply box has on existing conversations.

- **Split-button**: a chevron next to the Submit button opens a dropdown of statuses
- **Dynamic list**: pulls from the same statuses store as the reply box, filtered by the admin-controlled `show_on_send` flag — toggle a status's visibility once at Admin > Statuses, both pickers update
- **Scrollable**: dropdown caps at 60vh with overflow scroll for installs with many statuses
- Conversation is created and immediately set to the chosen status (Snoozed is excluded since no duration is collected)

### Advanced View Filters

Enhanced filter operators for personal and shared views, enabling multi-select agent/team filtering.

- **"is any of"** (`in`) — match conversations assigned to any of the selected agents/teams
- **"is none of"** (`not_in`) — exclude conversations assigned to specific agents/teams
- **"is any of (or unassigned)"** (`in_or_null`) — match selected agents/teams OR unassigned conversations (common pattern: "my tickets + unassigned")
- Multi-select dropdowns for agent and team fields in the view builder
- Filter pill bar on conversation list showing active filters

### Table View Layout

Switch between card view and table view for the conversation list via a toggle in the toolbar. Table view shows conversations in a compact, data-dense format.

### Bulk Actions & Conversation Selection

Select multiple conversations from the list and perform bulk operations — no more opening each ticket individually to triage.

- **Per-row checkboxes** on the conversation list
- **Shift+click** range selection (click one, hold shift, click another to select all in between)
- **Select All** toggle in the bulk action toolbar
- **Bulk Assign** to any agent or team via dropdown
- **Bulk Status** change (Open, Replied, Resolved, Closed)
- **Bulk Priority** change (Urgent, High, Medium, Low, None)
- **Bulk Move to Trash**
- Toast notifications with success/error counts

### Agent Collision Detection

Real-time awareness of other agents working on the same conversation, preventing duplicate replies.

- **Presence tracking**: Eye icon with agent avatars in the conversation header when others are viewing the same ticket
- **Blinking eye animation** draws attention to active viewers
- **Hover tooltips** on avatar initials show the agent's name
- **Viewer count** on conversation list items (both card and table view)
- **Reply collision warning**: Amber banner appears in the reply box when another agent sends a reply while you're composing
- **Send confirmation dialog**: Before sending, a confirmation prompt warns if another agent replied since you started typing
- Presence automatically clears when an agent navigates away or disconnects
- WebSocket-based with no polling overhead

### Customer Reply Notifications

Agents now receive in-app and push notifications when a customer replies to their assigned ticket.

- Fires on incoming customer messages (not agent replies or new conversations)
- Creates in-app notification + FCM push to the assigned agent
- **Signed image URLs** in notification emails — images render without requiring authentication

### Ticket Merging

Merge duplicate or related conversations into a single ticket, consolidating all messages and tags.

- **Merge by ticket number**: From any ticket's `...` menu, click Merge and enter the other ticket's reference number — no need to find both tickets on the same page
- Select 2+ conversations from the list using bulk checkboxes, or merge from a single ticket view
- **Merge button** appears in the bulk action toolbar
- **Primary ticket picker**: Choose which conversation keeps its identity (others merge into it)
- Messages from secondary tickets are moved to the primary, preserving chronological order
- Tags from secondary tickets are copied (duplicates skipped)
- Secondary tickets are marked as merged and closed with an activity note
- **Merge banner** on merged tickets links back to the primary conversation
- Cannot be undone — confirmation dialog warns before merging

### Contact Email Filter

Filter the conversation list by contact email address using a free-text search.

- Added as a pill bar filter option ("Contact email")
- Uses case-insensitive partial matching (ILIKE) — type `campbell` to find all conversations from emails containing "campbell"
- New `FilterTextInput` component for text-based pill bar filters (with Enter to apply)

### Multi-Status Filtering

The status dropdown now supports selecting multiple statuses simultaneously.

- **Checkboxes** instead of single-select radio behaviour
- Select any combination (e.g., "Open + Replied" to see all active conversations)
- Button label shows count when multiple selected (e.g., "2 statuses")
- At least one status must remain selected

### Smart Team Reassignment

Changing a conversation's team no longer blindly unassigns the agent.

- If the assigned agent is a member of the new team, they stay assigned
- If the agent is NOT a member of the new team, they are unassigned (previous behaviour)
- Uses the existing `UserBelongsToTeam` check — no additional database queries

### Quick-Assign Dropdowns on Conversation List

Each conversation row shows the assigned agent and team with inline dropdown menus for reassignment — no need to open the conversation.

- Agent assignment shown with user icon (orange "Unassigned" when empty)
- Team assignment shown with team icon
- Compact 2x2 grid layout alongside timestamp and unread badge
- Dropdown menus with full agent/team lists for quick reassignment

### Email Alias Filtering

Configure additional email addresses that forward to an inbox, preventing them from appearing in CC when replying.

- **Email aliases field** in Inbox Settings — pill-style input for adding multiple forwarding addresses
- Aliases are excluded from CC alongside the primary inbox email
- Handles common setups like `orders@` and `info@` forwarding to a shared inbox
- **Smart contact detection**: When the conversation contact is an inbox email (e.g., Magento order notifications), scans message history to find the real customer email

### Fresh Theme

An alternative UI theme inspired by legacy SaaS providers, selectable via a theme switcher in the sidebar.

- Teal colour palette with dark sidebar
- Conversation list hides when a ticket is open (full-width detail view)
- Sidebar collapsed by default
- Collapsible reply box with unified scrolling
- Theme persists via localStorage

### Conversation List Enhancements

- **Subject, ticket number, status, and priority** displayed on each row
- **Previous Conversations accordion** defaults to open
- **Conversation status and priority badges** with colour-coded indicators

### Email & Message Improvements

- **Inline image rendering** in conversation messages
- **Email HTML sanitisation** for incoming messages — cleaner rendering with tightened layout
- **Per-email remove buttons** on TO, CC, and BCC fields
- **Agent name in email From header** instead of generic inbox name

### Full-Width Layout Toggle

Toggle between split list/detail view and full-width conversation view. Messages render at full width for better readability on wide screens.

### Auto-Assign on Reply

Per-inbox setting that automatically assigns a conversation to the agent who replies, if it's currently unassigned.

### Per-Inbox Email Signatures

Each inbox can have its own email signature with dynamic placeholders, configured in inbox settings.

### Connection Testing

- **IMAP connection test** with debug logs in inbox settings
- **SMTP test** for email notification settings

### Gmail-Style Quoted Thread

Quoted message history in the reply editor, matching Gmail's UX.

- **Collapsible toggle**: `···` button below the editor shows/hides the quoted thread
- **Editable**: Expand and edit/remove quoted content before sending
- **Last 3 messages**: Shows the 3 most recent non-private messages
- **Gmail-compatible**: Sent as `<div class="gmail_quote">` for proper threading in email clients
- **Backend fallback**: Server still appends quotes if the frontend marker is absent (API clients, edge cases)

### Permanent Delete from Trash

Immediately and permanently delete trashed conversations without waiting for the auto-purge window.

- **Bulk delete**: Select multiple items in Trash view → "Delete Permanently" button (red, destructive)
- **Confirmation dialog**: Warns before irreversible deletion
- **Instant refresh**: List updates immediately after deletion

### Hover Preview with Latest Reply

Table view hover tooltip shows both the original message and the latest reply.

- **Original message**: First non-activity message (includes agent-initiated conversations)
- **Latest reply**: Most recent real message (excludes activity/status changes), labelled as agent or customer
- **No scrolling needed**: Quick at-a-glance view of conversation state

### Signed Image URLs in Emails

Inline images in outgoing and notification emails use signed URLs with 30-day expiry.

- **Outgoing emails**: Images sent to customers are accessible without authentication
- **Notification emails**: Agent notification emails display images correctly in Gmail
- **Handles quoted replies**: Regex matches both relative and absolute URLs (from email client quoting)

### DMARC / Forwarding Sender Detection

Google Workspace rewrites the `From:` header on forwarded emails for DMARC compliance, causing all messages to show the group address instead of the real sender. This fork detects and corrects the real sender:

- **X-Google-Original-From** header (priority 1): The original sender before Google rewrote the header
- **Reply-To** header (priority 2): Used when From and To domains match (forwarding indicator)
- **Smart name derivation**: When no display name is available, derives a name from the email local part (e.g., `jane.smith@gmail.com` → "Jane Smith")

### Email Rendering Fixes

- **Image sizing**: Images in emails now respect their original HTML dimensions instead of stretching to fill the container width
- **Non-image inline attachments**: When a non-image file (e.g., PDF) is referenced via CID in an `<img>` tag, it renders as a styled download link instead of a broken image
- **CID replacement**: Fixed missing CID-to-URL replacement after initial attachment upload

### Fullscreen Reply Editor

The fullscreen compose mode now uses 92% of the viewport (up from 60% width / 70% height), matching the Freshdesk compose experience. The sidebar toggle button also persists when viewing a conversation, allowing the nav sidebar to be collapsed for more screen space.

### Other UI Customisations

- **Ticket ID in header**: Shows contact name, reference number, and subject (e.g., "John Smith #105 - Order enquiry")
- **Simplified sidebar name**: Contact name only in sidebar to avoid overflow
- **Self-assign notification suppression**: Assigning to yourself doesn't trigger a notification
- **Macro toolbar button**: Quick-access Zap icon in the reply toolbar for canned responses
- **Image resize handles**: Drag to resize inline images in the editor
- **Macro import support**: Bulk import canned responses from Freshdesk (82 macros with folder prefixes)
- **Macro append mode**: Applying a macro appends to existing editor content instead of replacing it
- **Reply/Private Note button routing**: Clicking Reply opens reply mode, clicking Private Note opens note mode (instead of both opening the last-used mode)
- **Discard draft confirmation**: Discarding a draft now shows a confirmation dialog and collapses the reply box
- **Bulk Close button**: Quick-close selected conversations from the bulk actions bar
- **Full-height assign dropdown**: Assign dropdown uses viewport height instead of fixed scroll area
- **Shift+click range select in table view**: Hold shift to select a range of conversations in table view
- **"Group" renamed to "Team"**: Table view column header now says "Team" instead of "Group"
- **Automation "contains" fix**: Contains/not-contains operator now uses a simple comma-separated text input instead of the broken TagsInput component
- **Automation single-group fix**: Rules saved with only one condition group no longer crash on edit
- **Contact notes notifications**: Option to notify agents when adding contact notes
- **Relaxed HTML sanitisation**: Preserves intentional paragraph spacing in emails instead of stripping all empty elements
- **Empty paragraph handling**: Only collapses 3+ consecutive empty paragraphs (preserves intentional vertical spacing)
- **Fresh theme as default**: New users get the Fresh theme by default
- **Improved message typography**: Larger, more readable font in Fresh theme matching Freshdesk's style
- **Extended session timeout**: 96-hour sliding TTL so agents stay logged in over weekends (Friday to Monday)
- **Ctrl+K macro shortcut guard**: Prevents false triggers during Chinese IME composition and Grammarly synthetic key events
- **"Started last" default sort**: New users see conversations sorted by most recently started by default
- **Signature spacing consistency**: Uses HTML comment markers (`<!-- sig -->`) so signatures survive TipTap's DOM manipulation
- **Email table layout fix**: Removed `table-layout: fixed` from message bubbles so HTML table column widths render correctly
- **Contact form name parsing**: Enhanced parser handles HTML table forms (e.g., Spinfire contact forms) in addition to colon-separated fields
- **Drag-and-drop any file type**: Non-image files (PDFs, spreadsheets, docs) dragged into the editor are uploaded as attachments instead of being silently ignored
- **Attachment preview lightbox**: Full-screen lightbox with prev/next navigation for multi-image messages, loading spinner, adjacent image preloading. PDFs open in inline iframe preview
- **Private Note button fix**: Clicking "Private note" now correctly opens in note mode instead of defaulting to the last-used mode
- **"Add note" button text**: Send button shows "Add note" / "Add note and set as..." when composing a private note
- **Merge dialog layout fix**: Long ticket subjects no longer overflow the merge dialog — subjects truncate with ellipsis

---

## Installation

This fork is designed for self-hosting with local Docker builds. It is **not** published to Docker Hub.

### Docker (Recommended)

```shell
git clone https://github.com/mageaustralia/libredesk/.git
cd libredesk

cp config.sample.toml config.toml
# Edit config.toml as needed

docker compose up -d

# Set the System user password
docker exec -it libredesk_app ./libredesk --set-system-user-password
```

Go to `http://localhost:9000` and login with username `System` and the password you set.

## Keeping Up with Upstream

When a new upstream version is released:

```shell
git fetch origin --tags
git checkout -b feature/openrouter-vX.Y.Z vX.Y.Z
git cherry-pick <your-custom-commits>
# Resolve any conflicts, rebuild, deploy
```

---

## Contributing

For contributions to the core project, see [upstream Libredesk](https://github.com/abhinavxd/libredesk). For issues specific to this fork's features, open an issue on [mageaustralia/libredesk](https://github.com/mageaustralia/libredesk/).

The backend is written in Go and the frontend is Vue.js 3 with Shadcn for UI components. See [developer setup docs](https://docs.libredesk.io/contributing/developer-setup).
