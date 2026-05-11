# Demo setup — screenshot / screen-record recipe

This guide turns a fresh libredesk into a fully-loaded demo so you can
walk every fork feature in the top-level [README.md](../README.md) without
manual data entry first.

## Prerequisites

- Docker + Docker Compose
- Go >= 1.25 (for the seeder build)
- Repo cloned locally

## Quick start

```bash
# 1. Bring up libredesk
docker compose up -d

# 2. Set the System password (or use a different password and pass it via --pass)
docker exec -it libredesk_app ./libredesk --set-system-user-password
# (default password used by the seeder: changeme)

# 3. Load demo data
cd tools/demo-seeder
./seed.sh
```

Open <http://localhost:9000>. Log in with any of:

| Email | Password | Role |
|---|---|---|
| `admin@demo.local` | `DemoPassw0rd!` | Admin |
| `agent1@demo.local` | `DemoPassw0rd!` | Agent (Demo Sales) |
| `agent2@demo.local` | `DemoPassw0rd!` | Agent (Demo Support) |
| `System` | (whatever you set) | System |

## What the seeder loads

| Type | Count | Notes |
|---|---|---|
| Agents | 3 | All `@demo.local` |
| Teams | 2 | Demo Sales, Demo Support |
| Inboxes | 2 | Email (disabled IMAP/SMTP), live-chat widget |
| Macros | 5 | Across `[Demo: ...]` folders |
| Knowledge sources | 2 | Webpage + macro-based |
| Shared views | 1 | Unassigned + Open |
| Tags | 6 | vip, complaint, feature-request, spam-rescued, duplicate, demo |
| Conversations | ~20 | Spans every visible feature |
| Ecommerce config | 1 | magento1 type, fake creds |
| FCM push token | 1 | Fake demo token, registered to System user |

Re-run `./seed.sh` any time — it's idempotent. Use `./seed.sh --reset` to
wipe demo agents / inboxes / teams / macros / RAG / views first
(conversations are not wiped — they go via the Trash flow in production).

## Per-feature recipe

Click through these in order to record every fork feature. Each line is
one feature → one place to look.

### Spam & Trash
- **Spam list**: Sidebar → Spam — two spam convos (`WIN A FREE iPHONE NOW!!!`, `URGENT: Verify your account`) plus one "spam-rescued" conv (`Re: Your order from last week`) which stayed in the main inbox because the customer had a prior agent reply.
- **Trash list**: Sidebar → Trash — two trashed convos.
- **Restore / Mark as not spam / Permanent delete**: open any spam or trash item → header `...` menu.

### Submit & Set Status on New Conversations
- Click "+ New conversation" (top bar) → fill out → click the chevron next to Submit → pick a status from the dropdown.

### Customer Ticket History
- Open any conv from Jane Smith → click her name in the right sidebar → see 5 prior conversations under the "Previous Conversations" accordion.

### Advanced View Filters + Multi-Status Filtering
- Sidebar → Shared views → "Demo: Unassigned + Open" — pre-built filter showing status=Open AND assignee is_not_set. Edit it to demo the `in_or_null` ("is any of, or unassigned") operator.
- Status dropdown in toolbar → tick multiple checkboxes (Open + Replied) for multi-status.

### Table View Layout / Card View Toggle
- Top right of conv list → toggle the layout icon.

### Bulk Actions & Conversation Selection
- Conv list → tick 3+ checkboxes → bulk action toolbar at the top → try Assign, Status, Priority, Move to Trash, Merge.
- Shift+click range select: tick one row, hold shift, tick another.

### Ticket Merging
- In the conv list, tick two convs from Priya Kapoor (both tagged `duplicate`) → Bulk → Merge. OR open one and use the `...` menu → Merge → enter the other ref number.

### Smart Team Reassignment
- Open `B2B pricing question` (assigned to agent1 in Demo Sales). Change the team to Demo Support — agent1 isn't a member, so they'll be unassigned. Change back to Demo Sales, agent1 stays.

### Quick-Assign Dropdowns
- Conv list (card view) — each row shows agent + team icons with inline dropdowns.

### Per-Inbox AI Settings
- Admin → AI Settings → switch the scope selector from Global to `Demo Email` — the system prompt override is pre-loaded.

### Email Alias Filtering
- Admin → Inboxes → Demo Email → "Email aliases" pills field shows `orders@demo.local` and `info@demo.local`.

### Auto-Assign on Reply
- Admin → Inboxes → Demo Email → toggle "Auto-assign on reply" — already enabled.

### Per-Inbox Signatures
- Admin → Inboxes → Demo Email → "Signature" field has the templated signature with `{{agent.first_name}}` etc.

