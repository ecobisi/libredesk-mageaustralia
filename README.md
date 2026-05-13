# Libredesk (mageaustralia Fork)

This is a maintained fork of [Libredesk](https://github.com/abhinavxd/libredesk), an open-source, self-hosted customer support desk.

We run Libredesk in production and love the project. This fork exists because we need to ship features at the pace our business demands, and the upstream PR review cycle doesn't always align with that timeline. Rather than pressure the maintainers, we maintain our own fork with the features we need.

We're not trying to replace or compete with upstream Libredesk — we actively track releases and rebase our changes onto new versions as they come out. If any of our additions are useful to the broader project, we're happy to contribute them back.

**Upstream**: [abhinavxd/libredesk](https://github.com/abhinavxd/libredesk) | [libredesk.io](https://libredesk.io) | [Live demo](https://demo.libredesk.io/)  
**Base version**: v2.1.1  
**Companion branch**: [`v1.0.3-plus-enhancements`](https://github.com/mageaustralia/libredesk/tree/v1.0.3-plus-enhancements) — the production fork that this branch was ported from.

---

## Port Status

**Port complete.** This branch is feature-equivalent to `v1.0.3-plus-enhancements` (the production fork) but built on the v2.1.1 base, which adds upstream's frontend monorepo and the live chat widget. The port was executed unit-by-unit against a written spec rather than as a one-shot cherry-pick — 135/135 units done. The full unit-level history (source commits, v2 deltas, verification notes) is at [`docs/superpowers/specs/2026-04-27-v103-port-design.md`](./docs/superpowers/specs/2026-04-27-v103-port-design.md).

## v2 Upstream Features

Worth calling out separately from the fork additions: v2.1.1 introduces an embeddable **live chat widget** for customer websites (`frontend/apps/widget/`). It's not a fork feature, but it's a major capability now available on this branch:

- Embeddable chat widget shipped as a separate Vite app
- Visitor → contact merging when the user authenticates (JWT from the parent page passed into the iframe)
- Customisable pre-chat form
- Redis-backed session tokens with parent-domain cookies (iOS Safari compatible)
- Email fallback / continuity — if a visitor goes offline, the conversation continues by email
- Widget dark mode, audio notifications, rate limiting on the public endpoints
- Online / offline contact status in the agent sidebar (5-minute threshold)

The widget shares a `frontend/shared-ui/` package with the main agent app.

## Fork Features

Everything from upstream Libredesk is included. The following are additions present in this branch.

**Latest** — Complete v1.0.3 → v2.1.1 port: all RAG / ecommerce / voicemail / PCI / mobile-push features now on the v2 base. Plus refactors that fell out of the port: consolidated SSRF-guarded HTTP client builder, shared OpenAI/OpenRouter chat completion plumbing, structured logging in the Maho client, and i18n cleanup of error envelopes in `cmd/`.

### Recent Activities

A global activity feed under Reports showing all ticket activities across the helpdesk — similar to Freshdesk's "Recent Activities" panel but as a dedicated page with pagination.

- **Timeline view**: Shows status changes, assignments, priority changes, agent replies — all in a scrollable feed
- **Agent avatars**: Colour-coded initials with consistent hashing
- **Ticket links**: Click any reference number to jump to the conversation
- **Relative timestamps**: "5 minutes ago", "2 hours ago", etc.
- **"Load more" pagination**: Appends next page to the existing list
- **Auto-purge setting**: Configurable at Admin > Trash & Cleanup — purge activity messages older than N days (default 7, set to 0 to disable)
- **No new database tables**: Queries existing `conversation_messages` data directly
- Navigate to: Reports > Recent Activities

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

### Customer Ticket History on Contact Detail

The contact detail page (`/contacts/{id}`) now shows the full ticket history for that customer, à la Freshdesk's contact view.

- **Tabs**: Previous Conversations (default open) and Notes
- Lists up to 100 prior conversations with subject, last message preview, and relative timestamps
- Click any row to jump straight into that conversation
- Backed by `GET /api/v1/contacts/{id}/conversations` — gated on the existing `contacts:read` permission

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

### FCM Push Notifications (Mobile)

Firebase Cloud Messaging integration for the Flutter mobile app.

- **Push notifications** when tickets are assigned, customers reply, agents are mentioned, etc.
- **Firebase Admin SDK** for Go — sends notifications server-side
- **Auto-cleanup**: Invalid/expired device tokens are automatically removed
- **Push token API**: Register/unregister endpoints for mobile devices
- **Google mobile auth**: OIDC endpoint for Flutter app Google Sign-In
- **Docker volume mount**: Firebase service account key mounted read-only into container

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

### Per-Inbox AI Settings

Each inbox can have its own AI assistant configuration, overriding global defaults.

- **Inbox scope selector** in AI Settings — choose "Global" or a specific inbox
- **Per-inbox system prompt** — different tone/instructions per brand or inbox
- **Per-inbox knowledge sources** — restrict which knowledge bases the AI searches for each inbox
- **Per-inbox external search** — different product catalogues or search endpoints per inbox
- **Reset to Global** button removes inbox-specific settings to fall back to defaults
- Backend resolves effective settings: inbox-specific if available, otherwise global

### Email Alias Filtering

Configure additional email addresses that forward to an inbox, preventing them from appearing in CC when replying.

- **Email aliases field** in Inbox Settings — pill-style input for adding multiple forwarding addresses
- Aliases are excluded from CC alongside the primary inbox email
- Handles common setups like `orders@` and `info@` forwarding to a shared inbox
- **Smart contact detection**: When the conversation contact is an inbox email (e.g., Magento order notifications), scans message history to find the real customer email

### SKU-Level Stock Data in AI Context

Product search results now include per-SKU stock availability for AI responses.

- `sku_stock_data` field parsed from Meilisearch product documents
- Per-SKU stock details (quantity, in/out of stock) formatted in AI context
- AI can answer "is size X in stock?" with specific SKU-level information

### Meilisearch Multi-Search Support

External search now supports Meilisearch multi-search API for more flexible product/content queries.

- **Multi-search endpoint format**: `multi-search:indexUid` or `multi-search:indexUid:filter_expression`
- Single API call searches multiple indexes with optional filters
- Falls back to standard single-index search for non-multi-search endpoints

### OpenRouter AI Provider

Support for [OpenRouter](https://openrouter.ai/) as an AI provider, giving access to 100+ models (GPT-4o, Claude, Llama, Mistral, etc.) through a single API key.

### RAG AI Assistant Enhancements

Improvements to the built-in RAG AI assistant:

- **Knowledge Sources UI**: Admin page to manage knowledge sources (webpages, macros)
- **Context limiting**: Conversations trimmed to last 10 messages / 6000 chars to prevent timeouts on long threads
- **Ecommerce context injection**: Order and customer data included in AI prompts alongside knowledge base results
- **Extended timeouts**: AI provider HTTP timeouts increased to 60s for large prompts
- **Per-inbox knowledge source filtering**: RAG search can be scoped to specific knowledge sources per inbox
- **Inbox-aware settings resolution**: AI generates responses using inbox-specific or global settings automatically

### Ecommerce Integration (Maho Commerce)

Pull customer and order data from a Maho Commerce (Magento-compatible) store into AI-generated responses:

- Customer lookup by email
- Recent order fetching with items, prices, quantities
- Conversation scanning for order numbers with automatic detail retrieval
- Order status history and shipment tracking with carrier-specific URLs
- Supported carriers: Australia Post, Couriers Please, Team Global Express

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

### Multimodal AI (Image Support)

Conversation attachments (images) are extracted, resized to 500x500, and included as base64 in AI prompts for multimodal models that support vision.

### PCI Credit Card Redaction

Automatic detection and redaction of credit card numbers in incoming messages.

- **Auto-detect on ingest**: Scans incoming messages for card numbers (Luhn-validated with network prefix matching)
- **Warning banner**: Red banner with "Redact Now" button on messages containing card data
- **Manual redact**: Agents can immediately scrub card numbers, expiry, and CVV
- **7-day auto-redact**: Safety net — unreacted messages are automatically scrubbed after 7 days
- **IMAP deletion**: Attempts to delete the original email from Gmail after redaction
- **Notification emails scrubbed**: Card numbers are always masked in agent notification emails
- **Admin settings**: Configure who gets notified on IMAP delete failure (Admin > PCI Redaction)
- Uses [go-pci-scrub](https://github.com/mageaustralia/go-pci-scrub) library

### Voicemail Transcription

Automatic transcription of voicemail audio attachments using local whisper.cpp.

- **Auto-detect**: Audio attachments (WAV, MP3, OGG, etc.) are automatically queued for transcription
- **Local whisper.cpp**: Runs on the host via systemd worker — no API costs
- **OpenAI fallback**: Falls back to OpenAI Whisper API if local transcription fails
- **Private note**: Transcript inserted as a private note on the conversation
- **Admin toggle**: Enable/disable and choose provider at Admin > AI Settings
- Model: `ggml-base.en` (142MB, ~7x realtime on ARM64)

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

### Security Hardening

- **SSRF protection** on external URL fetching (webhook URLs, knowledge source URLs)
- **Prompt injection mitigation** in AI-generated content
- **Sensitive data redaction** in ecommerce API logs
- **AI-generated HTML sanitisation** before editor insertion
- **Internal error details** no longer leak to API clients
- **Inbox ID override validation** on message send
- **OpenRouter API key encryption** at rest in the database

### DMARC / Forwarding Sender Detection

Google Workspace rewrites the `From:` header on forwarded emails for DMARC compliance, causing all messages to show the group address instead of the real sender. This fork detects and corrects the real sender:

- **X-Google-Original-From** header (priority 1): The original sender before Google rewrote the header
- **Reply-To** header (priority 2): Used when From and To domains match (forwarding indicator)
- **Smart name derivation**: When no display name is available, derives a name from the email local part (e.g., `jane.smith@gmail.com` → "Jane Smith")

### Email Rendering Fixes

- **Image sizing**: Images in emails now respect their original HTML dimensions instead of stretching to fill the container width
- **Non-image inline attachments**: When a non-image file (e.g., PDF) is referenced via CID in an `<img>` tag, it renders as a styled download link instead of a broken image
- **CID replacement**: Fixed missing CID-to-URL replacement after initial attachment upload

### Relative Timestamps

Message timestamps show relative time with the full date in parentheses:
- "Just now", "5 minutes ago", "2 hours ago", "3 days ago"
- Format: `2 days ago (Mon, 3 Mar 2026 at 7:50 AM)`
- Falls back to full date format after 30 days

### Fullscreen Reply Editor

The fullscreen compose mode now uses 92% of the viewport (up from 60% width / 70% height), matching the Freshdesk compose experience. The sidebar toggle button also persists when viewing a conversation, allowing the nav sidebar to be collapsed for more screen space.

### Unread Count Accuracy

The unread message count badge now excludes activity messages (assignment changes, status updates, etc.), showing only actual messages from contacts and agents.

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
git clone https://github.com/mageaustralia/libredesk.git
cd libredesk

# Check out this branch
git checkout v2.1.1-plus-enhancements

cp config.sample.toml config.toml

# TODO: Edit config.toml as needed

# Compile the solution
make

docker compose up -d

# Set the System user password
docker exec -it libredesk_app ./libredesk --set-system-user-password
```

Go to `http://localhost:9000` and login with username `System` and the password you set.

### Demo Data (Optional)

If you want a fully-loaded instance for screenshots / screen recordings / demos
without manual setup, run the demo seeder:

```shell
cd tools/demo-seeder
./seed.sh
```

This loads 3 agents, 2 teams, 2 inboxes, 5 macros, ~20 conversations covering
every fork feature (spam, trash, merging, PCI redaction, voicemail transcripts,
inline images, ecommerce, RAG, etc.), and a per-feature screenshot recipe.
See [`docs/DEMO_SETUP.md`](./docs/DEMO_SETUP.md) for the recipe and the safety
notes (the seeder refuses to run against non-localhost hosts by default).

### Frontend Layout (v2)

v2 ships a frontend monorepo under `frontend/`:

- `frontend/apps/main/` — the agent / admin Vue app
- `frontend/apps/widget/` — the embeddable customer-facing live chat widget
- `frontend/shared-ui/` — shared components, composables, and assets consumed by both apps

The Docker build assembles both apps into the Go binary via `stuffbin`. If you're developing locally, each app has its own `package.json` and Vite config.

### AI-Powered Responses (RAG)

The AI assistant uses PostgreSQL with pgvector for semantic search.

**Docker:** Already included — uses `pgvector/pgvector:pg17` image.

**Manual install:** Install the pgvector extension:
- Ubuntu/Debian: `apt install postgresql-17-pgvector`
- Or compile from [pgvector/pgvector](https://github.com/pgvector/pgvector)

The extension is automatically enabled during database migration.

---
- If you are interested in contributing, **please read [CONTRIBUTING.md](./CONTRIBUTING.md) first**.
- For local development and setup, refer to the [developer setup](https://docs.libredesk.io/contributing/developer-setup).
- For planned features and project direction, see [ROADMAP.md](./ROADMAP.md).

## Keeping Up with Upstream

For minor upstream releases (same major version), the usual cherry-pick flow works:

```shell
git fetch origin --tags
git checkout -b vX.Y.Z-plus-enhancements vX.Y.Z
git cherry-pick <fork-commits>
# Resolve any conflicts, rebuild, deploy
```

For major-version transitions — like the v1.0.3 → v2.1.1 jump that produced this branch — we use a deliberate spec-driven port instead of a one-shot cherry-pick: every feature is listed as a unit in a design doc, each unit becomes one well-described commit, and the doc tracks `pending` → `done <hash>` status. See [`docs/superpowers/specs/2026-04-27-v103-port-design.md`](./docs/superpowers/specs/2026-04-27-v103-port-design.md) for the template — it's the source of truth for what got ported and why, and a reference for the next major-version transition.

---

## Contributing

For contributions to the core project, see [upstream Libredesk](https://github.com/abhinavxd/libredesk). For issues specific to this fork's features, open an issue on [mageaustralia/libredesk](https://github.com/mageaustralia/libredesk/).

The backend is written in Go and the frontend is Vue.js 3 with Shadcn for UI components. See [developer setup docs](https://docs.libredesk.io/contributing/developer-setup).