### Connection Testing
- Admin → Inboxes → Demo Email → "Test IMAP" / "Test SMTP" buttons. Both will fail (creds are intentionally `DISABLED-BY-DEMO-SAFETY`) — that's the point, the UI shows the connection-test flow.

### Gmail-Style Quoted Thread / Multiple Reply Chain
- Open `Multiple billing questions` (Linda Olsen) — 3+ agent replies. Click Reply → toggle the `···` for the quoted thread.

### Inline Image Rendering
- Open `Photo of damaged packaging` — placeholder image embedded in the customer message.

### PCI Credit Card Redaction
- Open `Update my saved card` — message contains `4111 1111 1111 1111` (Visa test number, passes Luhn). Red banner with "Redact Now" button appears on the message.

### Voicemail Transcription
- Open `Voicemail from customer (transcribed)` — initial message is a placeholder, but a private note follows with the transcribed audio (matches what the whisper.cpp worker produces in production).

### Customer Reply Notifications
- Open `Login problem after password reset` (assigned to agent2 with a recent customer reply). Log in as agent2 → see notification badge.

### FCM Push Notifications
- Admin → Push Tokens / agent profile — the System user has a fake registered FCM token. (Actual sends require Firebase service account creds.)

### Ecommerce Integration (Maho Commerce)
- Admin → Ecommerce — type=magento1, base URL and client_id pre-filled, client_secret masked. Click "Test Connection" — fails with a generic "Connection failed" message (the host is intentionally invalid).
- Open any conv → reply box → "+ Orders" button (only shows when ecommerce.type is set).

### RAG AI Assistant / OpenRouter / Multimodal
- Admin → AI → Knowledge Sources — two seeded sources.
- Admin → AI → Providers — configure OpenRouter (you'll need a real API key for actual generation). The Generate Response button in the reply box will be live once a provider is configured.

### Relative Timestamps
- Conv list shows a mix of "Just now", "X hours ago", "Yesterday", "X days ago" across the seeded convs. The Jane Smith customer-history convs span 0–14 days.

### Recent Activities
- Reports → Recent Activities — populates automatically from the agent replies, status changes, and assignments the seeder created.

### Hover Preview (table view)
- Switch to table view → hover any conv row → tooltip shows original message + latest reply.

### Fresh Theme
- Default for new users. Toggle via the theme switcher in the bottom-left sidebar.

### Full-Width Layout / Fullscreen Reply / Reply / Private Note toolbar
- Open any conv. Toggle the layout button in the toolbar for full-width. Click Reply / Private note buttons to demo the routing fix. Use the fullscreen icon in the reply box for the 92%-viewport compose.

### Macros
- Reply box → Zap icon (or Ctrl+K) → pick `[Demo: Order Status] Tracking info` → it appends to existing content.

### Mentions
- Open `Refund processing time?` — private note with `@Bob Agent` mention is pre-seeded.

### Search
- Top bar search → try "damaged" → matches Jane Smith's first conv plus Sophie Bernard's packaging conv.

## Features that can't be seeded

Some features are runtime-only and need live interaction:

| Feature | Why not seedable | How to demo manually |
|---|---|---|
| Agent Collision Detection | Needs two browser sessions on the same conv | Open the same conv in two browsers as different agents |
| Customer Reply Notifications (push) | Needs real FCM service account creds | Configure Firebase and reply from the customer side |
| IMAP / SMTP delivery | Inbox is intentionally `enabled=false` with `DISABLED-BY-DEMO-SAFETY` creds | Configure a real inbox in Admin → Inboxes |
| RAG generation | Needs a real OpenAI / OpenRouter API key | Configure provider in Admin → AI → Providers |
| Voicemail whisper.cpp pipeline | Worker runs on host, not in docker | Install whisper.cpp + the systemd worker per `docs/voice-transcription.md` |
| Ecommerce live data | Fake URL points to invalid.demo.local | Configure a real Maho / Magento 1 store |
| Customer Replies via Email Threading | Needs inbound IMAP polling | Enable a real inbox |

## TODO

- Upload actual sample PNGs in `tools/demo-seeder/data/images/` and wire
  the `/api/v1/media` endpoint into the inline-image scenario instead of
  the placeholder URL.
- Trigger the actual PCI scanner by ingesting via the IMAP path rather than
  via `POST /api/v1/conversations` (the scanner runs on inbound mail).
- Seed CSAT responses on resolved conversations once the CSAT endpoint
  accepts admin-side seed payloads.
- Seed Recent Activities pruning settings + an Automation rule to demo
  the rules page.
